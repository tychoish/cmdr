package cmdr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/urfave/cli/v3"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/srv"
)

// Action defines the core functionality for a command line entry
// point or handler, providing both the process' context (managed by
// the commander,) as well as the pre-operation hooks/validation
// hooks.
//
// Upon execution these functions get the context processed by the
// middleware, and the cli package's command. In practice, rather than
// defining action functions directly, use the AddOperation function
// to define more strongly typed operations.
type Action func(ctx context.Context, c *cli.Command) error

// Middleware processes the context, attaching timeouts, or values as
// needed. Middlware is processed after hooks but before the operation.
type Middleware func(ctx context.Context) context.Context

// Commander provides a chainable and ergonomic way of defining a
// command.
//
// The Commander objects largely mirror the semantics of the
// underlying cli library, which handles execution at runtime. Future
// versions may use different underlying tools.
//
// Commander provides a strong integration with the
// github.com/tychoish/fun/srv package's service orchestration
// framework. A service orchestrator is created and runs during the
// execution of the program and users can add services and rely
// on Commander to shut down the orchestrator service and wait for
// running services to return before returning.
//
// Commanders provide an integrated and strongly typed method for
// defining setup and configuration before running the command
// itself. For cleanup after the main operation finishes use the
// github.com/tychoish/fun/srv package's srv.AddCleanupHook() and
// srv.AddCleanupError().
type Commander struct {
	once       sync.Once
	cmd        cli.Command
	hidden     atomic.Bool
	blocking   atomic.Bool
	opts       adt.Atomic[AppOptions]
	name       adt.Atomic[string]
	usage      adt.Atomic[string]
	action     adt.Atomic[Action]
	flags      adt.Synchronized[*dt.List[Flag]]
	aliases    adt.Synchronized[*dt.List[string]]
	hook       adt.Synchronized[*dt.List[Action]]
	middleware adt.Synchronized[*dt.List[Middleware]]
	subcmds    adt.Synchronized[*dt.List[*Commander]]

	// this has to be a context producer (func() context.Context)
	// so that the interior atomic doesn't freak out when the
	// interface type changes.
	//
	// additionally, when resolving subcmds we have to make sure
	// that every subcommand get access to the same context
	// hierarchy (to do otherwise would be unexpected); so during
	// resolution of the command (but not adding)
	ctx *adt.Atomic[contextProducer]
}

// MakeRootCommander constructs a root commander object with basic
// services configured. From the tychoish/fun/srv package, this
// pre-populates a base context, shutdown signal, service
// orchestrator, and cleanup system.
//
// Use MakeCommander to create a commander without these services
// enabled/running.
func MakeRootCommander() *Commander {
	c := MakeCommander()
	c.SetName(filepath.Base(os.Args[0]))
	c.ctx = adt.NewAtomic(ctxMaker(context.Background()))
	c.middleware.With(func(in *dt.List[Middleware]) {
		in.PushBack(srv.SetBaseContext)
		in.PushBack(srv.SetShutdownSignal)
		in.PushBack(srv.WithOrchestrator) // this starts the orchestrator
		in.PushBack(srv.WithCleanup)
	})

	c.cmd.After = func(ctx context.Context, _ *cli.Command) error {
		if !c.blocking.Load() {
			// cancel the parent context
			srv.GetShutdownSignal(c.getContext())()
		}
		return srv.GetOrchestrator(c.getContext()).Wait()
	}

	return c
}

// MakeCommander constructs and initializes a command builder object.
func MakeCommander() *Commander {
	c := &Commander{}

	c.flags.Set(&dt.List[Flag]{})
	c.hook.Set(&dt.List[Action]{})
	c.subcmds.Set(&dt.List[*Commander]{})
	c.middleware.Set(&dt.List[Middleware]{})

	c.aliases.Set(&dt.List[string]{})

	c.cmd.Before = func(ctx context.Context, cc *cli.Command) (context.Context, error) {
		ec := &erc.Collector{}

		c.hook.With(func(hooks *dt.List[Action]) {
			defer ec.Recover()
			for op := range hooks.IteratorFront() {
				ec.Push(op(c.getContext(), cc))
			}
		})

		c.middleware.With(func(in *dt.List[Middleware]) {
			defer ec.Recover()
			for op := range in.IteratorFront() {
				c.setContext(op(c.getContext()))
			}
		})

		c.flags.With(func(flags *dt.List[Flag]) {
			defer ec.Recover()
			for flag := range flags.IteratorFront() {
				if af, ok := flag.value.(cli.ActionableFlag); ok {
					ec.Push(af.RunAction(ctx, cc))
				}
			}
		})

		return c.getContext(), ec.Resolve()
	}

	c.cmd.Action = func(ctx context.Context, cc *cli.Command) error {
		op := c.action.Get()
		if op != nil {
			return op(c.getContext(), cc)
		}

		// no commands defined, no action defined,
		if c.subcmds.Get().Len() == 0 {
			return fmt.Errorf("action: %w", ErrNotDefined)
		}

		if cc.Args().Len() == 0 {
			return erc.Join(cli.ShowAppHelp(cc), fmt.Errorf("no operation for %q: %w", c.cmd.Name, ErrNotSpecified))
		}

		return erc.Join(cli.ShowCommandHelp(ctx, cc, c.cmd.Name), fmt.Errorf("command %v: %w", cc.Args().Len(), ErrNotDefined))
	}

	return c
}

