// Package mediathek sucht frei verfügbare Sendungen der öffentlich-rechtlichen Mediatheken (ARD, ZDF, arte, 3sat …)
// über MediathekViewWeb und lädt sie nacheinander in einen Unterordner der Uploads. Abos laden neue Sendungen einer
// Suche automatisch. Die Warteschlange steht in der Tabelle downloads und übersteht so einen Neustart.
package mediathek

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Bavarianator/flimmer/internal/db"
)

// api ist die Such-API von MediathekViewWeb; Tests setzen einen httptest-Server ein.
var api = "https://mediathekviewweb.de/api/query"

var client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, ResponseHeaderTimeout: 30 * time.Second}}

// Treffer ist eine Sendung aus der Suche.
type Treffer struct {
	ID           string `json:"id"`
	Sender       string `json:"sender"`
	Thema        string `json:"thema"`
	Titel        string `json:"titel"`
	Beschreibung string `json:"beschreibung,omitempty"`
	Zeit         int64  `json:"zeit"`  // Ausstrahlung, Unix-Sekunden
	Dauer        int64  `json:"dauer"` // Sekunden
	Groesse      int64  `json:"groesse,omitempty"`
	Webseite     string `json:"webseite,omitempty"`
	Video        string `json:"video"` // HD, sonst Standardqualität
}

type Anfrage struct {
	Text       string
	Sender     string
	MinMinuten int
	Offset     int
	Anzahl     int // 0 = 30
}

// zahl nimmt Zahlen und null; MediathekViewWeb liefert bei unbekannter Größe null.
type zahl int64

func (z *zahl) UnmarshalJSON(b []byte) error {
	n, _ := strconv.ParseInt(strings.Trim(string(b), `"`), 10, 64)
	*z = zahl(n)
	return nil
}

var (
	// Fassungen mit Audiodeskription, Gebärdensprache usw. gibt es zu fast jedem Film zusätzlich.
	// Kurze Kennungen nur in Klammern („Englisch für Anfänger“ bleibt), die langen auch frei im Titel.
	reVariante = regexp.MustCompile(`(?i)\s*[(\[](ad|audiodeskription|dgs|gebärdensprache|klare sprache|leichte sprache|hörfassung|originalversion|ov|omu|englisch|english|mit untertiteln?|ut)[)\]]|\s*-?\s*\b(audiodeskription|gebärdensprache|klare sprache|leichte sprache|hörfassung|originalversion)\b`)
	reStaffel  = regexp.MustCompile(`(?i)\s*\(?\bS(\d{1,2})\s*/\s*E(\d{1,3})\)?`)
	reJahr     = regexp.MustCompile(`\s*\(((?:19|20)\d{2})\)`)
	// „Die Spionin - Spielfilm, Norwegen/Schweden/Belgien 2019“: Gattung, Länder und Jahr
	reZusatz    = regexp.MustCompile(`(?i)\s+[-–|]\s+(spielfilm|fernsehfilm|film|kinofilm|komödie|drama|thriller|krimi|dokumentarfilm)\b.*$`)
	reJahrFrei  = regexp.MustCompile(`\b((?:19|20)\d{2})\b`)
	reUnerlaubt = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]+`)
)

// Suche fragt MediathekViewWeb, neueste zuerst. gesamt zählt vor dem Filter, reicht aber für „Mehr laden“.
func Suche(ctx context.Context, a Anfrage) (treffer []Treffer, gesamt int, err error) {
	type feld struct {
		Fields []string `json:"fields"`
		Query  string   `json:"query"`
	}
	q := struct {
		Queries   []feld `json:"queries"`
		SortBy    string `json:"sortBy"`
		SortOrder string `json:"sortOrder"`
		Future    bool   `json:"future"`
		Offset    int    `json:"offset"`
		Size      int    `json:"size"`
		Min       int    `json:"duration_min,omitempty"`
	}{Queries: []feld{}, SortBy: "timestamp", SortOrder: "desc", Offset: max(a.Offset, 0), Size: a.Anzahl, Min: a.MinMinuten * 60}
	if q.Size <= 0 || q.Size > 200 {
		q.Size = 30
	}
	if t := strings.TrimSpace(a.Text); t != "" {
		q.Queries = append(q.Queries, feld{[]string{"title", "topic"}, t})
	}
	if s := strings.TrimSpace(a.Sender); s != "" {
		q.Queries = append(q.Queries, feld{[]string{"channel"}, s})
	}
	body, _ := json.Marshal(q)
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "text/plain") // so verlangt es die API
	res, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("MediathekViewWeb nicht erreichbar: %w", err)
	}
	defer res.Body.Close()
	var out struct {
		Err    any `json:"err"`
		Result struct {
			Results []struct {
				ID, Channel, Topic, Title, Description string
				Timestamp, Duration, Size              zahl
				URLWebsite                             string `json:"url_website"`
				URLVideo                               string `json:"url_video"`
				URLVideoHD                             string `json:"url_video_hd"`
			} `json:"results"`
			QueryInfo struct {
				TotalResults int `json:"totalResults"`
			} `json:"queryInfo"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(&out); err != nil || res.StatusCode != http.StatusOK || out.Err != nil {
		return nil, 0, fmt.Errorf("MediathekViewWeb: Antwort %s nicht lesbar (%v %v)", res.Status, out.Err, err)
	}
	treffer = []Treffer{}
	for _, r := range out.Result.Results {
		video := r.URLVideoHD
		if video == "" {
			video = r.URLVideo
		}
		// ponytail: HLS (.m3u8, fast nur ORF/SRF und dort meist geogesperrt) fällt heraus; ffmpeg-Remux, wenn sie fehlen.
		if reVariante.MatchString(r.Title) || strings.HasSuffix(strings.SplitN(video, "?", 2)[0], ".m3u8") || !strings.HasPrefix(video, "http") ||
			(strings.HasPrefix(r.Channel, "ARTE.") && r.Channel != "ARTE.DE") {
			continue
		}
		treffer = append(treffer, Treffer{ID: r.ID, Sender: r.Channel, Thema: r.Topic, Titel: r.Title, Beschreibung: r.Description,
			Zeit: int64(r.Timestamp), Dauer: int64(r.Duration), Groesse: int64(r.Size), Webseite: r.URLWebsite, Video: video})
	}
	return treffer, out.Result.QueryInfo.TotalResults, nil
}

