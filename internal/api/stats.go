package api

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Bavarianator/flimmer/internal/scan"
)

// --- Statistik (Dashboard › Statistik) ---

type laufwerk struct {
	Path  string `json:"path"`
	Free  uint64 `json:"free"`
	Total uint64 `json:"total"`
}

// laufwerke: freier und gesamter Platz pro Dateisystem der Pfade.
func laufwerke(paths []string) []laufwerk {
	out := []laufwerk{}
	for _, p := range paths {
		// ponytail: gleiche Größe und gleicher freier Platz = dasselbe Dateisystem; Fsid wäre genauer, ist aber nicht portabel
		if free, total := diskSpace(p); total > 0 && !slices.ContainsFunc(out, func(d laufwerk) bool { return d.Free == free && d.Total == total }) {
			out = append(out, laufwerk{p, free, total})
		}
	}
	return out
}

type tagWert struct {
	Tag      string  `json:"tag"` // 2006-01-02
	Sekunden float64 `json:"sekunden"`
}

type monatWert struct {
	Monat  string `json:"monat"` // 2006-01
	Anzahl int    `json:"anzahl"`
	Bytes  int64  `json:"bytes"`
}

type stufe struct {
	Name   string `json:"name"`
	Anzahl int    `json:"anzahl"`
	Bytes  int64  `json:"bytes"`
}

type groesster struct {
	ID    string `json:"id"`
	Titel string `json:"titel"`
	Serie string `json:"serie,omitempty"`
	Bytes int64  `json:"bytes"`
}

// GET /api/admin/stats?tage=30: Sehzeit, Bibliothek, Speicher und Downloads.
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tage, _ := strconv.Atoi(r.URL.Query().Get("tage"))
	if tage <= 0 || tage > 3660 {
		tage = 30
	}
	jetzt := time.Now()
	seit := time.Date(jetzt.Year(), jetzt.Month(), jetzt.Day()-tage+1, 0, 0, 0, 0, jetzt.Location())

	// Sehzeit: Tage und Tageszeit in Ortszeit aus den Stunden-Summen
	ws, err := s.DB.WatchStats(ctx, seit)
	if writeErr(w, err) {
		return
	}
	proTag, index := make([]tagWert, tage), map[string]int{}
	for i := range proTag {
		proTag[i].Tag = seit.AddDate(0, 0, i).Format(time.DateOnly)
		index[proTag[i].Tag] = i
	}
	proStunde, gesamt := make([]float64, 24), 0.0
	for h, sek := range ws.Stunden {
		t := time.UnixMilli(h)
		gesamt += sek
		proStunde[t.Hour()] += sek
		if i, ok := index[t.Format(time.DateOnly)]; ok {
			proTag[i].Sekunden += sek
		}
	}
	for i := range ws.Titel {
		ws.Titel[i].Titel = "(entfernt)"
		if it := s.Lib.Get(ws.Titel[i].ID); it != nil {
			ws.Titel[i].Titel, ws.Titel[i].Serie = titelVon(it), it.Series
		}
	}

	// Bibliothek: Zugang der letzten 12 Monate (Dateidatum), Auflösung, Codecs, größte Titel
	proMonat, monate := make([]monatWert, 12), map[string]int{}
	for i := range proMonat {
		proMonat[i].Monat = time.Date(jetzt.Year(), jetzt.Month()-11+time.Month(i), 1, 0, 0, 0, 0, jetzt.Location()).Format("2006-01")
		monate[proMonat[i].Monat] = i
	}
	stufen := []stufe{{Name: "4K"}, {Name: "1080p"}, {Name: "720p"}, {Name: "SD"}}
	codecs := map[string]int{}
	hdr, zuwachs90 := 0, int64(0)
	alle := s.Lib.All()
	for _, it := range alle {
		if i, ok := monate[it.Added.Format("2006-01")]; ok {
			proMonat[i].Anzahl++
			proMonat[i].Bytes += it.Size
		}
		if jetzt.Sub(it.Added) < 90*24*time.Hour {
			zuwachs90 += it.Size
		}
		if it.Media == nil {
			continue
		}
		v := it.Media.First("video")
		if v == nil {
			continue
		}
		i := 3
		switch {
		case v.Width >= 3200:
			i = 0
		case v.Width >= 1800:
			i = 1
		case v.Width >= 1200:
			i = 2
		}
		stufen[i].Anzahl++
		stufen[i].Bytes += it.Size
		codecs[v.Codec]++
		if v.HDR != "" {
			hdr++
		}
	}
	codecListe := []stufe{}
	for c, n := range codecs {
		codecListe = append(codecListe, stufe{Name: c, Anzahl: n})
	}
	slices.SortFunc(codecListe, func(a, b stufe) int {
		if a.Anzahl != b.Anzahl {
			return b.Anzahl - a.Anzahl
		}
		return strings.Compare(a.Name, b.Name)
	})
	nachGroesse := slices.Clone(alle)
	slices.SortFunc(nachGroesse, func(a, b *scan.Item) int { return int(min(max(b.Size-a.Size, -1), 1)) })
	groesste := []groesster{}
	for _, it := range nachGroesse[:min(10, len(nachGroesse))] {
		groesste = append(groesste, groesster{it.ID, titelVon(it), it.Series, it.Size})
	}

	// Speicher: Prognose aus dem Zuwachs der letzten 90 Tage
	set, err := s.DB.Settings(ctx)
	if writeErr(w, err) {
		return
	}
	lw := laufwerke(append(slices.Clone(set.Dirs), s.UploadDir, s.CacheDir))
	var frei uint64
	for _, l := range lw {
		frei += l.Free
	}
	speicher := map[string]any{"laufwerke": lw, "zuwachsMonat": zuwachs90 / 3}
	if zuwachs90 > 0 {
		speicher["monateBisVoll"] = float64(frei) / (float64(zuwachs90) / 3)
	}

	quellen, err := s.DB.DownloadStats(ctx, seit)
	if writeErr(w, err) {
		return
	}
	letzte, err := s.DB.Downloads(ctx, 10)
	if writeErr(w, err) {
		return
	}

	writeJSON(w, map[string]any{
		"wiedergabe": map[string]any{"sekunden": gesamt, "proTag": proTag, "proStunde": proStunde, "nutzer": ws.Nutzer,
			"titel": ws.Titel, "methoden": ws.Methoden, "clients": ws.Clients},
		"bibliothek": map[string]any{"proMonat": proMonat, "aufloesung": stufen, "codecs": codecListe, "hdr": hdr,
			"titel": len(alle), "gesehen": s.DB.WatchedItems(ctx), "groesste": groesste},
		"speicher":  speicher,
		"downloads": map[string]any{"quellen": quellen, "letzte": letzte},
	})
}

// titelVon: Filmtitel oder „Serie – S01E02 Titel“ wie in den Sitzungen.
func titelVon(it *scan.Item) string {
	if it.Series == "" {
		return it.Title
	}
	return fmt.Sprintf("%s – S%02dE%02d %s", it.Series, it.Season, it.Episode, it.Title)
}
