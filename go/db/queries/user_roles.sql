-- name: CreateUserRole :one
INSERT INTO user_roles (user_id, role_id)
VALUES (?, ?)
RETURNING *;

-- name: ListUserRolesByUserIDs :many
SELECT * FROM user_roles
WHERE user_id IN (sqlc.slice('user_ids'))
ORDER BY user_id, role_id;

-- name: CountUserRoleHoldersByRoleID :one
SELECT COUNT(*) FROM user_roles
JOIN users ON users.id = user_roles.user_id
WHERE user_roles.role_id = ? AND users.deleted_at IS NULL;

-- name: DeleteUserRoleByUserIDAndRoleID :exec
DELETE FROM user_roles WHERE user_id = ? AND role_id = ?;

-- name: DeleteUserRolesByUserID :exec
DELETE FROM user_roles WHERE user_id = ?;
