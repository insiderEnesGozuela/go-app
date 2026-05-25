// Package logger wraps zerolog with project-wide conventions:
//   - structured JSON in production, human-readable in dev
//   - context propagation so request-scoped fields (request_id, user_id) flow
//     through service/repository layers without being passed explicitly
package logger

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

type ctxKey struct{}

// Options controls logger construction. Service is added to every line so logs
// from multiple services in an aggregator (Loki, ELK) can be filtered.
type Options struct {
	Level   string
	Pretty  bool
	Service string
}

func New(opts Options) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(opts.Level)
	if err != nil || lvl == zerolog.NoLevel {
		lvl = zerolog.InfoLevel
	}

	var w io.Writer = os.Stdout
	if opts.Pretty {
		w = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	}

	zerolog.TimeFieldFormat = time.RFC3339Nano

	logger := zerolog.New(w).
		Level(lvl).
		With().
		Timestamp().
		Str("service", nonEmpty(opts.Service, "go-wallet")).
		Logger()

	return logger
}

// Into stores a logger on the context. Pair with [From] in downstream layers.
func Into(ctx context.Context, l zerolog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// From retrieves the request-scoped logger, or a no-op disabled logger if none
// was attached. Returning a disabled logger (not a default global) prevents
// surprise log spam from code paths that forgot to set up a logger.
func From(ctx context.Context) zerolog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(zerolog.Logger); ok {
		return l
	}
	return zerolog.Nop()
}

// With returns a new context carrying a logger derived with extra fields.
// Typical use: ctx = logger.With(ctx, "request_id", reqID, "user_id", uid).
func With(ctx context.Context, kv ...any) context.Context {
	l := From(ctx)
	c := l.With()
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			continue
		}
		c = c.Interface(key, kv[i+1])
	}
	return Into(ctx, c.Logger())
}

func nonEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
