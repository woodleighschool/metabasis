-- name: InsertSchedule :one
INSERT INTO schedules (userID, leaving_date, returning_date) 
VALUES ($1, $2, $3) 
RETURNING *;

-- name: FlipSchedule :exec
UPDATE schedules
SET overseas = true
WHERE id = $1;

-- name: UpdateSchedule :exec
UPDATE schedules
SET 
	leaving_date = $2, 
	returning_date = $3, 
	overseas = $4, 
	last_changed_by = $5, 
	last_changed = NOW()
WHERE id = $1;

-- name: GetSchdeule :one
SELECT * from schedules WHERE ID = $1;

-- name: ListScheduleSummaries :many
SELECT 
	schedules.id, 
	schedules.userID, 
	users.display_name, 
	users.upn, 
	schedules.leaving_date, 
	schedules.returning_date, 
	schedules.overseas, 
	schedules.last_changed_by, 
	schedules.last_changed 
FROM schedules 
INNER JOIN users ON schedules.userID = users.id;

-- name: DeleteSchedule :exec
DELETE FROM schedules WHERE id = $1;