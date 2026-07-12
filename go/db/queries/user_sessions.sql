-- name: GetUserSessionByToken :one
SELECT * FROM user_sessions WHERE token = $1 LIMIT 1;

-- name: CreateUserSession :one
INSERT INTO user_sessions (user_id, token, ip_address, user_agent)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteUserSessionByToken :exec
DELETE FROM user_sessions WHERE token = $1;

-- name: DeleteUserSessionsByUserID :exec
DELETE FROM user_sessions WHERE user_id = $1;
