package cmdr

import (
	"context"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/seq"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/send"
	"github.com/urfave/cli"
)

// Operation defines the core functionality for a command line entry
// point or handler, providing both the process' context (managed by
// the commander,) as well as the pre-operation hooks/validation
// hooks.
type Operation func(ctx context.Context, c *cli.Context) error

// Commander provides a chainable and ergonomic way of defining a
// command.
type Commander struct {
	ctx        context.Context
	cmd        cli.Command
	middleware adt.Atomic[func(context.Context) context.Context]
	action     adt.Atomic[Operation]
	sender     adt.Atomic[send.Sender]
	opts       adt.Atomic[AppOptions]
	flags      adt.Synchronized[*seq.List[Flag]]
	hook       adt.Synchronized[*seq.List[Operation]]
	subcmds    adt.Synchronized[*seq.List[*Commander]]
	once       sync.Once
}

// CommandOptions are the arguments to create a sub-command in a
// commander.
type CommandOptions struct {
	Name   string
	Usage  string
	Action Operation
	Hidden bool
}

func makeCommander(ctx context.Context) *Commander {
	c := &Commander{ctx: ctx}

	c.flags.Set(&seq.List[Flag]{})
	c.hook.Set(&seq.List[Operation]{})
	c.subcmds.Set(&seq.List[*Commander]{})
	c.middleware.Set(func(in context.Context) context.Context { return in })

	c.cmd.Action = func(cc *cli.Context) error {
		op := c.action.Get()
		if op == nil {
			return ErrNotDefined
		}
		return op(ctx, cc)
	}

	c.cmd.Before = func(cc *cli.Context) error {
		// inject user context at the very end of the setup
		defer func() { ctx = c.middleware.Get()(ctx) }()
		if !grip.HasLogger(ctx) {
			if s := c.sender.Get(); s != nil {
				ctx = grip.WithLogger(ctx, grip.NewLogger(s))
			}
		}
		ec := &erc.Collector{}
		c.hook.With(func(hooks *seq.List[Operation]) {
			ec.Add(fun.Observe(ctx, seq.ListValues(hooks.Iterator()),
				func(op Operation) { ec.Add(op(ctx, cc)) }))
		})
		c.flags.With(func(hooks *seq.List[Flag]) {
			ec.Add(fun.Observe(c.ctx, seq.ListValues(hooks.Iterator()),
				func(fl Flag) { ec.Add(fl.validate(cc)) }))
		})
		return ec.Resolve()
	}

	return c
}

// MakeRootCommand constructs a basic commander that can be modified
// by calling methods on the Commander.
func MakeRootCommand(ctx context.Context) *Commander {
	ctx = srv.SetBaseContext(ctx)
	ctx = srv.WithShutdownManager(ctx)
	ctx = srv.WithOrchestrator(ctx)

	c := makeCommander(ctx)

	c.sender.Set(grip.Sender())
	c.cmd.After = func(_ *cli.Context) error {
		// cancel the parent context
		srv.GetShutdownSignal(ctx)()
		return srv.GetOrchestrator(ctx).Wait()
	}

	return c
}

// Subcommand creates a new sub-command within the commander and
// returns a commander instance for the sub-command.
func (c *Commander) Subcommand(opts CommandOptions) *Commander {
	sub := makeCommander(c.ctx)

	fun.Invariant(opts.Action != nil, "action must not be nil")

	sub.action.Set(opts.Action)

	sub.cmd.Name = opts.Name
	sub.cmd.Usage = opts.Usage
	sub.cmd.Hidden = opts.Hidden

	c.subcmds.With(func(in *seq.List[*Commander]) { in.PushBack(sub) })

	return sub
}

// AddFlag adds a command-line flag in the specified command. This is
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
func (c *Commander) SetMiddleware(in func(context.Context) context.Context) *Commander {
	c.middleware.Set(in)
	return c
}

// SetSender sets the underlying logging provided in the context to
// operations and hooks.
func (c *Commander) SetSender(s send.Sender) *Commander { c.sender.Set(s); return c }

// SetAction defines the core operation for the commander.
func (c *Commander) SetAction(in Operation) *Commander { c.action.Set(in); return c }

// Command resolves the commander into a cli.Command instance. This
// operation is safe to call more options.
func (c *Commander) Command() cli.Command {
	c.once.Do(func() {
		c.flags.With(func(in *seq.List[Flag]) {
			fun.InvariantMust(fun.Observe(c.ctx, seq.ListValues(in.Iterator()), func(v Flag) {
				c.cmd.Flags = append(c.cmd.Flags, v.value)
			}))
		})

		c.subcmds.With(func(in *seq.List[*Commander]) {
			fun.InvariantMust(fun.Observe(c.ctx, seq.ListValues(in.Iterator()), func(v *Commander) {
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

// App resolves a command object from the commander and the provided options.
func (c *Commander) App() *cli.App {
	a := c.opts.Get()
	app := cli.NewApp()
	app.Name = a.Name
	app.Usage = a.Usage
	app.Version = a.Version

	cmd := c.Command()
	app.Action = cmd.Action
	app.Flags = cmd.Flags
	app.After = cmd.After
	app.Before = cmd.Before

	return app
}
