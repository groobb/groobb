-- name: GetUserPasswordByUserID :one
SELECT * FROM user_passwords WHERE user_id = ? LIMIT 1;

-- name: CreateUserPassword :one
INSERT INTO user_passwords (user_id, password_digest)
VALUES (?, ?)
RETURNING *;

-- name: UpdateUserPasswordDigestByUserID :exec
UPDATE user_passwords
SET password_digest = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE user_id = ?;
