-- name: GetUserTwoFactorAuthByUserID :one
SELECT * FROM user_two_factor_auths WHERE user_id = ? LIMIT 1;

-- name: GetEnabledUserTwoFactorAuthByUserID :one
SELECT * FROM user_two_factor_auths WHERE user_id = ? AND enabled = TRUE LIMIT 1;

-- name: CreateUserTwoFactorAuth :one
INSERT INTO user_two_factor_auths (user_id, secret)
VALUES (?, ?)
ON CONFLICT (user_id) DO NOTHING
RETURNING *;

-- name: EnableUserTwoFactorAuth :execrows
UPDATE user_two_factor_auths
SET enabled = TRUE,
    enabled_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    recovery_codes = ?,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE user_id = ? AND enabled = FALSE;

-- name: UpdateUserTwoFactorAuthRecoveryCodes :exec
UPDATE user_two_factor_auths
SET recovery_codes = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE user_id = ?;

-- name: DeleteUserTwoFactorAuthByUserID :exec
DELETE FROM user_two_factor_auths WHERE user_id = ?;
