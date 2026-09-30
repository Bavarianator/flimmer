package io.flimmer.app

import android.app.DownloadManager
import android.content.Context
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.unit.sp
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import io.flimmer.app.ui.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import okhttp3.Request
import java.io.File

/**
 * Offline-Titel (Handy/Tablet). Die Originaldatei lädt der System-DownloadManager: läuft im Hintergrund weiter,
 * zeigt eine Benachrichtigung, setzt nach Abbrüchen fort und braucht im App-eigenen Ordner kein Speicherrecht.
 * Daneben liegen <id>.json (das Item, damit die Liste ohne Server geht) und <id>.jpg (Poster).
 * ExoPlayer mit ffmpeg-Decoder spielt das Original fast immer direkt, eine umgewandelte Fassung braucht es nicht.
 */
class Downloads(private val ctx: Context) {
    private val dm = ctx.getSystemService(Context.DOWNLOAD_SERVICE) as DownloadManager
    private val dir get() = (ctx.getExternalFilesDir(DIR) ?: File(ctx.filesDir, DIR)).apply { mkdirs() }
    private val prefs = ctx.getSharedPreferences("flimmer-offline", Context.MODE_PRIVATE)
    private val json = Json { ignoreUnknownKeys = true; encodeDefaults = true }

    @Serializable
    private data class Entry(val item: Item, val downloadId: Long)

    enum class Status { Wartet, WartetWlan, Laedt, Fertig, Fehler }

    data class Offline(val item: Item, val status: Status, val done: Long, val total: Long, val poster: String, val file: File)

    /** Große Dateien standardmäßig nur im WLAN; gilt für neue Downloads. */
    var wifiOnly: Boolean
        get() = prefs.getBoolean("wifiOnly", true)
        set(v) = prefs.edit().putBoolean("wifiOnly", v).apply()

    /** Startet den Download (Original über GET /api/items/{id}/file); das Poster kommt gleich mit. */
    suspend fun start(api: ApiClient, item: Item) = withContext(Dispatchers.IO) {
        if (entry(item.id) != null) return@withContext
        if (item.poster.isNotEmpty()) runCatching {
            api.http.newCall(Request.Builder().url(api.abs(item.poster)).build()).execute().use { r ->
                if (r.isSuccessful) File(dir, "${item.id}.jpg").writeBytes(r.body.bytes())
            }
        }
        val req = DownloadManager.Request(Uri.parse(api.abs("/api/items/${item.id}/file")))
            .addRequestHeader("Authorization", "Bearer ${api.token}")
            .setTitle(item.series?.let { "$it · S${item.season} E${item.episode}" } ?: item.displayTitle)
            .setDescription("Flimmer · wird offline gespeichert")
            .setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
            .setDestinationInExternalFilesDir(ctx, DIR, "${item.id}.media")
            .setAllowedOverMetered(!wifiOnly)
            .setAllowedOverRoaming(false)
        File(dir, "${item.id}.json").writeText(json.encodeToString(Entry(item, dm.enqueue(req))))
    }

    private fun entry(id: String): Entry? =
        File(dir, "$id.json").takeIf { it.exists() }?.let { runCatching { json.decodeFromString<Entry>(it.readText()) }.getOrNull() }

    fun get(id: String): Offline? = entry(id)?.let(::read)

    fun list(): List<Offline> = dir.listFiles { f -> f.name.endsWith(".json") }.orEmpty()
        .mapNotNull { entry(it.name.removeSuffix(".json"))?.let(::read) }
        .sortedWith(compareBy({ it.item.series ?: it.item.displayTitle }, { it.item.season ?: 0 }, { it.item.episode ?: 0 }))

