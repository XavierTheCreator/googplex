-- Calendar events as last fetched, so the calendar draws instantly on open.
CREATE TABLE cal_events (
	calendar_id TEXT NOT NULL,
	event_id    TEXT NOT NULL,
	start_ms    INTEGER NOT NULL, -- all-day events: local midnight
	end_ms      INTEGER NOT NULL,
	data        TEXT NOT NULL,    -- the calendar.Event as JSON
	PRIMARY KEY (calendar_id, event_id)
);
CREATE INDEX cal_events_range ON cal_events(start_ms, end_ms);
