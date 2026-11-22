package drive

import (
	"database/sql"
	"encoding/json"
)

// CacheKey names a listing: "folder:ID", "shared", "recent", …
func CacheKey(view, folderID string) string {
	if view == ViewFolder || view == "" {
		if folderID == "" {
			folderID = "root"
		}
		return "folder:" + folderID
	}
	return view
}

// SaveListing replaces the cached first page of a listing.
func SaveListing(conn *sql.DB, key string, files []File) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM drive_items WHERE listing = ?", key); err != nil {
		return err
	}
	for i, f := range files {
		data, _ := json.Marshal(f)
		if _, err := tx.Exec("INSERT INTO drive_items (listing, position, id, data) VALUES (?, ?, ?, ?)", key, i, f.ID, string(data)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CachedListing returns a listing as last saved.
func CachedListing(conn *sql.DB, key string) ([]File, error) {
	rows, err := conn.Query("SELECT data FROM drive_items WHERE listing = ? ORDER BY position", key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := []File{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var f File
		if json.Unmarshal([]byte(data), &f) == nil {
			files = append(files, f)
		}
	}
	return files, rows.Err()
}
