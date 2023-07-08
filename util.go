package cmdr

import (
	"context"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
)

func secondValueWhenFirstIsZero[T comparable](a, b T) T {
	if ft.IsZero(a) {
		return b
	}
	return a
}

// context producer is so you can store a context in an atomic

type contextProducer func() context.Context

func ctxMaker(ctx context.Context) contextProducer { return func() context.Context { return ctx } }

func appendTo[T any](l *adt.Synchronized[*dt.List[T]], i ...T) {
	l.With(func(s *dt.List[T]) { s.Append(i...) })
}
