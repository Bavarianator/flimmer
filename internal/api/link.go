package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/Bavarianator/flimmer/internal/db"
)

// Video per Link (/hochladen): yt-dlp lädt von YouTube, Vimeo, Mediatheken, Teams/SharePoint … oder einer direkten
// Datei-URL. Links hinter einer Anmeldung (Teams der Firma, private Videos) brauchen die cookies.txt der Seite –
// oder das Lesezeichen „An Flimmer“: Es holt im angemeldeten Browser einen Link mit Zugangstoken (Hochladen.tsx).
// Gleiches Recht und gleiches Ziel wie POST /api/upload. Zwischendateien liegen im Cache (den scannt niemand),
// yt-dlp schiebt nur das fertige Video in UploadDir.
type linkJob struct {
	ID     int     `json:"id"`
	URL    string  `json:"url"`
	Name   string  `json:"name,omitempty"`
	Anteil float64 `json:"anteil"`
	Fehler string  `json:"fehler,omitempty"`
	Fertig bool    `json:"fertig"`
	user   string
}

// ponytail: Liste nur im Speicher, nach Neustart leer; laufende Downloads brechen dabei ab.
var (
	linkMu    sync.Mutex
	linkJobs  []*linkJob
	linkNext  int
	linkEiner = make(chan struct{}, 1) // ein yt-dlp zur Zeit: das NAS hat 2 Kerne und wenig RAM
	prozent   = regexp.MustCompile(`^\[download\]\s+([\d.]+)%`)
)

// POST /api/upload/link {"url": "…", "name": "<Dateiname, optional>", "cookies": "<Inhalt einer cookies.txt, optional>"}
// startet den Download und antwortet sofort mit dem Auftrag. Cookies gelten nur für diesen Download und landen nie im
// Auftrag oder Log; ins Log kommt auch nur Host und Pfad, die Query kann ein Zugangstoken sein.
func (s *Server) linkStart(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil || !(u.Upload || u.Admin) || s.isGuest(r.Context(), u.ID) || s.UploadDir == "" {
		http.Error(w, "Hochladen ist für dieses Profil nicht freigegeben", http.StatusForbidden)
		return
	}
	var req struct{ URL, Name, Cookies string }
	// nicht readJSON: dessen 64 KB reichen für eine cookies.txt nicht immer
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req) != nil {
		http.Error(w, "ungültige Anfrage (cookies.txt höchstens 1 MB)", http.StatusBadRequest)
		return
	}
	// Nur http(s): yt-dlp kennt auch file:// und lokale Pfade, die dürfen Nutzer nicht anfassen.
	p, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" || len(req.URL) > 16<<10 {
		http.Error(w, "Bitte einen Link mit http:// oder https://", http.StatusBadRequest)
		return
	}
	bin, err := exec.LookPath("yt-dlp")
	if err != nil {
		http.Error(w, "yt-dlp ist auf dem Server nicht installiert", http.StatusNotImplemented)
		return
	}
	// Name ohne Pfad und Endung; die Endung bestimmt yt-dlp. Unbrauchbar → Titel der Seite.
	name := strings.TrimLeft(filepath.Base(strings.TrimSpace(req.Name)), ".")
	if name = strings.TrimSuffix(name, filepath.Ext(name)); len(name) > 150 || strings.ContainsAny(name, "\x00\\") {
		name = ""
	}
	linkMu.Lock()
	linkNext++
	j := &linkJob{ID: linkNext, URL: p.String(), Name: name, user: u.ID}
	linkJobs = append(linkJobs, j)
	if len(linkJobs) > 50 {
		linkJobs = linkJobs[1:]
	}
	antwort := *j
	linkMu.Unlock()
	log.Printf("link %d: %s%s von %s", j.ID, p.Host, p.Path, u.Name)
	go s.linkLaden(bin, j, name, req.Cookies)
	writeJSON(w, antwort)
}

// GET /api/upload/link: die eigenen Aufträge, die Seite fragt jede Sekunde, solange einer läuft.
func (s *Server) linkListe(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	out := []linkJob{}
	linkMu.Lock()
	for _, j := range linkJobs {
		if u != nil && j.user == u.ID {
			out = append(out, *j)
		}
	}
	linkMu.Unlock()
	writeJSON(w, out)
}

