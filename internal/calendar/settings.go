package calendar

import (
	"database/sql"
	"encoding/json"

	"googleplex/internal/db"
)

// The calendar list and which calendars Googleplex shows live in settings:
// cal_list (the last list from Google) and cal_visible ({id: shown}). A
// calendar never toggled here follows its "selected" flag from Google.

// ApplyVisibility sets Visible on each calendar and caches the list.
func ApplyVisibility(conn *sql.DB, cals []Calendar) []Calendar {
	vis := visibility(conn)
	for i := range cals {
		if v, ok := vis[cals[i].ID]; ok {
			cals[i].Visible = v
		} else {
			cals[i].Visible = cals[i].Selected || cals[i].Primary
		}
	}
	if b, err := json.Marshal(cals); err == nil {
		db.SetSetting(conn, "cal_list", string(b))
	}
	return cals
}

// CachedCalendars is the list as last fetched, with visibility applied.
func CachedCalendars(conn *sql.DB) []Calendar {
	var cals []Calendar
	json.Unmarshal([]byte(db.GetSetting(conn, "cal_list")), &cals)
	vis := visibility(conn)
	for i := range cals {
		if v, ok := vis[cals[i].ID]; ok {
			cals[i].Visible = v
		}
	}
	return cals
}

// SetVisible shows or hides one calendar in Googleplex.
func SetVisible(conn *sql.DB, id string, visible bool) error {
	vis := visibility(conn)
	vis[id] = visible
	b, _ := json.Marshal(vis)
	return db.SetSetting(conn, "cal_visible", string(b))
}

// VisibleIDs lists the calendars currently shown.
func VisibleIDs(cals []Calendar) []string {
	ids := []string{}
	for _, c := range cals {
		if c.Visible {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

func visibility(conn *sql.DB) map[string]bool {
	vis := map[string]bool{}
	json.Unmarshal([]byte(db.GetSetting(conn, "cal_visible")), &vis)
	return vis
}
