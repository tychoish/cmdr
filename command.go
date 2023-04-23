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

// Operation defines the core functionality for a command line entry
// point or handler, providing both the process' context (managed by
// the commander,) as well as the pre-operation hooks/validation
// hooks.
type Operation func(ctx context.Context, c *cli.Context) error

type Middleware func(ctx context.Context) context.Context

// Commander provides a chainable and ergonomic way of defining a
// command.
type Commander struct {
	cmd        cli.Command
	ctx        context.Context
	action     adt.Atomic[Operation]
	opts       adt.Atomic[AppOptions]
	flags      adt.Synchronized[*seq.List[Flag]]
	hook       adt.Synchronized[*seq.List[Operation]]
	middleware adt.Synchronized[*seq.List[Middleware]]
	subcmds    adt.Synchronized[*seq.List[*Commander]]
	once       sync.Once
}

// CommandOptions are the arguments to create a sub-command in a
// commander.
type CommandOptions struct {
	Name       string
	Usage      string
	Action     Operation
	Flags      []Flag
	Hidden     bool
	Subcommand bool
}

func MakeRootCommander() *Commander {
	c := MakeCommander()
	c.cmd.Name = filepath.Base(os.Args[0])
	c.middleware.With(func(in *seq.List[Middleware]) {
		in.PushBack(srv.SetBaseContext)
		in.PushBack(srv.SetShutdownSignal)
		in.PushBack(srv.WithOrchestrator)
		in.PushBack(srv.WithCleanup)
	})

	c.cmd.After = func(_ *cli.Context) error {
		// cancel the parent context
		srv.GetShutdownSignal(c.ctx)()
		return srv.GetOrchestrator(c.ctx).Wait()
	}

	return c
}

func MakeCommander() *Commander {
	c := &Commander{}

	c.flags.Set(&seq.List[Flag]{})
	c.hook.Set(&seq.List[Operation]{})
	c.subcmds.Set(&seq.List[*Commander]{})
	c.middleware.Set(&seq.List[Middleware]{})

	c.cmd.Before = func(cc *cli.Context) error {
		ec := &erc.Collector{}

		c.middleware.With(func(in *seq.List[Middleware]) {
			ec.Add(fun.Observe(c.ctx, seq.ListValues(in.Iterator()),
				func(mw Middleware) { c.ctx = mw(c.ctx) }))

		})
		c.hook.With(func(hooks *seq.List[Operation]) {
			ec.Add(fun.Observe(c.ctx, seq.ListValues(hooks.Iterator()),
				func(op Operation) { ec.Add(op(c.ctx, cc)) }))
		})
		c.flags.With(func(hooks *seq.List[Flag]) {
			ec.Add(fun.Observe(c.ctx, seq.ListValues(hooks.Iterator()),
				func(fl Flag) {
					if fl.validate != nil {
						ec.Add(fl.validate(cc))
					}
				}))
		})
		return ec.Resolve()
	}

	c.cmd.Action = func(cc *cli.Context) error {
		op := c.action.Get()
		if op == nil {
			return fmt.Errorf("action: %w", ErrNotDefined)
		}
		return op(c.ctx, cc)
	}

	return c
}

func (c *Commander) Commander(sub *Commander) *Commander {
	c.subcmds.With(func(in *seq.List[*Commander]) { in.PushBack(sub) })
	return c
}

// Subcommand creates a new sub-command within the commander and
// returns a commander instance for the sub-command.
func (c *Commander) Subcommand(opts CommandOptions) *Commander {
	sub := MakeCommander()

	fun.Invariant(opts.Action != nil, "action must not be nil")

	sub.action.Set(opts.Action)

	sub.cmd.Name = opts.Name
	sub.cmd.Usage = opts.Usage
	sub.cmd.Hidden = opts.Hidden

	for idx := range opts.Flags {
		sub.AddFlag(opts.Flags[idx])
	}

	c.Commander(sub)

	return sub
}

// AddSubcommand adds a subcommand to the commander and returns the
// original commander.
func (c *Commander) AddSubcommand(opts CommandOptions) *Commander {
	c.Subcommand(opts)
	return c
}

// AddFlag adds a command-line flag in the specified command.
func (c *Commander) AddFlag(flag Flag) *Commander {
	c.flags.With(func(in *seq.List[Flag]) { in.PushBack(flag) })
	return c
}

// AddHook adds a new hook to the commander. Hooks are all executed
// before the command runs. While all hooks run and errors are
// collected, if any hook errors the action will not execute.
func (c *Commander) AddHook(op Operation) *Commander {
	c.hook.With(func(in *seq.List[Operation]) { in.PushBack(op) })
	return c
}

// SetMiddlware allows users to modify the context passed to the hooks
// and actions of a command.
func (c *Commander) AddMiddleware(mw Middleware) *Commander {
	c.middleware.With(func(in *seq.List[Middleware]) { in.PushBack(mw) })
	return c
}

// SetAction defines the core operation for the commander.
func (c *Commander) SetAction(in Operation) *Commander { c.action.Set(in); return c }

// Command resolves the commander into a cli.Command instance. This
// operation is safe to call more options.
//
// You should only call this function *after* setting the context on
// the commander.
func (c *Commander) Command() cli.Command {
	fun.Invariant(c.ctx != nil, "context must be set when calling command")
	c.once.Do(func() {
		c.flags.With(func(in *seq.List[Flag]) {
			fun.InvariantMust(fun.Observe(c.ctx, seq.ListValues(in.Iterator()), func(v Flag) {
				c.cmd.Flags = append(c.cmd.Flags, v.value)
			}))
		})

		c.subcmds.With(func(in *seq.List[*Commander]) {
			fun.InvariantMust(fun.Observe(c.ctx, seq.ListValues(in.Iterator()), func(v *Commander) {
				v.ctx = c.ctx
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
//
// Unlike other methods on the commander, SetContext NOT safe for
// concurrent use from multiple threads. make sure that you only call
// it once.
func (c *Commander) SetContext(ctx context.Context) *Commander { c.ctx = ctx; return c }

// App resolves a command object from the commander and the provided
// options. You must set the context on the Commander using the
// SetContext before calling this command directly.
//
// In most cases you will use the Run() or Main() methods, rather than
// App() to use the commander, and Run()/Main() provide their own contexts.
func (c *Commander) App() *cli.App {
	fun.Invariant(c.ctx != nil, "context must be set before calling the app")
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

func setWhenNotZero[T comparable](a, b T) T {
	if fun.IsZero(a) {
		return b
	}
	return a
}