// Name macht aus einem Treffer den Dateinamen (ohne Endung), den scan.Parse richtig einordnet:
// Filme als „Titel (Jahr)“, Reihen als „Thema - Titel (Jahr)“, Serienfolgen als „Thema S01E03 Titel“.
func Name(t Treffer) string {
	titel := reVariante.ReplaceAllString(t.Titel, "")
	thema := strings.TrimSpace(t.Thema)
	if m := reStaffel.FindStringSubmatchIndex(titel); m != nil && thema != "" {
		s, _ := strconv.Atoi(titel[m[2]:m[3]])
		e, _ := strconv.Atoi(titel[m[4]:m[5]])
		return datei(fmt.Sprintf("%s S%02dE%02d %s", thema, s, e, titel[:m[0]]+titel[m[1]:]))
	}
	jahr := ""
	if m := reJahr.FindStringSubmatchIndex(titel); m != nil {
		jahr, titel = titel[m[2]:m[3]], titel[:m[0]]+titel[m[1]:]
	}
	if m := reZusatz.FindStringIndex(titel); m != nil {
		if j := reJahrFrei.FindString(titel[m[0]:]); j != "" && jahr == "" {
			jahr = j
		}
		titel = titel[:m[0]]
	}
	titel = strings.TrimSpace(titel)
	l := strings.ToLower(thema)
	if thema != "" && !strings.Contains(l, "film") && !strings.Contains(l, "kino") && !strings.Contains(strings.ToLower(titel), l) {
		titel = thema + " - " + titel
	}
	if titel == "" {
		titel = "Sendung " + t.ID
	}
	if jahr != "" {
		titel += " (" + jahr + ")"
	}
	return datei(titel)
}

// datei ersetzt Zeichen, die Dateisysteme nicht mögen, und kürzt auf 150 Bytes.
func datei(s string) string {
	s = strings.Join(strings.Fields(reUnerlaubt.ReplaceAllString(s, " ")), " ")
	s = strings.Trim(s, " .-")
	for len(s) > 150 {
		_, n := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-n]
	}
	return s
}

// --- Warteschlange ---

// reserve bleibt auf dem Laufwerk immer frei.
const reserve = 10 << 30

type M struct {
	DB     *db.DB
	Dir    string        // Ziel der Downloads, z. B. <uploads>/Mediathek
	Frei   func() uint64 // freier Platz in Dir, 0 = unbekannt; nil = nicht prüfen
	Rescan func()        // nach jedem fertigen Download; nil = keiner

	kick, weck chan struct{}

	mu       sync.Mutex
	lauf     int64 // ID des laufenden Downloads, 0 = keiner
	anteil   float64
	stopp    context.CancelFunc
	geprueft time.Time // letzter Lauf von PruefeAbos
	fehler   string
}

