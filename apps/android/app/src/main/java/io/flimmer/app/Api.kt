package io.flimmer.app

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import okio.BufferedSink
import java.io.InputStream
import java.util.concurrent.TimeUnit

// Formen wie in internal/api (library.go, auth.go). Unbekannte Felder werden ignoriert,
// damit neue Server-Felder alte Apps nicht brechen.

@Serializable
data class User(val id: String, val name: String, val color: Int = 220, val admin: Boolean = false, val hasPassword: Boolean = false,
                val upload: Boolean = false) // darf Videos hochladen (vergibt der Admin; Admins immer)

@Serializable
data class LoginResult(val id: String = "", val name: String = "", val admin: Boolean = false, val token: String = "")

@Serializable
data class Meta(
    val title: String? = null,
    val year: Int? = null,
    val overview: String? = null,
    val rating: Double? = null,
    val genres: List<String> = emptyList(),
    val age: Int? = null,
    val imdbId: String? = null,
    val tmdbId: Int? = null,
    val studios: List<String> = emptyList(), // bei Serien die Sender
    val countries: List<String> = emptyList(),
    val tags: List<String> = emptyList(),
)

// ---------- Neue Endpunkte (docs/umbau-jellyfin.md). Fehlen sie auf dem Server, liefert optional() null. ----------

@Serializable
data class Person(val name: String, val role: String? = null, val kind: String = "actor", val image: String? = null)

@Serializable
data class Stream(val index: Int = 0, val codec: String = "", val lang: String? = null, val title: String? = null,
                  val channels: Int? = null, val default: Boolean = false, val forced: Boolean = false, val external: Boolean = false)

/** Sichtbarer Bildausschnitt ohne eingebrannte Balken, als Anteile 0..1 des Bildes. */
@Serializable
data class Crop(val x: Double, val y: Double, val w: Double, val h: Double)

@Serializable
data class VideoInfo(val codec: String = "", val width: Int = 0, val height: Int = 0, val hdr: String? = null, val fps: Double? = null, val crop: Crop? = null)

@Serializable
data class Chapter(val start: Double = 0.0, val name: String = "", val image: String? = null)

/** GET /api/items/{id}: nur die Zusatzfelder; das Item selbst kennt die App schon aus /api/library. */
@Serializable
data class Details(
    val tagline: String? = null,
    val studios: List<String> = emptyList(),
    val countries: List<String> = emptyList(),
    val people: List<Person> = emptyList(),
    val tags: List<String> = emptyList(),
    val size: Long = 0,
    val container: String = "",
    val bitrate: Long? = null,
    val video: VideoInfo? = null,
    val audio: List<Stream> = emptyList(),
    val subs: List<Stream> = emptyList(),
    val chapters: List<Chapter> = emptyList(),
)

@Serializable
data class SeriesInfo(
    val name: String = "",
    val title: String? = null, // Anzeigename aus den Metadaten, nur wenn er vom Schlüssel abweicht
    val overview: String? = null,
    val year: Int? = null,
    val endYear: Int? = null,
    val status: String? = null,
    val genres: List<String> = emptyList(),
    val studios: List<String> = emptyList(),
    val people: List<Person> = emptyList(),
    val age: Int? = null,
    val rating: Double? = null,
    val poster: String? = null,
    val backdrop: String? = null,
    val color: String? = null,
)

@Serializable
data class PersonPage(val name: String = "", val image: String? = null, val bio: String? = null, val items: List<Item> = emptyList())

/** Sammlung oder Wiedergabeliste: items sind Schlüssel (Item-ID oder "serie:<Name>"). */
@Serializable
data class Liste(val id: String, val name: String, val overview: String? = null, val items: List<String> = emptyList(), val auto: Boolean = false)

@Serializable
data class Item(
    val id: String,
    val title: String,
    val year: Int? = null,
    val series: String? = null,
    val season: Int? = null,
    val episode: Int? = null,
    val duration: Double = 0.0,
    val light: String = "green",
    val method: String = "",
    val meta: Meta? = null,
    val poster: String = "",
    val backdrop: String = "",
    val color: String = "",
    val progress: Double = 0.0,
    val watched: Boolean = false,
    val added: String = "",
) {
    val displayTitle get() = meta?.title ?: title
    /** Schlüssel für Favoriten, Sammlungen und Listen (Vertrag in docs/umbau-jellyfin.md): Serien als Ganzes. */
    val key get() = series?.let { "serie:$it" } ?: id
}

