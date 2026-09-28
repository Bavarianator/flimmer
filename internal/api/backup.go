package api

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Bavarianator/flimmer/internal/db"
)

// backupDownload liefert eine konsistente Kopie der Datenbank (VACUUM INTO) als Download.
func (s *Server) backupDownload(w http.ResponseWriter, r *http.Request) {
	f, err := os.CreateTemp("", "flimmer-backup-*.db")
	if err != nil {
		writeErr(w, err)
		return
	}
	f.Close()
	defer os.Remove(f.Name())
	if err := s.DB.BackupTo(r.Context(), f.Name()); writeErr(w, err) {
		return
	}
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="flimmer-backup-`+time.Now().Format("2006-01-02")+`.db"`)
	http.ServeFile(w, r, f.Name())
}

// restore spielt ein hochgeladenes Backup ein (roher Body, höchstens 4 GB). Danach lädt die Bibliothek neu;
// Sessions kommen aus dem Backup, der Admin meldet sich also ggf. neu an.
func (s *Server) restore(w http.ResponseWriter, r *http.Request) {
	f, err := os.CreateTemp("", "flimmer-restore-*.db")
	if err != nil {
		writeErr(w, err)
		return
	}
	defer os.Remove(f.Name())
	_, err = io.Copy(f, http.MaxBytesReader(w, r.Body, 4<<30))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		http.Error(w, "Upload abgebrochen", http.StatusBadRequest)
		return
	}
	if err := s.DB.Restore(r.Context(), f.Name()); err != nil {
		var bad db.ErrBadBackup
		if errors.As(err, &bad) {
			http.Error(w, bad.Error(), http.StatusBadRequest)
			return
		}
		writeErr(w, err)
		return
	}
	log.Print("Backup eingespielt")
	if err := s.Lib.Load(r.Context()); err != nil {
		log.Printf("Bibliothek neu laden: %v", err)
	}
	if set, err := s.DB.Settings(r.Context()); err == nil {
		s.Lib.SetDirs(set.Dirs)
	}
	w.WriteHeader(http.StatusNoContent)
}
