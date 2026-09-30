package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"time"
)

// stateFile ist das Format der früheren state.json (nur zum einmaligen Import).
type stateFile struct {
	Settings struct {
		ServerName string   `json:"serverName"`
		Language   string   `json:"language"`
		Dirs       []string `json:"dirs"`
		TMDBKey    string   `json:"tmdbKey"`
		NoUpdates  bool     `json:"noUpdates"`
		Secret     []byte   `json:"secret"`
	} `json:"settings"`
	Users []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Color    int    `json:"color"`
		Admin    bool   `json:"admin"`
		PassHash string `json:"passHash"`
	} `json:"users"`
	Sessions map[string]struct {
		UserID   string    `json:"userId"`
		Device   string    `json:"device"`
		LastSeen time.Time `json:"lastSeen"`
	} `json:"sessions"`
	Progress map[string]map[string]struct {
		Pos     float64   `json:"pos"`
		Dur     float64   `json:"dur"`
		Watched bool      `json:"watched"`
		Updated time.Time `json:"updated"`
	} `json:"progress"`
	Prefs   map[string]map[string]TrackPref `json:"prefs"`
	Devices map[string]json.RawMessage      `json:"devices"`
}

// ImportState übernimmt eine state.json einmalig und benennt sie in state.json.migrated um (nie löschen).
// Ohne Datei passiert nichts; hat die Datenbank schon Benutzer, wird nicht importiert.
func (d *DB) ImportState(ctx context.Context, path string) (bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var st stateFile
	if err := json.Unmarshal(b, &st); err != nil {
		return false, err
	}
	if d.SetupDone(ctx) {
		return false, nil
	}
	err = d.tx(ctx, func(tx *sql.Tx) error {
		s := Settings{ServerName: st.Settings.ServerName, Language: st.Settings.Language, Dirs: st.Settings.Dirs,
			TMDBKey: st.Settings.TMDBKey, NoUpdates: st.Settings.NoUpdates, Secret: st.Settings.Secret}
		if len(s.Secret) == 0 {
			old, err := settings(ctx, tx)
			if err != nil {
				return err
			}
			s.Secret = old.Secret
		}
		if err := writeSettings(ctx, tx, s); err != nil {
			return err
		}
		for _, u := range st.Users {
			if err := insertUser(ctx, tx, User{ID: u.ID, Name: u.Name, Color: u.Color, Admin: u.Admin, PassHash: u.PassHash}); err != nil {
				return err
			}
		}
		for h, s := range st.Sessions {
			if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO sessions(hash, user_id, device, last_seen) SELECT ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM users WHERE id = ?)",
				h, s.UserID, s.Device, s.LastSeen.UnixMilli(), s.UserID); err != nil {
				return err
			}
		}
		for uid, items := range st.Progress {
			for item, p := range items {
				if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO progress(user_id, item_id, pos, duration, watched, updated_at) SELECT ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM users WHERE id = ?)",
					uid, item, p.Pos, p.Dur, b2i(p.Watched), p.Updated.UnixMilli(), uid); err != nil {
					return err
				}
			}
		}
		for uid, series := range st.Prefs {
			for name, p := range series {
				if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO series_prefs(user_id, series, audio, subtitle) SELECT ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM users WHERE id = ?)",
					uid, name, p.Audio, p.Subtitle, uid); err != nil {
					return err
				}
			}
		}
		for id, prof := range st.Devices {
			if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO devices(id, profile, updated_at) VALUES(?, ?, ?)", id, string(prof), now()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return true, os.Rename(path, path+".migrated")
}
