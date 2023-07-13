package cmdr

import (
	"context"
	"testing"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
)

func TestFlags(t *testing.T) {
	ctx := testt.Context(t)
	t.Run("FlagConstruction", func(t *testing.T) {
		t.Run("Int", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(&FlagOptions[int]{
				Name:     "hello",
				Validate: func(in int) error { counter++; check.Equal(t, in, 42); return nil },
			})
			check.Equal(t, "hello", flag.value.Names()[0])
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, 42, cc.Int("hello"))
				check.Equal(t, 42, GetFlag[int](cc, "hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "42"}))
			assert.Equal(t, 2, counter)
		})
		t.Run("Uint", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(&FlagOptions[uint]{
				Name:     "hello",
				Validate: func(in uint) error { counter++; check.Equal(t, in, 42); return nil },
			})
			check.Equal(t, "hello", flag.value.Names()[0])
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, 42, cc.Uint("hello"))
				check.Equal(t, 42, GetFlag[uint](cc, "hello"))
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
			check.Equal(t, "hello", flag.value.Names()[0])
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, 42, cc.Int64("hello"))
				check.Equal(t, 42, GetFlag[int64](cc, "hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "42"}))
			assert.Equal(t, 2, counter)
		})
		t.Run("Uint64", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(&FlagOptions[uint64]{
				Name:     "hello",
				Validate: func(in uint64) error { counter++; check.Equal(t, in, 42); return nil },
			})
			check.Equal(t, "hello", flag.value.Names()[0])
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, 42, cc.Uint64("hello"))
				check.Equal(t, 42, GetFlag[uint64](cc, "hello"))
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
			check.Equal(t, "hello", flag.value.Names()[0])
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, 42*time.Second, cc.Duration("hello"))
				check.Equal(t, 42*time.Second, GetFlag[time.Duration](cc, "hello"))
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
			check.Equal(t, "hello", flag.value.Names()[0])
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, 42, cc.Float64("hello"))
				check.Equal(t, 42, GetFlag[float64](cc, "hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", "42"}))
			assert.Equal(t, 2, counter)
		})
		t.Run("BoolFalse", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(&FlagOptions[bool]{
				Name:    "hello",
				Default: false,
			})
			check.Equal(t, "hello", flag.value.Names()[0])
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.True(t, !cc.Bool("hello"))
				check.True(t, !GetFlag[bool](cc, "hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name()}))
			assert.Equal(t, 1, counter)
		})
		t.Run("BoolTrue", func(t *testing.T) {
			counter := 0

			flag := MakeFlag(&FlagOptions[bool]{
				Name:    "hello",
				Default: true,
			})
			check.Equal(t, "hello", flag.value.Names()[0])
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.True(t, cc.Bool("hello"))
				check.True(t, GetFlag[bool](cc, "hello"))

				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello"}))
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
			check.Equal(t, "hello", flag.value.Names()[0])
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				val := cc.StringSlice("hello")
				check.Equal(t, val[0], "not")
				check.Equal(t, val[1], "other")

				v2 := GetFlag[[]string](cc, "hello")
				check.EqualItems(t, val, v2)

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

					v2 := GetFlag[[]int](cc, "hello")
					check.EqualItems(t, val, v2)

					return nil
				})
			check.Equal(t, "hello", flag.value.Names()[0])
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

					v2 := GetFlag[[]int64](cc, "hello")
					check.EqualItems(t, val, v2)

					return nil
				})
			check.Equal(t, "hello", flag.value.Names()[0])
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
					check.Equal(t, GetFlag[string](cc, "world"), "hi")
					called = true
					return nil
				})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name()}))
			assert.True(t, called)
		})
		t.Run("Alaises", func(t *testing.T) {
			called := false
			now := time.Now()
			cmd := MakeCommander().
				Flags(FlagBuilder(&now).SetName("world", "fire").Flag()).
				SetAction(func(_ context.Context, cc *cli.Context) error {
					check.Equal(t, *cc.Timestamp("fire"), now)
					check.Equal(t, *GetFlag[*time.Time](cc, "world"), now)

					called = true
					return nil
				})
			check.NotError(t, Run(ctx, cmd, []string{t.Name()}))
			assert.True(t, called)
		})
		t.Run("TimestmapPtr", func(t *testing.T) {
			called := false
			now := time.Now().Truncate(time.Minute)
			cmd := MakeCommander().
				Flags(FlagBuilder(&now).SetName("world", "fire").
					SetTimestmapLayout(time.RFC822).
					Flag(),
				).
				SetAction(func(_ context.Context, cc *cli.Context) error {
					check.Equal(t, *cc.Timestamp("world"), now.Add(time.Hour))
					check.Equal(t, *GetFlag[*time.Time](cc, "world"), now.Add(time.Hour))
					called = true
					return nil
				})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--fire", now.Add(time.Hour).Format(time.RFC822)}))
			assert.True(t, called)
		})
		t.Run("Timestmap", func(t *testing.T) {
			counter := 0
			epoch := time.Unix(0, 0)
			flag := MakeFlag[*time.Time](&FlagOptions[*time.Time]{Name: "hello"})
			check.Equal(t, "hello", flag.value.Names()[0])
			cmd := MakeCommander().Flags(flag).SetAction(func(ctx context.Context, cc *cli.Context) error {
				counter++
				check.Equal(t, epoch, *cc.Timestamp("hello"))
				check.Equal(t, epoch, *GetFlag[*time.Time](cc, "hello"))
				return nil
			})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--hello", epoch.Format(time.RFC3339)}))
			assert.Equal(t, 1, counter)
		})
		t.Run("Timestmap", func(t *testing.T) {
			called := false
			now := time.Now().Truncate(time.Minute)
			cmd := MakeCommander().
				Flags(FlagBuilder(&now).SetName("world", "fire").
					SetTimestmapLayout(time.RFC822).
					Flag(),
				).
				SetAction(func(_ context.Context, cc *cli.Context) error {
					check.Equal(t, *cc.Timestamp("world"), now.Add(time.Hour))
					check.Equal(t, *GetFlag[*time.Time](cc, "world"), now.Add(time.Hour))
					called = true
					return nil
				})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name(), "--fire", now.Add(time.Hour).Format(time.RFC822)}))
			assert.True(t, called)
		})
		t.Run("Options", func(t *testing.T) {
			count := 0
			var dest string
			cmd := MakeCommander().
				Flags(FlagBuilder("hi").
					SetName("world").
					SetUsage("checked value").
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
					check.Equal(t, GetFlag[string](cc, "world"), "beep")
					count++
					return nil
				})
			assert.NotError(t, Run(ctx, cmd, []string{t.Name()}))
			assert.Equal(t, count, 2)
			assert.Equal(t, dest, "beep")
		})
		t.Run("SetTimeozone", func(t *testing.T) {
			flag := FlagBuilder("hi")
			check.Panic(t, func() { flag.SetTimestmapLayout("100") })
		})

	})

}
