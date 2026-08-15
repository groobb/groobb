-- name: CreateCommunityMember :one
INSERT INTO community_members (community_id, user_id)
VALUES ($1, $2)
RETURNING *;
