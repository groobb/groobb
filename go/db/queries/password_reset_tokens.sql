-- name: CreatePasswordResetToken :one
INSERT INTO password_reset_tokens (user_id, token_digest, expires_at)
VALUES (?, ?, ?)
RETURNING *;

-- name: DeleteUnusedPasswordResetTokensByUserID :exec
DELETE FROM password_reset_tokens
WHERE user_id = ?
  AND used_at IS NULL;

-- name: GetPasswordResetTokenByDigest :one
SELECT * FROM password_reset_tokens WHERE token_digest = ? LIMIT 1;

-- name: MarkPasswordResetTokenAsUsed :exec
UPDATE password_reset_tokens
SET used_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;
