-- name: GetThreadByID :one
SELECT * FROM threads WHERE id = ? LIMIT 1;

-- name: ListThreadsByIDs :many
SELECT * FROM threads
WHERE id IN (sqlc.slice('ids'))
ORDER BY id;

-- name: ListThreadsByBoardID :many
SELECT * FROM threads
WHERE board_id = ? AND unpublished_at IS NULL
ORDER BY last_posted_at DESC, id DESC;

-- name: ListRecentThreadsPerBoard :many
SELECT threads.* FROM boards
JOIN threads ON threads.id IN (
    SELECT recent.id FROM threads AS recent
    WHERE recent.board_id = boards.id AND recent.unpublished_at IS NULL
    ORDER BY recent.last_posted_at DESC, recent.id DESC
    LIMIT sqlc.arg(per_board)
)
ORDER BY boards.position, boards.id, threads.last_posted_at DESC, threads.id DESC;

-- name: CreateThread :one
INSERT INTO threads (board_id, user_id, title, language)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: UpdateThreadLastPost :exec
UPDATE threads
SET posts_count = ?,
    last_post_id = ?,
    last_posted_at = ?,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: LockThread :exec
UPDATE threads
SET locked_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: UnlockThread :exec
UPDATE threads
SET locked_at = NULL,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: UnpublishThread :exec
UPDATE threads
SET unpublished_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;
