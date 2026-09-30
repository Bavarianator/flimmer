package api

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/Bavarianator/flimmer/internal/images"
	"github.com/Bavarianator/flimmer/internal/meta"
	"github.com/Bavarianator/flimmer/internal/playback"
	"github.com/Bavarianator/flimmer/internal/probe"
	"github.com/Bavarianator/flimmer/internal/scan"
	"github.com/Bavarianator/flimmer/internal/subs"
)

type streamOut struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Lang     string `json:"lang,omitempty"`
	Title    string `json:"title,omitempty"`
	Channels int    `json:"channels,omitempty"`
	Default  bool   `json:"default,omitempty"`
	Forced   bool   `json:"forced,omitempty"`
	External bool   `json:"external,omitempty"` // Datei neben dem Video; index zählt dann in dieser Liste
}

type videoOut struct {
	Codec  string      `json:"codec"`
	Width  int         `json:"width"`
	Height int         `json:"height"`
	HDR    string      `json:"hdr,omitempty"`
	FPS    float64     `json:"fps,omitempty"`
	Crop   *probe.Rect `json:"crop,omitempty"` // sichtbares Bild ohne eingebrannte Balken, Anteile 0..1
}

type chapterOut struct {
	probe.Chapter
	Image string `json:"image,omitempty"` // Standbild kurz nach Kapitelbeginn
}

type personOut struct {
	Name  string `json:"name"`
	Role  string `json:"role,omitempty"`
	Kind  string `json:"kind"`
	Image string `json:"image,omitempty"`
}

type itemDetail struct {
	libraryItem
	Tagline   string       `json:"tagline,omitempty"`
	Studios   []string     `json:"studios,omitempty"`
	Countries []string     `json:"countries,omitempty"`
	People    []personOut  `json:"people,omitempty"`
	Tags      []string     `json:"tags,omitempty"`
	Path      string       `json:"path,omitempty"` // nur für Admins
	Container string       `json:"container"`
	Bitrate   int64        `json:"bitrate,omitempty"`
	Video     *videoOut    `json:"video,omitempty"`
	Audio     []streamOut  `json:"audio"`
	Subs      []streamOut  `json:"subs"`
	Chapters  []chapterOut `json:"chapters,omitempty"`
	Locked    []string     `json:"locked,omitempty"`
}

// deviceProfileOf liest das gespeicherte Geräteprofil aus ?device= (für GET-Routen ohne Body); ohne gilt ein leeres.
func (s *Server) deviceProfileOf(r *http.Request) playback.Profile {
	var p playback.Profile
	if dev := r.URL.Query().Get("device"); reDevice.MatchString(dev) {
		if b := s.DB.Device(r.Context(), dev); b != nil {
			json.Unmarshal(b, &p) // unlesbar: leeres Profil
		}
	}
	return p
}

// itemDetails: GET /api/items/{id}?device= – alles für die Detailseite.
func (s *Server) itemDetails(w http.ResponseWriter, r *http.Request) {
	if it := s.item(w, r); it != nil {
		writeJSON(w, s.detail(r, it))
	}
}

func (s *Server) detail(r *http.Request, it *scan.Item) itemDetail {
	d := itemDetail{libraryItem: s.items(r, s.deviceProfileOf(r), []*scan.Item{it})[0],
		Container: playback.ContainerKey(it.Media.Container), Bitrate: it.Media.Bitrate, Audio: []streamOut{}, Subs: []streamOut{}}
	if m := s.Lib.MetaFor(it.ID); m != nil { // Personen stehen fertig mit Bild-URL in d.People, nicht in d.Meta
		d.Tagline, d.Studios, d.Countries, d.Tags, d.Locked = m.Tagline, m.Studios, m.Countries, m.Tags, m.Locked
		d.People = personsOut(m.People)
	}
	if it.Series != "" && s.Meta != nil {
		if show := s.Meta.Show(r.Context(), it.Series); show != nil {
			if len(d.People) == 0 { // Folgen-NFOs können eigene Personen haben, sonst gilt die Serie
				d.People = personsOut(show.People)
			}
			if len(d.Studios) == 0 {
				d.Studios = show.Studios
			}
			if len(d.Countries) == 0 {
				d.Countries = show.Countries
			}
		}
	}
	if u := userFrom(r); u != nil && u.Admin {
		d.Path = it.Path
	}
	for _, st := range it.Media.Streams {
		out := streamOut{Index: st.Index, Codec: st.Codec, Lang: st.Language, Title: st.Title, Channels: st.Channels, Default: st.Default, Forced: st.Forced}
		switch st.Type {
		case "video":
			if d.Video == nil {
				d.Video = &videoOut{Codec: st.Codec, Width: st.Width, Height: st.Height, HDR: st.HDR}
			}
		case "audio":
			d.Audio = append(d.Audio, out)
		case "subtitle":
			d.Subs = append(d.Subs, out)
		}
	}
	for i, x := range subs.Find(it.Path) {
		d.Subs = append(d.Subs, streamOut{Index: i, Codec: x.Format, Lang: x.Language, Title: x.Title, Default: x.Default, Forced: x.Forced, External: true})
	}
	if s.FFmpeg.Load() {
		ex := s.extras.get(r.Context(), it)
		if d.Video != nil {
			d.Video.FPS = ex.FPS
			d.Video.Crop = s.Lib.Crop(it) // beim ersten Abruf noch nil, die Erkennung läuft dann im Hintergrund
		}
		for _, c := range ex.Chapters {
			co := chapterOut{Chapter: c}
			if d.Video != nil && s.Images != nil {
				co.Image = frameURL(it, c.Start+5, 300)
			}
			d.Chapters = append(d.Chapters, co)
		}
	}
	return d
}

