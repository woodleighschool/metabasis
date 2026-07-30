-- name: UpsertUserAsset :one
INSERT INTO user_assets (userID, content_type, data)
VALUES ($1, $2, $3)
ON CONFLICT (userID)
DO UPDATE SET
	content_type = EXCLUDED.content_type,
	data = EXCLUDED.data,
	updated_at = NOW()
RETURNING *;

-- name: GetUserAsset :one
SELECT *
FROM user_assets
WHERE userID = $1;