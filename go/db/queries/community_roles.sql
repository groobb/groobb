-- name: CreateCommunityRole :one
INSERT INTO community_roles (community_id, name)
VALUES ($1, $2)
RETURNING *;
