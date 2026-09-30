package io.flimmer.app.admin

import io.flimmer.app.ApiClient
import io.flimmer.app.ApiException
import io.flimmer.app.Person
import io.flimmer.app.User
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonObjectBuilder
import kotlinx.serialization.json.buildJsonObject
import java.net.URLEncoder
import java.text.SimpleDateFormat
import java.util.Calendar
import java.util.Locale

// Formen wie in web/src/lib/api-admin.ts und docs/api-neu.md. Go liefert leere Listen als null:
// coerceInputValues macht daraus den Standardwert (leere Liste).

internal val json = Json { ignoreUnknownKeys = true; explicitNulls = false; coerceInputValues = true; isLenient = true }
/** Zum Senden: null bleibt null (löscht z. B. die Altersfreigabe). */
private val jsonAus = Json { encodeDefaults = true }

@Serializable
data class Session(
    val id: String = "", val user: String = "", val userColor: Int = 0, val device: String = "", val client: String = "",
    val itemId: String = "", val title: String = "", val position: Double = 0.0, val duration: Double = 0.0, val paused: Boolean = false,
    val method: String = "", val light: String = "green", val reason: String? = null, val since: String = "",
)

@Serializable
data class Activity(val time: String = "", val kind: String = "", val user: String? = null, val text: String = "")

@Serializable
data class Update(val version: String = "", val url: String = "")

@Serializable
data class ServerInfo(val name: String = "", val version: String = "", val os: String = "", val arch: String = "", val uptime: Double = 0.0,
                      val started: String = "", val update: Update? = null)

@Serializable
data class LibInfo(val movies: Int = 0, val series: Int = 0, val episodes: Int = 0, val sizeBytes: Long = 0, val lastScan: String? = null)

@Serializable
data class Disk(val path: String = "", val free: Long = 0, val total: Long = 0)

@Serializable
data class Overview(val server: ServerInfo = ServerInfo(), val library: LibInfo = LibInfo(), val disks: List<Disk> = emptyList(),
                    val sessions: List<Session> = emptyList(), val activity: List<Activity> = emptyList())

@Serializable
data class Geraet(val id: String = "", val name: String = "", val client: String? = null, val user: String = "", val userId: String = "",
                  val lastSeen: String = "", val ip: String? = null, val current: Boolean = false)

@Serializable
data class Task(
    val id: String = "", val name: String = "", val group: String = "", val description: String = "", val lastRun: String? = null,
    val lastResult: String? = null, val lastError: String? = null, val duration: Double? = null, val next: String? = null,
    val running: Boolean = false, val progress: Double? = null,
)

@Serializable
data class LogZeile(val time: String = "", val level: String = "", val msg: String = "", val attrs: JsonObject? = null)

@Serializable
data class VpnAdresse(val provider: String = "", @SerialName("interface") val iface: String = "", val ip: String = "", val url: String = "")

@Serializable
data class Vpn(val addrs: List<VpnAdresse> = emptyList(), val hint: String = "")

@Serializable
data class LiveTVStatus(val source: String = "", val epg: String = "", val video: String = "", val channels: Int = 0, val programs: Int = 0,
                        val updated: String = "", val error: String = "")

/** Vorlage für „Schnell einrichten“; channels = 0: nicht gefunden (FRITZ!Box). */
@Serializable
data class LiveTVVorlage(val id: String = "", val source: String = "", val epg: String = "", val channels: Int = 0, val active: Boolean = false)

@Serializable
data class Sendung(val start: String = "", val stop: String = "", val title: String = "", val desc: String? = null)

@Serializable
data class Kanal(val id: String = "", val number: String? = null, val name: String = "", val logo: String? = null, val group: String? = null,
                 val now: Sendung? = null, val next: Sendung? = null)

@Serializable
data class FfmpegDownload(val running: Boolean = false, val percent: Double = 0.0, val error: String? = null)

