-- name: GetBoardByID :one
SELECT * FROM boards WHERE id = ? LIMIT 1;

-- name: GetBoardBySlug :one
SELECT * FROM boards WHERE slug = ? LIMIT 1;

-- name: ListBoards :many
SELECT * FROM boards ORDER BY position, id;

-- name: ListBoardsByCategoryID :many
SELECT * FROM boards WHERE category_id = ? ORDER BY position, id;

-- name: CreateBoard :one
INSERT INTO boards (category_id, slug, name, description, position)
VALUES (?, ?, ?, ?, ?)
RETURNING *;
