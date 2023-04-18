package cmdr

import (
	"fmt"

	"github.com/tychoish/fun"
	"github.com/urfave/cli"
)

// FlagOptions provide a generic way to generate a flag object.
type FlagOptions[T any] struct {
	Name        string
	Usage       string
	EnvVar      string
	FilePath    string
	Required    bool
	Hidden      bool
	TakesFile   bool
	Default     T
	Destination *T
	Validate    func(T) (T, error)
}

// Flag defines a command line flag, and is produced using the
// FlagOptions struct by the MakeFlag function.
type Flag struct {
	value    cli.Flag
	validate func(c *cli.Context) error
}

func getValidateFunction[T any](
	name string,
	required bool,
	in func(string, *cli.Context) T,
	validate func(T) (T, error),
) func(*cli.Context) error {
	return func(c *cli.Context) error {
		if validate != nil {
			value, err := validate(in(name, c))
			if err != nil {
				return err
			}
			return c.Set(name, fmt.Sprint(value))
		}

		return nil
	}
}

// MakeFlag builds a commandline flag instance and validation from a
// typed flag to options to a flag object for the command line.
func MakeFlag[T any](opts FlagOptions[T]) Flag {
	var out Flag

	switch any(fun.ZeroOf[T]()).(type) {
	case string:
		out.value = cli.StringFlag{
			Name:        opts.Name,
			Usage:       opts.Usage,
			EnvVar:      opts.EnvVar,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       any(opts.Default).(string),
			Destination: any(opts.Destination).(*string),
		}
		out.validate = getValidateFunction(
			opts.Name,
			opts.Required,
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
			Value:       any(opts.Default).(int),
			Destination: any(opts.Destination).(*int),
		}
		out.validate = getValidateFunction(
			opts.Name,
			opts.Required,
			func(in string, c *cli.Context) T { return any(c.Int(in)).(T) },
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
			opts.Required,
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
			opts.Required,
			func(in string, c *cli.Context) T { return any(c.Float64(in)).(T) },
			opts.Validate,
		)
	case bool:
		if any(opts.Default).(bool) {
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
		if opts.Destination != nil {
			vd := any(opts.Destination).(*cli.StringSlice)
			o.Value = vd
		} else {
			vd := cli.StringSlice(any(opts.Default).([]string))
			o.Value = &vd
		}

		out.value = o
		out.validate = getValidateFunction(
			opts.Name,
			opts.Required,
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
		if opts.Destination != nil {
			vd := any(opts.Destination).(*cli.IntSlice)
			o.Value = vd
		} else {
			vd := cli.IntSlice(any(opts.Default).([]int))
			o.Value = &vd
		}

		out.value = o
		out.validate = getValidateFunction(
			opts.Name,
			opts.Required,
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
		if opts.Destination != nil {
			vd := any(opts.Destination).(*cli.Int64Slice)
			o.Value = vd
		} else {
			vd := cli.Int64Slice(any(opts.Default).([]int64))
			o.Value = &vd
		}

		out.value = o
		out.validate = getValidateFunction(
			opts.Name,
			opts.Required,
			func(in string, c *cli.Context) T { return any(c.Int64Slice(in)).(T) },
			opts.Validate,
		)

	default:
		fun.Invariant(out.value == nil, fmt.Sprintf("flag constructor for %T is not defined", opts.Default))
	}

	return out
}