func New(d *db.DB, dir string) *M {
	return &M{DB: d, Dir: dir, kick: make(chan struct{}, 1), weck: make(chan struct{}, 1)}
}

func signal(c chan struct{}) {
	select {
	case c <- struct{}{}:
	default:
	}
}

// Kick prüft die Abos sofort (neues Abo).
func (m *M) Kick() { signal(m.kick) }

// Laden reiht einen Treffer ein; quelle ist "mediathek" (von Hand) oder "abo".
func (m *M) Laden(ctx context.Context, t Treffer, quelle, user string) (db.Download, error) {
	dl := db.Download{Quelle: quelle, Titel: Name(t), Sender: t.Sender, Bytes: t.Groesse, Status: "wartet", ExtID: t.ID, User: user, URL: t.Video,
		Erstellt: time.Now()}
	if t.ID == "" {
		return dl, errors.New("Treffer ohne ID")
	}
	id, err := m.DB.AddDownload(ctx, dl)
	if err != nil {
		return dl, err
	}
	dl.ID = id
	signal(m.weck)
	return dl, nil
}

// Downloads liefert die Mediathek-Downloads, beim laufenden mit Fortschritt.
func (m *M) Downloads(ctx context.Context) ([]db.Download, error) {
	l, err := m.DB.Downloads(ctx, 100, "mediathek", "abo")
	m.mu.Lock()
	for i := range l {
		if l[i].ID == m.lauf {
			l[i].Anteil = m.anteil
		}
	}
	m.mu.Unlock()
	return l, err
}

// Abbrechen hält einen wartenden oder laufenden Download an; er bleibt als „abgebrochen“ stehen, damit ein Abo
// ihn nicht neu einreiht. Fertige, fehlerhafte und abgebrochene Einträge verschwinden aus der Liste, die Datei bleibt.
func (m *M) Abbrechen(ctx context.Context, id int64) error {
	dl, err := m.DB.Download(ctx, id)
	if err != nil {
		return err
	}
	if dl.Status != "wartet" && dl.Status != "laeuft" {
		return m.DB.DeleteDownload(ctx, id)
	}
	if err := m.DB.FinishDownload(ctx, id, "abgebrochen", "", dl.Bytes, ""); err != nil {
		return err
	}
	m.mu.Lock()
	if m.lauf == id && m.stopp != nil {
		m.stopp()
	}
	m.mu.Unlock()
	return nil
}

// Run arbeitet die Warteschlange ab (ein Download zur Zeit, das NAS hat 2 Kerne) und prüft die Abos alle 6 h.
func (m *M) Run(ctx context.Context) {
	go func() {
		for {
			if err := m.PruefeAbos(ctx); err != nil {
				log.Printf("Mediathek-Abos: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-m.kick:
			case <-time.After(6 * time.Hour):
			}
		}
	}()
	if err := m.DB.ResetDownloads(ctx); err != nil {
		log.Printf("Mediathek: %v", err)
	}
	reste, _ := filepath.Glob(filepath.Join(m.Dir, ".mediathek-*.part"))
	for _, f := range reste {
		os.Remove(f) // eigene Zwischendateien abgebrochener Downloads
	}
	for {
		for ctx.Err() == nil {
			dl, ok, err := m.DB.NextDownload(ctx)
			if err != nil {
				log.Printf("Mediathek: %v", err)
			}
			if !ok {
				break
			}
			m.laden(ctx, dl)
		}
		select {
		case <-ctx.Done():
			return
		case <-m.weck:
		}
	}
}

func (m *M) laden(ctx context.Context, dl db.Download) {
	ctx, stopp := context.WithCancel(ctx)
	defer stopp()
	m.mu.Lock()
	m.lauf, m.anteil, m.stopp = dl.ID, 0, stopp
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.lauf, m.stopp = 0, nil
		m.mu.Unlock()
	}()
	datei, n, err := m.holen(ctx, dl)
	if ctx.Err() != nil {
		return // abgebrochen (Status steht schon) oder Server beendet (wartet nach dem Neustart wieder)
	}
	status, fehler := "fertig", ""
	if err != nil {
		status, fehler, n = "fehler", err.Error(), dl.Bytes
		log.Printf("Mediathek %d „%s“: %v", dl.ID, dl.Titel, err)
	} else {
		log.Printf("Mediathek %d „%s“ fertig (%d MB)", dl.ID, dl.Titel, n>>20)
	}
	if err := m.DB.FinishDownload(context.WithoutCancel(ctx), dl.ID, status, datei, n, fehler); err != nil {
		log.Printf("Mediathek %d: %v", dl.ID, err)
	}
	if status == "fertig" && m.Rescan != nil {
		m.Rescan()
	}
}

