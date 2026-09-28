// Flimmer – schlanker Medienserver.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/flimmer-media/flimmer/internal/api"
	"github.com/flimmer-media/flimmer/internal/scan"
	"github.com/flimmer-media/flimmer/internal/transcode"
	"github.com/flimmer-media/flimmer/web"
)

func main() {
	addr := flag.String("addr", ":8096", "Adresse, auf der der Server lauscht")
	media := flag.String("media", "", "Medienordner, mehrere mit Komma getrennt")
	data := flag.String("data", defaultDataDir(), "Ordner für Cache und Datenbank")
	flag.Parse()
	if *media == "" {
		log.Fatal("bitte -media /pfad/zu/filmen angeben")
	}

	tmp := filepath.Join(*data, "transcode")
	os.RemoveAll(tmp) // Reste vom letzten Lauf
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		log.Fatal(err)
	}

	lib := &scan.Library{Dirs: strings.Split(*media, ","), CacheFile: filepath.Join(*data, "probe-cache.json")}
	if err := lib.Scan(context.Background()); err != nil {
		log.Fatal(err)
	}
	log.Printf("%d Titel gefunden", len(lib.All()))

	srv := &api.Server{Lib: lib, HLS: transcode.NewManager(tmp), CacheDir: *data, Web: web.FS()}
	log.Printf("Flimmer läuft auf http://localhost%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}

func defaultDataDir() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "flimmer")
	}
	return ".flimmer"
}
