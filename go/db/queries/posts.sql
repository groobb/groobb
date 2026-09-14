-- name: ListPostsByThreadID :many
SELECT * FROM posts
WHERE thread_id = ?
ORDER BY number;

-- name: GetPostByThreadIDAndNumber :one
SELECT * FROM posts
WHERE thread_id = ? AND number = ?
LIMIT 1;

-- name: ListPostsByIDs :many
SELECT * FROM posts
WHERE id IN (sqlc.slice('ids'))
ORDER BY id;

-- name: CreatePost :one
INSERT INTO posts (thread_id, user_id, number, body)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetLatestPostByUserID :one
SELECT * FROM posts
WHERE user_id = ?
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: UnpublishPost :exec
UPDATE posts
SET unpublished_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;
