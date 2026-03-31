-- name: GetAllTodos :many
SELECT * FROM todos
WHERE user_id = ?
ORDER BY created_at DESC;

-- name: GetTodoByID :one
SELECT * FROM todos
WHERE id = ? AND user_id = ?
LIMIT 1;

-- name: CreateTodo :one
INSERT INTO todos (id, user_id, text, completed, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateTodoCompleted :exec
UPDATE todos
SET completed = ?
WHERE id = ? AND user_id = ?;

-- name: DeleteTodo :exec
DELETE FROM todos
WHERE id = ? AND user_id = ?;

-- name: DeleteCompletedTodos :exec
DELETE FROM todos
WHERE completed = 1 AND user_id = ?;

-- name: CountTodos :one
SELECT COUNT(*) FROM todos
WHERE user_id = ?;

-- name: CountCompletedTodos :one
SELECT COUNT(*) FROM todos
WHERE completed = 1 AND user_id = ?;