@Serializable
data class Ffmpeg(val ok: Boolean = false, val canDownload: Boolean = false, val hint: String = "", val download: FfmpegDownload = FfmpegDownload())

@Serializable
data class Optimize(val off: Boolean = false, val from: Int = 0, val to: Int = 0, val minFreeGB: Int = 0)

@Serializable
data class Settings(
    val serverName: String = "", val language: String = "", val dirs: List<String> = emptyList(), val tmdbKey: Boolean = false,
    val ffmpeg: Ffmpeg = Ffmpeg(), val lanUrl: String = "", val updateCheck: Boolean = false, val remote: Boolean = false,
    val optimize: Optimize = Optimize(), val remoteAvailable: Boolean = false, val update: Update? = null, val version: String = "",
)

@Serializable
data class Scope(val libraries: List<String> = emptyList(), val items: List<String> = emptyList())

@Serializable
data class Einladung(val id: String = "", val note: String = "", val scope: Scope = Scope(), val expires: String = "", val maxUses: Int = 0,
                     val uses: Int = 0, val guests: Int = 0, val created: String? = null)

@Serializable
data class NeueEinladung(val url: String = "", val qr: String? = null, val hint: String? = null)

@Serializable
data class Remote(val method: String = "", val publicUrl: String = "", val reachable: Boolean = false, val hint: String = "")

@Serializable
data class OptJetzt(val title: String = "", val percent: Double = 0.0)

@Serializable
data class OptStand(val waiting: Boolean = false, val on: Boolean = false, val window: String? = null, val current: OptJetzt? = null,
                    val done: Int = 0, val pending: Int = 0, val lastError: String? = null)

@Serializable
data class DiagStream(val user: String = "", val title: String = "", val device: String = "", val method: String = "", val light: String = "green",
                      val reasons: List<String> = emptyList())

@Serializable
data class DiagDb(val checkedAt: String = "", val integrity: String = "", val sizeBytes: Long = 0, val backupAt: String = "", val backupFile: String = "")

@Serializable
data class Diag(
    val version: String = "", val os: String = "", val cpus: Int = 0, val ffmpeg: String = "", val hw: String = "", val hwSpeed: Double = 0.0,
    val diskFree: Long = 0, val diskTotal: Long = 0, val cacheDir: String = "", val active: List<DiagStream> = emptyList(),
    val log: List<String> = emptyList(), val db: DiagDb = DiagDb(),
)

@Serializable
data class Unsicher(val id: String = "", val file: String = "", val guess: String = "")

@Serializable
data class Kandidat(val tmdbId: Int = 0, val title: String = "", val year: Int? = null)

@Serializable
data class OrdnerEintrag(val name: String = "", val path: String = "")

@Serializable
data class Ordner(val path: String = "", val parent: String = "", val dirs: List<OrdnerEintrag> = emptyList())

@Serializable
data class ScanStatus(val scanning: Boolean = false, val found: Int = 0)

@Serializable
data class AppCode(val code: String = "", val expires: String = "")

/** meta eines Titels, mit den Feldern, die der Editor braucht. */
@Serializable
data class MMeta(
    val title: String? = null, val originalTitle: String? = null, val sortTitle: String? = null, val year: Int? = null,
    val overview: String? = null, val rating: Double? = null, val genres: List<String> = emptyList(), val age: Int? = null,
    val tmdbId: Int? = null, val imdbId: String? = null, val source: String? = null, val uncertain: Boolean = false,
    val tagline: String? = null, val studios: List<String> = emptyList(), val tags: List<String> = emptyList(),
    val people: List<Person> = emptyList(),
)

/** Titel aus /api/library (nur was der Metadaten-Manager braucht). */
@Serializable
data class MItem(val id: String = "", val title: String = "", val year: Int? = null, val series: String? = null, val season: Int? = null,
                 val episode: Int? = null, val poster: String = "", val meta: MMeta? = null) {
    val name: String get() {
        val t = meta?.title ?: title
        return if (series != null) "$series · S${season ?: 0} E${episode ?: 0} · $t" else t + ((meta?.year ?: year)?.let { " ($it)" } ?: "")
    }
}

