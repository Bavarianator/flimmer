package livetv

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type Channel struct {
	ID     string `json:"id"`
	Number string `json:"number,omitempty"`
	Name   string `json:"name"`
	Logo   string `json:"logo,omitempty"`
	Group  string `json:"group,omitempty"`
	url    string // nie an Clients: enthält bei IPTV-Anbietern meist Zugangsdaten
	epg    string // XMLTV-Kanal-ID
}

type Program struct {
	Start time.Time `json:"start"`
	Stop  time.Time `json:"stop"`
	Title string    `json:"title"`
	Desc  string    `json:"desc,omitempty"`
}

// Nur diese Schemata gehen an ffmpeg; file:, concat: und Co. wären sonst über eine fremde Playlist einschleusbar.
var okScheme = map[string]bool{"http": true, "https": true, "rtsp": true, "rtmp": true, "udp": true, "rtp": true}

var attrRE = regexp.MustCompile(`([\w-]+)="([^"]*)"`)

// parseChannels erkennt M3U (#EXTINF) und die lineup.json eines HDHomeRun.
func parseChannels(r io.Reader) ([]Channel, error) {
	raw, err := io.ReadAll(io.LimitReader(r, 32<<20))
	if err != nil {
		return nil, err
	}
	raw = bytes.TrimSpace(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")))
	var chs []Channel
	if len(raw) > 0 && raw[0] == '[' {
		var lineup []struct{ GuideNumber, GuideName, URL string }
		if err := json.Unmarshal(raw, &lineup); err != nil {
			return nil, fmt.Errorf("lineup.json: %w", err)
		}
		for _, l := range lineup {
			chs = append(chs, Channel{Number: l.GuideNumber, Name: l.GuideName, url: l.URL})
		}
	} else if !bytes.HasPrefix(raw, []byte("#EXTM3U")) {
		return nil, fmt.Errorf("weder M3U noch HDHomeRun-Kanalliste")
	} else {
		var cur *Channel
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "#EXTINF"):
				cur = parseInf(line)
			case line == "" || line[0] == '#':
			case cur != nil:
				cur.url = line
				chs = append(chs, *cur)
				cur = nil
			}
		}
	}
	out := chs[:0]
	seen := map[string]bool{}
	for _, c := range chs {
		if u, err := url.Parse(c.url); err != nil || !okScheme[u.Scheme] || c.Name == "" {
			continue
		}
		base := c.epg
		if base == "" {
			base = c.Name
		}
		id := shortHash(base)
		for n := 2; seen[id]; n++ { // gleiche Namen (SD/HD) bekommen eigene IDs
			id = shortHash(fmt.Sprint(base, n))
		}
		seen[id], c.ID = true, id
		out = append(out, c)
	}
	return out, nil
}

func shortHash(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:5])
}

func parseInf(line string) *Channel {
	c := &Channel{}
	for _, m := range attrRE.FindAllStringSubmatch(line, -1) {
		switch m[1] {
		case "tvg-id":
			c.epg = m[2]
		case "tvg-logo":
			c.Logo = m[2]
		case "group-title":
			c.Group = m[2]
		case "tvg-chno":
			c.Number = m[2]
		case "tvg-name":
			c.Name = m[2]
		}
	}
	// Der Anzeigename steht hinter dem letzten Anführungszeichen (Attribute dürfen Kommas enthalten).
	rest := line[strings.LastIndex(line, `"`)+1:]
	if _, name, ok := strings.Cut(rest, ","); ok && strings.TrimSpace(name) != "" {
		c.Name = strings.TrimSpace(name)
	}
	return c
}

type guide struct {
	progs map[string][]Program // je XMLTV-Kanal, nach Start sortiert
	names map[string]string    // Anzeigename (klein) → XMLTV-Kanal, für Playlists ohne tvg-id
}

func (g *guide) count() (n int) {
	if g != nil {
		for _, p := range g.progs {
			n += len(p)
		}
	}
	return n
}

