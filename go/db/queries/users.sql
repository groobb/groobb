-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 LIMIT 1;

-- name: GetUserBySessionToken :one
SELECT users.* FROM users
JOIN user_sessions ON user_sessions.user_id = users.id
WHERE user_sessions.token = $1
LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (email, locale, time_zone)
VALUES ($1, $2, $3)
RETURNING *;
