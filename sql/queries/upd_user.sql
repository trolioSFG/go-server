-- name: UpdateUser :one
UPDATE USERS
SET email = $1, hashed_password = $2, updated_at = NOW()
WHERE ID = $3
RETURNING *; 
