-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByAtname :one
SELECT * FROM users WHERE atname = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetUserBySessionToken :one
SELECT users.* FROM users
JOIN user_sessions ON user_sessions.user_id = users.id
WHERE user_sessions.token = $1 AND users.deleted_at IS NULL
LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (email, atname, locale, time_zone)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateUserEmail :exec
UPDATE users
SET email = $2, updated_at = NOW()
WHERE id = $1;

-- name: SoftDeleteAndAnonymizeUser :exec
UPDATE users
SET deleted_at = NOW(), email = $2, atname = $3, updated_at = NOW()
WHERE id = $1;

-- name: PurgeUsersDeletedBefore :execrows
DELETE FROM users
WHERE deleted_at IS NOT NULL AND deleted_at < $1;
