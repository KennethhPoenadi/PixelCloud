// Package storage wraps MinIO (S3-compatible) object storage.
package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint       string
	PublicEndpoint string
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseSSL         bool
	PublicUseSSL   bool
	Region         string
}

type Storage struct {
	client *minio.Client
	// signer only computes presigned URLs for the public host; it never makes requests
	// because the region is fixed (no bucket-location lookup).
	signer *minio.Client
	bucket string
}

func New(cfg Config) (*Storage, error) {
	creds := credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")
	client, err := minio.New(cfg.Endpoint, &minio.Options{Creds: creds, Secure: cfg.UseSSL, Region: cfg.Region})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	signer, err := minio.New(cfg.PublicEndpoint, &minio.Options{Creds: creds, Secure: cfg.PublicUseSSL, Region: cfg.Region})
	if err != nil {
		return nil, fmt.Errorf("minio signer: %w", err)
	}
	return &Storage{client: client, signer: signer, bucket: cfg.Bucket}, nil
}

// EnsureBucket creates the (private) bucket if missing. Safe to call from every node.
func (s *Storage) EnsureBucket(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if ok {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		resp := minio.ToErrorResponse(err)
		if resp.Code == "BucketAlreadyOwnedByYou" || resp.Code == "BucketAlreadyExists" {
			return nil
		}
		return fmt.Errorf("make bucket: %w", err)
	}
	return nil
}

func (s *Storage) Ping(ctx context.Context) error {
	_, err := s.client.BucketExists(ctx, s.bucket)
	return err
}

func (s *Storage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

func (s *Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object %s: %w", key, err)
	}
	return obj, nil
}

// Remove deletes objects; missing objects are not an error.
func (s *Storage) Remove(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		if k == "" {
			continue
		}
		if err := s.client.RemoveObject(ctx, s.bucket, k, minio.RemoveObjectOptions{}); err != nil {
			return fmt.Errorf("remove object %s: %w", k, err)
		}
	}
	return nil
}

// PresignGet returns a short-lived URL on the public host. When downloadName is
// set, the browser is told to save the file under that name.
func (s *Storage) PresignGet(ctx context.Context, key string, ttl time.Duration, downloadName string) (string, error) {
	params := url.Values{}
	if downloadName != "" {
		params.Set("response-content-disposition", fmt.Sprintf("attachment; filename=%q", downloadName))
	}
	u, err := s.signer.PresignedGetObject(ctx, s.bucket, key, ttl, params)
	if err != nil {
		return "", fmt.Errorf("presign %s: %w", key, err)
	}
	return u.String(), nil
}
