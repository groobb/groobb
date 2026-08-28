-- name: GetCategoryByID :one
SELECT * FROM categories WHERE id = ? LIMIT 1;

-- name: GetCategoryBySlug :one
SELECT * FROM categories WHERE slug = ? LIMIT 1;

-- name: CreateCategory :one
INSERT INTO categories (slug, name, position)
VALUES (?, ?, ?)
RETURNING *;
