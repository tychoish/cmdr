package cmdr

import (
	"context"
	"errors"
	"os"

	"github.com/tychoish/fun"
	"github.com/tychoish/grip"
)

var ErrNotDefined = errors.New("not defined")

var ErrNotSet = errors.New("not set")

// Run executes a commander with the specified command line arguments.
func Run(ctx context.Context, c *Commander, args []string) error {
	fun.Invariant(len(args) != 0, "must specify one or more arguments")
	c.SetContext(ctx)
	app := c.App()
	return app.Run(args)
}

// Main provides an alternative to Run() for calling within in a
// program's main() function. Non-nil errors are logged at the
// "Emergency" level and and os.Exit(1) is called.
func Main(ctx context.Context, c *Commander) {
	grip.Context(ctx).EmergencyFatal(Run(ctx, c, os.Args))
}
