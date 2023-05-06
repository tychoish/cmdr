package cmdr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/seq"
	"github.com/tychoish/fun/srv"
)

// Action defines the core functionality for a command line entry
// point or handler, providing both the process' context (managed by
// the commander,) as well as the pre-operation hooks/validation
// hooks.
//
// Upon execution these functions get the context processed by the
// middleware, and the cli package's context. In practice, rather than
// defining action functions directly, use the AddOperation function
// to define more strongly typed operations.
type Action func(ctx context.Context, c *cli.Context) error

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
	ctx        adt.Atomic[contextProducer]
	opts       adt.Atomic[AppOptions]
	name       adt.Atomic[string]
	usage      adt.Atomic[string]
	action     adt.Atomic[Action]
	flags      adt.Synchronized[*seq.List[Flag]]
	aliases    adt.Synchronized[*seq.List[string]]
	hook       adt.Synchronized[*seq.List[Action]]
	middleware adt.Synchronized[*seq.List[Middleware]]
	subcmds    adt.Synchronized[*seq.List[*Commander]]
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
	c.middleware.With(func(in *seq.List[Middleware]) {
		in.PushBack(srv.SetBaseContext)
		in.PushBack(srv.SetShutdownSignal)
		in.PushBack(srv.WithOrchestrator) // this starts the orchestrator
		in.PushBack(srv.WithCleanup)
	})

	c.cmd.After = func(_ *cli.Context) error {
		ctx := c.getContext()
		if !c.blocking.Load() {
			// cancel the parent context
			srv.GetShutdownSignal(ctx)()
		}
		return srv.GetOrchestrator(ctx).Wait()
	}

	return c
}

// MakeCommander constructs and initializes a command builder object.
func MakeCommander() *Commander {
	c := &Commander{}

	c.flags.Set(&seq.List[Flag]{})
	c.hook.Set(&seq.List[Action]{})
	c.subcmds.Set(&seq.List[*Commander]{})
	c.middleware.Set(&seq.List[Middleware]{})
	c.aliases.Set(&seq.List[string]{})

	c.cmd.Before = func(cc *cli.Context) error {
		ec := &erc.Collector{}

		ctx := c.getContext()

		c.hook.With(func(hooks *seq.List[Action]) {
			ec.Add(fun.Observe(ctx, seq.ListValues(hooks.Iterator()),
				func(op Action) { ec.Add(op(ctx, cc)) }))
		})

		c.middleware.With(func(in *seq.List[Middleware]) {
			ec.Add(fun.Observe(ctx, seq.ListValues(in.Iterator()),
				func(mw Middleware) { ctx = mw(ctx) }))
		})

		c.flags.With(func(flags *seq.List[Flag]) {
			ec.Add(fun.Observe(ctx, seq.ListValues(flags.Iterator()),
				func(fl Flag) {
					if af, ok := fl.value.(cli.ActionableFlag); ok {
						ec.Add(af.RunAction(cc))
					}
				}))
		})

		c.SetContext(ctx)

		return ec.Resolve()
	}

	c.cmd.Action = func(cc *cli.Context) error {
		op := c.action.Get()
		if op != nil {
			return op(c.getContext(), cc)
		}

		// no commands defined, no action defined,
		if c.subcmds.Get().Len() == 0 {
			return fmt.Errorf("action: %w", ErrNotDefined)
		}

		if cc.Args().Len() == 0 {
			return erc.Merge(cli.ShowAppHelp(cc), fmt.Errorf("no operation for %q: %w", c.cmd.Name, ErrNotSpecified))
		}

		return erc.Merge(cli.ShowCommandHelp(cc, c.cmd.Name), fmt.Errorf("command %v: %w", cc.Args().Len(), ErrNotDefined))
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

// SetContext attaches a context to the commander. This is only needed
// if you are NOT using the commander with the Run() or Main()
// methods.
func (c *Commander) SetContext(ctx context.Context) *Commander { c.ctx.Set(ctxMaker(ctx)); return c }
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
	c.subcmds.With(func(in *seq.List[*Commander]) {
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
		ctx := c.getContext()
		fun.Invariant(ctx != nil, "context must be set when calling command")

		c.cmd.Name = secondValueWhenFirstIsZero(c.cmd.Name, c.name.Get())
		c.cmd.Usage = secondValueWhenFirstIsZero(c.cmd.Usage, c.usage.Get())
		c.cmd.Hidden = c.hidden.Load()

		if len(c.cmd.Aliases) == 0 {
			var aliases []string
			c.aliases.With(func(in *seq.List[string]) {
				aliases = fun.Must(itertool.CollectSlice(ctx, seq.ListValues(in.Iterator())))
			})
			c.cmd.Aliases = aliases
		}

		c.flags.With(func(in *seq.List[Flag]) {
			fun.InvariantMust(fun.Observe(ctx, seq.ListValues(in.Iterator()), func(v Flag) {
				c.cmd.Flags = append(c.cmd.Flags, v.value)
			}))
		})

		c.subcmds.With(func(in *seq.List[*Commander]) {
			fun.InvariantMust(fun.Observe(ctx, seq.ListValues(in.Iterator()), func(v *Commander) {
				v.SetContext(ctx)
				c.cmd.Subcommands = append(c.cmd.Subcommands, v.Command())
			}))
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
// SetContext before calling this command directly.
//
// In most cases you will use the Run() or Main() methods, rather than
// App() to use the commander, and Run()/Main() provide their own
// contexts.
func (c *Commander) App() *cli.App {
	fun.Invariant(c.ctx.Get() != nil, "context must be set before calling the app")
	a := c.opts.Get()
	cmd := c.Command()
	app := cli.NewApp()

	app.Name = secondValueWhenFirstIsZero(a.Name, cmd.Name)
	app.Usage = secondValueWhenFirstIsZero(a.Usage, cmd.Usage)
	app.Version = a.Version

	app.Commands = cmd.Subcommands
	app.Action = cmd.Action
	app.Flags = cmd.Flags
	app.After = cmd.After
	app.Before = cmd.Before

	return app
}
