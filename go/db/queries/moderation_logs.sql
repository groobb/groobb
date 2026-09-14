-- name: CreateModerationLog :one
INSERT INTO moderation_logs (user_id, action, thread_id, post_id, target_user_id, reason)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListModerationLogsPage :many
SELECT * FROM moderation_logs
ORDER BY id DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountModerationLogs :one
SELECT COUNT(*) FROM moderation_logs;
