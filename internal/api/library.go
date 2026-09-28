package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/flimmer-media/flimmer/internal/auth"
	"github.com/flimmer-media/flimmer/internal/db"
	"github.com/flimmer-media/flimmer/internal/images"
	"github.com/flimmer-media/flimmer/internal/meta"
	"github.com/flimmer-media/flimmer/internal/playback"
	"github.com/flimmer-media/flimmer/internal/scan"
)

type libraryItem struct {
	*scan.Item
	Duration float64         `json:"duration"`
	Light    playback.Light  `json:"light"`
	Method   playback.Method `json:"method"`
	Meta     *meta.Meta      `json:"meta"`
	Poster   string          `json:"poster"`
	Backdrop string          `json:"backdrop"`
	Color    string          `json:"color"`
	Progress float64         `json:"progress"`
	Watched  bool            `json:"watched"`
}

func readProfile(w http.ResponseWriter, r *http.Request) (playback.Profile, bool) {
	var p playback.Profile
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&p); err != nil {
		http.Error(w, "Geräteprofil fehlt", http.StatusBadRequest)
		return p, false
	}
	return p, true
}

// items reichert Titel für den angemeldeten Benutzer und das anfragende Gerät an.
func (s *Server) items(r *http.Request, p playback.Profile, list []*scan.Item) []libraryItem {
	prog, err := s.DB.Progress(r.Context(), userFrom(r).ID)
	if err != nil {
		log.Printf("Fortschritt lesen: %v", err)
	}
	speed := s.hw().Speed
	out := make([]libraryItem, 0, len(list))
	for _, it := range list {
		plan := playback.Decide(it.Media, p, speed)
		li := libraryItem{Item: it, Duration: it.Media.Duration, Light: plan.Light, Method: plan.Method,
			Meta: s.Lib.MetaFor(it.ID), Color: s.Lib.ColorFor(it.ID), Progress: prog[it.ID].Pos, Watched: prog[it.ID].Watched}
		if m := li.Meta; m != nil && m.PosterPath != "" {
			li.Poster = "/api/images/" + it.ID + "/poster?w=300&v=" + url.QueryEscape(m.PosterPath)
		}
		switch {
		case li.Meta != nil && li.Meta.BackdropPath != "":
			li.Backdrop = "/api/images/" + it.ID + "/backdrop?w=1280&v=" + url.QueryEscape(li.Meta.BackdropPath)
		case it.Series != "":
			li.Backdrop = "/api/images/" + it.ID + "/still?w=780" // Standbild aus der Episode
		}
		out = append(out, li)
	}
	return out
}

// library gibt alle Titel mit Ampel für genau das anfragende Gerät zurück.
func (s *Server) library(w http.ResponseWriter, r *http.Request) {
	p, ok := readProfile(w, r)
	if ok {
		writeJSON(w, s.items(r, p, s.Lib.All()))
	}
}

type homeRow struct {
	ID    string        `json:"id"`
	Title string        `json:"title"`
	Items []libraryItem `json:"items"`
}

// home: Weiterschauen, Als Nächstes, Neu hinzugefügt. Leere Reihen fallen weg.
func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	p, ok := readProfile(w, r)
	if !ok {
		return
	}
	prog, err := s.DB.Progress(r.Context(), userFrom(r).ID)
	if writeErr(w, err) {
		return
	}
	cont, next, recent := homeRows(s.Lib.All(), prog)
	rows := []homeRow{}
	for _, row := range []homeRow{{ID: "continue", Title: "Weiterschauen"}, {ID: "nextup", Title: "Als Nächstes"}, {ID: "recent", Title: "Neu hinzugefügt"}} {
		list := map[string][]*scan.Item{"continue": cont, "nextup": next, "recent": recent}[row.ID]
		if len(list) > 0 {
			row.Items = s.items(r, p, list)
			rows = append(rows, row)
		}
	}
	writeJSON(w, rows)
}

const rowMax = 20

