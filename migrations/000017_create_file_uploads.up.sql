-- ---------------------------------------------------------------------------
-- file_uploads
-- Tracks every file stored in object storage.  The actual bytes live in S3
-- (or compatible storage); this table holds the metadata and references
-- needed to generate presigned URLs, enforce tenant scoping, and audit
-- file access.
-- ---------------------------------------------------------------------------
CREATE TABLE file_uploads (
    id           UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id    UUID          NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- Who uploaded it
    uploaded_by  UUID          REFERENCES users(id) ON DELETE SET NULL,

    -- Object storage key (path within the bucket, e.g. "tenants/{tid}/vehicles/{vid}/image1.jpg")
    storage_key  VARCHAR(1000) NOT NULL,
    bucket       VARCHAR(255)  NOT NULL,

    -- File metadata
    original_name  VARCHAR(500)  NOT NULL,   -- original filename from the client
    mime_type      VARCHAR(100)  NOT NULL,
    size_bytes     BIGINT        NOT NULL,

    -- Classification
    resource_type  VARCHAR(50),  -- 'vehicle' | 'customer' | 'service_job' | 'company' | 'general'
    resource_id    UUID,         -- UUID of the associated record

    -- Lifecycle
    is_active      BOOLEAN       NOT NULL DEFAULT TRUE,

    -- Audit
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_file_upload_size CHECK (size_bytes > 0)
);

CREATE INDEX idx_file_uploads_tenant_id    ON file_uploads (tenant_id);
CREATE INDEX idx_file_uploads_resource     ON file_uploads (tenant_id, resource_type, resource_id);
CREATE INDEX idx_file_uploads_uploaded_by  ON file_uploads (uploaded_by) WHERE uploaded_by IS NOT NULL;
CREATE INDEX idx_file_uploads_storage_key  ON file_uploads (bucket, storage_key);
