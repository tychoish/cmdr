package cmdr

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/srv"
)

func (c *Commander) numFlags() int {
	var o int
	c.flags.With(func(i *dt.List[Flag]) { o = i.Len() })
	return o
}

func (c *Commander) numHooks() int {
	var o int
	c.hook.With(func(i *dt.List[Action]) { o = i.Len() })
	return o
}

func (c *Commander) numMiddleware() int {
	var o int
	c.middleware.With(func(i *dt.List[Middleware]) { o = i.Len() })
	return o
}

func (c *Commander) numSubcommands() int {
	var o int
	c.subcmds.With(func(i *dt.List[*Commander]) { o = i.Len() })
	return o
}

func TestCommander(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Zero", func(t *testing.T) {
		t.Run("Init", func(t *testing.T) {
			cmd := MakeCommander()
			assert.Zero(t, cmd.numHooks())
			assert.Zero(t, cmd.numFlags())
			assert.Zero(t, cmd.numSubcommands())
		})
		t.Run("ExpectedPanic", func(t *testing.T) {
			cmd := MakeCommander()
			assert.Panic(t, func() {
				// for context reasons
				_ = cmd.App().Run([]string{"hello"})
			})
		})
		t.Run("ErrorUndefined", func(t *testing.T) {
			cmd := MakeRootCommander().setContext(ctx)
			err := cmd.App().Run([]string{"hello"})
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrNotDefined)
		})
	})
	t.Run("DefineSubcommand", func(t *testing.T) {
		cmd := MakeRootCommander().setContext(ctx).Subcommanders(MakeCommander()).SetName("hello").SetAction(func(context.Context, *cli.Context) error { return nil })
		assert.NotError(t, cmd.App().Run([]string{t.Name(), "hello"}))
	})
	t.Run("EndToEnd", func(t *testing.T) {
		t.Run("Run", func(t *testing.T) {
			count := 0
			cmd := MakeRootCommander().
				Hooks(func(ctx context.Context, cc *cli.Context) error {
					count++
					return nil
				}).
				SetAction(func(ctx context.Context, cc *cli.Context) error {
					count++
					check.Equal(t, cc.String("hello"), "kip")
					return nil
				}).
				Flags(MakeFlag(&FlagOptions[string]{
					Name: "hello",
					Validate: func(in string) error {
						check.Equal(t, in, "kip")
						return nil
					},
				})).SetBlocking(true)
			assert.True(t, cmd.blocking.Load())

			cmd.SetBlocking(false)

			assert.True(t, !cmd.blocking.Load())

			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "kip"}))
			assert.Equal(t, count, 2)
		})
		t.Run("Operation", func(t *testing.T) {
			count := 0
			cmd := MakeRootCommander().
				Hooks(func(ctx context.Context, cc *cli.Context) error {
					count++
					return nil
				}).
				Flags(MakeFlag(&FlagOptions[string]{
					Name: "hello",
					Validate: func(in string) error {
						count++
						check.Equal(t, in, "kip")
						return nil
					},
				}))

			AddOperation(cmd,
				// process command line args
				func(ctx context.Context, cc *cli.Context) (string, error) {
					check.Equal(t, cc.String("world"), "merlin")
					count++
					return cc.String("hello"), nil
				},
				// do the op
				func(ctx context.Context, arg string) error {
					check.Equal(t, arg, "kip")
					return nil
				},
				// flags
				MakeFlag(&FlagOptions[string]{Name: "world, w", Default: "merlin"}),
			).SetName(t.Name())

			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "kip"}))
			assert.Equal(t, 3, count)
		})
		t.Run("Subcommanders", func(t *testing.T) {
			cmd := MakeRootCommander().SetName("foo").setContext(ctx)
			sub := Subcommander(cmd,
				func(ctx context.Context, cc *cli.Context) (string, error) { return "", nil },
				func(ctx context.Context, in string) error { return nil },
			).Aliases("one", "two")
			assert.NotEqual(t, cmd, sub)
			cmd = cmd.Subcommanders(sub)
			app := cmd.App()

			assert.NotError(t, app.Run([]string{"foo", "one"}))
			assert.Error(t, app.Run([]string{"foo", "error"}))
			assert.Error(t, app.Run([]string{"foo"}))
		})
		t.Run("OperationSpec", func(t *testing.T) {
			t.Run("Basic", func(t *testing.T) {
				count := 0
				cmd := MakeCommander()
				cmd.ctx = adt.NewAtomic(ctxMaker(ctx))
				AddOperationSpec(cmd, &OperationSpec[string]{
					Constructor: func(ctx context.Context, cc *cli.Context) (string, error) { count++; return "hi", nil },
					HookOperations: []Operation[string]{
						func(ctx context.Context, in string) error {
							count++
							check.Equal(t, in, "hi")
							return nil
						},
					},
					Middleware: func(ctx context.Context, in string) context.Context {
						count++
						check.Equal(t, in, "hi")
						return ctx
					},
					Action: func(ctx context.Context, in string) error { count++; check.Equal(t, in, "hi"); return nil },
				})
				assert.Equal(t, cmd.numHooks(), 1)
				assert.Equal(t, cmd.numMiddleware(), 1)
				assert.True(t, cmd.action.Get() != nil)
				assert.NotError(t, Run(ctx, cmd, []string{"comp"}))
				assert.Equal(t, count, 4)
			})
			t.Run("Builder", func(t *testing.T) {
				count := 0
				cmd := MakeCommander()
				cmd.ctx = adt.NewAtomic(ctxMaker(ctx))
				AddOperationSpec(cmd,
					SpecBuilder(func(ctx context.Context, cc *cli.Context) (string, error) {
						count++
						return "hi", nil
					}).Hooks(func(ctx context.Context, in string) error {
						count++
						check.Equal(t, in, "hi")
						return nil
					}).SetMiddleware(func(ctx context.Context, in string) context.Context {
						count++
						check.Equal(t, in, "hi")
						return ctx
					}).SetAction(func(ctx context.Context, in string) error {
						count++
						check.Equal(t, in, "hi")
						return nil
					}),
				)

				assert.Equal(t, cmd.numHooks(), 1)
				assert.Equal(t, cmd.numMiddleware(), 1)
				assert.True(t, cmd.action.Get() != nil)
				assert.NotError(t, Run(ctx, cmd, []string{"comp"}))
				assert.Equal(t, count, 4)
			})

			t.Run("HookErrorAborts", func(t *testing.T) {
				count := 0
				cmd := MakeRootCommander()
				AddOperationSpec(cmd, &OperationSpec[string]{
					Constructor: func(ctx context.Context, cc *cli.Context) (string, error) { count++; return "hi", nil },
					HookOperations: []Operation[string]{
						func(ctx context.Context, in string) error {
							count++
							check.Equal(t, in, "hi")
							return errors.New("abort")
						},
					},
					Action: func(ctx context.Context, in string) error { count++; check.Equal(t, in, "hi"); return nil },
				})
				assert.Equal(t, cmd.numHooks(), 1)
				assert.True(t, cmd.action.Get() != nil)
				assert.Error(t, Run(ctx, cmd, []string{"comp"}))
				assert.Equal(t, count, 2)
			})
		})
		t.Run("CompositeHook", func(t *testing.T) {
			t.Run("Hook", func(t *testing.T) {
				count := 0
				cmd := MakeRootCommander()
				AddOperation(cmd,
					CompositeHook(
						func(ctx context.Context, cc *cli.Context) (string, error) { count++; return "hi", nil },
						func(ctx context.Context, in string) error {
							count++
							check.Equal(t, in, "hi")
							return errors.New("abort")
						},
						func(ctx context.Context, in string) error { count++; check.Equal(t, in, "hi"); return nil },
					),
					// operation
					func(ctx context.Context, in string) error { count++; check.Equal(t, in, "hi"); return nil },
				)
				assert.Equal(t, cmd.numHooks(), 1)
				assert.True(t, cmd.action.Get() != nil)
				assert.Error(t, Run(ctx, cmd, []string{"comp"}))
				assert.Equal(t, count, 2)
			})
			t.Run("Errors", func(t *testing.T) {
				t.Run("Constructor", func(t *testing.T) {
					count := 0
					cmd := MakeRootCommander()
					AddOperation(cmd,
						CompositeHook(
							func(ctx context.Context, cc *cli.Context) (string, error) { count++; return "", errors.New("abort") },
							func(ctx context.Context, in string) error { count++; return nil },
						),
						// operation
						func(ctx context.Context, in string) error { count++; return nil },
					)
					assert.Equal(t, cmd.numHooks(), 1)
					assert.True(t, cmd.action.Get() != nil)
					assert.Error(t, Run(ctx, cmd, []string{"comp"}))
					assert.Equal(t, count, 1)
				})
			})
			t.Run("Run", func(t *testing.T) {
				count := 0
				cmd := MakeRootCommander()
				AddOperation(cmd,
					CompositeHook(
						func(ctx context.Context, cc *cli.Context) (string, error) { count++; return "hi", nil },
						func(ctx context.Context, in string) error { count++; check.Equal(t, in, "hi"); return nil },
						func(ctx context.Context, in string) error { count++; check.Equal(t, in, "hi"); return nil },
					),
					// operation
					func(ctx context.Context, in string) error { count++; check.Equal(t, in, "hi"); return nil },
				)
				assert.Equal(t, cmd.numHooks(), 1)
				assert.True(t, cmd.action.Get() != nil)
				assert.NotError(t, Run(ctx, cmd, []string{"comp"}))
				assert.Equal(t, count, 4)
			})
		})
		t.Run("RequiredFlag", func(t *testing.T) {
			count := 0
			cmd := MakeCommander().
				SetAction(func(ctx context.Context, cc *cli.Context) error {
					count++
					check.Equal(t, cc.String("hello"), "merlin")
					return nil
				}).
				Flags(MakeFlag(&FlagOptions[string]{
					Name:     "hello",
					Required: true,
					Validate: func(in string) error {
						count++
						return nil
					},
				}))

			assert.Error(t, Run(ctx, cmd, []string{t.Name()}))
			assert.Equal(t, count, 0)
		})
		t.Run("ValidationFailure", func(t *testing.T) {
			count := 0
			cmd := MakeCommander().
				SetAction(func(ctx context.Context, cc *cli.Context) error {
					count++
					t.Log(cc.IsSet("hello"))
					t.Fatal("should never be called because of flag validation")
					return nil
				}).
				With((&FlagOptions[string]{
					Name: "hello",
					Validate: func(in string) error {
						t.Log("hello", in)
						check.Equal(t, in, "")
						return errors.New("validation failure")
					},
				}).Add)

			assert.Error(t, Run(ctx, cmd, []string{t.Name()}))
			assert.Equal(t, count, 0)
		})
		t.Run("HookAbort", func(t *testing.T) {
			count := 0
			cmd := MakeCommander().
				Hooks(func(ctx context.Context, cc *cli.Context) error {
					count++
					check.Equal(t, cc.String("hello"), "kip")
					return errors.New("kip")
				}).
				SetAction(func(ctx context.Context, cc *cli.Context) error {
					count++
					return nil
				}).
				Flags(MakeFlag(&FlagOptions[string]{Name: "hello"}))

			assert.Error(t, Run(ctx, cmd, []string{t.Name(), "--hello", "kip"}))
			assert.Equal(t, count, 1)
		})
		t.Run("Middleware", func(t *testing.T) {
			t.Run("Panic", func(t *testing.T) {
				count := 0
				cmd := MakeRootCommander().
					Middleware(func(ctx context.Context) context.Context {
						count++
						return nil
					}).
					SetAction(func(ctx context.Context, cc *cli.Context) error {
						if ctx != nil {
							t.Error("middleware did not set context")
						}
						panic("woop")
						count++
						return nil
					}).
					Flags(MakeFlag(&FlagOptions[string]{Name: "hello"}))
				assert.Panic(t, func() {
					_ = Run(ctx, cmd, []string{t.Name(), "--hello", "kip"})
				})

				assert.Equal(t, count, 1)
			})
			t.Run("Succeeds", func(t *testing.T) {
				count := 0
				cmd := MakeRootCommander().
					Middleware(func(ctx context.Context) context.Context {
						count++
						return srv.SetBaseContext(ctx)
					}).
					SetAction(func(ctx context.Context, cc *cli.Context) error {
						count++
						assert.True(t, srv.HasBaseContext(ctx))
						return nil
					}).
					Flags(MakeFlag(&FlagOptions[string]{Name: "hello"}))
				assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "kip"}))
				assert.Equal(t, count, 2)
			})
			t.Run("AddCommand", func(t *testing.T) {
				count := 0
				cmd := MakeRootCommander().
					Hooks(func(ctx context.Context, cc *cli.Context) error {
						count++
						return nil
					}).
					SetAction(func(ctx context.Context, cc *cli.Context) error {
						count++
						check.Equal(t, cc.String("hello"), "kip")
						return nil
					}).
					Flags(MakeFlag(&FlagOptions[string]{
						Name: "hello",
						Validate: func(in string) error {
							count++
							check.Equal(t, in, "kip")
							return nil
						},
					})).
					setContext(ctx).
					SetName("sub").
					SetUsage("usage")

				ncmd := MakeCommander().UrfaveCommands(cmd.Command()).SetName(t.Name())

				assert.NotError(t, Run(ctx, ncmd, []string{t.Name(), "sub", "--hello", "kip"}))
				// assert.Equal(t, count, 3)
			})
		})
		t.Run("Main", func(t *testing.T) {
			count := 0
			cmd := MakeCommander().SetAction(func(ctx context.Context, cc *cli.Context) error { count++; return nil })
			assert.NotPanic(t, func() {
				args := os.Args
				defer func() { os.Args = args }()
				os.Args = []string{t.Name()}

				Main(ctx, cmd)
			})
			assert.Equal(t, count, 1)
		})
	})
	t.Run("OperationNotDefined", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			cmd := MakeCommander()
			err := Run(ctx, cmd, []string{t.Name()})
			assert.ErrorIs(t, err, ErrNotDefined)
		})
		t.Run("NonEmpty", func(t *testing.T) {
			cmd := MakeCommander().Subcommanders(MakeCommander().SetName("hi"))
			err := Run(ctx, cmd, []string{t.Name()})
			assert.ErrorIs(t, err, ErrNotSpecified)
		})
		t.Run("Incorrect", func(t *testing.T) {
			cmd := MakeCommander().Subcommanders(MakeCommander().SetName("hi"))
			err := Run(ctx, cmd, []string{"hi", t.Name()})
			assert.ErrorIs(t, err, ErrNotDefined)
		})
	})
	t.Run("ResolutionIsIdempotent", func(t *testing.T) {
		cmd := MakeRootCommander()
		cmd.setContext(ctx)
		assert.Equal(t, 0, cmd.numFlags())
		assert.Equal(t, 0, cmd.numHooks())
		assert.Equal(t, 0, cmd.numSubcommands())
		cmd.Flags(MakeFlag(&FlagOptions[string]{Name: "first"})).
			Hooks(func(context.Context, *cli.Context) error {
				return nil
			}).
			Subcommanders(OptionsCommander(CommandOptions[string]{
				Name: "second",
				Operation: func(context.Context, string) error {
					return nil
				},
			}))
		assert.Equal(t, 1, cmd.numFlags())
		assert.Equal(t, 1, cmd.numHooks())
		assert.Equal(t, 1, cmd.numSubcommands())
		out := cmd.Command()
		assert.Equal(t, len(out.Flags), 1)
		assert.Equal(t, len(out.Subcommands), 1)
		out = cmd.Command()
		assert.Equal(t, len(out.Flags), 1)
		assert.Equal(t, len(out.Subcommands), 1)
	})
	t.Run("AppResolutionIsUnique", func(t *testing.T) {
		cmd := MakeRootCommander().Flags(MakeFlag(&FlagOptions[string]{Name: "first"})).
			Hooks(func(context.Context, *cli.Context) error {
				return nil
			}).
			SetAppOptions(AppOptions{
				Name: t.Name(),
			})
		cmd.Subcommanders(OptionsCommander(CommandOptions[string]{
			Name: "second",
			Operation: func(context.Context, string) error {
				return nil
			},
			Flags: []Flag{MakeFlag(&FlagOptions[string]{Name: "hello"})},
		})).Subcommanders(OptionsCommander(CommandOptions[string]{
			Name: "third",
			Operation: func(context.Context, string) error {
				return nil
			},
		}))
		cmd.setContext(ctx)
		app1 := cmd.App()
		app2 := cmd.App()
		if app1 == app2 {
			t.Error("app instances are not equal")
		}
		assert.Equal(t, app1.Name, t.Name())
		assert.Equal(t, app2.Name, t.Name())
		assert.Equal(t, 2, len(app1.Commands))
		sub := app1.Commands[0]
		assert.Equal(t, 1, len(sub.Flags))
	})
	t.Run("Helpers", func(t *testing.T) {
		t.Run("SetWhenNotZero", func(t *testing.T) {
			const (
				a = "first"
				b = "second"
			)
			check.Equal(t, a, secondValueWhenFirstIsZero("", a))
			check.Equal(t, a, secondValueWhenFirstIsZero(a, b))
			check.Equal(t, b, secondValueWhenFirstIsZero("", b))
			check.Equal(t, "", secondValueWhenFirstIsZero("", ""))
		})
	})
}