// homeRows ist rein (ohne Server), damit die Regeln testbar sind. all ist nach Serie/Staffel/Episode sortiert.
func homeRows(all []*scan.Item, prog map[string]db.Progress) (cont, next, recent []*scan.Item) {
	type hit struct {
		it *scan.Item
		t  time.Time
	}
	var c, n []hit
	busy := map[string]bool{} // Serien mit angefangener Episode stehen schon unter „Weiterschauen“
	for _, it := range all {
		if pr := prog[it.ID]; pr.Pos > 0 && !pr.Watched {
			c = append(c, hit{it, pr.Updated})
			busy[it.Series] = it.Series != ""
		}
	}
	// Als Nächstes: pro Serie die Episode nach der zuletzt gesehenen.
	last := map[string]int{}
	lastT := map[string]time.Time{}
	for i, it := range all {
		if pr := prog[it.ID]; it.Series != "" && pr.Watched && pr.Updated.After(lastT[it.Series]) {
			last[it.Series], lastT[it.Series] = i, pr.Updated
		}
	}
	for series, i := range last {
		if busy[series] {
			continue
		}
		for _, it := range all[i+1:] {
			if it.Series != series {
				break
			}
			if !prog[it.ID].Watched {
				n = append(n, hit{it, lastT[series]})
				break
			}
		}
	}
	byTime := func(a, b hit) int { return b.t.Compare(a.t) }
	slices.SortFunc(c, byTime)
	slices.SortFunc(n, byTime)
	for _, h := range c[:min(len(c), rowMax)] {
		cont = append(cont, h.it)
	}
	for _, h := range n[:min(len(n), rowMax)] {
		next = append(next, h.it)
	}

	// Neu hinzugefügt: jüngste Dateien, pro Serie nur einmal.
	byAdded := slices.Clone(all)
	slices.SortStableFunc(byAdded, func(a, b *scan.Item) int { return b.Added.Compare(a.Added) })
	seen := map[string]bool{}
	for _, it := range byAdded {
		if len(recent) == rowMax {
			break
		}
		if it.Series != "" {
			if seen[it.Series] {
				continue
			}
			seen[it.Series] = true
		}
		recent = append(recent, it)
	}
	return cont, next, recent
}

type subtitleOut struct {
	playback.Subtitle
	URL string `json:"url"`
}

type playResponse struct {
	playback.Plan
	Subtitles []subtitleOut `json:"subtitles"` // überdeckt Plan.Subtitles, ergänzt um fertige URLs
	URL       string        `json:"url"`
	Duration  float64       `json:"duration"`
	Title     string        `json:"title"`
	Resume    float64       `json:"resume"` // Sekunden, 0 = von vorn
	Prefs     db.TrackPref  `json:"prefs"`  // zuletzt gewählte Sprachen dieser Serie
}

func (s *Server) play(w http.ResponseWriter, r *http.Request) {
	it := s.item(w, r)
	if it == nil {
		return
	}
	p, ok := readProfile(w, r)
	if !ok {
		return
	}
	u := userFrom(r)
	resume := s.DB.ProgressOf(r.Context(), u.ID, it.ID).Pos
	var prefs db.TrackPref
	if it.Series != "" {
		prefs = s.DB.Pref(r.Context(), u.ID, it.Series)
	}
	if p.AudioLang == "" {
		p.AudioLang = prefs.Audio // zuletzt gewählte Tonsprache der Serie
	}
	plan := playback.Decide(it.Media, p, s.hw().Speed)
	resp := playResponse{Plan: plan, Duration: it.Media.Duration, Title: it.Title, Subtitles: []subtitleOut{}, Resume: resume, Prefs: prefs}
	// Medien-Token im Pfad: <video src>, hls.js-Segmente und TVs schicken keine Header.
	base := "/api/m/" + auth.MediaToken(s.secret(), u.ID, mediaTTL) + "/items/" + it.ID
	if plan.Method == playback.DirectPlay {
		resp.URL = base + "/file"
	} else {
		resp.URL = base + "/hls/" + strconv.Itoa(plan.AudioIndex) + "/" + plan.AudioCodec + "/" + plan.VideoCodec + "/index.m3u8"
	}
	for _, sub := range plan.Subtitles {
		ext := ".vtt"
		if sub.Format == "pgs" {
			ext = ".sup"
		}
		resp.Subtitles = append(resp.Subtitles, subtitleOut{sub, base + "/subs/" + strconv.Itoa(sub.Index) + ext})
	}
	log.Printf("play %q für %s: %s (%v)", it.Title, u.Name, plan.Method, plan.Reasons)
	title := it.Title
	if it.Series != "" {
		title = fmt.Sprintf("%s – S%02dE%02d %s", it.Series, it.Season, it.Episode, it.Title)
	}
	s.streams.start(u.ID+"|"+it.ID, stream{User: u.Name, Title: title, Device: p.Name, Method: plan.Method, Light: plan.Light, Reasons: plan.Reasons})
	writeJSON(w, resp)
}