@Serializable
data class HomeRow(val id: String, val title: String, val items: List<Item> = emptyList())

@Serializable
data class Subtitle(val index: Int, val language: String? = null, val title: String? = null, val format: String = "vtt", val url: String = "")

@Serializable
data class AudioTrack(val index: Int, val language: String? = null, val title: String? = null, val codec: String = "", val channels: Int = 0)

@Serializable
data class TrackPref(val audio: String = "", val subtitle: String = "")

@Serializable
data class Plan(
    val method: String,
    val light: String = "green",
    val reasons: List<String>? = null,
    val url: String,
    val title: String = "",
    val duration: Double = 0.0,
    val audioIndex: Int = -1,
    val audio: List<AudioTrack>? = null,
    val subtitles: List<Subtitle>? = null,
    val resume: Double = 0.0,
    val prefs: TrackPref = TrackPref(),
)

/** Geräteprofil wie playback.Profile; audioTrack/audioLang sind Wünsche für eine Wiedergabe. */
@Serializable
data class Profile(
    val name: String,
    val containers: List<String>,
    val video: List<String>,
    val audio: List<String>,
    val nativeHls: Boolean = true,
    val maxBitrate: Long = 0,
    /** null = unbekannt (Server rechnet nichts um), [] = SDR-Display, sonst hdr10/hlg/hdr10+/dv */
    val hdr: List<String>? = null,
    val audioTrack: Int = 0,
    val audioLang: String = "",
    /** Qualitätswahl im Player: höchstens so viele Bildzeilen (1080/720/480), 0 = automatisch (meist Original). */
    val maxHeight: Int = 0,
    /** Vorlieben (Einstellungen › Wiedergabe/Untertitel): Sprachreihenfolge, Untertitel-Modus, Nachtmodus per Server. */
    val audioLangs: List<String>? = null,
    val subtitleMode: String = "",
    val night: Boolean = false,
)

/** Offene „Gemeinsam schauen“-Gruppe (GET /api/party): nur mit Mitgliedern und Titeln, die man sehen darf. */
@Serializable
data class OffeneGruppe(val id: String, val mediaId: String = "", val host: String = "", val members: List<String> = emptyList(), val paused: Boolean = true)

@Serializable
data class PairStart(val code: String, val secret: String, val expiresIn: Int = 600)

@Serializable
data class PairDone(val token: String = "")

@Serializable
private data class Progress(val pos: Long, val duration: Long, val audio: String, val subtitle: String, val paused: Boolean = false)

@Serializable
private data class LoginBody(val user: String, val password: String? = null, val device: String)

@Serializable
private data class Device(val device: String)

@Serializable
private data class RedeemBody(val token: String, val name: String, val password: String)

class ApiException(val code: Int, message: String) : Exception(message)

