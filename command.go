package cmdr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/seq"
	"github.com/tychoish/fun/srv"
	"github.com/urfave/cli"
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
// needed.
type Middleware func(ctx context.Context) context.Context

// Hook generates an object, typically a configuration struct, from
// the cli.Context provided.
type Hook[T any] func(context.Context, *cli.Context) (T, error)

// Operation takes a value, produced by Hook[T], and executes the
// function.
type Operation[T any] func(context.Context, T) error

// Commander provides a chainable and ergonomic way of defining a
// command.
type Commander struct {
	cmd        cli.Command
	ctx        adt.Atomic[contextProducer]
	name       adt.Atomic[string]
	usage      adt.Atomic[string]
	action     adt.Atomic[Action]
	opts       adt.Atomic[AppOptions]
	once       sync.Once
	flags      adt.Synchronized[*seq.List[Flag]]
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
		in.PushBack(srv.WithOrchestrator)
		in.PushBack(srv.WithCleanup)
	})

	c.cmd.After = func(_ *cli.Context) error {
		// cancel the parent context
		ctx := c.getContext()
		srv.GetShutdownSignal(ctx)()
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

	c.cmd.Before = func(cc *cli.Context) error {
		ec := &erc.Collector{}

		ctx := c.getContext()

		c.middleware.With(func(in *seq.List[Middleware]) {
			ec.Add(fun.Observe(ctx, seq.ListValues(in.Iterator()),
				func(mw Middleware) { ctx = mw(ctx) }))
		})

		c.hook.With(func(hooks *seq.List[Action]) {
			ec.Add(fun.Observe(ctx, seq.ListValues(hooks.Iterator()),
				func(op Action) { ec.Add(op(ctx, cc)) }))
		})

		c.flags.With(func(hooks *seq.List[Flag]) {
			ec.Add(fun.Observe(ctx, seq.ListValues(hooks.Iterator()),
				func(fl Flag) {
					if fl.validate != nil {
						ec.Add(fl.validate(cc))
					}
				}))
		})

		c.SetContext(ctx)

		return ec.Resolve()
	}

	c.cmd.Action = func(cc *cli.Context) error {
		op := c.action.Get()
		if op == nil {
			if c.subcmds.Get().Len() == 0 {
				return fmt.Errorf("action: %w", ErrNotDefined)
			}
			return nil
		}
		return op(c.getContext(), cc)
	}

	return c
}

func (c *Commander) SetName(n string) *Commander  { c.name.Set(n); return c }
func (c *Commander) SetUsage(u string) *Commander { c.usage.Set(u); return c }

// Commander adds a subcommander, returning the original parent
// commander object.
func (c *Commander) Commander(sub *Commander) *Commander {
	c.subcmds.With(func(in *seq.List[*Commander]) { in.PushBack(sub) })
	return c
}

// AddUrfaveCommand directly adds a urfae/cli.Command as a subcommand
// to the Commander.
func (c *Commander) AddUrfaveCommand(cc cli.Command) *Commander {
	sub := MakeCommander()
	sub.cmd = cc
	return c.Commander(sub)
}

// AddFlag adds a command-line flag in the specified command.
func (c *Commander) AddFlag(flag Flag) *Commander {
	c.flags.With(func(in *seq.List[Flag]) { in.PushBack(flag) })
	return c
}

// AddFlags adds one or more flags to the commander.
func (c *Commander) AddFlags(flags ...Flag) *Commander { return c.AppendFlags(flags) }

// AppendFlags adds a slice of flags to the commander.
func (c *Commander) AppendFlags(flags []Flag) *Commander {
	c.flags.With(func(in *seq.List[Flag]) {
		for idx := range flags {
			in.PushBack(flags[idx])
		}
	})
	return c
}

// AddHook adds a new hook to the commander. Hooks are all executed
// before the command runs. While all hooks run and errors are
// collected, if any hook errors the action will not execute.
func (c *Commander) AddHook(op Action) *Commander {
	c.hook.With(func(in *seq.List[Action]) { in.PushBack(op) })
	return c
}

// SetMiddlware allows users to modify the context passed to the hooks
// and actions of a command.
func (c *Commander) AddMiddleware(mw Middleware) *Commander {
	c.middleware.With(func(in *seq.List[Middleware]) { in.PushBack(mw) })
	return c
}

// SetAction defines the core operation for the commander.
func (c *Commander) SetAction(in Action) *Commander { c.action.Set(in); return c }

// Command resolves the commander into a cli.Command instance. This
// operation is safe to call more options.
//
// You should only call this function *after* setting the context on
// the commander.
func (c *Commander) Command() cli.Command {
	c.once.Do(func() {
		ctx := c.getContext()
		fun.Invariant(ctx != nil, "context must be set when calling command")

		c.cmd.Name = c.name.Get()
		c.cmd.Usage = c.usage.Get()

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

	return c.cmd
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

// SetContext attaches a context to the commander. This is only needed
// if you are NOT using the commander with the Run() or Main()
// methods.
func (c *Commander) SetContext(ctx context.Context) *Commander {
	c.ctx.Set(makeContextProducer(ctx))
	return c
}

func (c *Commander) getContext() context.Context { return c.ctx.Get()() }

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

	app.Name = setWhenNotZero(a.Name, cmd.Name)
	app.Usage = setWhenNotZero(a.Usage, cmd.Usage)
	app.Version = a.Version

	app.Commands = cmd.Subcommands
	app.Action = cmd.Action
	app.Flags = cmd.Flags
	app.After = cmd.After
	app.Before = cmd.Before

	return app
}
