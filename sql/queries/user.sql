-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: ResetUsers :exec
TRUNCATE chirps, users;

-- name: GetHashedPass :one
SELECT email, hashed_password, id, created_at, updated_at
FROM users
WHERE email = $1;