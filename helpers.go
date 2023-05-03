package cmdr

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/urfave/cli"
)

// CompositeHook builds a Hook for use with AddOperation and
// AddSubcommand that allows for factorking. When T is a mutable type,
// you can use these composite hooks to process and validate
// incrementally.
func CompositeHook[T any](constr Hook[T], ops ...Operation[T]) Hook[T] {
	return func(ctx context.Context, cc *cli.Context) (T, error) {
		out, err := constr(ctx, cc)
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
	var capture T

	c.AddHook(func(ctx context.Context, cc *cli.Context) (err error) { capture, err = hook(ctx, cc); return err })
	c.SetAction(func(ctx context.Context, _ *cli.Context) error { return op(ctx, capture) })
	c.AddFlags(flags...)

	return c
}

// AddSubcommand uses the same AddOperation semantics and form, but
// creates a new Commander adds the operation to that commander, and
// then adds the subcommand to the provided commander, returning the
// new subcommand.
func AddSubcommand[T any](c *Commander, hook Hook[T], op Operation[T], flags ...Flag) *Commander {
	sub := MakeCommander()
	c.Commander(sub)
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
	Hidden     bool
	Subcommand bool
}

// Subcommand creates a new commander as a sub-command returning the
// new subcommander. Typically you could use this as:
//
//	c := cmdr.MakeRootCommand().
//	       Commander(Subcommand(optsOne).SetName("one")).
//	       Commander(Subcommand(optsTwo).SetName("two"))
func Subcommand[T any](opts CommandOptions[T]) *Commander {
	sub := MakeCommander()
	sub.name.Set(opts.Name)

	fun.Invariant(opts.Operation != nil, "operation must not be nil")

	sub.cmd.Name = opts.Name
	sub.cmd.Usage = opts.Usage
	sub.cmd.Hidden = opts.Hidden

	AddOperation(sub, opts.Hook, opts.Operation, opts.Flags...)

	return sub
}
