------
-- Users
------

CREATE TABLE IF NOT EXISTS users (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	upn TEXT NOT NULL UNIQUE,
	display_name TEXT NOT NULL,
	staff bool DEFAULT true
);

-----
-- Schedules
-----

CREATE TABLE IF NOT EXISTS schedules (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	userID uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	leaving_date TIMESTAMPTZ NOT NULL,
	returning_date TIMESTAMPTZ NOT NULL,
	overseas BOOL NOT NULL DEFAULT false,
	last_changed_by TEXT NOT NULL DEFAULT 'SYSTEM',
	last_changed TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
