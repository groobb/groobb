-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 LIMIT 1;

-- name: GetUserByAtname :one
SELECT * FROM users WHERE atname = $1 LIMIT 1;

-- name: GetUserBySessionToken :one
SELECT users.* FROM users
JOIN user_sessions ON user_sessions.user_id = users.id
WHERE user_sessions.token = $1
LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (email, atname, locale, time_zone)
VALUES ($1, $2, $3, $4)
RETURNING *;