// SetAction defines the core operation for the commander.
func (c *Commander) SetAction(in Action) *Commander { c.action.Set(in); return c }
func (c *Commander) SetName(n string) *Commander    { c.name.Set(n); return c }
func (c *Commander) SetUsage(u string) *Commander   { c.usage.Set(u); return c }

// SetBlocking configures the blocking semantics of the command. This
// setting is only used by root Commander objects. It defaults to
// false, which means that the action function returns the context
// passed to services will be canceled.
//
// When true, commanders do not cancel the context after the Action
// function returns, including for relevant sub commands; instead
// waiting for any services, managed by the Commanders' orchestrator
// to return, for the services to signal shutdown, or the context
// passed to the cmdr.Run or cmdr.Main functions to expire.
func (c *Commander) SetBlocking(b bool) *Commander { c.blocking.Store(b); return c }

// setContext attaches a context to the commander. This is only needed
// if you are NOT using the commander with the Run() or Main()
// methods.
func (c *Commander) setContext(ctx context.Context) *Commander { c.ctx.Set(ctxMaker(ctx)); return c }
func (c *Commander) getContext() context.Context               { return c.ctx.Get()() }

// Subcommanders adds a subcommander, returning the original parent
// commander object.
func (c *Commander) Subcommanders(subs ...*Commander) *Commander {
	appendTo(&c.subcmds, subs...)
	return c
}

// UrfaveCommands directly adds a urfae/cli.Command as a subcommand
// to the Commander.
//
// Commanders do not modify the raw subcommands added in this way,
// with one exception. Because cli.Command.Action is untyped and it
// may be reasonable to add Action functions with different
// signatures, the Commander will attempt to convert common function
// to `func(*cli.Context) error` functions and avert the error.
//
// Comander will convert Action functions of following types:
//
//	func(context.Context) error
//	func(context.Context, *cli.Context) error
//	func(context.Context)
//	func() error
//	func()
//
// The commander processes the sub commands recursively. All wrapping
// happens when building the cli.App/cli.Command for the converter,
// and has limited overhead.
func (c *Commander) UrfaveCommands(cc ...*cli.Command) *Commander {
	c.subcmds.With(func(in *dt.List[*Commander]) {
		for idx := range cc {
			sub := MakeCommander()
			sub.cmd = *cc[idx]
			in.PushBack(sub)
		}
	})

	return c
}

func (c *Commander) Flags(flags ...Flag) *Commander { appendTo(&c.flags, flags...); return c }
func (c *Commander) Aliases(a ...string) *Commander { appendTo(&c.aliases, a...); return c }

// Hooks adds a new hook to the commander. Hooks are all executed
// before the command runs. While all hooks run and errors are
// collected, if any hook errors the action will not execute.
func (c *Commander) Hooks(op ...Action) *Commander { appendTo(&c.hook, op...); return c }

// SetMiddlware allows users to modify the context passed to the hooks
// and actions of a command.
func (c *Commander) Middleware(mws ...Middleware) *Commander {
	appendTo(&c.middleware, mws...)
	return c
}

// With makes it possible to embed helper functions in a Commander
// chain directly.
func (c *Commander) With(op func(c *Commander)) *Commander { op(c); return c }

// Command resolves the commander into a cli.Command instance. This
// operation is safe to call more options.
//
// You should only call this function *after* setting the context on
// the commander.
func (c *Commander) Command() *cli.Command {
	c.once.Do(func() {
		erc.InvariantOk(c.getContext() != nil, "context must be set when calling command")

		c.cmd.Name = secondValueWhenFirstIsZero(c.cmd.Name, c.name.Get())
		c.cmd.Usage = secondValueWhenFirstIsZero(c.cmd.Usage, c.usage.Get())
		c.cmd.Hidden = c.hidden.Load()

		if len(c.cmd.Aliases) == 0 {
			var aliases []string
			c.aliases.With(func(in *dt.List[string]) {
				aliases = irt.Collect(in.IteratorFront())
			})
			c.cmd.Aliases = aliases
		}

		c.flags.With(func(in *dt.List[Flag]) {
			for v := range in.IteratorFront() {
				c.cmd.Flags = append(c.cmd.Flags, v.value)
			}
		})

		c.subcmds.With(func(in *dt.List[*Commander]) {
			for v := range in.IteratorFront() {
				v.ctx = c.ctx
				c.cmd.Commands = append(c.cmd.Commands, v.Command())
			}
		})
	})

	return &c.cmd
}

// AppOptions provides the structure for construction a cli.App from a
// commander.
type AppOptions struct {
	Usage   string
	Name    string
	Version string
}

// SetAppOptions set's the commander's options. This is only used by
// the top-level root commands.
func (c *Commander) SetAppOptions(opts AppOptions) *Commander { c.opts.Set(opts); return c }

// App resolves a command object from the commander and the provided
// options. You must set the context on the Commander using the
// setContext before calling this command directly.
//
// In most cases you will use the Run() or Main() methods, rather than
// App() to use the commander, and Run()/Main() provide their own
// contexts.
func (c *Commander) App() *cli.Command {
	erc.InvariantOk(c.ctx.Get() != nil, "context must be set before calling the app")
	a := c.opts.Get()

	cmd := c.Command()
	app := &cli.Command{}

	app.Name = secondValueWhenFirstIsZero(a.Name, cmd.Name)
	app.Usage = secondValueWhenFirstIsZero(a.Usage, cmd.Usage)
	app.Version = a.Version

	app.Commands = cmd.Commands
	app.Action = cmd.Action
	app.Flags = cmd.Flags
	app.After = cmd.After
	app.Before = cmd.Before

	return app
}
