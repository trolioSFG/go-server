-- name: GetUserIDFromRefreshToken :one
SELECT user_id FROM refresh_tokens WHERE token = $1;

