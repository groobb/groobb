-- name: ListPostReferencesByReferencedPostIDs :many
SELECT * FROM post_references
WHERE referenced_post_id IN (sqlc.slice('referenced_post_ids'))
ORDER BY referenced_post_id, post_id;

-- name: CreatePostReference :one
INSERT INTO post_references (post_id, referenced_post_id)
VALUES (?, ?)
RETURNING *;

-- name: CreatePostReferencesByNumbers :exec
INSERT INTO post_references (post_id, referenced_post_id)
SELECT sqlc.arg('post_id'), referenced.id
FROM posts AS referenced
WHERE referenced.thread_id = sqlc.arg('thread_id')
  AND referenced.number < sqlc.arg('number')
  AND referenced.number IN (sqlc.slice('numbers'));
