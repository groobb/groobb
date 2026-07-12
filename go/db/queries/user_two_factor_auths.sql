-- name: GetUserTwoFactorAuthByUserID :one
SELECT * FROM user_two_factor_auths WHERE user_id = $1 LIMIT 1;

-- name: GetEnabledUserTwoFactorAuthByUserID :one
SELECT * FROM user_two_factor_auths WHERE user_id = $1 AND enabled = true LIMIT 1;

-- name: CreateUserTwoFactorAuth :one
INSERT INTO user_two_factor_auths (user_id, secret)
VALUES ($1, $2)
ON CONFLICT (user_id) DO NOTHING
RETURNING *;

-- name: EnableUserTwoFactorAuth :execrows
UPDATE user_two_factor_auths
SET enabled = true, enabled_at = NOW(), recovery_codes = $2, updated_at = NOW()
WHERE user_id = $1 AND enabled = false;

-- name: UpdateUserTwoFactorAuthRecoveryCodes :exec
UPDATE user_two_factor_auths
SET recovery_codes = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: DeleteUserTwoFactorAuthByUserID :exec
DELETE FROM user_two_factor_auths WHERE user_id = $1;
