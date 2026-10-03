package api

import (
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Bavarianator/flimmer/internal/scan"
)

const maxUpload = 64 << 30 // 64 GiB je Datei

// upload: POST /api/upload?name=<Dateiname>, die Datei ist der Body (kein Multipart: streamt ohne RAM-Puffer).
// Nur mit Recht „Hochladen“ (Admins immer), nie für Gäste. Ziel ist UploadDir – nie die Medienordner des Admins,
// die können anderen Diensten gehören. Erst .part, am Ende umbenannt: der Scan sieht keine halben Dateien.
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil || !(u.Upload || u.Admin) || s.isGuest(r.Context(), u.ID) || s.UploadDir == "" {
		http.Error(w, "Hochladen ist für dieses Profil nicht freigegeben", http.StatusForbidden)
		return
	}
	name := filepath.Base(strings.TrimSpace(r.URL.Query().Get("name")))
	if strings.HasPrefix(name, ".") || len(name) > 200 || strings.ContainsAny(name, "\x00/\\") || !scan.IsVideo(name) {
		http.Error(w, "Nur Videodateien (mp4, mkv, …) mit gültigem Namen", http.StatusBadRequest)
		return
	}
	if err := os.MkdirAll(s.UploadDir, 0o755); writeErr(w, err) {
		return
	}
	f, err := os.CreateTemp(s.UploadDir, ".upload-*.part")
	if writeErr(w, err) {
		return
	}
	n, err := io.Copy(f, http.MaxBytesReader(w, r.Body, maxUpload))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(f.Name())
		http.Error(w, "Hochladen abgebrochen: "+err.Error(), http.StatusBadRequest)
		return
	}
	ziel, err := freierName(s.UploadDir, name)
	if err == nil {
		err = os.Rename(f.Name(), ziel)
	}
	if err != nil {
		os.Remove(f.Name())
		writeErr(w, err)
		return
	}
	log.Printf("upload: %s von %s", filepath.Base(ziel), u.Name)
	s.notiereDownload("upload", u.ID, filepath.Base(ziel), "", n, nil)
	s.Lib.Rescan()
	writeJSON(w, map[string]string{"name": filepath.Base(ziel)})
}

// freierName reserviert name bzw. „name (2).ext“ … per O_EXCL, damit zwei gleichzeitige Uploads nichts überschreiben.
func freierName(dir, name string) (string, error) {
	ext := filepath.Ext(name)
	stamm := strings.TrimSuffix(name, ext)
	for i := 1; i < 1000; i++ {
		p := filepath.Join(dir, name)
		if i > 1 {
			p = filepath.Join(dir, stamm+" ("+strconv.Itoa(i)+")"+ext)
		}
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			return p, f.Close()
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	return "", errors.New("kein freier Dateiname")
}