// touch hält eine Wiedergabe in der Diagnose als „aktiv“.
func (s *Server) touch(r *http.Request, it *scan.Item) {
	if u := userFrom(r); u != nil {
		s.streams.touch(u.ID + "|" + it.ID)
	}
}

func (s *Server) progress(w http.ResponseWriter, r *http.Request) {
	it := s.item(w, r)
	if it == nil {
		return
	}
	s.touch(r, it)
	var req struct {
		Pos      float64 `json:"pos"`
		Duration float64 `json:"duration"`
		Audio    string  `json:"audio"`
		Subtitle string  `json:"subtitle"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	dur := it.Media.Duration
	if dur <= 0 {
		dur = req.Duration
	}
	uid := userFrom(r).ID
	out, err := s.DB.SetProgress(r.Context(), uid, it.ID, max(req.Pos, 0), dur)
	if writeErr(w, err) {
		return
	}
	if it.Series != "" && (req.Audio != "" || req.Subtitle != "") {
		if err := s.DB.SetPref(r.Context(), uid, it.Series, db.TrackPref{Audio: req.Audio, Subtitle: req.Subtitle}); err != nil {
			log.Printf("Sprachwahl speichern: %v", err)
		}
	}
	writeJSON(w, out)
}

func (s *Server) watched(w http.ResponseWriter, r *http.Request) {
	it := s.item(w, r)
	if it == nil {
		return
	}
	var req struct {
		Watched bool `json:"watched"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	uid := userFrom(r).ID
	var err error
	if req.Watched {
		_, err = s.DB.SetProgress(r.Context(), uid, it.ID, it.Media.Duration, it.Media.Duration)
	} else {
		err = s.DB.DeleteProgress(r.Context(), uid, it.ID)
	}
	if !writeErr(w, err) {
		w.WriteHeader(http.StatusNoContent)
	}
}

var reDevice = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Geräteprofile speichert der Server nur; das Format gehört playback bzw. der UI.
func (s *Server) deviceProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("device")
	if !reDevice.MatchString(id) {
		http.Error(w, "ungültige Geräte-ID", http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodGet {
		b := s.DB.Device(r.Context(), id)
		if b == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
		return
	}
	var b json.RawMessage
	if !readJSON(w, r, &b) {
		return
	}
	if !writeErr(w, s.DB.SetDevice(r.Context(), id, b)) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// ImageSource sagt internal/images, woraus ein Bild entsteht (auch für die Platzhalterfarbe im Scan).
func ImageSource(it *scan.Item, m *meta.Meta, imgDir, kind string) (images.Source, bool) {
	var file string
	if m != nil {
		switch kind {
		case "poster":
			file = m.PosterPath
		case "backdrop", "still":
			file = m.BackdropPath
		}
	}
	switch {
	case file != "":
		return images.Source{Path: filepath.Join(imgDir, filepath.Base(file))}, true
	case kind == "still" && it.Series != "":
		return images.Source{Path: it.Path, Video: true, Duration: it.Media.Duration}, true
	}
	return images.Source{}, false
}

func (s *Server) imageLookup(id, kind string) (images.Source, bool) {
	it := s.Lib.Get(id)
	if it == nil || s.Meta == nil {
		return images.Source{}, false
	}
	return ImageSource(it, s.Lib.MetaFor(id), s.Meta.ImgDir(), kind)
}

func (s *Server) metaLookup(id string) (meta.Query, bool) {
	it := s.Lib.Get(id)
	if it == nil {
		return meta.Query{}, false
	}
	return it.Query(), true
}

// review: unsichere Treffer für „Bitte prüfen“ in den Einstellungen.
func (s *Server) review(w http.ResponseWriter, r *http.Request) {
	type entry struct {
		ID    string     `json:"id"`
		File  string     `json:"file"`
		Meta  *meta.Meta `json:"meta"`
		Guess string     `json:"guess"`
	}
	out := []entry{}
	for _, it := range s.Lib.All() {
		if m := s.Lib.MetaFor(it.ID); m != nil && (m.Uncertain || m.Source == "filename") {
			guess := it.Title
			if it.Series != "" {
				guess = it.Series
			}
			out = append(out, entry{it.ID, filepath.Base(it.Path), m, strings.TrimSpace(guess)})
		}
	}
	writeJSON(w, out)
}
