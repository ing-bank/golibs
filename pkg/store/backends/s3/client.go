package s3

import (
	"context"
)

// Client defines the minimal S3 operations needed for the store.
type Client interface {
	PutObject(ctx context.Context, bucket, key string, body []byte) error
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	ListObjects(ctx context.Context, bucket string) ([]string, error)
	CreateBucket(ctx context.Context, bucket string) error
	DeleteBucket(ctx context.Context, bucket string) error
}
