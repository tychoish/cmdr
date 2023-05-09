package cmdr

import (
	"context"
	"errors"
	"os"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/grip"
)

var ErrNotDefined = errors.New("not defined")

var ErrNotSpecified = errors.New("not specified")

var ErrNotSet = errors.New("not set")

// Run executes a commander with the specified command line arguments.
func Run(ctx context.Context, c *Commander, args []string) error {
	if c.ctx == nil {
		grip.Alertf("commander %q is not a root commander, and ought to be", c.name.Get())
		c.ctx = adt.NewAtomic(ctxMaker(ctx))
	}

	c.setContext(ctx)
	app := c.App()
	return app.RunContext(c.getContext(), args)
}

// Main provides an alternative to Run() for calling within in a
// program's main() function. Non-nil errors are logged at the
// "Emergency" level and os.Exit(1) is called.
func Main(ctx context.Context, c *Commander) {
	err := Run(ctx, c, os.Args)
	grip.Context(c.getContext()).EmergencyFatal(err)
}