    private fun read(e: Entry): Offline {
        val file = File(dir, "${e.item.id}.media")
        var status = if (file.exists()) Status.Fertig else Status.Fehler // Eintrag beim DownloadManager weg: Datei zählt
        var done = file.length()
        var total = done
        dm.query(DownloadManager.Query().setFilterById(e.downloadId))?.use { c ->
            if (!c.moveToFirst()) return@use
            done = c.getLong(c.getColumnIndexOrThrow(DownloadManager.COLUMN_BYTES_DOWNLOADED_SO_FAR))
            total = c.getLong(c.getColumnIndexOrThrow(DownloadManager.COLUMN_TOTAL_SIZE_BYTES))
            status = when (c.getInt(c.getColumnIndexOrThrow(DownloadManager.COLUMN_STATUS))) {
                DownloadManager.STATUS_SUCCESSFUL -> Status.Fertig
                DownloadManager.STATUS_RUNNING -> Status.Laedt
                DownloadManager.STATUS_PAUSED ->
                    if (c.getInt(c.getColumnIndexOrThrow(DownloadManager.COLUMN_REASON)) == DownloadManager.PAUSED_QUEUED_FOR_WIFI) Status.WartetWlan else Status.Wartet
                DownloadManager.STATUS_PENDING -> Status.Wartet
                else -> Status.Fehler
            }
        }
        val poster = File(dir, "${e.item.id}.jpg").takeIf { it.exists() }?.let { Uri.fromFile(it).toString() } ?: ""
        return Offline(e.item, status, done, total, poster, file)
    }

    /** Gesamter und freier Speicher dort, wo die Downloads liegen (Bytes). */
    fun speicher(): Pair<Long, Long> = runCatching { android.os.StatFs(dir.path).let { it.totalBytes to it.availableBytes } }.getOrDefault(0L to 0L)

    /** Bricht ab bzw. löscht Datei, Poster und Eintrag. */
    fun remove(id: String) {
        entry(id)?.let { dm.remove(it.downloadId) }
        listOf(".json", ".jpg", ".media").forEach { File(dir, id + it).delete() }
    }

    // ---------- Fortschritt, der den Server nicht erreicht hat (offline geschaut) ----------

    fun remember(id: String, pos: Long, dur: Long, audio: String, subtitle: String) =
        prefs.edit().putString("p:$id", Pending(pos, dur, audio, subtitle).encode()).apply()

    fun forget(id: String) = prefs.edit().remove("p:$id").apply()

    /** Offline gemerkte Position in Sekunden, sonst null. */
    fun pendingPos(id: String): Double? = prefs.getString("p:$id", null)?.let(Pending::decode)?.pos?.toDouble()

    /** Sendet gemerkten Fortschritt beim nächsten Kontakt; bricht beim ersten Netzfehler ab.
     *  ponytail: letzte Meldung gewinnt – wer danach woanders weiterschaut, bevor das Handy online war, springt zurück. */
    suspend fun flush(api: ApiClient) {
        for ((k, v) in prefs.all) {
            if (!k.startsWith("p:") || v !is String) continue
            val id = k.removePrefix("p:")
            val p = Pending.decode(v)
            if (p == null) {
                forget(id)
                continue
            }
            val r = runCatching { api.progress(id, p.pos, p.dur, p.audio, p.subtitle) }
            if (r.isSuccess || (r.exceptionOrNull() as? ApiException)?.code == 404) forget(id) else return
        }
    }

    internal data class Pending(val pos: Long, val dur: Long, val audio: String, val subtitle: String) {
        fun encode() = "$pos|$dur|$audio|$subtitle"

        companion object {
            fun decode(s: String): Pending? {
                val p = s.split('|')
                if (p.size != 4) return null
                return Pending(p[0].toLongOrNull() ?: return null, p[1].toLongOrNull() ?: return null, p[2], p[3])
            }
        }
    }

    companion object {
        const val DIR = "downloads"
    }
}

private fun gb(bytes: Long) = if (bytes < 1e9) "%d MB".format(bytes / 1_000_000) else "%.1f GB".format(java.util.Locale.GERMANY, bytes / 1e9)

private fun statusText(o: Downloads.Offline) = when (o.status) {
    Downloads.Status.Fertig -> "OFFLINE VERFÜGBAR"
    Downloads.Status.Laedt -> "LÄDT"
    Downloads.Status.Wartet -> "WARTET"
    Downloads.Status.WartetWlan -> "WARTET AUF WLAN"
    Downloads.Status.Fehler -> "FEHLGESCHLAGEN – ENTFERNEN UND NEU LADEN"
}

