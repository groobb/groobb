-- name: GetCommunityByIdentifier :one
SELECT * FROM communities WHERE identifier = $1 LIMIT 1;

-- name: CreateCommunity :one
INSERT INTO communities (name, identifier)
VALUES ($1, $2)
RETURNING *;
