// Package ffmpeg findet ffmpeg/ffprobe und lädt statische Builds bei Bedarf herunter – jeweils mit SHA-256-Prüfung.
//
//   - Windows x64, Linux x64/arm64: BtbN-Builds (GitHub), Prüfsumme aus checksums.sha256 desselben Releases
//     (BtbN baut rollierend, feste Hashes würden nach wenigen Tagen veralten).
//   - macOS arm64/x64: osxexperts.net, Prüfsummen der Binaries fest eingebaut; ändert sich die Datei, bricht
//     der Download sicher ab und die Einrichtung zeigt „brew install ffmpeg“.
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

// btbn: Plattformname in den BtbN-Dateinamen.
var btbn = map[string]string{"windows/amd64": "win64", "linux/amd64": "linux64", "linux/arm64": "linuxarm64"}

type pinned struct{ url, file, sha string }

var mac = map[string][]pinned{
	"darwin/arm64": {
		{"https://www.osxexperts.net/ffmpeg9arm.zip", "ffmpeg", "591260c945d0eef150e3bf82b0ef988bd36a9cecc18ff05d6679617159f0a95e"},
		{"https://www.osxexperts.net/ffprobe9arm.zip", "ffprobe", "e11c17e8200b3ee4c4c186d245e2b4053f01d56957336c1817fca0b997469106"},
	},
	"darwin/amd64": {
		{"https://www.osxexperts.net/ffmpeg80intel.zip", "ffmpeg", "df3f1e3facdc1ae0ad0bd898cdfb072fbc9641bf47b11f172844525a05db8d11"},
		{"https://www.osxexperts.net/ffprobe80intel.zip", "ffprobe", "5228e651e2bd67bb55819b27f6138351587b16d2b87446007bf35b7cf930d891"},
	},
}

func platform() string { return runtime.GOOS + "/" + runtime.GOARCH }

// CanDownload: Gibt es für dieses System einen geprüften Build? Linux braucht zum Entpacken tar und xz.
func CanDownload() bool {
	p := platform()
	if mac[p] != nil {
		return true
	}
	if btbn[p] == "" {
		return false // z. B. 32-bit-Raspberry-Pi: dort per apt
	}
	if runtime.GOOS == "linux" {
		_, e1 := exec.LookPath("tar")
		_, e2 := exec.LookPath("xz")
		return e1 == nil && e2 == nil
	}
	return true
}

// Hint erklärt, wie man ffmpeg auf diesem System bekommt.
func Hint() string {
	auto := ""
	if CanDownload() {
		auto = "Auf „Automatisch installieren“ klicken – oder selbst: "
	}
	switch runtime.GOOS {
	case "darwin":
		return auto + "im Terminal brew install ffmpeg, danach Flimmer neu starten."
	case "windows":
		return auto + "im Terminal winget install ffmpeg, danach Flimmer neu starten."
	default:
		return auto + "im Terminal sudo apt install ffmpeg (bzw. dnf/pacman), danach Flimmer neu starten. Im Docker-Image ist ffmpeg schon dabei."
	}
}

const release = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/"

// Installer lädt ffmpeg im Hintergrund herunter und meldet den Fortschritt.
type Installer struct {
	Dir    string // Ziel, z. B. <config>/bin
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
		s.Percent = int(min(i.read*100/i.total, 100))
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
	if !CanDownload() {
		return errors.New("für dieses System gibt es keinen automatischen Download")
	}
	if err := os.MkdirAll(i.Dir, 0o755); err != nil {
		return err
	}
	if pins := mac[platform()]; pins != nil {
		for _, p := range pins {
			if err := i.installPinned(ctx, p); err != nil {
				return err
			}
		}
		return nil
	}

	sums, err := get(ctx, release+"checksums.sha256", nil, nil)
	if err != nil {
		return err
	}
	name, want, err := pickAsset(sums, btbn[platform()])
	if err != nil {
		return err
	}
	archive, err := i.download(ctx, release+name, want)
	if err != nil {
		return err
	}
	defer os.Remove(archive)
	if strings.HasSuffix(name, ".zip") {
		for _, bin := range []string{"ffmpeg.exe", "ffprobe.exe"} {
			if err := extractZip(archive, bin, filepath.Join(i.Dir, bin), ""); err != nil {
				return err
			}
		}
		return nil
	}
	return extractTarXZ(ctx, archive, i.Dir)
}

