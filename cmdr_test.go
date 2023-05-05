package cmdr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/urfave/cli"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/seq"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/fun/testt"
)

func (c *Commander) numFlags() int {
	var o int
	c.flags.With(func(i *seq.List[Flag]) { o = i.Len() })
	return o
}

func (c *Commander) numHooks() int {
	var o int
	c.hook.With(func(i *seq.List[Action]) { o = i.Len() })
	return o
}
func (c *Commander) numMiddleware() int {
	var o int
	c.middleware.With(func(i *seq.List[Middleware]) { o = i.Len() })
	return o
}

func (c *Commander) numSubcommands() int {
	var o int
	c.subcmds.With(func(i *seq.List[*Commander]) { o = i.Len() })
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
			cmd := MakeCommander().SetContext(ctx)
			err := cmd.App().Run([]string{"hello"})
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrNotDefined)
		})
	})
	t.Run("DefineSubcommand", func(t *testing.T) {
		cmd := MakeCommander().SetContext(ctx).Subcommanders(MakeCommander()).SetName("hello").SetAction(func(context.Context, *cli.Context) error { return nil })
		assert.NotError(t, cmd.App().Run([]string{t.Name(), "hello"}))
	})
	t.Run("EndToEnd", func(t *testing.T) {
		t.Run("Run", func(t *testing.T) {
			count := 0
			cmd := MakeCommander().
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
				}))

			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "kip"}))
			assert.Equal(t, count, 2)
		})
		t.Run("Operation", func(t *testing.T) {
			count := 0
			cmd := MakeCommander().
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
			assert.Equal(t, count, 3)
		})
		t.Run("Subcommanders", func(t *testing.T) {
			cmd := MakeRootCommander().SetName("foo").SetContext(ctx)
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
				cmd := MakeCommander()
				AddOperationSpec(cmd, &OperationSpec[string]{
					Constructor: func(ctx context.Context, cc *cli.Context) (string, error) { count++; return "hi", nil },
					HookOperations: []Operation[string]{func(ctx context.Context, in string) error {
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
				cmd := MakeCommander()
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
					cmd := MakeCommander()
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
				cmd := MakeCommander()
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
					check.Equal(t, cc.String("hello"), "merlin")
					return nil
				}).
				With((&FlagOptions[string]{
					Name: "hello",
					Validate: func(in string) error {
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
				cmd := MakeCommander().
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
					SetContext(ctx).
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
		cmd := MakeCommander()
		err := Run(ctx, cmd, []string{t.Name()})
		assert.ErrorIs(t, err, ErrNotDefined)
	})
	t.Run("ResolutionIsIdempotent", func(t *testing.T) {
		cmd := MakeCommander()
		cmd.SetContext(ctx)
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
		cmd := MakeCommander().Flags(MakeFlag(&FlagOptions[string]{Name: "first"})).
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
		cmd.SetContext(ctx)
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
	t.Run("FlagConstruction", func(t *testing.T) {
		t.Run("Int", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(&FlagOptions[int]{
				Name:     "hello",
				Validate: func(in int) error { counter++; check.Equal(t, in, 42); return nil },
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, 42, cc.Int("hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "42"}))
			assert.Equal(t, 2, counter)
		})
		t.Run("Int64", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(&FlagOptions[int64]{
				Name:     "hello",
				Validate: func(in int64) error { counter++; check.Equal(t, in, 42); return nil },
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, 42, cc.Int64("hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "42"}))
			assert.Equal(t, 2, counter)
		})
		t.Run("Duration", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(&FlagOptions[time.Duration]{
				Name:     "hello",
				Validate: func(in time.Duration) error { counter++; check.Equal(t, in, 42*time.Second); return nil },
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, 42*time.Second, cc.Duration("hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "42s"}))
			assert.Equal(t, 2, counter)
		})
		t.Run("Float64", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(&FlagOptions[float64]{
				Name:     "hello",
				Validate: func(in float64) error { counter++; check.Equal(t, in, 42); return nil },
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, 42, cc.Float64("hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "42"}))
			assert.Equal(t, 2, counter)
		})
		t.Run("BoolFalse", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(&FlagOptions[bool]{
				Name: "hello",
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.True(t, !cc.Bool("hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name()}))
			assert.Equal(t, 1, counter)
		})
		t.Run("BoolTrue", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(&FlagOptions[bool]{
				Name: "hello",
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.True(t, cc.Bool("hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello"}))
			assert.Equal(t, 1, counter)
		})
		t.Run("BoolT", func(t *testing.T) {
			counter := 0

			flag := FlagBuilder(true).SetName("hello").Flag()
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.True(t, cc.BoolT("hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name()}))
			assert.Equal(t, 1, counter)
		})
		t.Run("StringSlice", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(&FlagOptions[[]string]{
				Name: "hello",
				Validate: func(in []string) error {
					counter++
					check.Equal(t, 2, len(in))
					return nil
				},
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				val := cc.StringSlice("hello")
				check.Equal(t, val[0], "not")
				check.Equal(t, val[1], "other")
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "not", "--hello", "other"}))
			assert.Equal(t, 2, counter)
		})
		t.Run("IntSlice", func(t *testing.T) {
			counter := 0

			flag := FlagBuilder[[]int](nil).SetValidate(func(in []int) error {
				counter++
				check.Equal(t, 2, len(in))
				return nil
			}).SetName("hello").Flag()
			cmd := MakeCommander().
				Flags(flag).
				SetAction(func(ctx context.Context, cc *cli.Context) error {
					counter++
					val := cc.IntSlice("hello")
					assert.Equal(t, len(val), 2)
					assert.Equal(t, val[0], 300)
					assert.Equal(t, val[1], 100)
					return nil
				})
			check.Equal(t, "hello", flag.value.GetName())
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "300", "--hello", "100"}))
			assert.Equal(t, 2, counter)
		})
		t.Run("Int64Slice", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(&FlagOptions[[]int64]{
				Name: "hello",
				Validate: func(in []int64) error {
					counter++
					check.Equal(t, 2, len(in))
					return nil
				},
			})
			cmd := MakeCommander().
				Flags(flag).
				SetAction(func(ctx context.Context, cc *cli.Context) error {
					counter++
					val := cc.Int64Slice("hello")
					assert.Equal(t, len(val), 2)
					assert.Equal(t, val[0], 300)
					assert.Equal(t, val[1], 100)
					return nil
				})
			check.Equal(t, "hello", flag.value.GetName())
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "300", "--hello", "100"}))
			assert.Equal(t, 2, counter)
		})
	})
	t.Run("FlagBuilder", func(t *testing.T) {
		t.Run("Default", func(t *testing.T) {
			called := false
			cmd := MakeCommander().
				Flags(FlagBuilder("hi").SetName("world").Flag()).
				SetAction(func(_ context.Context, cc *cli.Context) error {
					check.Equal(t, cc.String("world"), "hi")
					called = true
					return nil
				})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name()}))
			assert.True(t, called)
		})
		t.Run("Options", func(t *testing.T) {
			count := 0
			var dest string
			cmd := MakeCommander().
				Flags(FlagBuilder("hi").
					SetName("world").
					SetUsage("checked value").
					SetEnvVar("hello").
					SetFilePath("/tmp/conf").
					SetRequired(false).
					SetHidden(false).
					SetTakesFile(false).
					SetDefault("beep").
					SetDestination(&dest).
					SetValidate(func(op string) error {
						count++
						check.Equal(t, op, "beep")
						return nil
					}).
					Flag(),
				).
				SetAction(func(_ context.Context, cc *cli.Context) error {
					check.Equal(t, cc.String("world"), "beep")
					count++
					return nil
				})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name()}))
			assert.Equal(t, count, 2)
			assert.Equal(t, dest, "beep")
		})
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
		t.Run("PostProcessAction", func(t *testing.T) {
			t.Run("Converers", func(t *testing.T) {
				var called bool
				for _, action := range []any{
					func(context.Context) error { called = true; return nil },
					func(*cli.Context) error { called = true; return nil },
					func(context.Context, *cli.Context) error { called = true; return nil },
					func() error { called = true; return nil },
					func(context.Context) { called = true },
					func() { called = true },
				} {
					assert.True(t, !called)
					assert.True(t, action != nil)
					cmds := []cli.Command{{Action: action}}
					reformCommands(ctx, cmds)
					assert.True(t, cmds[0].Action != nil)
					op, ok := cmds[0].Action.(func(*cli.Context) error)
					testt.Logf(t, "%T", cmds[0].Action)
					assert.True(t, ok)
					assert.NotError(t, op(nil))
					assert.True(t, called)
					called = false
				}
			})
			t.Run("Nil", func(t *testing.T) {
				cmd := cli.Command{Action: nil}
				reformCommands(ctx, []cli.Command{cmd})
				assert.True(t, cmd.Action == nil)
			})
			t.Run("Passthrough", func(t *testing.T) {
				act := func(*cli.Context) error { return errors.New("foo") }
				cmd := []cli.Command{{Action: act}}
				reformCommands(ctx, cmd)
				assert.Equal(t, fmt.Sprintf("%p", act), fmt.Sprintf("%p", cmd[0].Action))
			})

		})
	})
}
