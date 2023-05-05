package cmdr

import (
	"context"
	"errors"
	"os"

	"github.com/tychoish/grip"
)

var ErrNotDefined = errors.New("not defined")

var ErrNotSpecified = errors.New("not specified")

var ErrNotSet = errors.New("not set")

// Run executes a commander with the specified command line arguments.
func Run(ctx context.Context, c *Commander, args []string) error {
	c.SetContext(ctx)
	app := c.App()
	return app.Run(args)
}

// Main provides an alternative to Run() for calling within in a
// program's main() function. Non-nil errors are logged at the
// "Emergency" level and os.Exit(1) is called.
func Main(ctx context.Context, c *Commander) {
	grip.Context(ctx).EmergencyFatal(Run(ctx, c, os.Args))
}