// installPinned lädt ein ZIP, holt die Binary heraus und prüft deren fest eingebauten Hash.
func (i *Installer) installPinned(ctx context.Context, p pinned) error {
	archive, err := i.download(ctx, p.url, "")
	if err != nil {
		return err
	}
	defer os.Remove(archive)
	return extractZip(archive, p.file, filepath.Join(i.Dir, p.file), p.sha)
}

// download speichert url in eine Temp-Datei; mit want wird die SHA-256 der Datei geprüft.
func (i *Installer) download(ctx context.Context, url, want string) (string, error) {
	tmp, err := os.CreateTemp("", "ffmpeg-dl-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()
	h := sha256.New()
	if _, err := get(ctx, url, io.MultiWriter(tmp, h, (*counter)(i)), (*counter)(i)); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	if want != "" && hex.EncodeToString(h.Sum(nil)) != want {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("Prüfsumme von %s stimmt nicht – Download abgebrochen", filepath.Base(url))
	}
	return tmp.Name(), nil
}

// get lädt url; mit w wird gestreamt (c erfährt die Gesamtgröße), sonst der Inhalt zurückgegeben.
func get(ctx context.Context, url string, w io.Writer, c *counter) ([]byte, error) {
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
	if c != nil {
		c.addTotal(res.ContentLength)
	}
	_, err = io.Copy(w, res.Body)
	return nil, err
}

// Stabile Release-Builds heißen z. B. ffmpeg-n9.0-latest-linux64-gpl-9.0.tar.xz; die höchste Version gewinnt.
func pickAsset(sums []byte, plat string) (name, sum string, err error) {
	re := regexp.MustCompile(`^([0-9a-f]{64})\s+(ffmpeg-n(\d+)\.(\d+)-latest-` + regexp.QuoteMeta(plat) + `-gpl-[\d.]+\.(?:zip|tar\.xz))$`)
	best := -1
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		m := re.FindStringSubmatch(strings.TrimSpace(sc.Text()))
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

// extractZip holt die Datei base aus dem Archiv nach dst; mit want wird ihre SHA-256 geprüft.
func extractZip(zipPath, base, dst, want string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if filepath.Base(f.Name) != base || strings.Contains(f.Name, "__MACOSX") {
			continue
		}
		src, err := f.Open()
		if err != nil {
			return err
		}
		defer src.Close()
		return writeBinary(src, dst, want)
	}
	return fmt.Errorf("%s nicht im Archiv", base)
}

// extractTarXZ entpackt ffmpeg und ffprobe mit dem System-tar (spart eine xz-Bibliothek).
func extractTarXZ(ctx context.Context, archive, dir string) error {
	tmp, err := os.MkdirTemp(dir, "unpack-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if b, err := exec.CommandContext(ctx, "tar", "-xJf", archive, "-C", tmp).CombinedOutput(); err != nil {
		return fmt.Errorf("Entpacken fehlgeschlagen: %v %s", err, b)
	}
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		matches, _ := filepath.Glob(filepath.Join(tmp, "*", "bin", bin))
		if len(matches) != 1 {
			return fmt.Errorf("%s nicht im Archiv", bin)
		}
		f, err := os.Open(matches[0])
		if err != nil {
			return err
		}
		err = writeBinary(f, filepath.Join(dir, bin), "")
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// writeBinary schreibt atomar und ausführbar; mit want muss die SHA-256 passen, sonst bleibt nichts liegen.
func writeBinary(src io.Reader, dst, want string) error {
	out, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".*")
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, h), src)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && want != "" && hex.EncodeToString(h.Sum(nil)) != want {
		err = fmt.Errorf("Prüfsumme von %s stimmt nicht – Download abgebrochen", filepath.Base(dst))
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

// counter zählt heruntergeladene Bytes über alle Dateien eines Downloads.
type counter Installer

func (c *counter) addTotal(n int64) {
	c.mu.Lock()
	c.total += max(n, 0)
	c.mu.Unlock()
}

func (c *counter) Write(p []byte) (int, error) {
	c.mu.Lock()
	c.read += int64(len(p))
	c.mu.Unlock()
	return len(p), nil
}
