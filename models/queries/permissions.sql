-- name: CreatePermission :one
INSERT INTO permissions (id, name, object, action) VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPermissionByName :one
SELECT * FROM permissions WHERE name = $1;

-- name: GetAllPermissions :many
SELECT * FROM permissions ORDER BY id ASC;

-- name: InsertPermissions :one
INSERT INTO permissions (id, name, object, action) VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPermissionsByUserID :many
SELECT p.object, p.action 
FROM permissions p
JOIN role_permissions rp ON p.id = rp.id
JOIN roles r ON rp.id = r.role_id
JOIN employees e ON e.role_id = r.role_id
WHERE e.id = $1;

-- name: HasPermission :one
SELECT EXISTS (
  SELECT 1 FROM role_permissions rp
  JOIN permissions p ON rp.permission_id = p.id
  WHERE rp.role_id = $1
  AND p.object = $2 
  AND p.action = $3
);

