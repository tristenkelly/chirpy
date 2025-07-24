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
SELECT email, hashed_password, id, created_at, updated_at, is_chirpy_red
FROM users
WHERE email = $1;

-- name: ChangePassword :exec
UPDATE users
SET email = $2,
hashed_password = $3,
updated_at = $4
WHERE id = $1;

-- name: UpgradeUser :exec
UPDATE users
SET is_chirpy_red = $2,
updated_at = $3
WHERE id = $1;

-- name: GetUser :exec
SELECT email, id
FROM users
WHERE id = $1;