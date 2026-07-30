-- name: InsertUrgentSchedule :one
INSERT INTO urgent_schedules (userID) 
VALUES ($1) 
RETURNING *;

-- name: ListUrgentSchedules :many
SELECT * FROM urgent_schedules
INNER JOIN users ON urgent_schedules.userID = users.id;

-- name: DeleteUrgentSchedule :exec
DELETE FROM urgent_schedules WHERE id = $1;