// Package ffmpeg findet ffmpeg/ffprobe und lädt sie unter Windows bei Bedarf herunter.
// Linux/macOS: Paketmanager bzw. Docker-Image bringen ffmpeg mit, dort zeigt die Einrichtung eine Anleitung.
package ffmpeg

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
)

// Find ergänzt PATH um ffmpeg neben der Binary oder in dir und meldet, ob ffmpeg und ffprobe nutzbar sind.
func Find(dir string) bool {
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	for _, d := range append(dirs, dir) {
		if _, err := os.Stat(filepath.Join(d, name)); err == nil {
			if !slices.Contains(filepath.SplitList(os.Getenv("PATH")), d) {
				os.Setenv("PATH", d+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			break
		}
	}
	_, e1 := exec.LookPath("ffmpeg")
	_, e2 := exec.LookPath("ffprobe")
	return e1 == nil && e2 == nil
}

// CanDownload: nur für Windows x64 gibt es verlässliche statische Builds (BtbN).
func CanDownload() bool { return runtime.GOOS == "windows" && runtime.GOARCH == "amd64" }

// Hint erklärt, wie man ffmpeg auf diesem System installiert.
func Hint() string {
	switch runtime.GOOS {
	case "darwin":
		return "Im Terminal: brew install ffmpeg – danach Flimmer neu starten."
	case "windows":
		return "Auf „Automatisch installieren“ klicken oder im Terminal: winget install ffmpeg"
	default:
		return "Im Terminal: sudo apt install ffmpeg (bzw. dnf/pacman) – danach Flimmer neu starten. Oder das Docker-Image nutzen, dort ist ffmpeg dabei."
	}
}

const release = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/"

// Installer lädt ffmpeg im Hintergrund herunter und meldet den Fortschritt.
type Installer struct {
	Dir    string // Ziel, z. B. <data>/bin
	OnDone func()

	mu          sync.Mutex
	running     bool
	read, total int64
	err         string
}

type Status struct {
	Running bool   `json:"running"`
	Percent int    `json:"percent"`
	Error   string `json:"error,omitempty"`
}

func (i *Installer) Status() Status {
	i.mu.Lock()
	defer i.mu.Unlock()
	s := Status{Running: i.running, Error: i.err}
	if i.total > 0 {
		s.Percent = int(i.read * 100 / i.total)
	}
	return s
}

// Start beginnt den Download, falls noch keiner läuft.
func (i *Installer) Start(ctx context.Context) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.running {
		return
	}
	i.running, i.err, i.read, i.total = true, "", 0, 0
	go func() {
		err := i.install(ctx)
		i.mu.Lock()
		i.running = false
		if err != nil {
			i.err = err.Error()
		}
		i.mu.Unlock()
		if err == nil && i.OnDone != nil {
			i.OnDone()
		}
	}()
}

func (i *Installer) install(ctx context.Context) error {
	sums, err := get(ctx, release+"checksums.sha256", nil)
	if err != nil {
		return err
	}
	name, want, err := pickAsset(sums)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp("", "ffmpeg-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	h := sha256.New()
	if _, err := get(ctx, release+name, io.MultiWriter(tmp, h, (*counter)(i))); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("Prüfsumme von %s stimmt nicht – Download abgebrochen", name)
	}
	return extract(tmp.Name(), i.Dir)
}

// get lädt url; mit w wird gestreamt, sonst der Inhalt zurückgegeben.
func get(ctx context.Context, url string, w io.Writer) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Download fehlgeschlagen – besteht eine Internetverbindung? (%w)", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Download fehlgeschlagen: %s", res.Status)
	}
	if w == nil {
		return io.ReadAll(io.LimitReader(res.Body, 1<<20))
	}
	if c, ok := w.(interface{ setTotal(int64) }); ok {
		c.setTotal(res.ContentLength)
	}
	_, err = io.Copy(w, res.Body)
	return nil, err
}

// Stabile Release-Builds heißen z. B. ffmpeg-n9.0-latest-win64-gpl-9.0.zip; der höchste gewinnt.
var reAsset = regexp.MustCompile(`^([0-9a-f]{64})\s+(ffmpeg-n(\d+)\.(\d+)-latest-win64-gpl-[\d.]+\.zip)$`)

func pickAsset(sums []byte) (name, sum string, err error) {
	best := -1
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		m := reAsset.FindStringSubmatch(strings.TrimSpace(sc.Text()))
		if m == nil {
			continue
		}
		var major, minor int
		fmt.Sscan(m[3], &major)
		fmt.Sscan(m[4], &minor)
		if v := major*1000 + minor; v > best {
			best, name, sum = v, m[2], m[1]
		}
	}
	if best < 0 {
		return "", "", errors.New("kein passender ffmpeg-Build gefunden")
	}
	return name, sum, nil
}

// extract holt nur ffmpeg.exe und ffprobe.exe aus dem Archiv.
func extract(zipPath, dir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	found := 0
	for _, f := range r.File {
		base := filepath.Base(f.Name)
		if base != "ffmpeg.exe" && base != "ffprobe.exe" {
			continue
		}
		if err := extractFile(f, filepath.Join(dir, base)); err != nil {
			return err
		}
		found++
	}
	if found != 2 {
		return errors.New("ffmpeg.exe/ffprobe.exe nicht im Archiv")
	}
	return nil
}

func extractFile(f *zip.File, dst string) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".*")
	if err != nil {
		return err
	}
	_, err = io.Copy(out, src)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(out.Name(), 0o755)
	}
	if err != nil {
		os.Remove(out.Name())
		return err
	}
	return os.Rename(out.Name(), dst)
}

type counter Installer

func (c *counter) setTotal(n int64) {
	c.mu.Lock()
	c.total = n
	c.mu.Unlock()
}

func (c *counter) Write(p []byte) (int, error) {
	c.mu.Lock()
	c.read += int64(len(p))
	c.mu.Unlock()
	return len(p), nil
}
