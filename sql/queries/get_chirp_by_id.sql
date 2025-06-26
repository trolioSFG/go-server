-- name: GetChirpByID :one
SELECT * FROM chirps WHERE ID = $1;

