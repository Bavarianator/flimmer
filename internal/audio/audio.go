// Package audio liefert ffmpeg-Filterketten für den Ton: Stereo-Downmix mit angehobenem Center (Sprache)
// und Nachtmodus (Dynamikkompression). Beides greift nur, wenn der Ton ohnehin neu kodiert wird.
// Passthrough (TrueHD/Atmos) ist Sache des Geräteprofils: Kann das Gerät den Codec, wird kopiert.
package audio

import "strings"

// downmix: erst auf 5.1 bringen (7.1 und 5.1(side) rechnet ffmpeg selbst um), dann Stereo mit Center 1,0
// statt der üblichen 0,707 (+3 dB Sprache). „<“ normalisiert die Summe, damit nichts übersteuert.
const downmix = "aformat=channel_layouts=5.1,pan=stereo|FL<FC+0.707*FL+0.5*BL+0.3*LFE|FR<FC+0.707*FR+0.5*BR+0.3*LFE"

// night: leise Stellen hoch, laute runter; der Limiter fängt Spitzen ab.
const night = "acompressor=threshold=-30dB:ratio=6:attack=5:release=300:makeup=8dB,alimiter=limit=0.9:level=disabled"

// Filter baut die -af-Kette für eine Spur mit in Kanälen, die mit out Kanälen kodiert wird ("" = nichts zu tun).
func Filter(in, out int, nightMode bool) string {
	var f []string
	if out == 2 && in > 2 {
		f = append(f, downmix)
	}
	if nightMode {
		f = append(f, night)
	}
	return strings.Join(f, ",")
}
