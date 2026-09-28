package io.flimmer.app

import android.content.Context
import android.content.pm.PackageManager
import android.hardware.display.DisplayManager
import android.os.Build
import android.view.Display
import android.media.MediaCodecInfo.CodecProfileLevel
import android.media.MediaCodecList
import android.net.wifi.WifiManager
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.net.DatagramPacket
import java.net.InetAddress
import java.net.MulticastSocket
import java.net.SocketTimeoutException
import java.util.UUID

/** Gespeicherte Einstellungen: Server, Token, Geräte-ID. */
class Store(ctx: Context) {
    private val p = ctx.getSharedPreferences("flimmer", Context.MODE_PRIVATE)
    var server: String
        get() = p.getString("server", "") ?: ""
        set(v) = p.edit().putString("server", v).apply()
    var token: String
        get() = p.getString("token", "") ?: ""
        set(v) = p.edit().putString("token", v).apply()
    /** Merkliste (lokal, bis der Server eine hat). */
    var watchlist: Set<String>
        get() = p.getStringSet("watchlist", emptySet()) ?: emptySet()
        set(v) = p.edit().putStringSet("watchlist", v).apply()
    val deviceId: String
        get() = p.getString("device", null) ?: UUID.randomUUID().toString().replace("-", "").also { p.edit().putString("device", it).apply() }
}

fun isTv(ctx: Context) = ctx.packageManager.hasSystemFeature(PackageManager.FEATURE_LEANBACK)

/**
 * Geräteprofil aus den echten Decodern (MediaCodecList) statt Probe-Clips. Ton: Media3 + ffmpeg-Decoder
 * dekodiert alles Gängige in Software, also meldet die App auch DTS/TrueHD – der Server spielt dann direkt.
 */
fun deviceProfile(ctx: Context): Profile {
    val codecs = MediaCodecList(MediaCodecList.REGULAR_CODECS).codecInfos.filter { !it.isEncoder }
    fun has(mime: String, profile: Int? = null) = codecs.any { c ->
        c.supportedTypes.any { t ->
            t.equals(mime, true) && (profile == null || c.getCapabilitiesForType(t).profileLevels.any { it.profile == profile })
        }
    }
    val video = buildList {
        if (has("video/avc")) add("h264")
        if (has("video/hevc")) add("hevc")
        if (has("video/hevc", CodecProfileLevel.HEVCProfileMain10)) add("hevc10")
        if (has("video/av01")) add("av1")
        if (has("video/x-vnd.on2.vp9")) add("vp9")
    }
    return Profile(
        name = if (isTv(ctx)) "Android TV" else "Android",
        containers = listOf("mp4", "mkv", "webm", "ts"),
        video = video,
        audio = listOf("aac", "mp3", "ac3", "eac3", "opus", "flac", "dts", "truehd"),
        hdr = hdrTypes(ctx) { has("video/dolby-vision") },
    )
}

/** HDR-Fähigkeit des Displays (Vertrag mit playback: null = unbekannt, leer = SDR). */
private fun hdrTypes(ctx: Context, hasDvDecoder: () -> Boolean): List<String>? {
    if (Build.VERSION.SDK_INT < Build.VERSION_CODES.N) return null
    val display = (ctx.getSystemService(Context.DISPLAY_SERVICE) as DisplayManager).getDisplay(Display.DEFAULT_DISPLAY) ?: return null
    @Suppress("DEPRECATION") // ab API 34 gibt es Mode.supportedHdrTypes; die alte API liefert dasselbe
    val types = display.hdrCapabilities?.supportedHdrTypes ?: return emptyList()
    return types.toList().mapNotNull {
        when (it) {
            Display.HdrCapabilities.HDR_TYPE_HDR10 -> "hdr10"
            Display.HdrCapabilities.HDR_TYPE_HLG -> "hlg"
            Display.HdrCapabilities.HDR_TYPE_HDR10_PLUS -> "hdr10+"
            Display.HdrCapabilities.HDR_TYPE_DOLBY_VISION -> if (hasDvDecoder()) "dv" else null
            else -> null
        }
    }.distinct()
}

/** Sucht Flimmer-Server im Heimnetz per SSDP (internal/discovery antwortet auf diesen ST). */
suspend fun discover(ctx: Context, timeoutMs: Int = 2500): List<String> = withContext(Dispatchers.IO) {
    val wifi = ctx.applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
    val lock = wifi.createMulticastLock("flimmer-ssdp").apply { setReferenceCounted(false); acquire() }
    val found = linkedSetOf<String>()
    try {
        MulticastSocket().use { s ->
            s.soTimeout = 400
            val msg = ("M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\n" +
                "MX: 1\r\nST: urn:flimmer-media:service:flimmer:1\r\n\r\n").toByteArray()
            val group = InetAddress.getByName("239.255.255.250")
            val end = System.currentTimeMillis() + timeoutMs
            var nextSend = 0L
            val buf = ByteArray(2048)
            while (System.currentTimeMillis() < end) {
                if (System.currentTimeMillis() >= nextSend) { // UDP geht verloren – dreimal fragen
                    s.send(DatagramPacket(msg, msg.size, group, 1900))
                    nextSend = System.currentTimeMillis() + 800
                }
                try {
                    val pkt = DatagramPacket(buf, buf.size)
                    s.receive(pkt)
                    parseLocation(String(pkt.data, 0, pkt.length))?.let(found::add)
                } catch (_: SocketTimeoutException) {
                }
            }
        }
    } catch (_: Exception) {
        // kein WLAN/Multicast → manuelle Eingabe
    } finally {
        lock.release()
    }
    found.toList()
}

internal fun parseLocation(response: String): String? =
    response.lineSequence().firstOrNull { it.startsWith("LOCATION:", ignoreCase = true) }
        ?.substringAfter(':')?.trim()?.trimEnd('/')?.takeIf { it.startsWith("http") }

/** "192.168.1.5" → "http://192.168.1.5:8096" */
fun normalizeServer(input: String): String {
    var s = input.trim().trimEnd('/')
    if (!s.startsWith("http://") && !s.startsWith("https://")) s = "http://$s"
    val host = s.substringAfter("://")
    if (!host.substringAfterLast(']').contains(':')) s += ":8096"
    return s
}
