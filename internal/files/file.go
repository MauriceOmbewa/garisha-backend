// Package files is the domain module for file upload management.
// It uses a presigned-URL pattern so file bytes never pass through the
// API server:
//
//  1. POST /api/v1/files/presign  → returns a presigned PUT URL + storage key
//  2. Client PUTs the file directly to S3 using that URL
//  3. POST /api/v1/files/confirm  → registers the upload in file_uploads
//
// Download URLs are generated on GET /api/v1/files/{id}/url.
package files

import "time"

// Upload is the file_uploads table entity.
type Upload struct {
	ID          string
	TenantID    string
	UploadedBy  *string

	StorageKey   string
	Bucket       string
	OriginalName string
	MimeType     string
	SizeBytes    int64

	ResourceType *string // "vehicle" | "customer" | "service_job" | "company" | "general"
	ResourceID   *string

	IsActive  bool
	CreatedAt time.Time
}

// PresignResponse is returned by the presign endpoint.
type PresignResponse struct {
	UploadURL  string `json:"upload_url"`   // presigned PUT URL for the client
	StorageKey string `json:"storage_key"`  // key to pass back to /confirm
	ExpiresIn  int    `json:"expires_in"`   // seconds until the URL expires
}

// DownloadURLResponse is returned by the URL endpoint.
type DownloadURLResponse struct {
	URL       string `json:"url"`
	ExpiresIn int    `json:"expires_in"`
}

// AllowedMimeTypes is the set of MIME types the API accepts.
// Reject anything not in this list at the presign step.
var AllowedMimeTypes = map[string]bool{
	"image/jpeg":            true,
	"image/png":             true,
	"image/webp":            true,
	"image/gif":             true,
	"application/pdf":       true,
	"application/msword":    true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"text/csv":              true,
}

// MaxFileSizeBytes is the maximum allowed file size (20 MB).
const MaxFileSizeBytes int64 = 20 * 1024 * 1024
