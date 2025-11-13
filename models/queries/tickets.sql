-- name: CreateTicket :one
INSERT INTO tickets (id, created_by, created_by_type,assigned_to, team_id, category, priority, status, tags, response_time, watchers)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetTicketByID :one
SELECT * FROM tickets WHERE id = $1;

-- name: ListTickets :many
SELECT * FROM tickets ORDER BY created_at DESC;

-- -- name: UpdateTicketMedia :exec
-- UPDATE tickets SET image_url = $1, updated_at = Now() WHERE id = $2;

-- name: GetLastTicketForUser :one
SELECT id 
FROM tickets 
WHERE created_by = $1 
ORDER BY created_at DESC 
LIMIT 1;

-- name: UpdateTicketAssignment :exec
UPDATE tickets
SET assigned_to = $1,
    team_id = $2,
    updated_at = Now()
WHERE id = $3;

-- name: UpdateTicketStatus :exec
UPDATE tickets
SET status = $1,
    updated_at = Now()
WHERE id = $2;

-- name: UpdateWachersList :exec
UPDATE tickets 
SET watchers = $1, 
    updated_at = NOW() 
WHERE id = $2;



-- name: GetAdminIDs :many
SELECT id FROM employees WHERE role_id = (SELECT id FROM roles WHERE name = 'admin');

-- name: GetWatchersEmailsAndRoles :many
SELECT 
    COALESCE(c.email, e.email) AS email,
    CASE 
        WHEN c.id IS NOT NULL THEN 'customer'
        WHEN e.id IS NOT NULL THEN r.name 
    END AS role
FROM tickets t,
LATERAL jsonb_array_elements_text(t.watchers) AS watcher_id
LEFT JOIN customers c ON c.id = watcher_id
LEFT JOIN employees e ON e.id = watcher_id
LEFT JOIN roles r ON r.id = e.role_id
WHERE t.id = $1;

-- name: DeleteTicketByID :exec
UPDATE tickets SET deleted_at = NOW() WHERE id = $1;

-- name: GetRecipientsEmailsAndRoles :many
SELECT 
    COALESCE(c.email, e.email) AS email,
    CASE 
        WHEN c.id IS NOT NULL THEN 'customer'
        WHEN e.id IS NOT NULL THEN r.name 
    END AS role
FROM (
    SELECT unnest($1::text[]) AS recipient_id  
) AS recipients
LEFT JOIN customers c ON c.id = recipients.recipient_id
LEFT JOIN employees e ON e.id = recipients.recipient_id
LEFT JOIN roles r ON r.id = e.role_id;
