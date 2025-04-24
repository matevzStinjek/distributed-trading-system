package interfaces

import (
	"context"
	"time"
)

type CacheClient interface {
	Set(context.Context, string, any, time.Duration) error
	Close() error
}

type PubsubClient interface {
	Publish(context.Context, string, any) error
	Close() error
}