private fun titel(o: Downloads.Offline) = o.item.series?.let { "$it · ${folge(o.item)} · ${o.item.displayTitle}" } ?: o.item.displayTitle

/** Downloads wie im Entwurf: Speicher, „Wird geladen“, „Auf diesem Gerät“ (Serien zum Aufklappen). Aktualisiert sich jede Sekunde. */
@Composable
fun DownloadsScreen(downloads: Downloads, onPlay: (Item) -> Unit) {
    var list by remember { mutableStateOf(downloads.list()) }
    var wifi by remember { mutableStateOf(downloads.wifiOnly) }
    var offen by remember { mutableStateOf(setOf<String>()) }
    var frei by remember { mutableStateOf(0L to 0L) }
    LaunchedEffect(Unit) {
        while (true) {
            list = withContext(Dispatchers.IO) { downloads.list() }
            frei = withContext(Dispatchers.IO) { downloads.speicher() }
            delay(1000)
        }
    }
    val laden = list.filter { it.status != Downloads.Status.Fertig }
    val fertig = list.filter { it.status == Downloads.Status.Fertig }
    val gruppen = fertig.groupBy { it.item.series ?: ("film:" + it.item.id) }
    val belegt = list.sumOf { it.done }
    val remove: (Downloads.Offline) -> Unit = { o -> downloads.remove(o.item.id); list = downloads.list() }
    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(start = Pad, end = Pad, top = 16.dp, bottom = 24.dp)) {
        item {
            Column(Modifier.fillMaxWidth().background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusM))) {
                Column(Modifier.padding(16.dp)) {
                    Label("Speicher auf diesem Gerät")
                    val (gesamt, rest) = frei
                    T("Belegt ${gb(gesamt - rest)} von ${gb(gesamt)}", 15.sp, K.Text, FontWeight.Medium, modifier = Modifier.padding(top = 8.dp))
                    Fortschritt(if (gesamt > 0) (gesamt - rest).toFloat() / gesamt else 0f, Modifier.padding(top = 10.dp))
                    Row(Modifier.padding(top = 8.dp)) {
                        T("Flimmer: ${gb(belegt)}", 13.sp, K.Text2, modifier = Modifier.weight(1f))
                        T("Frei: ${gb(rest)}", 13.sp, K.Text2)
                    }
                }
                Row(Modifier.fillMaxWidth().heightIn(min = 64.dp).klick({ wifi = !wifi; downloads.wifiOnly = wifi }, skala = false).padding(horizontal = 16.dp, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        T("Nur im WLAN herunterladen", 15.sp, K.Text, FontWeight.Medium)
                        T("Gilt für neue Downloads. Mobile Daten bleiben unberührt.", 13.sp, K.Text2)
                    }
                    Schalter(wifi)
                }
            }
        }
        if (laden.isNotEmpty()) {
            item { DlKopf("Wird geladen", laden.size) }
            items(laden, key = { "l" + it.item.id }) { o ->
                DlZeile(o, { }, trailing = { IconKnopf(Ic.SCHLIESSEN, { remove(o) }, groesse = 44.dp, rahmen = true) }) {
                    T(titel(o), 15.sp, K.Text, FontWeight.Medium, maxLines = 1)
                    T(if (o.total > 0) "${100 * o.done / o.total} % · ${gb(o.done)} von ${gb(o.total)}" else "…", 13.sp, K.Text2, maxLines = 1)
                    T(statusText(o), 11.sp, K.Text3, family = Mono, spacing = 1.sp)
                }
            }
        }
        item { DlKopf("Auf diesem Gerät", fertig.size) }
        if (fertig.isEmpty()) item { T("Noch nichts heruntergeladen – bei einem Titel auf „Herunterladen“ tippen.", 15.sp, K.Text2, modifier = Modifier.padding(vertical = 12.dp)) }
        gruppen.forEach { (k, l) ->
            val erster = l.first()
            if (erster.item.series == null) item(key = k) {
                DlZeile(erster, { onPlay(erster.item) }, trailing = { IconKnopf(Ic.LOESCHEN, { remove(erster) }, groesse = 44.dp, farbe = K.Text2) }) {
                    T(erster.item.displayTitle, 15.sp, K.Text, FontWeight.Medium, maxLines = 1)
                    T(gb(erster.total), 13.sp, K.Text2)
                    if (downloads.pendingPos(erster.item.id) != null) T("Fortschritt wird beim nächsten Kontakt abgeglichen", 13.sp, K.Text3)
                }
            } else {
                val auf = k in offen
                item(key = k) {
                    DlZeile(erster, { offen = if (auf) offen - k else offen + k }, trailing = { Ico(if (auf) Ic.HOCH else Ic.RUNTER, 24.dp, K.Text2, modifier = Modifier.padding(10.dp)) }) {
                        T(k, 15.sp, K.Text, FontWeight.Medium, maxLines = 1)
                        T("${l.size} ${if (l.size == 1) "Folge" else "Folgen"} · ${gb(l.sumOf { it.total })}", 13.sp, K.Text2)
                    }
                }
                if (auf) items(l, key = { "e" + it.item.id }) { o ->
                    Row(Modifier.padding(start = 108.dp).fillMaxWidth().heightIn(min = 52.dp).drawBehind { drawRect(K.Linie, size = Size(size.width, 1.dp.toPx())) },
                        verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f).klick({ onPlay(o.item) }, skala = false).padding(vertical = 4.dp)) {
                            T("${folge(o.item)} · ${o.item.displayTitle}", 14.sp, K.Text, FontWeight.Medium, maxLines = 1)
                            T(listOfNotNull(fmtDauer(o.item.duration), gb(o.total), if (o.item.watched) "gesehen" else null).joinToString(" · "), 13.sp, K.Text2)
                            Fortschritt(if (o.item.watched) 1f else o.item.anteil(), Modifier.padding(top = 3.dp), 3.dp)
                        }
                        IconKnopf(Ic.LOESCHEN, { remove(o) }, groesse = 44.dp, farbe = K.Text2)
                    }
                }
            }
        }
        item {
            Row(Modifier.padding(top = 24.dp).fillMaxWidth(), horizontalArrangement = Arrangement.Center) {
                Ico(Ic.INFO, 20.dp, K.Text3)
                T("Heruntergeladene Titel laufen auch ohne Verbindung zum Server.", 13.sp, K.Text3, modifier = Modifier.padding(start = 10.dp))
            }
        }
    }
}

