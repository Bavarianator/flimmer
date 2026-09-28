package io.flimmer.app

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.util.concurrent.TimeUnit

// Formen wie in internal/api (library.go, auth.go). Unbekannte Felder werden ignoriert,
// damit neue Server-Felder alte Apps nicht brechen.

@Serializable
data class User(val id: String, val name: String, val color: Int = 220, val admin: Boolean = false, val hasPassword: Boolean = false)

@Serializable
data class LoginResult(val id: String = "", val name: String = "", val admin: Boolean = false, val token: String = "")

@Serializable
data class Meta(
    val title: String? = null,
    val year: Int? = null,
    val overview: String? = null,
    val rating: Double? = null,
    val genres: List<String> = emptyList(),
)

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
) {
    val displayTitle get() = meta?.title ?: title
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
)

@Serializable
data class PairStart(val code: String, val secret: String, val expiresIn: Int = 600)

@Serializable
data class PairDone(val token: String = "")

@Serializable
private data class Progress(val pos: Long, val duration: Long, val audio: String, val subtitle: String)

@Serializable
private data class LoginBody(val user: String, val password: String? = null, val device: String)

@Serializable
private data class Device(val device: String)

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
    fun abs(path: String) = if (path.startsWith("http")) path else base + path

    private suspend fun call(method: String, path: String, body: String? = null): String = withContext(Dispatchers.IO) {
        val req = Request.Builder().url(base + path).method(method, body?.toRequestBody(jsonType))
        if (token.isNotEmpty()) req.header("Authorization", "Bearer $token")
        http.newCall(req.build()).execute().use { r ->
            val text = r.body.string()
            if (!r.isSuccessful) throw ApiException(r.code, text.ifEmpty { r.message })
            text
        }
    }

    private suspend inline fun <reified T> get(path: String): T = json.decodeFromString(call("GET", path))
    private suspend inline fun <reified T, reified B> post(path: String, body: B): T =
        json.decodeFromString(call("POST", path, json.encodeToString(body)))

    suspend fun users(): List<User> = get("/api/users")
    suspend fun me(): User = get("/api/me")

    suspend fun login(user: String, password: String?, device: String): LoginResult =
        post<LoginResult, LoginBody>("/api/login", LoginBody(user, password, device)).also { token = it.token }

    suspend fun logout() {
        runCatching { call("POST", "/api/logout", "{}") }
        token = ""
    }

    suspend fun home(p: Profile): List<HomeRow> = post("/api/home", p)
    suspend fun library(p: Profile): List<Item> = json.decodeFromString<List<Item>?>(call("POST", "/api/library", json.encodeToString(p))) ?: emptyList()
    suspend fun play(id: String, p: Profile): Plan = post("/api/items/$id/play", p)

    suspend fun progress(id: String, posSec: Long, durationSec: Long, audio: String, subtitle: String) {
        call("POST", "/api/items/$id/progress", json.encodeToString(Progress(posSec, durationSec, audio, subtitle)))
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
}
