package cmdr

import (
	"strings"
	"time"

	"github.com/urfave/cli"

	"github.com/tychoish/fun"
)

// FlagTypes defines the limited set of types which are supported by
// the flag parsing system.
type FlagTypes interface {
	string | int | int64 | float64 | bool | []string | []int | []int64 | time.Duration
}

// FlagOptions provide a generic way to generate a flag
// object. Methods on FlagOptions are provided for consistency and
// ergonomics: they are not safe for concurrent use.
type FlagOptions[T FlagTypes] struct {
	Name      string
	Usage     string
	EnvVar    string
	FilePath  string
	Required  bool
	Hidden    bool
	TakesFile bool
	Validate  func(T) error

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
	fo.Name = strings.Join(s, ", ")
	return fo
}

func (fo *FlagOptions[T]) SetUsage(s string) *FlagOptions[T]           { fo.Usage = s; return fo }
func (fo *FlagOptions[T]) SetEnvVar(s string) *FlagOptions[T]          { fo.EnvVar = s; return fo }
func (fo *FlagOptions[T]) SetFilePath(s string) *FlagOptions[T]        { fo.FilePath = s; return fo }
func (fo *FlagOptions[T]) SetRequired(b bool) *FlagOptions[T]          { fo.Required = b; return fo }
func (fo *FlagOptions[T]) SetHidden(b bool) *FlagOptions[T]            { fo.Hidden = b; return fo }
func (fo *FlagOptions[T]) SetTakesFile(b bool) *FlagOptions[T]         { fo.TakesFile = b; return fo }
func (fo *FlagOptions[T]) SetValidate(v func(T) error) *FlagOptions[T] { fo.Validate = v; return fo }
func (fo *FlagOptions[T]) SetDefault(d T) *FlagOptions[T]              { fo.Default = d; return fo }
func (fo *FlagOptions[T]) SetDestination(p *T) *FlagOptions[T]         { fo.Destination = p; return fo }
func (fo *FlagOptions[T]) Flag() Flag                                  { return MakeFlag(fo) }
func (fo *FlagOptions[T]) Add(c *Commander)                            { c.Flags(fo.Flag()) }

// Flag defines a command line flag, and is produced using the
// FlagOptions struct by the MakeFlag function.
type Flag struct {
	value    cli.Flag
	validate func(c *cli.Context) error
}

func getValidateFunction[T any](
	name string,
	in func(string, *cli.Context) T,
	validate func(T) error,
) func(*cli.Context) error {
	return func(c *cli.Context) error {
		if validate != nil {
			if err := validate(in(name, c)); err != nil {
				return err
			}
		}

		return nil
	}
}

// MakeFlag builds a commandline flag instance and validation from a
// typed flag to options to a flag object for the command
// line.
func MakeFlag[T FlagTypes](opts *FlagOptions[T]) Flag {
	var out Flag

	switch dval := any(opts.Default).(type) {
	case string:
		out.value = cli.StringFlag{
			Name:        opts.Name,
			Usage:       opts.Usage,
			EnvVar:      opts.EnvVar,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       dval,
			Destination: any(opts.Destination).(*string),
		}
		out.validate = getValidateFunction(
			opts.Name,
			func(in string, c *cli.Context) T { return any(c.String(in)).(T) },
			opts.Validate,
		)
	case int:
		out.value = cli.IntFlag{
			Name:        opts.Name,
			Usage:       opts.Usage,
			EnvVar:      opts.EnvVar,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       dval,
			Destination: any(opts.Destination).(*int),
		}
		out.validate = getValidateFunction(
			opts.Name,
			func(in string, c *cli.Context) T { return any(c.Int(in)).(T) },
			opts.Validate,
		)
	case time.Duration:
		out.value = cli.DurationFlag{
			Name:     opts.Name,
			Usage:    opts.Usage,
			EnvVar:   opts.EnvVar,
			FilePath: opts.FilePath,
			Required: opts.Required,
			Hidden:   opts.Hidden,
			Value:    dval,
		}

		out.validate = getValidateFunction(
			opts.Name,
			func(in string, c *cli.Context) T { return any(c.Duration(in)).(T) },
			opts.Validate,
		)
	case int64:
		out.value = cli.Int64Flag{
			Name:        opts.Name,
			Usage:       opts.Usage,
			EnvVar:      opts.EnvVar,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       any(opts.Default).(int64),
			Destination: any(opts.Destination).(*int64),
		}
		out.validate = getValidateFunction(
			opts.Name,
			func(in string, c *cli.Context) T { return any(c.Int64(in)).(T) },
			opts.Validate,
		)
	case float64:
		out.value = cli.Float64Flag{
			Name:        opts.Name,
			Usage:       opts.Usage,
			EnvVar:      opts.EnvVar,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       any(opts.Default).(float64),
			Destination: any(opts.Destination).(*float64),
		}
		out.validate = getValidateFunction(
			opts.Name,
			func(in string, c *cli.Context) T { return any(c.Float64(in)).(T) },
			opts.Validate,
		)
	case bool:
		if dval {
			out.value = cli.BoolTFlag{
				Name:        opts.Name,
				Usage:       opts.Usage,
				EnvVar:      opts.EnvVar,
				FilePath:    opts.FilePath,
				Required:    opts.Required,
				Hidden:      opts.Hidden,
				Destination: any(opts.Destination).(*bool),
			}
		} else {
			out.value = cli.BoolFlag{
				Name:        opts.Name,
				Usage:       opts.Usage,
				EnvVar:      opts.EnvVar,
				FilePath:    opts.FilePath,
				Required:    opts.Required,
				Hidden:      opts.Hidden,
				Destination: any(opts.Destination).(*bool),
			}
		}
	case []string:
		o := cli.StringSliceFlag{
			Name:     opts.Name,
			Usage:    opts.Usage,
			EnvVar:   opts.EnvVar,
			FilePath: opts.FilePath,
			Required: opts.Required,
			Hidden:   opts.Hidden,
		}
		fun.Invariant(len(dval) == 0, "slice flags should not have default values")
		fun.Invariant(opts.Destination == nil, "cannot specify destination for slice values")

		out.value = o
		out.validate = getValidateFunction(
			opts.Name,
			func(in string, c *cli.Context) T { return any(c.StringSlice(in)).(T) },
			opts.Validate,
		)
	case []int:
		o := cli.IntSliceFlag{
			Name:     opts.Name,
			Usage:    opts.Usage,
			EnvVar:   opts.EnvVar,
			FilePath: opts.FilePath,
			Required: opts.Required,
			Hidden:   opts.Hidden,
		}
		fun.Invariant(len(dval) == 0, "slice flags should not have default values")
		fun.Invariant(opts.Destination == nil, "cannot specify destination for slice values")

		out.value = o
		out.validate = getValidateFunction(
			opts.Name,
			func(in string, c *cli.Context) T { return any(c.IntSlice(in)).(T) },
			opts.Validate,
		)
	case []int64:
		o := cli.Int64SliceFlag{
			Name:     opts.Name,
			Usage:    opts.Usage,
			EnvVar:   opts.EnvVar,
			FilePath: opts.FilePath,
			Required: opts.Required,
			Hidden:   opts.Hidden,
		}

		fun.Invariant(len(dval) == 0, "slice flags should not have default values")
		fun.Invariant(opts.Destination == nil, "cannot specify destination for slice values")

		out.value = o
		out.validate = getValidateFunction(
			opts.Name,
			func(in string, c *cli.Context) T { return any(c.Int64Slice(in)).(T) },
			opts.Validate,
		)
	}

	return out
}
