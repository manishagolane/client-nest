CREATE TABLE ticket_attachments (
  id         VARCHAR(26) PRIMARY KEY,
  ticket_id  VARCHAR(26) REFERENCES tickets(id) ON DELETE CASCADE, -- Links attachment to ticket
  file_url   TEXT NULL, -- Stores presigned S3 URL or file path
  uploaded_by VARCHAR(26) NOT NULL, -- Tracks who uploaded the file
  uploader_type TEXT NULL, -- Defines uploader type
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL
);
