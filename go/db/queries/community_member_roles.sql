-- name: CreateCommunityMemberRole :one
INSERT INTO community_member_roles (community_id, community_member_id, community_role_id)
VALUES ($1, $2, $3)
RETURNING *;
