-- name: ListPostReferencesByReferencedPostIDs :many
SELECT * FROM post_references
WHERE referenced_post_id IN (sqlc.slice('referenced_post_ids'))
ORDER BY referenced_post_id, post_id;

-- name: CreatePostReference :one
INSERT INTO post_references (post_id, referenced_post_id)
VALUES (?, ?)
RETURNING *;