/** GET /api/items/{id} für den Editor. */
@Serializable
data class MDetails(
    val id: String = "", val title: String = "", val year: Int? = null, val meta: MMeta? = null, val tagline: String? = null,
    val studios: List<String> = emptyList(), val tags: List<String> = emptyList(), val people: List<Person> = emptyList(),
    val path: String? = null, val locked: List<String> = emptyList(), val added: String = "",
)

/** PUT /api/items/{id}/meta. locked immer mitschicken: fehlt es, sperrt der Server die geänderten Felder; [] entsperrt alles. */
@Serializable
data class MetaAenderung(
    val title: String, val originalTitle: String, val sortTitle: String, val year: Int, val rating: Double, val age: Int?,
    val tagline: String, val overview: String, val genres: List<String>, val studios: List<String>, val tags: List<String>,
    val people: List<Person>, val locked: List<String>,
)

// ---------- Zugriff (nur über ApiClient.roh) ----------

internal fun k(s: String): String = URLEncoder.encode(s, "UTF-8").replace("+", "%20")

internal suspend inline fun <reified T> ApiClient.hol(path: String): T = json.decodeFromString(roh("GET", path))
internal suspend inline fun <reified T> ApiClient.liste(path: String): List<T> = json.decodeFromString<List<T>?>(roh("GET", path)) ?: emptyList()
internal suspend inline fun <reified T> ApiClient.schick(method: String, path: String, body: String): T = json.decodeFromString(roh(method, path, body))
internal suspend fun ApiClient.tu(method: String, path: String, body: String? = if (method == "DELETE") null else "{}") { roh(method, path, body) }

internal fun obj(b: JsonObjectBuilder.() -> Unit): String = buildJsonObject(b).toString()

internal suspend fun ApiClient.uebersicht(): Overview = hol("/api/admin/overview")
internal suspend fun ApiClient.aktivitaeten(limit: Int = 200): List<Activity> = liste("/api/activity?limit=$limit")
internal suspend fun ApiClient.geraete(): List<Geraet> = liste("/api/devices")
internal suspend fun ApiClient.aufgaben(): List<Task> = liste("/api/tasks")
internal suspend fun ApiClient.aufgabeStarten(id: String) = tu("POST", "/api/tasks/${k(id)}/run")
internal suspend fun ApiClient.protokoll(level: String, limit: Int = 300): List<LogZeile> = liste("/api/logs?level=${k(level)}&limit=$limit")
internal suspend fun ApiClient.einstellungen(): Settings = hol("/api/settings")
internal suspend fun ApiClient.einstellungenSetzen(b: JsonObjectBuilder.() -> Unit) { roh("PUT", "/api/settings", obj(b)) }
internal suspend fun ApiClient.metaSpeichern(id: String, a: MetaAenderung) { roh("PUT", "/api/items/${k(id)}/meta", jsonAus.encodeToString(MetaAenderung.serializer(), a)) }
internal suspend fun ApiClient.benutzer(): List<User> = liste("/api/users")

/** Endpunkt fehlt (noch): 404, bzw. 405, wenn nur die Methode unbekannt ist. */
internal fun fehlt(e: Throwable?) = e is ApiException && (e.code == 404 || e.code == 405)

/** Fehlertext ohne Statuscode, wie im Web. */
internal fun fehlerText(e: Throwable): String = (e.message ?: "").trim().ifEmpty { "Server nicht erreichbar" }

// ---------- Formate (wie web/src/screens/admin/teile.tsx) ----------

