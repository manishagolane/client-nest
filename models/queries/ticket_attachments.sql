-- name: InsertTicketAttachment :one
INSERT INTO ticket_attachments (id, ticket_id, file_url, uploaded_by, uploader_type)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateTickeAttachmentFileUrl :exec
UPDATE ticket_attachments
SET file_url = $1,
    updated_at = Now()
WHERE id = $2;

-- name: GetTicketAttachmentByID :one
SELECT * FROM ticket_attachments WHERE id = $1;
