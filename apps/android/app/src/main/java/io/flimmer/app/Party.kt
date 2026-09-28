package io.flimmer.app

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.isActive
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.util.concurrent.TimeUnit
import kotlin.math.abs

// „Gemeinsam schauen“ – Vertrag aus internal/party (Server ist die Autorität, Kanal per SSE + POST).

@Serializable
data class PartyState(
    val mediaId: String = "",
    val pos: Double = 0.0,
    val serverTs: Long = 0,
    val rate: Double = 1.0,
    val paused: Boolean = true,
    val waiting: List<String> = emptyList(),
    val host: String = "",
)

@Serializable
data class PartyRoom(val id: String = "", val state: PartyState = PartyState(), val eventsUrl: String = "", val members: List<String> = emptyList())

@Serializable
data class PartyAction(
    val member: String,
    val type: String, // play | pause | seek | rate | media | buffering | chat | reaction
    val pos: Double = 0.0,
    val rate: Double = 0.0,
    val mediaId: String = "",
    val buffering: Boolean = false,
    val text: String = "",
)

data class SseEvent(val name: String, val data: String)

/** Zerlegt einen SSE-Strom zeilenweise (event:/data:, Leerzeile = Ende, „:“ = Kommentar/Ping). */
class SseParser {
    private var name = "message"
    private val data = StringBuilder()

    fun line(l: String): SseEvent? {
        when {
            l.isEmpty() -> {
                if (data.isEmpty()) { name = "message"; return null }
                val ev = SseEvent(name, data.toString())
                name = "message"; data.clear()
                return ev
            }
            l.startsWith(":") -> {}
            l.startsWith("event:") -> name = l.substringAfter(':').trim()
            l.startsWith("data:") -> { if (data.isNotEmpty()) data.append('\n'); data.append(l.substringAfter(':').removePrefix(" ")) }
        }
        return null
    }
}

object PartySync {
    /** Uhrabweichung Server − Gerät in ms: Median über (now − (t0+t1)/2) mehrerer Messungen. */
    fun offset(samples: List<Triple<Long, Long, Long>>): Long =
        samples.map { (t0, now, t1) -> now - (t0 + t1) / 2 }.sorted().let { it[it.size / 2] }

    /** Soll-Position in Sekunden zum Server-Zeitpunkt [serverNow]. */
    fun target(s: PartyState, serverNow: Long): Double =
        if (s.paused) s.pos else s.pos + (serverNow - s.serverTs) / 1000.0 * s.rate

    sealed interface Fix {
        data object None : Fix
        data class Rate(val speed: Float) : Fix
        data class Seek(val to: Double) : Fix
    }

    /** Drift < 0,3 s: nichts; bis 1 s: ±5 % Tempo; darüber: springen. */
    fun correct(current: Double, target: Double, baseRate: Double): Fix {
        val drift = target - current // > 0: wir hängen hinterher
        return when {
            abs(drift) < 0.3 -> Fix.Rate(baseRate.toFloat())
            abs(drift) <= 1.0 -> Fix.Rate((baseRate * if (drift > 0) 1.05 else 0.95).toFloat())
            else -> Fix.Seek(target)
        }
    }
}

class PartyClient(private val api: ApiClient) {
    private val json = Json { ignoreUnknownKeys = true; encodeDefaults = true }
    private val type = "application/json".toMediaType()
    private val sse = api.http.newBuilder().readTimeout(0, TimeUnit.MILLISECONDS).build() // Strom bleibt offen

    private suspend fun call(method: String, path: String, body: String? = null): String = kotlinx.coroutines.withContext(Dispatchers.IO) {
        val req = Request.Builder().url(api.abs(path)).method(method, body?.toRequestBody(type))
        if (api.token.isNotEmpty()) req.header("Authorization", "Bearer ${api.token}")
        api.http.newCall(req.build()).execute().use { r ->
            val t = r.body.string()
            if (!r.isSuccessful) throw ApiException(r.code, t)
            t
        }
    }

    suspend fun offset(): Long {
        val samples = (1..5).map {
            val t0 = System.currentTimeMillis()
            val now = json.decodeFromString<Map<String, Long>>(call("GET", "/api/time"))["now"] ?: t0
            Triple(t0, now, System.currentTimeMillis())
        }
        return PartySync.offset(samples)
    }

    suspend fun create(mediaId: String): PartyRoom = json.decodeFromString(call("POST", "/api/party", """{"mediaId":${json.encodeToString(mediaId)}}"""))
    suspend fun get(id: String): PartyRoom = json.decodeFromString<PartyRoom>(call("GET", "/api/party/$id")).copy(id = id)
    suspend fun act(id: String, a: PartyAction) { runCatching { call("POST", "/api/party/$id/actions", json.encodeToString(a)) } }

    /** Ereignisse des Raums; bei Abbruch wird nach 2 s neu verbunden (wie „retry: 2000“). */
    fun events(eventsUrl: String): Flow<SseEvent> = flow {
        while (currentCoroutineContext().isActive) {
            runCatching {
                sse.newCall(Request.Builder().url(api.abs(eventsUrl)).header("Accept", "text/event-stream").build()).execute().use { r ->
                    val src = r.body.source()
                    val p = SseParser()
                    while (!src.exhausted()) p.line(src.readUtf8LineStrict())?.let { emit(it) }
                }
            }
            delay(2000)
        }
    }.flowOn(Dispatchers.IO)

    fun state(data: String): PartyState = json.decodeFromString<Map<String, kotlinx.serialization.json.JsonElement>>(data)["state"]
        ?.let { json.decodeFromJsonElement(PartyState.serializer(), it) } ?: PartyState()

    fun field(data: String, key: String): String =
        (json.parseToJsonElement(data) as? kotlinx.serialization.json.JsonObject)?.get(key)?.let { (it as? kotlinx.serialization.json.JsonPrimitive)?.content } ?: ""
}
