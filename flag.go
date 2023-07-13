package cmdr

import (
	"time"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
)

// FlagTypes defines the limited set of types which are supported by
// the flag parsing system.
type FlagTypes interface {
	string | int | uint | int64 | uint64 | float64 | bool | *time.Time | time.Duration | []string | []int | []int64
}

// FlagOptions provide a generic way to generate a flag
// object. Methods on FlagOptions are provided for consistency and
// ergonomics: they are not safe for concurrent use.
type FlagOptions[T FlagTypes] struct {
	Name      string
	Aliases   []string
	Usage     string
	FilePath  string
	Required  bool
	Hidden    bool
	TakesFile bool
	Validate  func(T) error

	TimestampLayout string

	// Default values are provided to the parser for many
	// types. However, slice-types do not support default values.
	Default T
	// Destination provides a pointer to a variable where the flag
	// parser will store the result. The parser only supports this
	// for a subset of types, and this will panic if the type does
	// not support this.
	Destination *T
}

// FlagBuilder provides a constructor that you can use to build a
// FlagOptions. Provide the constructor with the default value, which
// you can override later, if needed. Slice values *must* be the empty
// list.
func FlagBuilder[T FlagTypes](defaultVal T) *FlagOptions[T] {
	return &FlagOptions[T]{Default: defaultVal}
}

func (fo *FlagOptions[T]) SetName(s ...string) *FlagOptions[T] {
	switch len(s) {
	case 0:
	case 1:
		fo.Name = s[0]
	default:
		fo.Name = s[0]
		fo.AddAliases(s[1:]...)
	}

	return fo
}

func (fo *FlagOptions[T]) AddAliases(a ...string) *FlagOptions[T] {
	return fo.SetAliases(append(fo.Aliases, a...))
}

func (fo *FlagOptions[T]) SetTimestmapLayout(l string) *FlagOptions[T] {
	switch any(fo.Default).(type) {
	case time.Time, *time.Time:
		fo.TimestampLayout = l
	default:
		fun.Invariant.OK(false, "cannot set timestamp layout for non-timestamp flags")
	}
	return fo
}

func (fo *FlagOptions[T]) SetAliases(a []string) *FlagOptions[T]       { fo.Aliases = a; return fo }
func (fo *FlagOptions[T]) SetUsage(s string) *FlagOptions[T]           { fo.Usage = s; return fo }
func (fo *FlagOptions[T]) SetFilePath(s string) *FlagOptions[T]        { fo.FilePath = s; return fo }
func (fo *FlagOptions[T]) SetRequired(b bool) *FlagOptions[T]          { fo.Required = b; return fo }
func (fo *FlagOptions[T]) SetHidden(b bool) *FlagOptions[T]            { fo.Hidden = b; return fo }
func (fo *FlagOptions[T]) SetTakesFile(b bool) *FlagOptions[T]         { fo.TakesFile = b; return fo }
func (fo *FlagOptions[T]) SetValidate(v func(T) error) *FlagOptions[T] { fo.Validate = v; return fo }
func (fo *FlagOptions[T]) SetDefault(d T) *FlagOptions[T]              { fo.Default = d; return fo }
func (fo *FlagOptions[T]) SetDestination(p *T) *FlagOptions[T]         { fo.Destination = p; return fo }
func (fo *FlagOptions[T]) Flag() Flag                                  { return MakeFlag(fo) }
func (fo *FlagOptions[T]) Add(c *Commander)                            { c.Flags(fo.Flag()) }

func (fo *FlagOptions[T]) doValidate(in T) error {
	if fo.Validate == nil {
		return nil
	}
	return fo.Validate(in)
}

// Flag defines a command line flag, and is produced using the
// FlagOptions struct by the MakeFlag function.
type Flag struct {
	value        cli.Flag
	validateOnce *adt.Once[error]
}

