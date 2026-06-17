-- name: CreatePasswordResetToken :one
INSERT INTO password_reset_tokens (user_id, token_digest, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DeleteUnusedPasswordResetTokensByUserID :exec
DELETE FROM password_reset_tokens
WHERE user_id = $1
  AND used_at IS NULL;

-- name: GetPasswordResetTokenByDigest :one
SELECT * FROM password_reset_tokens WHERE token_digest = $1 LIMIT 1;

-- name: MarkPasswordResetTokenAsUsed :exec
UPDATE password_reset_tokens
SET used_at = NOW(), updated_at = NOW()
WHERE id = $1;
