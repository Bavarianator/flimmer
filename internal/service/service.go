// Package service richtet Flimmer als Autostart des aktuellen Benutzers ein – ohne root/Admin:
// systemd-User-Unit (Linux), LaunchAgent (macOS), Startup-Ordner (Windows).
package service

import (
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Austauschbar für Tests.
var (
	goos    = runtime.GOOS
	homeDir = os.UserHomeDir
	run     = func(name string, args ...string) error {
		out, err := exec.Command(name, args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
		return nil
	}
)

const label = "io.flimmer.server"

// Install trägt bin mit -data dataDir (leer = Standardordner) und weiteren args als Autostart ein und startet ihn sofort.
// Der Rückgabetext sagt dem Nutzer auf Deutsch, was passiert ist.
func Install(bin, dataDir string, args ...string) (string, error) {
	bin, err := filepath.Abs(bin)
	if err != nil {
		return "", err
	}
	if dataDir != "" { // leer = Standardordner des Servers; Abs("") wäre sonst das aktuelle Verzeichnis
		if dataDir, err = filepath.Abs(dataDir); err != nil {
			return "", err
		}
		args = append([]string{"-data", dataDir}, args...)
	}
	path, err := unitPath()
	if err != nil {
		return "", err
	}
	var content string
	switch goos {
	case "linux":
		content = systemdUnit(bin, args)
	case "darwin":
		logDir := dataDir
		if logDir == "" {
			logDir = filepath.Join(filepath.Dir(path), "..", "Logs")
		}
		content = launchdPlist(bin, args, filepath.Join(logDir, "flimmer.log"))
	case "windows":
		content = startupScript(bin, args)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}

	switch goos {
	case "linux":
		if err := run("systemctl", "--user", "daemon-reload"); err != nil {
			return "", err
		}
		if err := run("systemctl", "--user", "enable", "--now", "flimmer.service"); err != nil {
			return "", err
		}
		msg := "Flimmer läuft jetzt als Dienst (" + path + ") und startet automatisch mit.\nStatus: systemctl --user status flimmer"
		// Ohne Linger startet ein User-Dienst erst bei der Anmeldung und endet beim Abmelden.
		if err := run("loginctl", "enable-linger"); err != nil {
			msg += "\nHinweis: Damit Flimmer schon beim Hochfahren startet (ohne Anmeldung), einmal ausführen: sudo loginctl enable-linger " + os.Getenv("USER")
		}
		return msg, nil
	case "darwin":
		uid := fmt.Sprint(os.Getuid())
		_ = run("launchctl", "bootout", "gui/"+uid+"/"+label) // alte Version entladen, Fehler egal
		if err := run("launchctl", "bootstrap", "gui/"+uid, path); err != nil {
			return "", err
		}
		return "Flimmer läuft jetzt im Hintergrund (" + path + ") und startet bei der Anmeldung automatisch.", nil
	default:
		if err := run("wscript.exe", path); err != nil {
			return "", err
		}
		return "Flimmer läuft jetzt im Hintergrund und startet bei der Anmeldung automatisch (" + path + ").", nil
	}
}

// Uninstall stoppt den Autostart und entfernt den Eintrag. Daten und Medien bleiben unangetastet.
func Uninstall() (string, error) {
	path, err := unitPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return "Flimmer war nicht als Dienst eingerichtet.", nil
	}
	msg := "Autostart entfernt. Deine Daten bleiben erhalten."
	switch goos {
	case "linux":
		if err := run("systemctl", "--user", "disable", "--now", "flimmer.service"); err != nil {
			return "", err
		}
	case "darwin":
		_ = run("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", os.Getuid(), label))
	case "windows":
		// ponytail: den laufenden Prozess beenden wir nicht (taskkill träfe auch diesen Aufruf); endet beim Abmelden.
		msg += " Der laufende Server endet beim nächsten Abmelden."
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	if goos == "linux" {
		_ = run("systemctl", "--user", "daemon-reload")
	}
	return msg, nil
}

func unitPath() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	switch goos {
	case "linux":
		if cfg := os.Getenv("XDG_CONFIG_HOME"); cfg != "" {
			return filepath.Join(cfg, "systemd", "user", "flimmer.service"), nil
		}
		return filepath.Join(home, ".config", "systemd", "user", "flimmer.service"), nil
	case "darwin":
		return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "Flimmer.vbs"), nil
	}
	return "", fmt.Errorf("Autostart wird auf %s nicht unterstützt", goos)
}

// systemdUnit ist die User-Variante von deploy/flimmer.service: kein User=, WantedBy=default.target.
// Die Härtungsoptionen der System-Unit fehlen, weil sie in User-Units eigene Namespaces bräuchten.
func systemdUnit(bin string, args []string) string {
	q := func(s string) string { // systemd: Anführungszeichen für Leerzeichen, % ist ein Platzhalterzeichen
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%").Replace(s) + `"`
	}
	cmd := q(bin)
	for _, a := range args {
		cmd += " " + q(a)
	}
	return `[Unit]
Description=Flimmer Medienserver
Documentation=https://github.com/Bavarianator/flimmer
After=network-online.target

[Service]
ExecStart=` + cmd + `
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`
}

func launchdPlist(bin string, args []string, logFile string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + label + `</string>
	<key>ProgramArguments</key>
	<array>
`)
	for _, a := range append([]string{bin}, args...) {
		b.WriteString("\t\t<string>" + html.EscapeString(a) + "</string>\n")
	}
	b.WriteString(`	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>StandardOutPath</key>
	<string>` + html.EscapeString(logFile) + `</string>
	<key>StandardErrorPath</key>
	<string>` + html.EscapeString(logFile) + `</string>
</dict>
</plist>
`)
	return b.String()
}

// startupScript startet Flimmer ohne Konsolenfenster (Fenster-Stil 0).
// ponytail: VBScript ist bei Microsoft abgekündigt; wenn es wegfällt, die Windows-Binary mit -H windowsgui bauen.
func startupScript(bin string, args []string) string {
	q := func(s string) string { return `""` + strings.ReplaceAll(s, `"`, `""""`) + `""` } // VBS-String im Kommando
	cmd := q(bin)
	for _, a := range args {
		cmd += " " + q(a)
	}
	return "' Flimmer-Autostart, entfernen mit: flimmer uninstall\r\n" +
		`CreateObject("WScript.Shell").Run "` + cmd + `", 0, False` + "\r\n"
}
