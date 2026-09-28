// Package state hält Einstellungen, Benutzer, Sessions und Fortschritt in einer einzigen JSON-Datei.
// Backup = diese Datei kopieren.
// ponytail: JSON-Datei statt DB; SQLite erst, wenn >20 Nutzer oder Messungen es verlangen.
package state

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Version des Dateiformats. Beim Hochzählen wird vorher automatisch state.json.bak angelegt.
const Version = 1

type Data struct {
	Version  int                             `json:"version"`
	Settings Settings                        `json:"settings"`
	Users    []User                          `json:"users"`
	Sessions map[string]Session              `json:"sessions"` // Schlüssel: SHA-256 des Tokens, nie das Token selbst
	Progress map[string]map[string]Progress  `json:"progress"` // Benutzer → Titel
	Prefs    map[string]map[string]TrackPref `json:"prefs"`    // Benutzer → Serie → Sprachen
}

type Settings struct {
	ServerName string   `json:"serverName"`
	Language   string   `json:"language"`
	Dirs       []string `json:"dirs"`
	Secret     []byte   `json:"secret"` // HMAC-Schlüssel für Medien-Tokens
}

type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    int    `json:"color"` // Farbton 0–359 für den Avatar
	Admin    bool   `json:"admin"`
	PassHash string `json:"passHash,omitempty"` // leer = Profil ohne Passwort (nur im Heimnetz)
}

type Session struct {
	UserID   string    `json:"userId"`
	Device   string    `json:"device,omitempty"`
	LastSeen time.Time `json:"lastSeen"`
}

type Progress struct {
	Pos     float64   `json:"pos"`
	Dur     float64   `json:"dur"`
	Watched bool      `json:"watched"`
	Updated time.Time `json:"updated"`
}

type TrackPref struct {
	Audio    string `json:"audio,omitempty"`
	Subtitle string `json:"subtitle,omitempty"` // "off" = bewusst aus
}

type Store struct {
	path  string
	mu    sync.Mutex
	d     Data
	timer *time.Timer
}

// Open lädt die Datei. Ist sie kaputt, wird state.json.bak genommen; fehlt beides, beginnt ein leerer Zustand.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	err := load(path, &s.d)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("state: %v – versuche Backup", err)
		if err := load(path+".bak", &s.d); err != nil {
			return nil, fmt.Errorf("%s ist beschädigt und kein Backup lesbar: %w", path, err)
		}
	}
	if s.d.Version != 0 && s.d.Version < Version {
		if b, err := os.ReadFile(path); err == nil {
			if err := writeAtomic(path+".bak", b); err != nil {
				return nil, err
			}
		}
	}
	s.d.Version = Version
	if s.d.Sessions == nil {
		s.d.Sessions = map[string]Session{}
	}
	if s.d.Progress == nil {
		s.d.Progress = map[string]map[string]Progress{}
	}
	if s.d.Prefs == nil {
		s.d.Prefs = map[string]map[string]TrackPref{}
	}
	if len(s.d.Settings.Secret) == 0 {
		s.d.Settings.Secret = make([]byte, 32)
		rand.Read(s.d.Settings.Secret)
		if err := s.save(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func load(path string, d *Data) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, d)
}

// View liest unter Sperre. f darf nichts aus d nach außen geben, was später verändert wird (Maps kopieren).
func (s *Store) View(f func(d *Data)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.d)
}

// Update ändert unter Sperre und speichert gebündelt spätestens nach einer Sekunde.
func (s *Store) Update(f func(d *Data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := f(&s.d); err != nil {
		return err
	}
	if s.timer == nil {
		s.timer = time.AfterFunc(time.Second, func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.timer = nil
			if err := s.save(); err != nil {
				log.Printf("state: speichern fehlgeschlagen: %v", err)
			}
		})
	}
	return nil
}

// Flush schreibt sofort (beim Herunterfahren und nach wichtigen Änderungen wie dem Setup).
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	return s.save()
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(&s.d, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	return writeAtomic(s.path, b)
}

// writeAtomic: erst Temp-Datei, dann umbenennen – nach einem Absturz liegt nie eine halbe Datei da.
// 0600, weil Passwort-Hashes und Session-Hashes drinstehen.
func writeAtomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(f.Name(), 0o600)
	}
	if err != nil {
		os.Remove(f.Name())
		return err
	}
	return os.Rename(f.Name(), path)
}

// SetupDone: Es gibt einen Admin, der Assistent ist abgeschlossen.
func (d *Data) SetupDone() bool {
	for _, u := range d.Users {
		if u.Admin {
			return true
		}
	}
	return false
}

func (d *Data) User(id string) *User {
	for i := range d.Users {
		if d.Users[i].ID == id {
			return &d.Users[i]
		}
	}
	return nil
}

// SetProgress wendet die Regeln an: unter 3 % gilt als nicht angefangen, ab 90 % als gesehen.
func (d *Data) SetProgress(user, item string, pos, dur float64) Progress {
	p := Progress{Pos: pos, Dur: dur, Updated: time.Now()}
	switch {
	case dur > 0 && pos/dur >= 0.9:
		p.Pos, p.Watched = 0, true
	case dur > 0 && pos/dur < 0.03:
		p.Pos = 0
	}
	if d.Progress[user] == nil {
		d.Progress[user] = map[string]Progress{}
	}
	if p.Pos == 0 && !p.Watched {
		if old, ok := d.Progress[user][item]; ok && old.Watched {
			p.Watched = true // kurzes Reinschauen macht „gesehen“ nicht rückgängig
		} else {
			delete(d.Progress[user], item)
			return p
		}
	}
	d.Progress[user][item] = p
	return p
}

// NewID liefert eine kurze zufällige ID.
func NewID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
