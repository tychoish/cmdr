package cmdr

import (
	"context"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/fun"
)

// Hook generates an object, typically a configuration struct, from
// the cli.Context provided. Hooks are always processed first, before
// middleware and the main opreation.
type Hook[T any] func(context.Context, *cli.Context) (T, error)

// Operation takes a value, produced by Hook[T], and executes the
// function.
type Operation[T any] func(context.Context, T) error

// OperationSpec defines a set of functions that The AddOperationSpec
// functions use to modify a Commander. Unlike using the commander
// directly, these operations make it possible to
type OperationSpec[T any] struct {
	// Constructor is required and constructs the output object
	// for the operation.
	Constructor Hook[T]
	// The functions in HookOperations are a sequence of
	// type-specialized hooks that you can use to post-process the
	// constructed object, particularly if T is mutable. These run
	// after the constructor during the "hook" phase of the
	// commander.
	HookOperations []Operation[T]
	// Middlware is optional and makes it possible to attach T to
	// a context for later use. Middlewares
	Middleware func(context.Context, T) context.Context
	// Action, the core action.  may be (optionally) specified here as an Operation
	// or directly on the command.
	Action Operation[T]
}

// SpecBuilder provides an alternate (chainable) method for building
// an OperationSpec that is consistent with the Commander interface,
// and that the compiler can correctly infer the correct type for T.
//
// This builder is not safe for concurrent use.
func SpecBuilder[T any](constr Hook[T]) *OperationSpec[T] {
	return &OperationSpec[T]{Constructor: constr}
}

func (s *OperationSpec[T]) SetMiddleware(mw func(context.Context, T) context.Context) *OperationSpec[T] {
	s.Middleware = mw
	return s
}

func (s *OperationSpec[T]) SetAction(op Operation[T]) *OperationSpec[T] { s.Action = op; return s }

func (s *OperationSpec[T]) Hooks(hook ...Operation[T]) *OperationSpec[T] {
	s.HookOperations = append(s.HookOperations, hook...)
	return s
}

// Add is an option end of a spec builder chain, that adds the chain
// to the provided Commander. Use this directly or indirectly with the
// Commander.With method.
func (s *OperationSpec[T]) Add(c *Commander) {
	var out T
	c.Hooks(func(ctx context.Context, cc *cli.Context) (err error) {
		out, err = s.Constructor(ctx, cc)
		if err != nil {
			return err
		}

		for idx := range s.HookOperations {
			if err := s.HookOperations[idx](ctx, out); err != nil {
				return err
			}
		}
		return nil
	})

	if s.Middleware != nil {
		c.Middleware(func(ctx context.Context) context.Context {
			return s.Middleware(ctx, out)
		})
	}

	if s.Action != nil {
		c.SetAction(func(ctx context.Context, _ *cli.Context) error {
			return s.Action(ctx, out)
		})
	}
}

// AddOperationSpec adds an operation to a Commander (and returns the
// commander.) The OperationSpec makes it possible to define (most) of
// the operation using strongly typed operations, while passing state
// directly. A single object of the type T is captured between the
// function calls.
//
// Because the operation spec builds hooks, middleware, and operations
// and adds these functions to the convert, it's possible to use
// AddOperationSpec more than once on a single command. However, there
// is only ever one action on a commander, so the last non-nil Action
// specified will be used.
func AddOperationSpec[T any](c *Commander, s *OperationSpec[T]) *Commander { return c.With(s.Add) }

// CompositeHook builds a Hook for use with AddOperation and
// Subcommander that allows for factorking. When T is a mutable type,
// you can use these composite hooks to process and validate
// incrementally.
func CompositeHook[T any](constr Hook[T], ops ...Operation[T]) Hook[T] {
	var out T
	return func(ctx context.Context, cc *cli.Context) (_ T, err error) {
		out, err = constr(ctx, cc)
		if err != nil {
			return fun.ZeroOf[T](), err
		}

		for idx := range ops {
			if err := ops[idx](ctx, out); err != nil {
				return fun.ZeroOf[T](), err
			}
		}
		return out, nil
	}
}

// AddOperation uses generics to create a hook/operation pair that
// splits interacting with the cli.Context from the core
// operation: the Hook creates an object--likely a structure--with the
// data from the cli args, while the operation can use that structure
// for the core business logic of the entry point.
//
// An optional number of flags can be added to the command as well.
//
// This form for producing operations separates the parsing of inputs
// from the operation should serve to make these operations easier to
// test.
func AddOperation[T any](c *Commander, hook Hook[T], op Operation[T], flags ...Flag) *Commander {
	return c.Flags(flags...).With((&OperationSpec[T]{
		Constructor: hook,
		Action:      op,
	}).Add)
}

// Subcommander uses the same AddOperation semantics and form, but
// creates a new Commander adds the operation to that commander, and
// then adds the subcommand to the provided commander, returning the
// new sub-commander.
func Subcommander[T any](c *Commander, hook Hook[T], op Operation[T], flags ...Flag) *Commander {
	sub := MakeCommander()
	c.Subcommanders(sub)
	return AddOperation(sub, hook, op, flags...)
}

// CommandOptions are the arguments to create a sub-command in a
// commander.
type CommandOptions[T any] struct {
	Name       string
	Usage      string
	Hook       Hook[T]
	Operation  Operation[T]
	Flags      []Flag
	Middleware func(context.Context, T) context.Context
	Hidden     bool
	Subcommand bool
}

// Add modifies the provided commander with the options and operation
// defined by the CommandOptions. Use in combination with the
// Command.With method.
func (opts CommandOptions[T]) Add(c *Commander) {
	c.name.Set(opts.Name)

	fun.Invariant(opts.Operation != nil, "operation must not be nil")
	c.name.Set(opts.Name)
	c.usage.Set(opts.Usage)
	c.hidden.Store(opts.Hidden)

	AddOperation(c, opts.Hook, opts.Operation, opts.Flags...)
}

// OptionsCommander creates a new commander as a sub-command returning the
// new subcommander. Typically you could use this as:
//
//	c := cmdr.MakeRootCommand().
//	       Commander(OptionsCommander(optsOne).SetName("one")).
//	       Commander(OptionsCommander(optsTwo).SetName("two"))
func OptionsCommander[T any](opts CommandOptions[T]) *Commander {
	return MakeCommander().With(opts.Add)
}
