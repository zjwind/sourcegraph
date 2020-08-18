package persistence

import (
	"context"
)

type Database interface {
	Writer
	Reader
	PatchDatabase(ctx context.Context, patch Reader) error
}
