package livetv

import (
	"bytes"
	"context"
	_ "embed"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Vorlagen für die Einrichtung per Knopf (Dashboard › Live-TV): Sender und Programm ohne Adressen eintippen.

// FreeSource steht in livetv_source für die eingebettete Liste öffentlich-rechtlicher Sender (Streams von ARD/ZDF,
// meist nur in Deutschland abrufbar). Ändert ein Sender seine Adresse, hilft erst ein Flimmer-Update.
const FreeSource = "flimmer:freie-sender"

// EPGDE ist ein freies XMLTV für deutsche Sender von epgshare01 (ein Dritter). Fällt es aus, laufen die Kanäle ohne Programm.
const EPGDE = "https://epgshare01.online/epgshare01/epg_ripper_DE1.xml.gz"

//go:embed freie-sender.m3u
var freeM3U []byte

// fritzLists: Kabel-TV-Liste einer FRITZ!Box Cable (DVB-C, HD-Sender). fritz.box löst nur im FRITZ!Box-Netz richtig
// auf; die feste Adresse hilft, wenn ein anderer DNS (z. B. Pi-hole) davor sitzt.
var fritzLists = []string{"http://fritz.box/dvb/m3u/tvhd.m3u", "http://192.168.178.1/dvb/m3u/tvhd.m3u"}

type preset struct {
	ID       string `json:"id"` // "frei" oder "fritz"
	Source   string `json:"source"`
	EPG      string `json:"epg"`
	Channels int    `json:"channels"` // 0 = nicht gefunden
	Active   bool   `json:"active"`   // ist gerade eingestellt
}

// PresetsHandler: GET, nur Admin. Sucht höchstens 2 s nach einer FRITZ!Box mit Kabel-TV und liefert beide Vorlagen.
// Übernommen wird eine Vorlage mit PUT /api/livetv {source, epg}.
func (tv *TV) PresetsHandler(w http.ResponseWriter, r *http.Request) {
	free, _ := parseChannels(bytes.NewReader(freeM3U))
	fritz := preset{ID: "fritz", Source: fritzLists[0], EPG: EPGDE}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	found := make([]int, len(fritzLists))
	var wg sync.WaitGroup
	for i, u := range fritzLists {
		wg.Go(func() {
			if chs, err := fetch(ctx, u, 2*time.Second, parseChannels); err == nil {
				found[i] = len(chs)
			}
		})
	}
	wg.Wait()
	for i, n := range found {
		if n > 0 {
			fritz.Source, fritz.Channels = fritzLists[i], n
			break
		}
	}
	out := []preset{{ID: "frei", Source: FreeSource, EPG: EPGDE, Channels: len(free)}, fritz}
	cur := tv.config(r.Context()).Source
	for i := range out {
		out[i].Active = out[i].Channels > 0 && out[i].Source == cur
	}
	writeJSON(w, out)
}

// nameKey macht Sendernamen vergleichbar, wenn die Liste keine tvg-id hat: „Das Erste HD“ (FRITZ!Box) und
// „Das Erste“ (XMLTV) ergeben beide „daserste“.
func nameKey(s string) string {
	s = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), " hd")
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, s)
}
