-- name: ListPostsByThreadID :many
SELECT * FROM posts
WHERE thread_id = ?
ORDER BY number;

-- name: CreatePost :one
INSERT INTO posts (thread_id, user_id, number, body)
VALUES (?, ?, ?, ?)
RETURNING *;
