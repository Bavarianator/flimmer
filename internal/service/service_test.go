package service

import (
	"encoding/xml"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSystemdUnit(t *testing.T) {
	u := systemdUnit("/opt/Mein Ordner/flimmer", []string{"-data", "/home/a/100%"})
	for _, want := range []string{
		`ExecStart="/opt/Mein Ordner/flimmer" "-data" "/home/a/100%%"`,
		"WantedBy=default.target",
		"Restart=on-failure",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("fehlt: %s\n%s", want, u)
		}
	}
	if strings.Contains(u, "User=") {
		t.Error("User-Unit darf kein User= setzen")
	}
}

func TestLaunchdPlist(t *testing.T) {
	p := launchdPlist("/Applications/Flimmer & Co/flimmer", []string{"-data", "/Users/a/d"}, "/Users/a/d/flimmer.log")
	var v struct {
		Strings []string `xml:"dict>array>string"`
	}
	if err := xml.Unmarshal([]byte(p), &v); err != nil {
		t.Fatalf("kein gültiges XML: %v\n%s", err, p)
	}
	if want := []string{"/Applications/Flimmer & Co/flimmer", "-data", "/Users/a/d"}; !slices.Equal(v.Strings, want) {
		t.Errorf("ProgramArguments = %q, will %q", v.Strings, want)
	}
	if !strings.Contains(p, "<string>io.flimmer.server</string>") || !strings.Contains(p, "<key>RunAtLoad</key>") {
		t.Error(p)
	}
}

func TestStartupScript(t *testing.T) {
	s := startupScript(`C:\Program Files\Flimmer\flimmer.exe`, []string{"-data", `C:\Users\a\Flimmer`})
	want := `CreateObject("WScript.Shell").Run """C:\Program Files\Flimmer\flimmer.exe"" ""-data"" ""C:\Users\a\Flimmer""", 0, False`
	if !strings.Contains(s, want) {
		t.Errorf("\n got %s\nwant %s", s, want)
	}
}

// fake ersetzt OS, Home und Kommandos; liefert die Liste der ausgeführten Kommandos.
func fake(t *testing.T, os_ string, failing ...string) (home string, cmds *[]string) {
	home = t.TempDir()
	var calls []string
	oldGoos, oldHome, oldRun := goos, homeDir, run
	t.Cleanup(func() { goos, homeDir, run = oldGoos, oldHome, oldRun })
	goos = os_
	homeDir = func() (string, error) { return home, nil }
	run = func(name string, args ...string) error {
		c := name + " " + strings.Join(args, " ")
		calls = append(calls, c)
		for _, f := range failing {
			if strings.HasPrefix(c, f) {
				return errors.New("geht nicht")
			}
		}
		return nil
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", "")
	return home, &calls
}

func TestInstallUninstallLinux(t *testing.T) {
	home, cmds := fake(t, "linux", "loginctl")
	msg, err := Install("/usr/local/bin/flimmer", "/var/lib/flimmer")
	if err != nil {
		t.Fatal(err)
	}
	unit := filepath.Join(home, ".config/systemd/user/flimmer.service")
	if b, err := os.ReadFile(unit); err != nil || !strings.Contains(string(b), `"-data" "/var/lib/flimmer"`) {
		t.Fatalf("Unit: %s %v", b, err)
	}
	if !slices.Contains(*cmds, "systemctl --user enable --now flimmer.service") {
		t.Errorf("nicht gestartet: %q", *cmds)
	}
	if !strings.Contains(msg, "sudo loginctl enable-linger") {
		t.Errorf("Linger-Hinweis fehlt: %s", msg)
	}

	if _, err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(unit); !errors.Is(err, os.ErrNotExist) {
		t.Error("Unit nicht entfernt")
	}
	if !slices.Contains(*cmds, "systemctl --user disable --now flimmer.service") {
		t.Errorf("nicht gestoppt: %q", *cmds)
	}
	if msg, err := Uninstall(); err != nil || !strings.Contains(msg, "nicht") {
		t.Errorf("zweites Uninstall: %q %v", msg, err)
	}
}

func TestInstallFehlerWirdGemeldet(t *testing.T) {
	fake(t, "linux", "systemctl --user enable")
	if _, err := Install("/bin/flimmer", "/tmp/x"); err == nil {
		t.Error("Fehler von systemctl verschluckt")
	}
}

func TestInstallWindows(t *testing.T) {
	home, cmds := fake(t, "windows")
	if _, err := Install("/c/flimmer.exe", "/c/data"); err != nil {
		t.Fatal(err)
	}
	vbs := filepath.Join(home, "AppData/Roaming/Microsoft/Windows/Start Menu/Programs/Startup/Flimmer.vbs")
	if _, err := os.Stat(vbs); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(*cmds, "wscript.exe "+vbs) {
		t.Errorf("nicht gestartet: %q", *cmds)
	}
}

func TestUnbekanntesOS(t *testing.T) {
	fake(t, "plan9")
	if _, err := Install("/bin/flimmer", "/tmp/x"); err == nil {
		t.Error("plan9 sollte abgelehnt werden")
	}
}

// Die erzeugte Unit muss systemd wirklich gefallen (Quoting, Escapes).
func TestSystemdVerify(t *testing.T) {
	if _, err := exec.LookPath("systemd-analyze"); err != nil {
		t.Skip("systemd-analyze fehlt")
	}
	p := filepath.Join(t.TempDir(), "flimmer.service")
	os.WriteFile(p, []byte(systemdUnit("/bin/true", []string{"-data", "/tmp/mit leer zeichen/100%"})), 0o644)
	if out, err := exec.Command("systemd-analyze", "--user", "verify", p).CombinedOutput(); err != nil {
		t.Errorf("%v: %s", err, out)
	}
}
