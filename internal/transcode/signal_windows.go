package transcode

import "os"

// ponytail: Windows kann Prozesse nicht per Signal anhalten; ffmpeg läuft dort ungebremst durch.
// Upgrade: NtSuspendProcess oder -re/-readrate.
func pause(p *os.Process) bool { return false }
func resume(p *os.Process)     {}
