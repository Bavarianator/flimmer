package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Bavarianator/flimmer/internal/auth"
	"github.com/Bavarianator/flimmer/internal/db"
)

// resetpwCmd setzt das Passwort direkt in der Datenbank – für den Fall, dass niemand mehr ins Admin-Konto kommt.
// Wer das kann, hat ohnehin Zugriff auf den Server-Ordner. Läuft auch bei laufendem Server.
func resetpwCmd(args []string) {
	fs := flag.NewFlagSet("resetpw", flag.ExitOnError)
	data := fs.String("data", "", "Datenordner des Servers (wie bei -data)")
	user := fs.String("user", "", "Benutzername (Standard: der erste Admin)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Aufruf: flimmer resetpw [-data ORDNER] [-user NAME] NEUES-PASSWORT")
	}
	fs.Parse(args)
	pw := fs.Arg(0)
	if len(pw) < 4 {
		fs.Usage()
		fmt.Fprintln(os.Stderr, "Das Passwort braucht mindestens 4 Zeichen.")
		os.Exit(2)
	}
	cfgDir, _ := dataDirs(*data)
	path := filepath.Join(cfgDir, "flimmer.db")
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintln(os.Stderr, "Keine Datenbank gefunden:", path, "(-data prüfen)")
		os.Exit(1)
	}
	store, err := db.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()
	ctx := context.Background()
	users, err := store.Users(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var target *db.User
	for i, u := range users {
		if (*user == "" && u.Admin) || (*user != "" && strings.EqualFold(u.Name, *user)) {
			target = &users[i]
			break
		}
	}
	if target == nil {
		fmt.Fprintln(os.Stderr, "Benutzer nicht gefunden. Vorhanden:")
		for _, u := range users {
			fmt.Fprintln(os.Stderr, " -", u.Name)
		}
		os.Exit(1)
	}
	_, err = store.UpdateUser(ctx, target.ID, func(u *db.User, _ int) error {
		u.PassHash = auth.HashPassword(pw)
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Passwort von %q ist gesetzt.\n", target.Name)
}