func personsOut(list []meta.Person) []personOut {
	var out []personOut
	for _, p := range list {
		po := personOut{Name: p.Name, Role: p.Role, Kind: p.Kind}
		if p.Image != "" {
			po.Image = personImageURL(p)
		}
		out = append(out, po)
	}
	return out
}

// personImageURL: v ändert sich mit der Bildquelle (Cache „immutable“), verrät aber keine Pfade des Servers.
func personImageURL(p meta.Person) string {
	h := sha1.Sum([]byte(p.Image))
	return "/api/images/person/" + url.PathEscape(p.Name) + "/profile?w=300&v=" + hex.EncodeToString(h[:6])
}

// extraCache hält Bildrate und Kapitel je Titel und Dateigröße; ffprobe liest dafür nur den Kopf der Datei.
type extraCache struct {
	mu sync.Mutex
	m  map[string]probe.Extra
}

func (c *extraCache) get(ctx context.Context, it *scan.Item) probe.Extra {
	key := it.ID + "|" + strconv.FormatInt(it.Size, 10)
	c.mu.Lock()
	e, ok := c.m[key]
	c.mu.Unlock()
	if ok {
		return e
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	e, err := probe.Extras(ctx, it.Path)
	if err != nil {
		log.Printf("Kapitel lesen: %v", err)
		return e // Fehler nicht merken: Laufwerk kann gleich wieder da sein
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil || len(c.m) > 2000 { // ponytail: ganz leeren statt LRU, reicht für die Detailseite
		c.m = map[string]probe.Extra{}
	}
	c.m[key] = e
	return e
}

type seriesOut struct {
	Name     string      `json:"name"`            // Schlüssel wie in den Titeln (series), auch für "serie:<Name>"
	Title    string      `json:"title,omitempty"` // Anzeigename aus den Metadaten, falls anders
	Overview string      `json:"overview,omitempty"`
	Year     int         `json:"year,omitempty"`
	EndYear  int         `json:"endYear,omitempty"`
	Status   string      `json:"status,omitempty"`
	Genres   []string    `json:"genres,omitempty"`
	Studios  []string    `json:"studios,omitempty"`
	People   []personOut `json:"people,omitempty"`
	Age      *int        `json:"age,omitempty"`
	Rating   float64     `json:"rating,omitempty"`
	Poster   string      `json:"poster,omitempty"`
	Backdrop string      `json:"backdrop,omitempty"`
	Color    string      `json:"color,omitempty"`
}

// series: GET /api/series/{name} – Kopf der Serienseite; die Folgen kommen wie bisher aus /api/library.
func (s *Server) series(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	eps := s.visible(r, s.Lib.Episodes(name))
	if len(eps) == 0 {
		http.NotFound(w, r)
		return
	}
	first := s.items(r, playback.Profile{}, eps[:1])[0]
	out := seriesOut{Name: name, Poster: first.Poster, Backdrop: first.Backdrop, Color: first.Color, Year: eps[0].Year}
	if m := first.Meta; m != nil {
		out.Genres, out.Age, out.Rating = m.Genres, m.Age, m.Rating
		if m.Series != name {
			out.Title = m.Series
		}
	}
	if s.Meta != nil {
		if show := s.Meta.Show(r.Context(), name); show != nil {
			out.Overview, out.Year, out.EndYear, out.Status = show.Overview, show.Year, show.EndYear, show.Status
			out.Studios, out.People = show.Studios, personsOut(show.People)
			if show.Rating > 0 {
				out.Rating = show.Rating
			}
			if show.BackdropPath != "" {
				out.Backdrop = "/api/images/" + url.PathEscape("serie:"+name) + "/backdrop?w=1280&v=" + url.QueryEscape(show.BackdropPath)
			}
		}
	}
	writeJSON(w, out)
}

// frameStep: Standbilder gibt es auf einem 5-s-Raster. Die Route ist ohne Anmeldung erreichbar (TVs),
// so bleibt der Cache pro Film begrenzt.
const frameStep = 5

func frameURL(it *scan.Item, t float64, w int) string {
	return "/api/images/" + it.ID + "/frame?t=" + strconv.Itoa(frameAt(it, t)) + "&w=" + strconv.Itoa(w)
}

// frameAt rundet auf das Raster, frühestens 5 s (das erste Bild ist oft schwarz) und vor dem Ende der Datei.
// Bei sehr kurzen Clips kommt 0 heraus, dann nimmt images das Bild bei 20 %.
func frameAt(it *scan.Item, t float64) int {
	last := max(int(it.Media.Duration)-1, 0) / frameStep * frameStep
	return min(max(int(math.Round(t/frameStep))*frameStep, frameStep), last)
}

// frameImage: GET /api/images/{id}/frame?t=<Sekunden>&w= – Einzelbild aus dem Video, beim ersten Abruf erzeugt.
func (s *Server) frameImage(w http.ResponseWriter, r *http.Request) {
	it := s.Lib.Get(r.PathValue("id"))
	t, err := strconv.ParseFloat(r.URL.Query().Get("t"), 64)
	if it == nil || it.Media.First("video") == nil {
		http.NotFound(w, r)
		return
	}
	if err != nil || t < 0 || t > it.Media.Duration {
		http.Error(w, "t muss zwischen 0 und der Laufzeit liegen", http.StatusBadRequest)
		return
	}
	s.Images.Serve(w, r, images.Source{Path: it.Path, Video: true, Duration: it.Media.Duration, At: float64(frameAt(it, t))})
}
