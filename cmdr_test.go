package cmdr

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/seq"
	"github.com/tychoish/fun/srv"
	"github.com/urfave/cli"
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
func (c *Commander) numSubcommands() int {
	var o int
	c.subcmds.With(func(i *seq.List[*Commander]) { o = i.Len() })
	return o
}

func TestCommander(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Zero", func(t *testing.T) {
		cmd := MakeCommander()
		assert.Zero(t, cmd.numHooks())
		assert.Zero(t, cmd.numFlags())
		assert.Zero(t, cmd.numSubcommands())
	})
	t.Run("EndToEnd", func(t *testing.T) {
		t.Run("Run", func(t *testing.T) {
			count := 0
			cmd := MakeCommander().
				AddHook(func(ctx context.Context, cc *cli.Context) error {
					count++
					return nil
				}).
				SetAction(func(ctx context.Context, cc *cli.Context) error {
					count++
					check.Equal(t, cc.String("hello"), "kip")
					return nil
				}).
				AddFlag(MakeFlag(FlagOptions[string]{
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
				AddHook(func(ctx context.Context, cc *cli.Context) error {
					count++
					return nil
				}).
				AddFlag(MakeFlag(FlagOptions[string]{
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
					count++
					return cc.String("hello"), nil
				},
				// do the op
				func(ctx context.Context, arg string) error {
					check.Equal(t, arg, "kip")
					return nil
				},
			)

			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "kip"}))
			assert.Equal(t, count, 3)
		})
		t.Run("RequiredFlag", func(t *testing.T) {
			count := 0
			cmd := MakeCommander().
				SetAction(func(ctx context.Context, cc *cli.Context) error {
					count++
					check.Equal(t, cc.String("hello"), "merlin")
					return nil
				}).
				AddFlag(MakeFlag(FlagOptions[string]{
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
				AddFlag(MakeFlag(FlagOptions[string]{
					Name: "hello",
					Validate: func(in string) error {
						check.Equal(t, in, "")
						return errors.New("validation failure")
					},
				}))

			assert.Error(t, Run(ctx, cmd, []string{t.Name()}))
			assert.Equal(t, count, 0)
		})
		t.Run("HookAbort", func(t *testing.T) {
			count := 0
			cmd := MakeCommander().
				AddHook(func(ctx context.Context, cc *cli.Context) error {
					count++
					check.Equal(t, cc.String("hello"), "kip")
					return errors.New("kip")
				}).
				SetAction(func(ctx context.Context, cc *cli.Context) error {
					count++
					return nil
				}).
				AddFlag(MakeFlag(FlagOptions[string]{Name: "hello"}))

			assert.Error(t, Run(ctx, cmd, []string{t.Name(), "--hello", "kip"}))
			assert.Equal(t, count, 1)
		})
		t.Run("Middleware", func(t *testing.T) {
			t.Run("Panic", func(t *testing.T) {
				count := 0
				cmd := MakeRootCommander().
					AddMiddleware(func(ctx context.Context) context.Context {
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
					AddFlag(MakeFlag(FlagOptions[string]{Name: "hello"}))
				assert.Panic(t, func() {
					_ = Run(ctx, cmd, []string{t.Name(), "--hello", "kip"})
				})

				assert.Equal(t, count, 1)

			})
			t.Run("Succeeds", func(t *testing.T) {
				count := 0
				cmd := MakeRootCommander().
					AddMiddleware(func(ctx context.Context) context.Context {
						count++
						return srv.SetBaseContext(ctx)
					}).
					SetAction(func(ctx context.Context, cc *cli.Context) error {
						count++
						assert.True(t, srv.HasBaseContext(ctx))
						return nil
					}).
					AddFlag(MakeFlag(FlagOptions[string]{Name: "hello"}))
				assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "kip"}))
				assert.Equal(t, count, 2)
			})
			t.Run("AddCommand", func(t *testing.T) {
				count := 0
				cmd := MakeCommander().
					AddHook(func(ctx context.Context, cc *cli.Context) error {
						count++
						return nil
					}).
					SetAction(func(ctx context.Context, cc *cli.Context) error {
						count++
						check.Equal(t, cc.String("hello"), "kip")
						return nil
					}).
					AddFlag(MakeFlag(FlagOptions[string]{
						Name: "hello",
						Validate: func(in string) error {
							count++
							check.Equal(t, in, "kip")
							return nil
						},
					})).
					SetContext(ctx).
					SetName("sub")

				ncmd := MakeCommander().AddCommand(cmd.Command()).SetName(t.Name())

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
		cmd.AddFlag(MakeFlag(FlagOptions[string]{Name: "first"})).
			AddHook(func(context.Context, *cli.Context) error {
				return nil
			}).
			Commander(Subcommand(CommandOptions[string]{
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
		cmd := MakeCommander().AddFlag(MakeFlag(FlagOptions[string]{Name: "first"})).
			AddHook(func(context.Context, *cli.Context) error {
				return nil
			}).
			SetAppOptions(AppOptions{
				Name: t.Name(),
			})
		cmd.Commander(Subcommand(CommandOptions[string]{
			Name: "second",
			Operation: func(context.Context, string) error {
				return nil
			},
			Flags: []Flag{MakeFlag(FlagOptions[string]{Name: "hello"})},
		})).Commander(Subcommand(CommandOptions[string]{
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

			flag := MakeFlag(FlagOptions[int]{
				Name:     "hello",
				Validate: func(in int) error { counter++; check.Equal(t, in, 42); return nil },
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().AddFlag(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, 42, cc.Int("hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "42"}))
			assert.Equal(t, 2, counter)
		})
		t.Run("Int64", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(FlagOptions[int64]{
				Name:     "hello",
				Validate: func(in int64) error { counter++; check.Equal(t, in, 42); return nil },
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().AddFlag(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, 42, cc.Int64("hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "42"}))
			assert.Equal(t, 2, counter)
		})
		t.Run("Float64", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(FlagOptions[float64]{
				Name:     "hello",
				Validate: func(in float64) error { counter++; check.Equal(t, in, 42); return nil },
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().AddFlag(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, 42, cc.Float64("hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "42"}))
			assert.Equal(t, 2, counter)
		})
		t.Run("BoolFalse", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(FlagOptions[bool]{
				Name: "hello",
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().AddFlag(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.True(t, !cc.Bool("hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name()}))
			assert.Equal(t, 1, counter)
		})
		t.Run("BoolTrue", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(FlagOptions[bool]{
				Name: "hello",
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().AddFlag(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.True(t, cc.Bool("hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello"}))
			assert.Equal(t, 1, counter)
		})
		t.Run("BoolT", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(FlagOptions[bool]{
				Name:    "hello",
				Default: true,
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().AddFlag(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.True(t, cc.BoolT("hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name()}))
			assert.Equal(t, 1, counter)
		})
		t.Run("StringSlice", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(FlagOptions[[]string]{
				Name: "hello",
				Validate: func(in []string) error {
					counter++
					check.Equal(t, 2, len(in))
					return nil
				},
			})
			check.Equal(t, "hello", flag.value.GetName())
			cmd := MakeCommander().AddFlag(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
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

			flag := MakeFlag(FlagOptions[[]int]{
				Name: "hello",
				Validate: func(in []int) error {
					counter++
					check.Equal(t, 2, len(in))
					return nil
				},
			})
			cmd := MakeCommander().
				AddFlag(flag).
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

			flag := MakeFlag(FlagOptions[[]int64]{
				Name: "hello",
				Validate: func(in []int64) error {
					counter++
					check.Equal(t, 2, len(in))
					return nil
				},
			})
			cmd := MakeCommander().
				AddFlag(flag).
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

	t.Run("Helpers", func(t *testing.T) {
		t.Run("SetWhenNotZero", func(t *testing.T) {
			const (
				a = "first"
				b = "second"
			)
			check.Equal(t, a, setWhenNotZero("", a))
			check.Equal(t, a, setWhenNotZero(a, b))
			check.Equal(t, b, setWhenNotZero("", b))
			check.Equal(t, "", setWhenNotZero("", ""))
		})
	})
}
