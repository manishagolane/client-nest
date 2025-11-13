-- name: AddEmployee :one
INSERT INTO employees (id, name, email, password, phone, role_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetEmployeeByEmail :one
SELECT * FROM employees WHERE email = $1;

-- name: ListUEmployees :many
SELECT * FROM employees ORDER BY created_at DESC;

-- name: GetEmployeeById :one
SELECT * FROM employees WHERE id = $1 AND deleted_at IS NULL;

-- name: GetEmployeesByIds :many
SELECT * FROM employees WHERE id = ANY(@id::VARCHAR[]) AND deleted_at IS NULL;

-- name: GetRoleIdById :one
SELECT role_id FROM employees WHERE id = $1;
