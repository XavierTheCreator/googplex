-- The last listing of each Drive folder or view, so Drive draws instantly.
CREATE TABLE drive_items (
	listing  TEXT NOT NULL,    -- "folder:ID", "shared", "recent", "starred", "trash"
	position INTEGER NOT NULL,
	id       TEXT NOT NULL,
	data     TEXT NOT NULL,    -- the drive.File as JSON
	PRIMARY KEY (listing, position)
);
