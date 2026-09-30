package io.flimmer.app.livetv

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.util.Calendar
import java.util.Locale
import java.util.TimeZone
import kotlin.math.max
import kotlin.math.min
import kotlin.math.roundToInt

// Daten aus internal/livetv (docs/livetv-vpn.md): GET /api/livetv/channels, GET /api/livetv/guide?hours=,
// POST /api/livetv/channels/{id}/play. Zeiten sind RFC 3339, leere Listen kommen von Go als null.

@Serializable
data class Sendung(val start: String = "", val stop: String = "", val title: String = "", val desc: String? = null) {
    val von: Long get() = zeit(start)
    val bis: Long get() = zeit(stop)
    fun laeuft(jetzt: Long) = von <= jetzt && jetzt < bis
    fun anteil(jetzt: Long): Float = if (bis > von) ((jetzt - von).toFloat() / (bis - von)).coerceIn(0f, 1f) else 0f
}

@Serializable
data class Kanal(
    val id: String,
    val number: String? = null,
    val name: String = "",
    val logo: String? = null,
    val group: String? = null,
    val now: Sendung? = null,
    val next: Sendung? = null,
)

@Serializable
data class FuehrerKanal(val id: String, val programs: List<Sendung>? = null)

@Serializable
data class Fuehrer(val from: String = "", val to: String = "", val channels: List<FuehrerKanal>? = null)

@Serializable
data class Einschalten(val url: String = "", val title: String = "")

internal val json = Json { ignoreUnknownKeys = true; coerceInputValues = true }

fun kanaele(text: String): List<Kanal> = if (text.isBlank()) emptyList() else json.decodeFromString<List<Kanal>?>(text).orEmpty()
fun fuehrer(text: String): Fuehrer = json.decodeFromString(text)

private val rfc3339 = Regex("""(\d{4})-(\d\d)-(\d\d)T(\d\d):(\d\d):(\d\d)(?:\.\d+)?(Z|[+-]\d\d:\d\d)""")

/** RFC 3339 von Go → Millisekunden; 0, wenn es nicht passt. Ohne java.time, weil minSdk 23. */
fun zeit(s: String): Long {
    val g = rfc3339.matchEntire(s)?.groupValues ?: return 0
    val c = Calendar.getInstance(TimeZone.getTimeZone("UTC"))
    c.clear()
    c.set(g[1].toInt(), g[2].toInt() - 1, g[3].toInt(), g[4].toInt(), g[5].toInt(), g[6].toInt())
    val z = g[7]
    val versatz = if (z == "Z") 0 else (if (z[0] == '-') -1 else 1) * (z.substring(1, 3).toInt() * 60 + z.substring(4, 6).toInt())
    return c.timeInMillis - versatz * 60_000L
}

/** „20:15“ in der Zeitzone des Geräts. */
fun hm(ms: Long): String = java.text.SimpleDateFormat("HH:mm", Locale.GERMANY).format(java.util.Date(ms))

const val HALBE_STUNDE = 30 * 60_000L

/**
 * Zeitraster des Programmführers wie im Web: Beginn auf die halbe Stunde abgerundet, [ppm] Punkte (dp) pro Minute,
 * Blöcke mit 4 dp Luft und mindestens fünf Minuten breit.
 */
class Raster(from: String, to: String, val ppm: Int) {
    val von = zeit(from) / HALBE_STUNDE * HALBE_STUNDE
    val bis = zeit(to)
    fun px(t: Long): Int = ((t - von) / 60_000.0 * ppm).roundToInt()
    val breite: Int get() = max(0, px(bis))
    /** Marken alle 30 Minuten. */
    val marken: List<Long> get() = if (bis <= von) emptyList() else (von until bis step HALBE_STUNDE).toList()
    /** Links und Breite eines Blocks. */
    fun block(s: Sendung): Pair<Int, Int> {
        val links = max(0, px(s.von))
        return links to max(ppm * 5, px(min(s.bis, bis)) - links - 4)
    }
}