// holen lädt dl.URL nach Dir/<Titel>.mp4; die Zwischendatei .mediathek-<id>.part übersieht der Scan.
func (m *M) holen(ctx context.Context, dl db.Download) (string, int64, error) {
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return "", 0, err
	}
	if m.Frei != nil {
		if f := m.Frei(); f > 0 && f < uint64(max(dl.Bytes, 0))+reserve {
			return "", 0, errors.New("Zu wenig Speicherplatz (10 GB bleiben immer frei)")
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dl.URL, nil)
	if err != nil {
		return "", 0, err
	}
	res, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone {
			return "", 0, errors.New("Die Sendung ist in der Mediathek nicht mehr verfügbar")
		}
		return "", 0, fmt.Errorf("Mediathek antwortet %s", res.Status)
	}
	gesamt := res.ContentLength
	if gesamt <= 0 {
		gesamt = dl.Bytes
	}
	part := filepath.Join(m.Dir, fmt.Sprintf(".mediathek-%d.part", dl.ID))
	f, err := os.Create(part)
	if err != nil {
		return "", 0, err
	}
	n, err := io.Copy(f, &zaehler{r: res.Body, m: m, gesamt: gesamt})
	if e := f.Close(); err == nil {
		err = e
	}
	if err == nil && res.ContentLength > 0 && n != res.ContentLength {
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		os.Remove(part)
		return "", 0, err
	}
	// gleicher Titel schon da (anderer Sender, Wiederholung): Sender anhängen, notfalls die ID
	for _, name := range []string{dl.Titel, dl.Titel + " – " + dl.Sender, fmt.Sprintf("%s – %d", dl.Titel, dl.ID)} {
		ziel := filepath.Join(m.Dir, datei(name)+".mp4")
		if _, err := os.Lstat(ziel); errors.Is(err, os.ErrNotExist) {
			return filepath.Base(ziel), n, os.Rename(part, ziel)
		}
	}
	os.Remove(part)
	return "", 0, errors.New("Zieldatei gibt es schon")
}

type zaehler struct {
	r      io.Reader
	m      *M
	n      int64
	gesamt int64
}

func (z *zaehler) Read(p []byte) (int, error) {
	n, err := z.r.Read(p)
	z.n += int64(n)
	if z.gesamt > 0 {
		z.m.mu.Lock()
		z.m.anteil = min(float64(z.n)/float64(z.gesamt), 1)
		z.m.mu.Unlock()
	}
	return n, err
}

// --- Abos ---

// PruefeAbos reiht neue Sendungen aller Abos ein: nur aus den letzten 7 Tagen (ein neues Abo lädt nicht das ganze
// Archiv), jede Sendung einmal (ext_id) und jeden Titel einmal (gleicher Film auf ARD und 3sat).
func (m *M) PruefeAbos(ctx context.Context) (err error) {
	defer func() {
		m.mu.Lock()
		m.geprueft, m.fehler = time.Now(), ""
		if err != nil {
			m.fehler = err.Error()
		}
		m.mu.Unlock()
	}()
	abos, err := m.DB.Abos(ctx)
	if err != nil {
		return err
	}
	seit := time.Now().AddDate(0, 0, -7).Unix()
	var fehler []error
	for _, a := range abos {
		treffer, _, err := Suche(ctx, Anfrage{Text: a.Text, Sender: a.Sender, MinMinuten: a.MinMinuten, Anzahl: 50})
		if err != nil {
			fehler = append(fehler, fmt.Errorf("Abo „%s“: %w", a.Text, err))
			continue
		}
		for _, t := range treffer {
			if t.Zeit < seit || m.DB.HasDownloadTitle(ctx, Name(t)) {
				continue
			}
			if dl, err := m.Laden(ctx, t, "abo", ""); err == nil {
				log.Printf("Mediathek-Abo „%s“: „%s“ eingereiht", a.Text, dl.Titel)
			} else if !errors.Is(err, db.ErrSchonDa) {
				fehler = append(fehler, err)
			}
		}
	}
	return errors.Join(fehler...)
}

// Geprueft meldet den letzten Lauf von PruefeAbos (Dashboard › Geplante Aufgaben).
func (m *M) Geprueft() (time.Time, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.geprueft, m.fehler
}