func (s *Server) linkLaden(bin string, j *linkJob, name, cookies string) {
	linkEiner <- struct{}{}
	defer func() { <-linkEiner }()
	tmp := filepath.Join(s.CacheDir, "link")
	defer os.RemoveAll(tmp) // Fragmente abgebrochener Downloads, Cookies
	setze := func(f func()) { linkMu.Lock(); f(); linkMu.Unlock() }
	vorlage := "%(title).150B.%(ext)s"
	if name != "" {
		vorlage = strings.ReplaceAll(name, "%", "%%") + ".%(ext)s"
	}

	args := []string{"--ignore-config", "--no-playlist", "--newline", "--progress",
		// h264/aac zuerst: spielen fast überall direkt, das NAS muss nicht transkodieren.
		"-S", "vcodec:h264,res,acodec:aac", "--merge-output-format", "mp4/mkv",
		// Direktlinks ohne Endung (download.aspx …) kämen als .unknown_video, die übersieht der Scan.
		"--remux-video", "mp4>mp4/webm>webm/mkv",
		"-P", s.UploadDir, "-P", "temp:" + tmp, "-o", vorlage,
		"--print", "after_move:filepath"}
	var err error
	if cookies != "" {
		c := filepath.Join(tmp, "cookies.txt") // fester Name reicht, es läuft nur ein Download
		if err = os.MkdirAll(tmp, 0o700); err == nil {
			err = os.WriteFile(c, []byte(cookies), 0o600)
		}
		args = append(args, "--cookies", c)
	}
	cmd := exec.Command(bin, append(args, "--", j.URL)...)
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err == nil {
		err = cmd.Start()
	}
	if err == nil {
		go func() { pw.CloseWithError(cmd.Wait()) }()
		var datei, fehler string
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			zeile := strings.TrimSpace(sc.Text())
			switch m := prozent.FindStringSubmatch(zeile); {
			case m != nil:
				a, _ := strconv.ParseFloat(m[1], 64)
				setze(func() { j.Anteil = a / 100 })
			case strings.HasPrefix(zeile, "ERROR: "):
				fehler = strings.TrimPrefix(zeile, "ERROR: ")
			case filepath.IsAbs(zeile):
				datei = filepath.Base(zeile)
			}
		}
		io.Copy(io.Discard, pr) // bricht der Scanner ab (Zeile zu lang), hinge yt-dlp sonst am vollen Pipe
		err = sc.Err()          // Exit-Code von yt-dlp, über CloseWithError
		// yt-dlps Hinweis bei Anmeldepflicht, auf Deutsch
		if strings.Contains(fehler, "--cookies") {
			fehler = "Der Link braucht eine Anmeldung: aktuelle cookies.txt der Seite anhängen"
		}
		if err != nil && fehler != "" {
			err = errMsg(fehler)
		}
		if err == nil && datei == "" {
			err = errMsg("kein Video gefunden")
		}
		if datei != "" {
			setze(func() { j.Name = datei })
		}
	}
	// Statistik: nur Host und Pfad, die Query kann ein Zugangstoken sein
	quelle := j.URL
	if p, perr := url.Parse(j.URL); perr == nil {
		quelle = p.Host + p.Path
	}
	if err != nil {
		log.Printf("link %d: %v", j.ID, err)
		setze(func() { j.Fehler = err.Error() })
		s.notiereDownload("link", j.user, cmp(j.Name, quelle), quelle, 0, err)
		return
	}
	log.Printf("link %d: %s fertig", j.ID, j.Name)
	setze(func() { j.Anteil, j.Fertig = 1, true })
	var n int64
	if fi, err := os.Stat(filepath.Join(s.UploadDir, j.Name)); err == nil {
		n = fi.Size()
	}
	s.notiereDownload("link", j.user, j.Name, quelle, n, nil)
	s.Lib.Rescan()
}

// notiereDownload trägt einen fertigen oder gescheiterten Upload/Link-Download für die Statistik ein.
func (s *Server) notiereDownload(quelle, user, titel, url string, bytes int64, fehler error) {
	dl := db.Download{Quelle: quelle, User: user, Titel: titel, URL: url, Bytes: bytes, Status: "fertig"}
	if fehler != nil {
		dl.Status, dl.Fehler = "fehler", fehler.Error()
	}
	if _, err := s.DB.AddDownload(context.Background(), dl); err != nil {
		log.Printf("Download notieren: %v", err)
	}
}
