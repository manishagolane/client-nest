-- name: CreateRole :one
INSERT INTO roles (id, name) VALUES ($1, $2)
RETURNING *;

-- name: GetRoleByName :one
SELECT * FROM roles WHERE name = $1;

-- name: GetAllRoles :many
SELECT * FROM roles ORDER BY id ASC;

-- name: GetRoleById :one
SELECT * FROM roles WHERE id = $1;
