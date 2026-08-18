package cache

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Null is the cache used when caching is switched off. Every read misses and
// every write is discarded, so behaviour is identical to an empty cache that
// is never populated — correct, just not fast.
type Null struct{}

var _ Cache = Null{}

func NewNull() Null { return Null{} }

func (Null) Get(context.Context, string, any) (bool, error)        { return false, nil }
func (Null) Set(context.Context, string, any, time.Duration) error { return nil }
func (Null) Delete(context.Context, ...string) error               { return nil }
func (Null) DeleteByPrefix(context.Context, string) error          { return nil }
func (Null) Ping(context.Context) error                            { return nil }
func (Null) Close() error                                          { return nil }

// join builds a colon-separated key from mixed parts.
func join(parts []any) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, fmt.Sprint(p))
	}
	return strings.Join(out, ":")
}
