------
-- Users
------

CREATE TABLE IF NOT EXISTS users (
	id 					UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	upn 				TEXT NOT NULL UNIQUE,
	display_name 		TEXT NOT NULL,
	staff 				BOOL DEFAULT true
);

CREATE TABLE IF NOT EXISTS user_assets (
	userID 				UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
	content_type 		TEXT NOT NULL,
	data 				BYTEA NOT NULL,
	created_at   		TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at   		TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-----
-- Schedules
-----

CREATE TABLE IF NOT EXISTS schedules (
	id 					UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	userID 				UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	leaving_date 		TIMESTAMPTZ NOT NULL,
	returning_date 		TIMESTAMPTZ NOT NULL,
	overseas 			BOOL NOT NULL DEFAULT false,
	last_changed_by 	TEXT NOT NULL DEFAULT 'SYSTEM',
	last_changed 		TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS urgent_schedules (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	userID UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);