/** Schlanker Client gegen die Flimmer-API. Anmeldung per Bearer-Token (wie TVs). */
class ApiClient(base: String, var token: String = "", http: OkHttpClient? = null) {
    val base = base.trimEnd('/')
    val http = http ?: OkHttpClient.Builder()
        .connectTimeout(5, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .build()

    private val json = Json { ignoreUnknownKeys = true; explicitNulls = false; encodeDefaults = true }
    private val jsonType = "application/json".toMediaType()

    /** Absolute URL für Pfade, die der Server liefert (Poster, Stream, Untertitel). */
    fun abs(path: String) = if (path.isEmpty() || path.startsWith("http")) path else base + path // leer bleibt leer: dann Tonfläche statt Bild

    private suspend fun call(method: String, path: String, body: String? = null): String = withContext(Dispatchers.IO) {
        val req = Request.Builder().url(base + path).method(method, body?.toRequestBody(jsonType))
        if (token.isNotEmpty()) req.header("Authorization", "Bearer $token")
        http.newCall(req.build()).execute().use { r ->
            val text = r.body.string()
            if (!r.isSuccessful) throw ApiException(r.code, text.ifEmpty { r.message })
            // Ältere Server liefern für unbekannte /api-Pfade die Web-App (index.html, 200) – das heißt: gibt es nicht.
            if (r.header("Content-Type").orEmpty().startsWith("text/html")) throw ApiException(404, "Endpunkt fehlt")
            text
        }
    }

    /** Roher Zugriff für die Module admin, livetv, einstellungen und gemeinsam (eigene Datenklassen dort). Leere Antwort = "". */
    suspend fun roh(method: String, path: String, body: String? = null): String = call(method, path, body)

    private suspend inline fun <reified T> get(path: String): T = json.decodeFromString(call("GET", path))
    private suspend inline fun <reified T, reified B> post(path: String, body: B): T =
        json.decodeFromString(call("POST", path, json.encodeToString(body)))

    suspend fun users(): List<User> = get("/api/users")
    suspend fun me(): User = get("/api/me")

    suspend fun login(user: String, password: String?, device: String): LoginResult =
        post<LoginResult, LoginBody>("/api/login", LoginBody(user, password, device)).also { token = it.token }

    /** Einladung einlösen: legt das Konto an und meldet an (Token wie beim Login). */
    suspend fun redeem(invite: String, name: String, password: String): String =
        post<LoginResult, RedeemBody>("/api/invites/redeem", RedeemBody(invite, name, password)).token.also { token = it }

    suspend fun logout() {
        runCatching { call("POST", "/api/logout", "{}") }
        token = ""
    }

    suspend fun home(p: Profile): List<HomeRow> = post("/api/home", p)
    suspend fun library(p: Profile): List<Item> = json.decodeFromString<List<Item>?>(call("POST", "/api/library", json.encodeToString(p))) ?: emptyList()
    suspend fun play(id: String, p: Profile): Plan = post("/api/items/$id/play", p)

    /** Fortschritt; [paused] = Herzschlag im Pausenzustand (hält die Sitzung im Dashboard am Leben). */
    suspend fun progress(id: String, posSec: Long, durationSec: Long, audio: String, subtitle: String, paused: Boolean = false) {
        call("POST", "/api/items/$id/progress", json.encodeToString(Progress(posSec, durationSec, audio, subtitle, paused)))
    }

    suspend fun pairStart(device: String): PairStart = post("/api/pair", Device(device))

    /** null solange am Handy noch nicht bestätigt (202). */
    suspend fun pairPoll(code: String, secret: String): String? = withContext(Dispatchers.IO) {
        val req = Request.Builder().url("$base/api/pair/$code?secret=$secret").build()
        http.newCall(req).execute().use { r ->
            when (r.code) {
                200 -> json.decodeFromString<PairDone>(r.body.string()).token.also { token = it }
                202 -> null
                else -> throw ApiException(r.code, "Code abgelaufen")
            }
        }
    }

    suspend fun putProfile(deviceId: String, p: Profile) {
        call("PUT", "/api/devices/$deviceId/profile", json.encodeToString(p))
    }

    /** Schnellverbindung: Code eines anderen Geräts (TV) bestätigen; liefert dessen Namen. */
    suspend fun pairConfirm(code: String): String =
        json.decodeFromString<Map<String, String>>(call("POST", "/api/pair/${enc(code.filter { !it.isWhitespace() })}/confirm", "{}"))["device"] ?: ""

    suspend fun watched(id: String, on: Boolean) {
        call("POST", "/api/items/$id/watched", """{"watched":$on}""")
    }

    /** Serverseitige Suche (fehlertolerant, je Serie ein Treffer). */
    suspend fun search(q: String, deviceId: String): List<Item> =
        json.decodeFromString<List<Item>?>(call("GET", "/api/search?q=${enc(q)}&limit=60&device=$deviceId")) ?: emptyList()

    suspend fun favorites(): List<String> = json.decodeFromString<List<String>?>(call("GET", "/api/favorites")) ?: emptyList()
    suspend fun favorite(key: String, on: Boolean) { call(if (on) "PUT" else "DELETE", "/api/favorites/${enc(key)}", if (on) "{}" else null) }

    /** [deviceId]: gespeichertes Geräteprofil, damit die Ampel für dieses Gerät gilt. */
    suspend fun details(id: String, deviceId: String): Details = get("/api/items/$id?device=$deviceId")
    suspend fun seriesInfo(name: String): SeriesInfo = get("/api/series/${enc(name)}")
    suspend fun person(name: String, deviceId: String): PersonPage = get("/api/people/${enc(name)}?device=$deviceId")
    suspend fun collections(): List<Liste> = json.decodeFromString<List<Liste>?>(call("GET", "/api/collections")) ?: emptyList()
    suspend fun playlists(): List<Liste> = json.decodeFromString<List<Liste>?>(call("GET", "/api/playlists")) ?: emptyList()
    // Sammlungen (sammlung = true, bearbeiten nur Admins, auto-* gesperrt) und Wiedergabelisten (pro Profil) verwalten
    private fun listenPfad(sammlung: Boolean) = if (sammlung) "/api/collections" else "/api/playlists"

    @Serializable
    private data class ListeAenderung(val name: String? = null, val add: List<String>? = null, val remove: List<String>? = null, val items: List<String>? = null)

    suspend fun listeAnlegen(sammlung: Boolean, name: String, items: List<String> = emptyList()): Liste =
        json.decodeFromString(call("POST", listenPfad(sammlung), json.encodeToString(ListeAenderung(name = name, items = items))))
    /** Teiländerung: nur gesetzte Felder; [items] ersetzt die Reihenfolge. */
    suspend fun listeAendern(sammlung: Boolean, id: String, name: String? = null, add: List<String>? = null, remove: List<String>? = null, items: List<String>? = null) {
        call("PUT", "${listenPfad(sammlung)}/${enc(id)}", json.encodeToString(ListeAenderung(name, add, remove, items)))
    }
    suspend fun listeLoeschen(sammlung: Boolean, id: String) { call("DELETE", "${listenPfad(sammlung)}/${enc(id)}") }

    suspend fun offeneGruppen(): List<OffeneGruppe> = json.decodeFromString<List<OffeneGruppe>?>(call("GET", "/api/party")) ?: emptyList()

    /**
     * POST /api/upload?name=…: die Datei als Body, gestreamt (kein Multipart, nichts im RAM). [size] -1 = unbekannt
     * (dann kein Fortschritt). Liefert den Namen auf dem Server („Film (2).mkv“, wenn es den schon gab).
     */
    suspend fun upload(name: String, size: Long, open: () -> InputStream, fortschritt: (Float) -> Unit = {}): String = withContext(Dispatchers.IO) {
        val body = object : RequestBody() {
            override fun contentType() = "application/octet-stream".toMediaType()
            override fun contentLength() = size
            override fun writeTo(sink: BufferedSink) {
                open().use { inp ->
                    val buf = ByteArray(256 * 1024)
                    var gesendet = 0L
                    while (true) {
                        val n = inp.read(buf)
                        if (n < 0) break
                        sink.write(buf, 0, n)
                        gesendet += n
                        if (size > 0) fortschritt(gesendet.toFloat() / size)
                    }
                }
            }
        }
        val req = Request.Builder().url("$base/api/upload?name=${enc(name)}").post(body)
        if (token.isNotEmpty()) req.header("Authorization", "Bearer $token")
        // Langsame NAS-Platte bremst per TCP: einzelne Schreibvorgänge dürfen länger dauern als die üblichen 10 s.
        http.newBuilder().writeTimeout(2, TimeUnit.MINUTES).build().newCall(req.build()).execute().use { r ->
            val text = r.body.string()
            if (!r.isSuccessful) throw ApiException(r.code, text.trim().ifEmpty { r.message })
            json.decodeFromString<Map<String, String>>(text)["name"] ?: name
        }
    }

    private fun enc(s: String) = java.net.URLEncoder.encode(s, "UTF-8").replace("+", "%20")
}

/** Endpunkt aus dem neuen Vertrag: null, wenn der Server ihn (noch) nicht hat oder die Antwort nicht passt. */
suspend fun <T> optional(block: suspend () -> T): T? = try {
    block()
} catch (e: ApiException) {
    if (e.code in listOf(404, 405, 501)) null else throw e
} catch (_: kotlinx.serialization.SerializationException) {
    null
}
