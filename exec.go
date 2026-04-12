package cmdr

import (
	"context"
	"log"
	"os"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/srv"
)

const ErrNotDefined = ers.Error("not defined")

const ErrNotSpecified = ers.Error("not specified")

const ErrNotSet = ers.Error("not set")

// Run executes a commander with the specified command line arguments.
func Run(ctx context.Context, c *Commander, args []string) error {
	if c.ctx == nil {
		c.ctx = adt.NewAtomic(ctxMaker(ctx))
	}

	c.setContext(ctx)
	app := c.App()
	err := app.Run(c.getContext(), args)

	cctx := c.getContext()
	if srv.HasShutdownSignal(cctx) {
		srv.GetShutdownSignal(cctx)()
	}
	if srv.HasOrchestrator(cctx) {
		err = erc.Join(err, srv.GetOrchestrator(cctx).Wait())
	}

	return err
}

// Main provides an alternative to Run() for calling within in a
// program's main() function. Non-nil errors are logged at the
// "Emergency" level and os.Exit(1) is called.
func Main(ctx context.Context, c *Commander) {
	if err := Run(ctx, c, os.Args); err != nil {
		log.Panic(err)
	}
}
