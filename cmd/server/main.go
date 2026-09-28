// Flimmer – schlanker Medienserver.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
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
	"sync"
	"syscall"
	"time"

	"github.com/flimmer-media/flimmer/internal/api"
	"github.com/flimmer-media/flimmer/internal/db"
	"github.com/flimmer-media/flimmer/internal/discovery"
	"github.com/flimmer-media/flimmer/internal/ffmpeg"
	"github.com/flimmer-media/flimmer/internal/hwaccel"
	"github.com/flimmer-media/flimmer/internal/images"
	"github.com/flimmer-media/flimmer/internal/meta"
	"github.com/flimmer-media/flimmer/internal/remote"
	"github.com/flimmer-media/flimmer/internal/scan"
	"github.com/flimmer-media/flimmer/internal/service"
	"github.com/flimmer-media/flimmer/internal/setup"
	"github.com/flimmer-media/flimmer/internal/transcode"
	"github.com/flimmer-media/flimmer/internal/update"
	"github.com/flimmer-media/flimmer/web"
)

const defaultPort = 8096

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "install" || os.Args[1] == "uninstall") {
		serviceCmd(os.Args[1], os.Args[2:])
		return
	}
	addr := flag.String("addr", ":"+strconv.Itoa(defaultPort), "Adresse, auf der der Server lauscht")
	media := flag.String("media", "", "Medienordner, mehrere mit Komma getrennt (sonst aus der Einrichtung)")
	data := flag.String("data", "", "ein Ordner für alles (Einstellungen und Cache); Standard: Benutzerordner des Systems")
	every := flag.Duration("rescan", 15*time.Minute, "Abstand zwischen automatischen Scans")
	relayURL := flag.String("relay", "", "Rendezvous-Dienst für den Fernzugriff (leer = keine Prüfung von außen)")
	flag.Parse()
	ring := &api.LogRing{}
	log.SetOutput(io.MultiWriter(os.Stderr, ring))
	addrSet := false
	flag.Visit(func(f *flag.Flag) { addrSet = addrSet || f.Name == "addr" })

	cfgDir, cacheDir := dataDirs(*data)
	// Caches der Vorversion (vor SQLite) – der Katalog liegt jetzt in flimmer.db.
	os.Remove(filepath.Join(cacheDir, "probe-cache.json"))
	os.RemoveAll(filepath.Join(cacheDir, "keyframes"))
	tmp := filepath.Join(cacheDir, "transcode")
	os.RemoveAll(tmp) // Reste vom letzten Lauf
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		log.Fatal(err)
	}
	store, err := db.Open(filepath.Join(cfgDir, "flimmer.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	bg := context.Background()
	if ok, err := store.ImportState(bg, filepath.Join(cfgDir, "state.json")); err != nil {
		log.Fatalf("state.json übernehmen: %v", err)
	} else if ok {
		log.Print("Einstellungen aus state.json in die Datenbank übernommen (state.json.migrated bleibt als Sicherung)")
	}
	set, err := store.Settings(bg)
	if err != nil {
		log.Fatal(err)
	}
	setupDone := store.SetupDone(bg)

	// -media gewinnt (z. B. aus systemd/Docker) und wird übernommen, sonst gilt die Einrichtung.
	dirs := set.Dirs
	if *media != "" {
		dirs = nil
		for _, d := range strings.Split(*media, ",") {
			if d = strings.TrimSpace(d); d != "" {
				dirs = append(dirs, d)
			}
		}
		if err := store.UpdateSettings(bg, func(s *db.Settings) { s.Dirs = dirs }); err != nil {
			log.Fatal(err)
		}
	}

	ffDir := filepath.Join(cfgDir, "bin")
	hasFF := ffmpeg.Find(ffDir)
	if !hasFF {
		log.Print("ffmpeg/ffprobe nicht gefunden. " + ffmpeg.Hint())
	}

	ln, err := listen(*addr, !addrSet)
	if err != nil {
		log.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	res, err := meta.New(store.DB, cacheDir, set.TMDBKey)
	if err != nil {
		log.Fatal(err)
	}
	img := images.New(filepath.Join(cacheDir, "thumbs"))
	lib := scan.NewLibrary(store.DB, dirs)
	if err := lib.Load(bg); err != nil {
		log.Fatal(err)
	}
	lib.Meta = res
	lib.Colorize = func(ctx context.Context, it *scan.Item, m *meta.Meta) string {
		src, ok := api.ImageSource(it, m, res.ImgDir(), "poster")
		if !ok {
			return ""
		}
		c, _ := img.Color(ctx, src)
		return c
	}
	hls := transcode.NewManager(tmp)
	lanURL := discovery.LANURL(port)
	srv := &api.Server{Lib: lib, HLS: hls, DB: store, Meta: res, Images: img, CacheDir: cacheDir,
		Web: web.FS(), Pages: setup.FS(), LANURL: lanURL, QR: discovery.QRHandler(port), Log: ring, Updates: &update.Checker{}}
	if rem, err := remote.New(remote.Options{Port: port, KeyFile: filepath.Join(cfgDir, "remote.key"), RelayURL: *relayURL}); err != nil {
		log.Printf("Fernzugriff: %v", err)
	} else {
		srv.Remote = rem
		var mu sync.Mutex
		var stopRemote context.CancelFunc
		srv.RemoteToggle = func(on bool) {
			mu.Lock()
			defer mu.Unlock()
			if stopRemote != nil { // Run gibt die Router-Freigaben beim Ende von ctx wieder frei
				stopRemote()
				stopRemote = nil
			}
			if on {
				var rctx context.Context
				rctx, stopRemote = context.WithCancel(ctx)
				go rem.Run(rctx)
			}
		}
		if set.Remote {
			srv.RemoteToggle(true)
		}
	}
	go srv.Updates.Run(ctx, func() bool {
		s, err := store.Settings(ctx)
		return err == nil && !s.NoUpdates
	})
	srv.FFmpeg.Store(hasFF)

	// Scan und Hardware-Erkennung brauchen ffmpeg – starten sofort oder nach dem Download aus der Einrichtung.
	var startMedia sync.Once
	start := func() {
		startMedia.Do(func() {
			go lib.Run(ctx, *every)
			go func() {
				a := hwaccel.Detect(ctx)
				log.Printf("Transcoding: %s (%.1f× Echtzeit bei 1080p)", a.Name, a.Speed)
				srv.HW.Store(&a)
			}()
		})
	}
	srv.FF = &ffmpeg.Installer{Dir: ffDir, OnDone: func() {
		if ffmpeg.Find(ffDir) {
			log.Print("ffmpeg installiert")
			srv.FFmpeg.Store(true)
			start()
		}
	}}
	if hasFF {
		start()
	}

	httpSrv := &http.Server{Handler: srv.Handler()}
	go func() {
		if err := httpSrv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	name := set.ServerName
	if name == "" {
		name, _ = os.Hostname()
	}
	if err := discovery.Start(ctx, name, port); err != nil {
		log.Printf("Discovery: %v", err)
	}

	local := "http://localhost:" + strconv.Itoa(port)
	log.Printf("Flimmer läuft auf %s", local)
	if lanURL != "" {
		log.Printf("Im Heimnetz (TV, Handy): %s", lanURL)
	}
	if !setupDone {
		log.Printf("Zur Einrichtung im Browser öffnen: %s/setup", local)
		if desktop() {
			openBrowser(local + "/setup")
		}
	}

	<-ctx.Done()
	log.Print("Flimmer wird beendet …")
	hls.Close() // wartende Segment-Anfragen enden sofort statt nach Timeout
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpSrv.Shutdown(shutdown)
	hls.Close() // falls während des Herunterfahrens noch ein Segment angefragt wurde
}

// dataDirs: Einstellungen in den Konfig-Ordner (wird nicht „aufgeräumt“), Caches in den Cache-Ordner.
// Mit -data (Docker, systemd) liegt beides in einem Ordner.
func dataDirs(data string) (cfg, cache string) {
	if data != "" {
		return data, data
	}
	cfg, cache = ".flimmer", ".flimmer"
	if d, err := os.UserConfigDir(); err == nil {
		cfg = filepath.Join(d, "flimmer")
	}
	if d, err := os.UserCacheDir(); err == nil {
		cache = filepath.Join(d, "flimmer")
	}
	return cfg, cache
}

// serviceCmd: „flimmer install [flags]“ richtet den Autostart ein, „flimmer uninstall“ entfernt ihn.
// Übergebene Flags (z. B. -addr) landen im Dienst.
func serviceCmd(cmd string, args []string) {
	var msg string
	var err error
	if cmd == "install" {
		exe, _ := os.Executable()
		if resolved, e := filepath.EvalSymlinks(exe); e == nil {
			exe = resolved
		}
		data := ""
		for i, a := range args {
			if (a == "-data" || a == "--data") && i+1 < len(args) {
				data = args[i+1]
			}
		}
		cfg, _ := dataDirs(data) // der Dienst findet so dieselbe flimmer.db wie der Start per Doppelklick
		msg, err = service.Install(exe, cfg, args...)
	} else {
		msg, err = service.Uninstall()
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(msg)
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

// desktop meldet, ob ein Mensch vor dem Bildschirm sitzt – nicht in Docker, nicht als Dienst.
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
