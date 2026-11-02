// Package storage abstracts the S3-compatible object store holding photos.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var ErrNotFound = errors.New("object not found")

type Storage interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	// SignedURL returns a short-lived URL for a private object.
	SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
	// PublicURL returns the CDN URL for key, or "" if no public base is configured.
	PublicURL(key string) string
	Ping(ctx context.Context) error
}

type S3Config struct {
	Endpoint      string
	Bucket        string
	AccessKey     string
	SecretKey     string
	Region        string
	UseSSL        bool
	PublicBaseURL string
	// PublicEndpoint is the endpoint browsers use (signed URLs are bound to
	// the host). Defaults to Endpoint.
	PublicEndpoint string
}

type S3 struct {
	client *minio.Client
	signer *minio.Client
	cfg    S3Config
}

func newClient(rawEndpoint string, cfg S3Config) (*minio.Client, error) {
	endpoint, useSSL := rawEndpoint, cfg.UseSSL
	if u, err := url.Parse(rawEndpoint); err == nil && u.Host != "" {
		endpoint = u.Host
		useSSL = u.Scheme == "https"
	}
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: useSSL,
		Region: cfg.Region, // set explicitly: presigning never calls the network
	})
}

func NewS3(cfg S3Config) (*S3, error) {
	c, err := newClient(cfg.Endpoint, cfg)
	if err != nil {
		return nil, err
	}
	signer := c
	if cfg.PublicEndpoint != "" && cfg.PublicEndpoint != cfg.Endpoint {
		if signer, err = newClient(cfg.PublicEndpoint, cfg); err != nil {
			return nil, err
		}
	}
	return &S3{client: c, signer: signer, cfg: cfg}, nil
}

func (s *S3) Put(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, s.cfg.Bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType:  contentType,
		CacheControl: "public, max-age=31536000, immutable",
	})
	return err
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.cfg.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(obj); err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.cfg.Bucket, key, minio.RemoveObjectOptions{})
}

func (s *S3) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := s.signer.PresignedGetObject(ctx, s.cfg.Bucket, key, ttl, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *S3) PublicURL(key string) string {
	if s.cfg.PublicBaseURL == "" {
		return ""
	}
	return strings.TrimRight(s.cfg.PublicBaseURL, "/") + "/" + key
}

func (s *S3) Ping(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.cfg.Bucket)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("bucket %q does not exist", s.cfg.Bucket)
	}
	return nil
}

// Memory is an in-memory Storage used by tests and by the dev mode without MinIO.
type Memory struct {
	mu      sync.Mutex
	objects map[string][]byte
	BaseURL string
}

func NewMemory() *Memory {
	return &Memory{objects: map[string][]byte{}, BaseURL: "https://photos.test"}
}

func (m *Memory) Put(_ context.Context, key string, data []byte, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = append([]byte(nil), data...)
	return nil
}

func (m *Memory) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.objects[key]
	if !ok {
		return nil, ErrNotFound
	}
	return d, nil
}

func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

func (m *Memory) SignedURL(_ context.Context, key string, ttl time.Duration) (string, error) {
	return fmt.Sprintf("%s/%s?signed=1&ttl=%d", m.BaseURL, key, int(ttl.Seconds())), nil
}

func (m *Memory) PublicURL(key string) string { return m.BaseURL + "/" + key }

func (m *Memory) Ping(context.Context) error { return nil }

func (m *Memory) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.objects)
}