// parseXMLTV liest den Stream und behält nur Sendungen zwischen jetzt und 72 h voraus. Ein 7-Tage-EPG bleibt so klein.
func parseXMLTV(r io.Reader, now time.Time) (*guide, error) {
	g := &guide{progs: map[string][]Program{}, names: map[string]string{}}
	d := xml.NewDecoder(r)
	d.CharsetReader = func(label string, in io.Reader) (io.Reader, error) {
		if strings.EqualFold(label, "utf-8") {
			return in, nil
		}
		return latin1{in}, nil // ponytail: jede andere Kodierung gilt als Latin-1
	}
	from, to := now.Add(-time.Hour), now.Add(72*time.Hour)
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("XMLTV: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "channel":
			var c struct {
				ID    string   `xml:"id,attr"`
				Names []string `xml:"display-name"`
			}
			if d.DecodeElement(&c, &se) == nil {
				for _, n := range c.Names {
					g.names[strings.ToLower(strings.TrimSpace(n))] = c.ID
				}
			}
		case "programme":
			var startS, stopS, channel string
			for _, a := range se.Attr {
				switch a.Name.Local {
				case "start":
					startS = a.Value
				case "stop":
					stopS = a.Value
				case "channel":
					channel = a.Value
				}
			}
			var body struct {
				Title []string `xml:"title"`
				Desc  []string `xml:"desc"`
			}
			if d.DecodeElement(&body, &se) != nil || len(body.Title) == 0 {
				continue
			}
			start, ok1 := xmltvTime(startS)
			stop, ok2 := xmltvTime(stopS)
			if !ok1 || !ok2 || !stop.After(from) || start.After(to) {
				continue
			}
			pr := Program{Start: start, Stop: stop, Title: strings.TrimSpace(body.Title[0])}
			if len(body.Desc) > 0 {
				pr.Desc = clip(strings.TrimSpace(body.Desc[0]), 500)
			}
			g.progs[channel] = append(g.progs[channel], pr)
		}
	}
	for _, l := range g.progs {
		sort.Slice(l, func(i, j int) bool { return l[i].Start.Before(l[j].Start) })
	}
	return g, nil
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

func xmltvTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"20060102150405 -0700", "20060102150405", "200601021504 -0700", "200601021504"} {
		if t, err := time.Parse(layout, s); err == nil { // ohne Zone: UTC
			return t, true
		}
	}
	return time.Time{}, false
}

type latin1 struct{ r io.Reader }

func (l latin1) Read(p []byte) (int, error) {
	if len(p) < 4 {
		return 0, io.ErrShortBuffer
	}
	tmp := make([]byte, len(p)/2)
	n, err := l.r.Read(tmp)
	out := 0
	for _, b := range tmp[:n] {
		out += utf8.EncodeRune(p[out:], rune(b))
	}
	return out, err
}

// fetch öffnet eine URL oder Datei (auch .gz) und übergibt sie an parse. Fehlertexte enthalten nie die volle
// Adresse, weil Anbieter Zugangsdaten in den Pfad legen.
func fetch[T any](ctx context.Context, src string, timeout time.Duration, parse func(io.Reader) (T, error)) (out T, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	defer func() {
		if err != nil {
			err = fmt.Errorf("%s", strings.ReplaceAll(err.Error(), src, redact(src)))
		}
	}()
	var rc io.ReadCloser
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
		if e != nil {
			return out, e
		}
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			return out, e
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return out, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		rc = resp.Body
	} else if rc, err = os.Open(src); err != nil {
		return out, err
	}
	defer rc.Close()
	br := bufio.NewReader(io.LimitReader(rc, 1<<30))
	if b, _ := br.Peek(2); len(b) == 2 && b[0] == 0x1f && b[1] == 0x8b {
		gz, e := gzip.NewReader(br)
		if e != nil {
			return out, e
		}
		defer gz.Close()
		return parse(gz)
	}
	return parse(br)
}

// redact zeigt nur Schema und Rechner (für Status und Fehlertexte).
func redact(src string) string {
	if u, err := url.Parse(src); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return src
}
