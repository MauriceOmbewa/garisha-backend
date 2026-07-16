// Package storage provides an S3-compatible object storage client.
// It works with AWS S3, MinIO, Cloudflare R2, and DigitalOcean Spaces.
//
// The upload flow uses presigned URLs to avoid routing file bytes through
// the API server:
//
//  1. Client calls POST /api/v1/files/presign to get a presigned PUT URL.
//  2. Client uploads the file directly to S3 using that URL.
//  3. Client calls POST /api/v1/files/confirm with the storage key to
//     register the upload in the file_uploads table.
//
// Download URLs are also presigned (or a public CDN URL if configured).
package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client wraps the S3 client with the app's storage configuration.
type Client struct {
	s3     *s3.Client
	pre    *s3.PresignClient
	bucket string
	pubURL string // public CDN base URL (optional)
}

// Config holds the credentials and settings for the storage client.
type Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	PublicBaseURL   string
}

// New creates a Client.  If AccessKeyID is empty the client is in no-op
// mode — the app starts without storage configured.
func New(cfg Config) (*Client, error) {
	if cfg.AccessKeyID == "" {
		return &Client{bucket: cfg.Bucket, pubURL: cfg.PublicBaseURL}, nil
	}

	scheme := "https"
	if !cfg.UseSSL {
		scheme = "http"
	}

	customEndpoint := fmt.Sprintf("%s://%s", scheme, cfg.Endpoint)

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		),
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("storage: load aws config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(customEndpoint)
		// Required for MinIO and other path-style providers.
		o.UsePathStyle = true
	})

	return &Client{
		s3:     s3Client,
		pre:    s3.NewPresignClient(s3Client),
		bucket: cfg.Bucket,
		pubURL: cfg.PublicBaseURL,
	}, nil
}

// Enabled reports whether the client has credentials configured.
func (c *Client) Enabled() bool {
	return c.s3 != nil
}

// ── Presigned PUT (upload) ────────────────────────────────────────────────────

// PresignUpload generates a presigned PUT URL that allows the client to upload
// a file directly to S3 without routing bytes through the API server.
// The URL expires after ttl (typically 15 minutes).
func (c *Client) PresignUpload(ctx context.Context, key, mimeType string, ttl time.Duration) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("storage: client not configured")
	}

	req, err := c.pre.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(mimeType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("storage: presign upload for key %q: %w", key, err)
	}

	return req.URL, nil
}

// ── Presigned GET (download) ──────────────────────────────────────────────────

// PresignDownload generates a presigned GET URL for a private object.
// If a public CDN base URL is configured and the object is public, use
// PublicURL instead to avoid presign overhead.
func (c *Client) PresignDownload(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("storage: client not configured")
	}

	req, err := c.pre.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("storage: presign download for key %q: %w", key, err)
	}

	return req.URL, nil
}

// PublicURL builds a public CDN URL for a key (no presigning).
// Only valid when the bucket/object is publicly accessible.
func (c *Client) PublicURL(key string) string {
	if c.pubURL == "" {
		return ""
	}
	return c.pubURL + "/" + key
}

// ── Delete ────────────────────────────────────────────────────────────────────

// Delete removes an object from the bucket.
func (c *Client) Delete(ctx context.Context, key string) error {
	if !c.Enabled() {
		return fmt.Errorf("storage: client not configured")
	}

	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("storage: delete key %q: %w", key, err)
	}

	return nil
}

// Bucket returns the configured bucket name.
func (c *Client) Bucket() string {
	return c.bucket
}