// MakeFlag builds a commandline flag instance and validation from a
// typed flag to options to a flag object for the command
// line.
func MakeFlag[T FlagTypes](opts *FlagOptions[T]) Flag {
	out := Flag{validateOnce: &adt.Once[error]{}}

	switch dval := any(opts.Default).(type) {
	case string:
		out.value = &cli.StringFlag{
			Name:        opts.Name,
			Aliases:     opts.Aliases,
			Usage:       opts.Usage,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       dval,
			Destination: any(opts.Destination).(*string),
			Action: func(cc *cli.Context, val string) error {
				out.validateOnce.Do(func() error {
					return opts.doValidate(any(val).(T))
				})
				return out.validateOnce.Resolve()
			},
		}
	case int:
		out.value = &cli.IntFlag{
			Name:        opts.Name,
			Aliases:     opts.Aliases,
			Usage:       opts.Usage,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       dval,
			Destination: any(opts.Destination).(*int),
			Action: func(cc *cli.Context, val int) error {
				out.validateOnce.Do(func() error {
					return opts.doValidate(any(val).(T))
				})
				return out.validateOnce.Resolve()
			},
		}
	case uint:
		out.value = &cli.UintFlag{
			Name:        opts.Name,
			Aliases:     opts.Aliases,
			Usage:       opts.Usage,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       dval,
			Destination: any(opts.Destination).(*uint),
			Action: func(cc *cli.Context, val uint) error {
				out.validateOnce.Do(func() error {
					return opts.doValidate(any(val).(T))
				})
				return out.validateOnce.Resolve()
			},
		}
	case int64:
		out.value = &cli.Int64Flag{
			Name:        opts.Name,
			Aliases:     opts.Aliases,
			Usage:       opts.Usage,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       dval,
			Destination: any(opts.Destination).(*int64),
			Action: func(cc *cli.Context, val int64) error {
				out.validateOnce.Do(func() error {
					return opts.doValidate(any(val).(T))
				})
				return out.validateOnce.Resolve()
			},
		}
	case uint64:
		out.value = &cli.Uint64Flag{
			Name:        opts.Name,
			Aliases:     opts.Aliases,
			Usage:       opts.Usage,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       dval,
			Destination: any(opts.Destination).(*uint64),
			Action: func(cc *cli.Context, val uint64) error {
				out.validateOnce.Do(func() error {
					return opts.doValidate(any(val).(T))
				})
				return out.validateOnce.Resolve()
			},
		}
	case float64:
		out.value = &cli.Float64Flag{
			Name:        opts.Name,
			Aliases:     opts.Aliases,
			Usage:       opts.Usage,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       dval,
			Destination: any(opts.Destination).(*float64),
			Action: func(cc *cli.Context, val float64) error {
				out.validateOnce.Do(func() error {
					return opts.doValidate(any(val).(T))
				})
				return out.validateOnce.Resolve()
			},
		}
	case bool:
		out.value = &cli.BoolFlag{
			Name:        opts.Name,
			Aliases:     opts.Aliases,
			Usage:       opts.Usage,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       dval,
			Destination: any(opts.Destination).(*bool),
			Action: func(cc *cli.Context, val bool) error {
				out.validateOnce.Do(func() error {
					return opts.doValidate(any(val).(T))
				})
				return out.validateOnce.Resolve()
			},
		}
	case *time.Time:
		if opts.TimestampLayout == "" {
			opts.TimestampLayout = time.RFC3339
		}
		if dval == nil {
			dval = &time.Time{}
		}
		out.value = &cli.TimestampFlag{
			Name:     opts.Name,
			Aliases:  opts.Aliases,
			Usage:    opts.Usage,
			FilePath: opts.FilePath,
			Required: opts.Required,
			Hidden:   opts.Hidden,
			Value:    cli.NewTimestamp(*dval),
			Layout:   opts.TimestampLayout,
			Action: func(cc *cli.Context, val *time.Time) error {
				out.validateOnce.Do(func() error {
					return opts.doValidate(any(val).(T))
				})
				return out.validateOnce.Resolve()
			},
		}
	case time.Duration:
		out.value = &cli.DurationFlag{
			Name:     opts.Name,
			Aliases:  opts.Aliases,
			Usage:    opts.Usage,
			FilePath: opts.FilePath,
			Required: opts.Required,
			Hidden:   opts.Hidden,
			Value:    dval,
			Action: func(cc *cli.Context, val time.Duration) error {
				out.validateOnce.Do(func() error {
					return opts.doValidate(any(val).(T))
				})
				return out.validateOnce.Resolve()
			},
		}
	case []string:
		o := &cli.StringSliceFlag{
			Name:     opts.Name,
			Aliases:  opts.Aliases,
			Usage:    opts.Usage,
			FilePath: opts.FilePath,
			Required: opts.Required,
			Hidden:   opts.Hidden,
			Action: func(cc *cli.Context, val []string) error {
				out.validateOnce.Do(func() error {
					return opts.doValidate(any(val).(T))
				})
				return out.validateOnce.Resolve()
			},
		}
		fun.Invariant.OK(len(dval) == 0, "slice flags should not have default values")
		fun.Invariant.OK(opts.Destination == nil, "cannot specify destination for slice values")

		out.value = o
	case []int:
		out.value = &cli.IntSliceFlag{
			Name:     opts.Name,
			Aliases:  opts.Aliases,
			Usage:    opts.Usage,
			FilePath: opts.FilePath,
			Required: opts.Required,
			Hidden:   opts.Hidden,
			Action: func(cc *cli.Context, val []int) error {
				out.validateOnce.Do(func() error {
					return opts.doValidate(any(val).(T))
				})
				return out.validateOnce.Resolve()
			},
		}
		fun.Invariant.OK(len(dval) == 0, "slice flags should not have default values")
		fun.Invariant.OK(opts.Destination == nil, "cannot specify destination for slice values")
	case []int64:
		out.value = &cli.Int64SliceFlag{
			Name:     opts.Name,
			Aliases:  opts.Aliases,
			Usage:    opts.Usage,
			FilePath: opts.FilePath,
			Required: opts.Required,
			Hidden:   opts.Hidden,
			Action: func(cc *cli.Context, val []int64) error {
				out.validateOnce.Do(func() error {
					return opts.doValidate(any(val).(T))
				})
				return out.validateOnce.Resolve()
			},
		}

		fun.Invariant.OK(len(dval) == 0, "slice flags should not have default values")
		fun.Invariant.OK(opts.Destination == nil, "cannot specify destination for slice values")
	}

	return out
}

// GetFlag resolves a flag of the specified name to the type as
// specified.
//
// This will panic at runtime if the type of the flag specified does
// not match the type of the flag as defined.
func GetFlag[T FlagTypes](cc *cli.Context, name string) T {
	var out T

	switch any(out).(type) {
	case string:
		out = any(cc.String(name)).(T)
	case int:
		out = any(cc.Int(name)).(T)
	case uint:
		out = any(cc.Uint(name)).(T)
	case int64:
		out = any(cc.Int64(name)).(T)
	case uint64:
		out = any(cc.Uint64(name)).(T)
	case float64:
		out = any(cc.Float64(name)).(T)
	case bool:
		out = any(cc.Bool(name)).(T)
	case *time.Time:
		out = any(cc.Timestamp(name)).(T)
	case time.Duration:
		out = any(cc.Duration(name)).(T)
	case []string:
		out = any(cc.StringSlice(name)).(T)
	case []int:
		out = any(cc.IntSlice(name)).(T)
	case []int64:
		out = any(cc.Int64Slice(name)).(T)
	}

	return out
}
