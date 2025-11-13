-- name: AddCustomer :one
INSERT INTO customers (id, name, email, password, phone)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetCustomerByEmail :one
SELECT * FROM customers WHERE email = $1;

-- name: ListCustomers :many
SELECT * FROM customers ORDER BY created_at DESC;

-- name: GetCustomerById :one
SELECT * FROM customers WHERE id = $1 AND deleted_at IS NULL;

-- name: GetCustomersByIds :many
SELECT * FROM customers WHERE id = ANY(@id::VARCHAR[]) AND deleted_at IS NULL;

