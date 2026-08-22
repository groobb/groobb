-- name: GetUserSessionByToken :one
SELECT * FROM user_sessions WHERE token = ? LIMIT 1;

-- name: CreateUserSession :one
INSERT INTO user_sessions (user_id, token, ip_address, user_agent)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: DeleteUserSessionByToken :exec
DELETE FROM user_sessions WHERE token = ?;

-- name: DeleteUserSessionsByUserID :exec
DELETE FROM user_sessions WHERE user_id = ?;
