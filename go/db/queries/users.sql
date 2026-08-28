-- name: GetUserByID :one
SELECT * FROM users WHERE id = ? AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ? AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByAtname :one
SELECT * FROM users WHERE atname = ? AND deleted_at IS NULL LIMIT 1;

-- name: ListUsersByIDs :many
SELECT * FROM users
WHERE id IN (sqlc.slice('ids')) AND deleted_at IS NULL
ORDER BY id;

-- name: GetUserBySessionToken :one
SELECT users.* FROM users
JOIN user_sessions ON user_sessions.user_id = users.id
WHERE user_sessions.token = ? AND users.deleted_at IS NULL
LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (email, atname, locale, time_zone)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: UpdateUserEmail :exec
UPDATE users
SET email = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: SoftDeleteAndAnonymizeUser :exec
UPDATE users
SET deleted_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    email = ?,
    atname = ?,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: PurgeUsersDeletedBefore :execrows
DELETE FROM users
WHERE deleted_at IS NOT NULL AND deleted_at < ?;
