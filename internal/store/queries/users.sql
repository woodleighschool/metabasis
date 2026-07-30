-- name: UpsertUser :one
INSERT INTO users (id, upn, display_name, staff)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id)
DO UPDATE SET
	upn = EXCLUDED.upn,
	display_name = EXCLUDED.display_name
RETURNING *;

-- name: GetUsers :many
SELECT id, upn, display_name, staff
FROM users;

-- name: GetUser :one
SELECT id, upn, display_name, staff
FROM users
WHERE id = $1;

-- name: GetUserByUPN :one
SELECT id, upn, display_name, staff
FROM users
WHERE upn = $1;

-- name: DeleteUser :exec
DELETE
FROM users
WHERE id = $1;

-- name: DeleteUserByUPN :exec
DELETE
FROM users
WHERE upn = $1;
