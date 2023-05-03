package cmdr

import (
	"context"

	"github.com/tychoish/fun"
)

func setWhenNotZero[T comparable](a, b T) T {
	if fun.IsZero(a) {
		return b
	}
	return a
}

// context producer is so you can store a context in an atomic

type contextProducer func() context.Context

func makeContextProducer(ctx context.Context) contextProducer {
	return func() context.Context { return ctx }
}
