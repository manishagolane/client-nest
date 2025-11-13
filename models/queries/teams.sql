-- name: CreateTeam :one
INSERT INTO teams (id, name, manager_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTeamById :one
SELECT * FROM teams WHERE id = $1;
