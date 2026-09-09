-- name: GetRoleByName :one
SELECT * FROM roles WHERE name = ? LIMIT 1;

-- name: ListRolesByUserID :many
SELECT roles.* FROM roles
JOIN user_roles ON user_roles.role_id = roles.id
WHERE user_roles.user_id = ?
ORDER BY roles.id;

-- name: ListRolesByUserIDs :many
SELECT sqlc.embed(roles), user_roles.user_id FROM roles
JOIN user_roles ON user_roles.role_id = roles.id
WHERE user_roles.user_id IN (sqlc.slice('user_ids'))
ORDER BY user_roles.user_id, roles.id;
