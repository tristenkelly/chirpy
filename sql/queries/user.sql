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
TRUNCATE refresh_tokens, chirps, users;

-- name: GetHashedPass :one
SELECT email, hashed_password, id, created_at, updated_at
FROM users
WHERE email = $1;

-- name: ChangePassword :exec
UPDATE users
SET email = $2,
hashed_password = $3,
updated_at = $4
WHERE id = $1;