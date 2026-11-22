-- Task lists ("lists") and each list's tasks ("list:ID") as last fetched, as JSON.
CREATE TABLE tasks_cache (
	key  TEXT PRIMARY KEY,
	data TEXT NOT NULL
);