@Composable
private fun DlKopf(titel: String, n: Int) {
    Row(Modifier.padding(top = 24.dp, bottom = 4.dp).height(32.dp), verticalAlignment = Alignment.CenterVertically) {
        T(titel, 18.sp, K.Text, FontWeight.SemiBold)
        T("$n", 12.sp, K.Text3, family = Mono, modifier = Modifier.padding(start = 8.dp))
    }
}

@Composable
private fun DlZeile(o: Downloads.Offline, onClick: () -> Unit, trailing: @Composable () -> Unit, text: @Composable ColumnScope.() -> Unit) {
    Row(Modifier.fillMaxWidth().heightIn(min = 76.dp).drawBehind { drawRect(K.Linie, size = Size(size.width, 1.dp.toPx())) }.padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically) {
        Row(Modifier.weight(1f).klick(onClick, skala = false), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(96.dp, 54.dp)) {
                Art(o.poster, o.item.series ?: o.item.displayTitle, Modifier.fillMaxSize(), ton(o.item.color))
                if (o.status != Downloads.Status.Fertig && o.total > 0) Fortschritt(o.done.toFloat() / o.total, Modifier.align(Alignment.BottomStart))
                else if (o.item.anteil() > 0f) Fortschritt(o.item.anteil(), Modifier.align(Alignment.BottomStart))
            }
            Column(Modifier.padding(start = 12.dp).weight(1f), content = text)
        }
        trailing()
    }
}
