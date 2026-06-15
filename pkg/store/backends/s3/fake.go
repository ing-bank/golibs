package s3

import (
	"context"
	"errors"
	"sync"
)

// --- Mock S3 client for testing ---

type MockS3Client struct {
	mu      sync.RWMutex
	objects map[string][]byte
	buckets map[string]bool
}

func NewMockS3Client() *MockS3Client {
	return &MockS3Client{
		objects: make(map[string][]byte),
		buckets: make(map[string]bool),
	}
}

func (m *MockS3Client) PutObject(ctx context.Context, bucket, key string, body []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[bucket+"|"+key] = body
	return nil
}

func (m *MockS3Client) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.objects[bucket+"|"+key]
	if !ok {
		return nil, errors.New("not found")
	}
	return data, nil
}

func (m *MockS3Client) DeleteObject(ctx context.Context, bucket, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, bucket+"|"+key)
	return nil
}

func (m *MockS3Client) ListObjects(ctx context.Context, bucket string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var keys []string
	prefix := bucket + "|"
	for k := range m.objects {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			keys = append(keys, k[len(prefix):])
		}
	}
	return keys, nil
}

func (m *MockS3Client) CreateBucket(ctx context.Context, bucket string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.buckets[bucket] {
		return errors.New("bucket already exists")
	}
	m.buckets[bucket] = true
	return nil
}

func (m *MockS3Client) DeleteBucket(ctx context.Context, bucket string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.buckets[bucket] {
		return errors.New("bucket does not exist")
	}
	delete(m.buckets, bucket)
	// Optionally, remove all objects in the bucket
	prefix := bucket + "|"
	for k := range m.objects {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			delete(m.objects, k)
		}
	}
	return nil
}
