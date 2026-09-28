// Flimmer – schlanker Medienserver.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/flimmer-media/flimmer/internal/api"
	"github.com/flimmer-media/flimmer/internal/scan"
	"github.com/flimmer-media/flimmer/internal/transcode"
	"github.com/flimmer-media/flimmer/web"
)

const defaultPort = 8096

func main() {
	addr := flag.String("addr", ":"+strconv.Itoa(defaultPort), "Adresse, auf der der Server lauscht")
	media := flag.String("media", "", "Medienordner, mehrere mit Komma getrennt")
	data := flag.String("data", defaultDataDir(), "Ordner für Cache und Datenbank")
	every := flag.Duration("rescan", 15*time.Minute, "Abstand zwischen automatischen Scans")
	flag.Parse()
	addrSet := false
	flag.Visit(func(f *flag.Flag) { addrSet = addrSet || f.Name == "addr" })

	tmp := filepath.Join(*data, "transcode")
	os.RemoveAll(tmp) // Reste vom letzten Lauf
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		log.Fatal(err)
	}
	_, err := os.Stat(filepath.Join(*data, "probe-cache.json"))
	firstStart := errors.Is(err, os.ErrNotExist)

	var dirs []string
	for _, d := range strings.Split(*media, ",") {
		if d = strings.TrimSpace(d); d != "" {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		log.Print("Noch kein Medienordner angegeben – starte mit -media /pfad/zu/filmen")
	}

	ffmpeg := findFFmpeg(*data)
	if !ffmpeg {
		log.Print("ffmpeg/ffprobe nicht gefunden. Installieren (z. B. „sudo apt install ffmpeg“, „brew install ffmpeg“, „winget install ffmpeg“) " +
			"oder ffmpeg und ffprobe neben flimmer bzw. nach " + filepath.Join(*data, "bin") + " legen.")
	}

	ln, err := listen(*addr, !addrSet)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	lib := scan.NewLibrary(dirs, *data)
	if ffmpeg {
		go lib.Run(ctx, *every)
	}
	hls := transcode.NewManager(tmp)
	srv := &http.Server{Handler: (&api.Server{Lib: lib, HLS: hls, CacheDir: *data, Web: web.FS(), FFmpeg: ffmpeg}).Handler()}
	go func() {
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	local := "http://localhost:" + strconv.Itoa(port)
	log.Printf("Flimmer läuft auf %s", local)
	for _, ip := range lanIPs() {
		log.Printf("Im Heimnetz (TV, Handy): http://%s", net.JoinHostPort(ip, strconv.Itoa(port)))
	}
	if firstStart && desktop() {
		openBrowser(local)
	}

	<-ctx.Done()
	log.Print("Flimmer wird beendet …")
	hls.Close() // wartende Segment-Anfragen enden sofort statt nach Timeout
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdown)
	hls.Close() // falls während des Herunterfahrens noch ein Segment angefragt wurde
}

func defaultDataDir() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "flimmer")
	}
	return ".flimmer"
}

// findFFmpeg bevorzugt ffmpeg neben der Binary oder im Datenordner (für Nutzer ohne Paketmanager), sonst PATH.
func findFFmpeg(data string) bool {
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	for _, d := range append(dirs, filepath.Join(data, "bin")) {
		if _, err := os.Stat(filepath.Join(d, name)); err == nil {
			os.Setenv("PATH", d+string(os.PathListSeparator)+os.Getenv("PATH"))
			break
		}
	}
	_, e1 := exec.LookPath("ffmpeg")
	_, e2 := exec.LookPath("ffprobe")
	return e1 == nil && e2 == nil
}

// listen weicht auf die nächsten Ports aus, wenn der Standardport belegt ist (z. B. durch Jellyfin).
func listen(addr string, fallback bool) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err == nil || !fallback {
		return ln, err
	}
	for p := defaultPort + 1; p <= defaultPort+4; p++ {
		if ln, e := net.Listen("tcp", ":"+strconv.Itoa(p)); e == nil {
			log.Printf("Port %d ist belegt, nutze %d", defaultPort, p)
			return ln, nil
		}
	}
	return nil, err
}

func lanIPs() []string {
	addrs, _ := net.InterfaceAddrs()
	var out []string
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && n.IP.IsPrivate() {
			out = append(out, n.IP.String())
		}
	}
	return out
}

// desktop meldet, ob ein Mensch vor dem Bildschirm sitzt – nicht in Docker, nicht als systemd-Dienst.
func desktop() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil || os.Getenv("INVOCATION_ID") != "" {
		return false
	}
	if runtime.GOOS == "linux" {
		return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
	}
	return true
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err == nil {
		go cmd.Wait()
	}
}
