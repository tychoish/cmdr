============================================
``cmdr`` -- urfave/cli command line builder
============================================

``cmdr`` (a "Commander" tool), is a toolkit for quickly and ergonomically
building CLI tools, using a clean and opinionated platform including:

- `github.com/urfave/cli <https://github.com/urfave/cli>`_ (cli orchestrator)
- `github.com/tychoish/grip <https://github.com/tychoish/grip>`_ (logging)
- `github.com/tychoish/fun <https://github.com/tychoish/fun>`_ (tooling, data
  structures, service orchestration.)

The Commander, Flag interfaces provide a great deal of flexibility for
defining and building commands either using a declarative (e.g. structures of
options,) or programatically using a chain-able interface and builders. 

The top-level `MakeRootCommander()` constructor also initializes a service
orchestration framework using components of ``github.com/tychoish/fun/srv``
for service orchestration.

Consider the following example program: 

.. code-block:: go

   package main
   
   import (
   	"context"
   	"fmt"
   	"net/http"
   	"os"
   	"os/signal"
   	"sync/atomic"
   	"syscall"
   	"time"
   
   	"github.com/tychoish/cmdr"
   	"github.com/tychoish/fun/srv"
   	"github.com/tychoish/grip"
   	"github.com/urfave/cli"
   )
   
   type ServiceConfig struct {
   	Message string
   	Timeout time.Duration
   }
   
   func StartService(ctx context.Context, conf *ServiceConfig) error {
   	// a simple web server
   
   	counter := &atomic.Int64{}
   	web := &http.Server{
   		Addr: "127.0.0.1:9001",
   		Handler: http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
   
   			num := counter.Add(1)
   
   			grip.Infof("got request: %d", num)
   
   			rw.Write([]byte(conf.Message))
   		}),
   	}
   
   	// cleanup functions run as soon as the context is canceled.
   	srv.AddCleanup(ctx, func(context.Context) error {
   		grip.Info("beginning cleanup")
   		return nil
   	})
   
   	grip.Infof("starting web service, pid=%d", os.Getpid())
   
   	return srv.GetOrchestrator(ctx).Add(srv.HTTP("hello-world", time.Minute, web))
   }
   
   func BuildCommand() *cmdr.Commander {
   	// initialize flag with default value
   	msgOpt := cmdr.FlagBuilder("hello world").
   		SetName("message", "m").
   		SetUsage("message returned by handler")
   
   	timeoutOpt := cmdr.FlagBuilder(time.Hour).
   		SetName("timeout", "t").
   		SetUsage("timeout for service lifecycle")
   
   	// create an operation spec; initialize the builder with the
   	// constructor for the configuration type. While you can use
   	// the commander directly and have more access to the
   	// cli.Context for interacting with command line arguments,
   	// the Spec model makes it possible to write more easily
   	// testable functions, and limit your exposure to the CLI
   	operation := cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Context) (*ServiceConfig, error) {
   		return &ServiceConfig{Message: fmt.Sprintln(cc.String("message"))}, nil
   	},
   	).SetMiddleware(func(ctx context.Context, conf *ServiceConfig) context.Context {
   
   		// create a new context with a timeout
   		ctx, cancel := context.WithTimeout(ctx, conf.Timeout)
   
   		// this is maybe not meaningful, but means that we
   		// cancel this timeout during shutdown and means that
   		// we cancel this context during shut down and
   		// therefore cannot leak it.
   		srv.AddCleanup(ctx, func(context.Context) error { cancel(); return nil })
   
   		// this context is passed to all subsequent options.
   		return ctx
   	}).SetAction(StartService)
   
   	// build a commander. The root Commander adds service
   	// orchestration to the context and manages the lifecylce of
   	// services started by commands.
   	cmd := cmdr.MakeRootCommander()
   
   	// this that the service will wait for the srv.Orchestrator's
   	// services to return rather than canceling the context when
   	// the action runs.
   	cmd.SetBlocking(true)
   
   	// add flags to Commander
   	cmd.Flags(msgOpt.Flag(), timeoutOpt.Flag())
   
   	// add operation to Commander
   	cmdr.AddOperationSpec(cmd, operation)
   
   	// return the operation
   	return cmd
   }
   
   func main() {
   	// because the build command is blocking this context means
   	// that we'll catch and handle the sig term correctly.
   	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
   	defer cancel()
   
   	// run the command
   	cmdr.Main(ctx, BuildCommand())
   }