/** ISO-Zeit von Go (RFC 3339, auch mit Nanosekunden) → Millisekunden; null bei leer oder Nullzeit. minSdk 23 kennt java.time nicht. */
internal fun zeit(s: String?): Long? {
    if (s.isNullOrEmpty()) return null
    val n = s.replace(Regex("\\.\\d+"), "").replace(Regex("Z$"), "+0000").replace(Regex("([+-]\\d\\d):(\\d\\d)$"), "$1$2")
    val d = runCatching { SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ssZ", Locale.GERMANY).parse(n) }.getOrNull() ?: return null
    val c = Calendar.getInstance().apply { time = d }
    return if (c.get(Calendar.YEAR) < 2000) null else d.time
}

private fun fmt(p: String, ms: Long) = SimpleDateFormat(p, Locale.GERMANY).format(java.util.Date(ms))

internal fun datum(s: String?, mitZeit: Boolean = true): String = zeit(s)?.let { fmt(if (mitZeit) "dd.MM.yyyy, H:mm" else "dd.MM.yyyy", it) } ?: "noch nie"
internal fun uhrzeit(s: String?): String = zeit(s)?.let { fmt("HH:mm", it) } ?: ""

/** „vor 26 Min.“, „gestern, 19:12“, sonst Datum. */
internal fun vor(s: String?, jetzt: Long = System.currentTimeMillis()): String {
    val t = zeit(s) ?: return "noch nie"
    val min = Math.round((jetzt - t) / 60000.0)
    if (min < 1) return "gerade eben"
    if (min < 60) return "vor $min Min."
    if (min < 12 * 60) return "vor ${Math.round(min / 60.0)} Std."
    val gestern = Calendar.getInstance().apply { timeInMillis = jetzt; add(Calendar.DAY_OF_YEAR, -1) }
    val c = Calendar.getInstance().apply { timeInMillis = t }
    if (c.get(Calendar.YEAR) == gestern.get(Calendar.YEAR) && c.get(Calendar.DAY_OF_YEAR) == gestern.get(Calendar.DAY_OF_YEAR)) return "gestern, ${fmt("HH:mm", t)}"
    return datum(s)
}

internal fun groesse(b: Long): String = when {
    b >= 1e12 -> "%.1f TB".format(Locale.GERMANY, b / 1e12)
    b >= 1e9 -> "%.1f GB".format(Locale.GERMANY, b / 1e9)
    else -> "%.1f MB".format(Locale.GERMANY, maxOf(0.1, b / 1e6))
}

/** Laufzeit eines Servers: „6 Tagen“ bzw. „3 Std. 12 Min.“ */
internal fun seit(sek: Double): String {
    val tage = (sek / 86400).toInt()
    if (tage > 1) return "$tage Tagen"
    if (tage == 1) return "1 Tag"
    val h = (sek / 3600).toInt()
    val m = Math.round((sek % 3600) / 60)
    return (if (h > 0) "$h Std. " else "") + "$m Min."
}

/** „1 Std. 12 Min.“ bzw. „34 Min.“ (web: dauer()). */
internal fun dauer(sek: Double): String {
    val h = (sek / 3600).toInt()
    val m = Math.round((sek % 3600) / 60).toInt()
    if (h == 0) return "${maxOf(1, m)} Min."
    return "$h Std." + if (m > 0) " $m Min." else ""
}

internal fun methode(m: String) = when (m) {
    "direct", "direct-play" -> "Direktes Abspielen"
    "remux", "direct-stream" -> "Neu verpackt, ohne Umwandlung"
    "transcode-audio" -> "Nur der Ton wird umgewandelt"
    "transcode" -> "Wird umgewandelt"
    else -> m
}

internal fun komma(d: Double, stellen: Int = 1) = "%.${stellen}f".format(Locale.GERMANY, d)

/** Letzter Teil eines Pfads („/srv/medien/filme“ → „filme“). */
internal fun ordnerName(p: String) = p.split('/', '\\').lastOrNull { it.isNotEmpty() } ?: p
