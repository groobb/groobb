-- name: CreateEmailConfirmation :one
INSERT INTO email_confirmations (email, event, code)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetActiveEmailConfirmationByID :one
SELECT * FROM email_confirmations
WHERE id = $1
  AND succeeded_at IS NULL
  AND started_at > NOW() - INTERVAL '15 minutes'
  AND failed_attempts_count < 5
LIMIT 1;

-- name: GetSucceededEmailConfirmationByID :one
SELECT * FROM email_confirmations
WHERE id = $1
  AND succeeded_at IS NOT NULL
LIMIT 1;

-- name: UpdateEmailConfirmationSucceededAt :exec
UPDATE email_confirmations
SET succeeded_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: IncrementEmailConfirmationFailedAttempts :exec
UPDATE email_confirmations
SET failed_attempts_count = failed_attempts_count + 1, updated_at = NOW()
WHERE id = $1;

-- name: CreateEmailChangeConfirmation :one
INSERT INTO email_confirmations (user_id, email, event, code)
VALUES ($1, $2, 'email_change', $3)
RETURNING *;

-- name: GetActiveEmailChangeConfirmationByUserID :one
SELECT * FROM email_confirmations
WHERE user_id = $1
  AND event = 'email_change'
  AND succeeded_at IS NULL
  AND started_at > NOW() - INTERVAL '15 minutes'
  AND failed_attempts_count < 5
ORDER BY started_at DESC
LIMIT 1;

-- name: DeleteUnusedEmailChangeConfirmationsByUserID :exec
DELETE FROM email_confirmations
WHERE user_id = $1
  AND event = 'email_change'
  AND succeeded_at IS NULL;
