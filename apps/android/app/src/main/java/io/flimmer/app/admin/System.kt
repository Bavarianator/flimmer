package io.flimmer.app.admin

import android.content.Context
import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.flimmer.app.ApiClient
import io.flimmer.app.ApiException
import io.flimmer.app.ui.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonObjectBuilder
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.put
import kotlinx.serialization.json.putJsonObject
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Request
import okhttp3.RequestBody
import okio.BufferedSink
import okio.source
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.concurrent.TimeUnit

// ---------- Umwandlung ----------

/** Erkannte Hardware-Beschleunigung und „Nachts vorbereiten“. Preset, CRF und Tone-Mapping stellt Flimmer selbst ein. */
@Composable
internal fun Umwandlung(api: ApiClient) {
    val d = laden(api) { api.hol<Diag>("/api/diagnostics") }
    Seite {
        Gruppe("Hardwarebeschleunigung", "Flimmer testet beim Start, welche Beschleunigung der Server hat, und nimmt die schnellste.") {
            Geladen(d) { x ->
                if (x.ffmpeg.isNotEmpty()) Kacheln {
                    Messwert("Encoder", x.hw.ifEmpty { "CPU" }, Modifier.weight(1f))
                    Messwert("Tempo (1080p)", if (x.hwSpeed > 0) komma(x.hwSpeed) + "×" else "–", Modifier.weight(1f))
                    Messwert("Prozessorkerne", "${x.cpus}", Modifier.weight(1f))
                    Messwert("ffmpeg", x.ffmpeg.substringBefore(' '), Modifier.weight(1f))
                } else Zeile("ffmpeg fehlt – Umwandeln ist nicht möglich.", icon = AI.FEHLER, farbe = K.AmpelRot)
            }
        }
        Nachts(api)
    }
}

@Composable
private fun Nachts(api: ApiClient) {
    val s = laden(api) { api.einstellungen() }
    val st = laden(api) { api.hol<OptStand>("/api/settings/optimize") }
    val a = rememberAktion()
    var von by remember { mutableStateOf("") }
    var bis by remember { mutableStateOf("") }
    var frei by remember { mutableStateOf("") }
    LaunchedEffect(s.daten) {
        s.daten?.optimize?.let { o ->
            val std = o.from == 0 && o.to == 0
            von = (if (std) 2 else o.from).toString()
            bis = (if (std) 6 else o.to).toString()
            frei = (if (o.minFreeGB > 0) o.minFreeGB else 20).toString()
        }
    }
    Gruppe("Nachts vorbereiten") {
        Geladen(s) { e ->
            val o = e.optimize
            val sichern = { off: Boolean ->
                a.los("Gespeichert", danach = { s.neu(); st.neu() }) {
                    api.einstellungenSetzen {
                        putJsonObject("optimize") {
                            put("off", off); put("from", von.toIntOrNull() ?: 0); put("to", bis.toIntOrNull() ?: 0); put("minFreeGB", frei.toIntOrNull() ?: 0)
                        }
                    }
                }
            }
            Zeile("Titel nachts vorbereiten", icon = Ic.UHR, unter = "Wandelt schwierige Titel vorab um, damit sie abends direkt laufen.", schalter = !o.off,
                onClick = { sichern(!o.off) })
            st.daten?.let { x ->
                Leise(if (x.waiting) "Startet, sobald der Server bereit ist."
                else "${x.done} fertig · ${x.pending} warten · Fenster ${x.window.orEmpty()}" +
                    (x.current?.let { " · Gerade: ${it.title} (${Math.round(it.percent)} %)" } ?: ""), Modifier.padding(top = 12.dp))
                x.lastError?.takeIf { it.isNotEmpty() }?.let { Leise(it, Modifier.padding(top = 4.dp), K.AmpelRot) }
            }
            val felder = listOf(Triple(von, { v: String -> von = v }, "Von (Uhr)"), Triple(bis, { v: String -> bis = v }, "Bis (Uhr)"),
                Triple(frei, { v: String -> frei = v }, "Mindestens frei (GB)"))
            if (kompakt) Column(Modifier.padding(top = 12.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                felder.forEach { (w, set, l) -> Eingabe(w, set, l, Modifier.fillMaxWidth(), zahl = true) }
                Knopf("Speichern", { sichern(o.off) })
            } else Row(Modifier.padding(top = 16.dp), verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                felder.forEach { (w, set, l) -> Eingabe(w, set, l, Modifier.weight(1f), zahl = true) }
                Knopf("Speichern", { sichern(o.off) })
            }
        }
    }
}

// ---------- Geräte & Aktivitäten ----------

private val SYSTEM = setOf("scan", "task", "backup", "invite", "user")
private val HANDY = Regex("android|ios|handy|phone", RegexOption.IGNORE_CASE)

/** Angemeldete Geräte (abmelden) und das Aktivitätsprotokoll mit Filter. */
@Composable
internal fun Geraete(api: ApiClient) {
    val z = laden(api) { api.geraete() }
    val a = rememberAktion()
    var sicher by remember { mutableStateOf("") }
    val eng = kompakt
    Seite {
        Gruppe("Geräte", "Ein abgemeldetes Gerät muss sich neu anmelden oder neu gekoppelt werden.") {
            Geladen(z) { l ->
                if (l.isEmpty()) Leise("Noch kein Gerät angemeldet.")
                l.forEach { d ->
                    val c = d.client.orEmpty()
                    val unter = listOf(d.client, d.user, d.ip).filter { !it.isNullOrEmpty() }.joinToString(" · ")
                    Zeile(d.name + if (d.current) " · dieses Gerät" else "",
                        icon = if (HANDY.containsMatchIn(c)) Ic.HANDY else if (c.contains("cast", true)) Ic.CAST else AI.FERNSEHER,
                        unter = if (eng) listOf(unter, vor(d.lastSeen)).filter { it.isNotEmpty() }.joinToString(" · ") else unter,
                        wert = if (eng) null else vor(d.lastSeen),
                        rechts = if (d.current) null else ({
                            Knopf(if (sicher == d.id) "Wirklich abmelden?" else "Abmelden", {
                                if (sicher == d.id) a.los(danach = z.neu) { api.tu("DELETE", "/api/devices/${k(d.id)}") } else sicher = d.id
                            })
                        }))
                }
            }
        }
        Aktivitaeten(api)
    }
}

@Composable
private fun Aktivitaeten(api: ApiClient) {
    val z = laden(api) { api.aktivitaeten(200) }
    var f by remember { mutableStateOf("") }
    Nachladen(15_000) { z.neu() }
    Gruppe("Aktivitäten") {
        Wahl(listOf("" to "Alle", "play" to "Wiedergabe", "login" to "Anmeldung", "system" to "System", "error" to "Fehler"), f) { f = it }
        Spacer(Modifier.height(12.dp))
        Geladen(z) { l ->
            val liste = l.filter { f.isEmpty() || if (f == "system") it.kind in SYSTEM else it.kind == f }
            if (liste.isEmpty()) Leise("Keine Einträge.")
            liste.forEach { x ->
                Zeile(x.text, icon = aktivitaetIcon(x.kind), farbe = if (x.kind == "error") K.AmpelRot else K.Text, unter = x.user ?: "System", wert = vor(x.time))
            }
        }
    }
}

// ---------- Live-TV ----------

private const val FREI = "flimmer:freie-sender"
private fun vorlageName(id: String) = when (id) {
    "frei" -> "Freie Sender (ARD, ZDF, 3sat, arte …)"
    "fritz" -> "FRITZ!Box mit Kabel-TV"
    else -> id
}
private fun vorlageText(p: LiveTVVorlage) = when {
    p.channels <= 0 -> "Keine FRITZ!Box mit Kabel-TV im Heimnetz gefunden."
    p.id == "fritz" -> "${p.channels} Sender aus deinem Kabelanschluss gefunden, mit Programm."
    else -> "${p.channels} öffentlich-rechtliche Sender übers Internet, mit Programm. Meist nur in Deutschland abrufbar."
}

/** Schnell einrichten (Vorlagen), eigene Senderliste (M3U oder HDHomeRun), Programm (XMLTV), Bildumwandlung, Kanäle. DVR gibt es nicht. */
@Composable
internal fun LiveTVAdmin(api: ApiClient) {
    val z = laden(api) { api.hol<LiveTVStatus>("/api/livetv") }
    val v = laden(api) { api.liste<LiveTVVorlage>("/api/livetv/vorlagen") }
    val kz = laden(api) { api.liste<Kanal>("/api/livetv/channels") }
    val a = rememberAktion()
    val scope = rememberCoroutineScope()
    var quelle by remember { mutableStateOf("") }
    var epg by remember { mutableStateOf("") }
    val spaeter = { ms: Long -> scope.launch { delay(ms); z.neu(); kz.neu(); v.neu() }; Unit }
    // Der Server zeigt danach nur Schema://Rechner (ohne Zugangsdaten), die Felder bleiben deshalb leer.
    fun setzen(danach: () -> Unit = {}, b: JsonObjectBuilder.() -> Unit) =
        a.los("Gespeichert", danach = { danach(); spaeter(1500) }) { api.roh("PUT", "/api/livetv", obj(b)) }
    Seite {
        Geladen(z) { s ->
            Gruppe("Schnell einrichten", "Ein Knopf trägt Sender und Programm ein. Adressen braucht es dafür nicht.") {
                Geladen(v) { l ->
                    l.forEach { p ->
                        val da = p.channels > 0
                        Zeile(vorlageName(p.id), Modifier.alpha(if (da) 1f else 0.5f), icon = if (p.id == "fritz") AI.FERNSEHER else Ic.LIVE,
                            unter = vorlageText(p), farbe = if (da) K.Text else K.AmpelGelb, an = p.active,
                            wert = if (p.active) "Aktiv" else if (da) "Übernehmen" else null,
                            onClick = if (da && !p.active) ({ setzen { put("source", p.source); put("epg", p.epg) } }) else null)
                    }
                }
            }
            Gruppe("Eigene Quelle", zusatz = {
                Knopf("Programmdaten jetzt aktualisieren", { a.los("Wird neu geladen …", danach = { spaeter(2000) }) { api.tu("POST", "/api/livetv/refresh") } }, icon = Ic.NEUSTART)
            }) {
                if (s.error.isNotEmpty()) Hinweis(s.error, icon = AI.FEHLER, farbe = K.AmpelRot)
                Leise("${s.channels} Kanäle · ${s.programs} Sendungen · aktualisiert ${vor(s.updated)}", Modifier.padding(vertical = 12.dp))
                val quelleOk = { if (quelle.isNotEmpty()) setzen({ quelle = "" }) { put("source", quelle) } }
                FormZeile({ Eingabe(quelle, { quelle = it }, "Senderliste (M3U oder HDHomeRun)", it, onEnter = quelleOk) }) {
                    AKnopf("Speichern", quelleOk, aus = quelle.isEmpty())
                }
                Leise(if (s.source.isNotEmpty()) "Gespeichert: ${if (s.source == FREI) vorlageName("frei") else s.source}"
                else "Adresse einer M3U-Liste, z. B. von Tvheadend, oder http://<HDHomeRun>/lineup.json. Auch ein Dateipfad geht.", Modifier.padding(top = 8.dp, bottom = 16.dp))
                val epgOk = { if (epg.isNotEmpty()) setzen({ epg = "" }) { put("epg", epg) } }
                FormZeile({ Eingabe(epg, { epg = it }, "Programmführer (XMLTV)", it, onEnter = epgOk) }) {
                    AKnopf("Speichern", epgOk, aus = epg.isEmpty())
                }
                Leise(if (s.epg.isNotEmpty()) "Gespeichert: ${s.epg}"
                else "XMLTV-Adresse oder -Datei, auch .xml.gz. Die Zuordnung läuft über tvg-id, sonst über den Sendernamen.", Modifier.padding(top = 8.dp))
                Unterkopf("Bild")
                Wahl(listOf("copy" to "Kopieren (schnell)", "h264" to "In H.264 umwandeln"), s.video.ifEmpty { "copy" }) { v -> setzen { put("video", v) } }
                Leise("Kopieren braucht kaum Leistung, aber SD-Sender (MPEG-2) laufen dann nicht im Browser.", Modifier.padding(top = 8.dp))
            }
            Gruppe("Kanäle", "Live-TV wird ohne Umweg nach HLS verpackt. Höchstens 3 Kanäle laufen gleichzeitig.") {
                Geladen(kz) { l ->
                    if (l.isEmpty()) Leise("Noch keine Kanäle.")
                    l.forEach { c ->
                        Zeile((c.number?.takeIf { it.isNotEmpty() }?.let { "$it · " } ?: "") + c.name, icon = Ic.LIVE,
                            farbe = if (c.now == null) K.AmpelGelb else K.Text,
                            unter = c.now?.let { "${uhrzeit(it.start)} ${it.title}" } ?: "kein Programm zugeordnet", wert = c.group)
                    }
                }
            }
        }
    }
}

// ---------- Netzwerk & Fernzugriff ----------

private val ANBIETER = mapOf("tailscale" to "Tailscale", "netbird" to "NetBird", "vpn" to "VPN")

/** Adresse im Heimnetz, Fernzugriff über das Relay (Status, Prüfen, App-Code), erkannte VPNs. */
@Composable
internal fun Netzwerk(api: ApiClient) {
    val s = laden(api) { api.einstellungen() }
    val a = rememberAktion()
    Seite {
        Geladen(s) { d ->
            Gruppe("Heimnetz") {
                Zeile("Adresse im Heimnetz", icon = AI.NETZWERK, unter = d.lanUrl, rechts = { Knopf("Kopieren", { a.kopieren(d.lanUrl) }, icon = Ic.TEILEN) })
            }
            Gruppe("Fernzugriff") {
                if (d.remoteAvailable) Fern(api, d.remote) { a.los("Gespeichert", danach = s.neu) { api.einstellungenSetzen { put("remote", !d.remote) } } }
                else Leise("Fernzugriff ist in diesem Build nicht verfügbar.")
            }
        }
        VpnTeil(api)
    }
}

@Composable
private fun Fern(api: ApiClient, an: Boolean, umschalten: () -> Unit) {
    val st = laden(api) { api.hol<Remote>("/api/remote") }
    val a = rememberAktion()
    var prueft by remember { mutableStateOf(false) }
    var code by remember { mutableStateOf<AppCode?>(null) }
    Zeile("Von unterwegs schauen", icon = AI.FERNZUGRIFF, unter = "Filme laufen direkt von hier zu dir, über keinen fremden Server.", schalter = an, onClick = umschalten)
    st.daten?.let { x ->
        Zeile(if (x.reachable) "Von unterwegs erreichbar" else "Nicht erreichbar", icon = if (x.reachable) Ic.HAKEN else AI.FEHLER,
            farbe = if (x.reachable) K.Text else K.AmpelRot,
            unter = listOf(x.publicUrl, x.hint.ifEmpty { if (x.method.isNotEmpty() && x.method != "none") x.method.uppercase() else "" }).filter { it.isNotEmpty() }.joinToString(" · "),
            rechts = {
                AKnopf(if (prueft) "Prüft … (bis 30 s)" else "Fernzugriff prüfen", {
                    prueft = true
                    a.los(danach = st.neu) { try { api.tu("POST", "/api/remote/check") } finally { prueft = false } }
                }, icon = Ic.NEUSTART, aus = prueft)
            })
    }
    val c = code
    Zeile("Code für die App", icon = Ic.SCHLUESSEL,
        unter = if (c != null) "Gilt bis ${uhrzeit(c.expires)} Uhr" else "Die Android-App verbindet sich damit auch von unterwegs.",
        rechts = {
            if (c != null) T(c.code, if (LocalTv.current) 40.sp else 22.sp, K.Text, family = Mono, spacing = 2.sp)
            else Knopf("Code erzeugen", { a.los { code = api.schick("POST", "/api/remote/pair", "{}") } })
        })
}

@Composable
private fun VpnTeil(api: ApiClient) {
    val z = laden(api) { api.hol<Vpn>("/api/vpn") }
    val a = rememberAktion()
    Gruppe("VPN") {
        Geladen(z) { v ->
            if (v.addrs.isEmpty()) Leise("Kein VPN erkannt.")
            v.addrs.forEach { x ->
                Zeile(ANBIETER[x.provider] ?: x.provider, icon = AI.NETZWERK, unter = "${x.url} · ${x.iface}",
                    rechts = { Knopf("Adresse kopieren", { a.kopieren(x.url) }, icon = Ic.TEILEN) })
            }
            if (v.hint.isNotEmpty()) Leise(v.hint, Modifier.padding(top = 8.dp))
        }
    }
}

// ---------- Geplante Aufgaben ----------

/** Nach Gruppe, mit letztem Lauf, nächstem Termin und „Jetzt starten“. Die Zeiten legt der Server fest. */
@Composable
internal fun Aufgaben(api: ApiClient) {
    val z = laden(api) { api.aufgaben() }
    val a = rememberAktion()
    val laeuft = z.daten.orEmpty().any { it.running }
    Nachladen(if (laeuft) 2000 else 30_000) { z.neu() }
    Seite {
        Geladen(z) { l ->
            if (l.isEmpty()) Leise("Keine Aufgaben.", Modifier.padding(top = 16.dp))
            l.map { it.group }.distinct().forEach { g ->
                Gruppe(g) { l.filter { it.group == g }.forEach { t -> AufgabeZeile(t) { a.los(danach = z.neu) { api.aufgabeStarten(t.id) } } } }
            }
        }
    }
}

/** Stand einer Aufgabe wie im Web: „Läuft · 42 %“, „Zuletzt vor 3 Std. · Dauer 7 Min.“, „Noch nie gelaufen“ … */
internal fun aufgabeStand(t: Task, jetzt: Long = System.currentTimeMillis()): String {
    val fehler = t.lastResult == "error" && !t.running
    val stand = when {
        t.running -> "Läuft · ${Math.round((t.progress ?: 0.0) * 100)} %"
        zeit(t.lastRun) != null -> (if (t.lastResult == "error") "Fehlgeschlagen ${vor(t.lastRun, jetzt)}" else "Zuletzt ${vor(t.lastRun, jetzt)}") +
            (t.duration?.takeIf { it > 0 }?.let { " · Dauer ${dauer(it)}" } ?: "")
        else -> "Noch nie gelaufen"
    }
    return stand + (if (fehler && !t.lastError.isNullOrEmpty()) " · ${t.lastError}" else "") +
        (if (zeit(t.next) != null && !t.running) " · Nächster Lauf ${datum(t.next)}" else "")
}

@Composable
private fun AufgabeZeile(t: Task, starten: () -> Unit) {
    val fehler = t.lastResult == "error" && !t.running
    Zeile(t.name, unter = t.description, farbe = if (fehler) K.AmpelRot else K.Text,
        unten = {
            T(aufgabeStand(t), LocalTypo.current.klein, if (fehler) K.AmpelRot else K.Text3)
            if (t.running) Balken((t.progress ?: 0.0).toFloat(), Modifier.padding(top = 8.dp))
        },
        rechts = {
            IconKnopf(Ic.ABSPIELEN, { if (!t.running) starten() }, Modifier.alpha(if (t.running) 0.4f else 1f)
                .semantics { contentDescription = "${t.name} jetzt starten" }, rahmen = true)
        })
}

// ---------- Sicherung & Protokoll ----------

/** Sicherung laden/einspielen (Android-Speichern-Dialog), Serverprotokoll mit Stufenfilter, Diagnose. */
@Composable
internal fun Sicherung(api: ApiClient) {
    val d = laden(api) { val s = api.roh("GET", "/api/diagnostics"); s to json.decodeFromString(Diag.serializer(), s) }
    val a = rememberAktion()
    Seite {
        Sicherungen(api, d.daten?.second, a)
        Protokoll(api, d.daten?.second?.log.orEmpty(), a)
        Gruppe("Diagnose") { Geladen(d) { (roh, x) -> Diagnose(roh, x, a) } }
    }
}

// Binär, deshalb nicht über roh() (liefert Text): derselbe OkHttp-Client und dasselbe Token wie ApiClient, ohne Lese-Zeitlimit.
private fun ApiClient.anfrage(pfad: String) = Request.Builder().url(base + pfad).apply { if (token.isNotEmpty()) header("Authorization", "Bearer $token") }
private fun ApiClient.ohneLimit() = http.newBuilder().readTimeout(0, TimeUnit.SECONDS).writeTimeout(0, TimeUnit.SECONDS).build()

private suspend fun ApiClient.sicherungLaden(ctx: Context, ziel: Uri) = withContext(Dispatchers.IO) {
    ohneLimit().newCall(anfrage("/api/settings/backup").build()).execute().use { r ->
        if (!r.isSuccessful) throw ApiException(r.code, r.body.string().ifEmpty { r.message })
        val aus = ctx.contentResolver.openOutputStream(ziel) ?: error("Die Datei lässt sich nicht schreiben.")
        aus.use { r.body.byteStream().copyTo(it) }
    }
}

private suspend fun ApiClient.sicherungEinspielen(ctx: Context, quelle: Uri) = withContext(Dispatchers.IO) {
    val body = object : RequestBody() {
        override fun contentType() = "application/octet-stream".toMediaType()
        override fun writeTo(sink: BufferedSink) {
            (ctx.contentResolver.openInputStream(quelle) ?: error("Die Datei lässt sich nicht lesen.")).source().use { sink.writeAll(it) }
        }
    }
    ohneLimit().newCall(anfrage("/api/settings/restore").post(body).build()).execute().use { r ->
        if (!r.isSuccessful) throw ApiException(r.code, r.body.string().ifEmpty { r.message })
    }
}

@Composable
private fun Sicherungen(api: ApiClient, d: Diag?, a: Aktion) {
    val ctx = LocalContext.current
    var laeuft by remember { mutableStateOf(false) }
    val speichern = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("application/vnd.sqlite3")) { uri ->
        if (uri != null) a.los("Sicherung gespeichert") { api.sicherungLaden(ctx, uri) }
    }
    val oeffnen = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) {
            laeuft = true
            a.los("Sicherung eingespielt") { try { api.sicherungEinspielen(ctx, uri) } finally { laeuft = false } }
        }
    }
    val db = d?.db
    Gruppe("Sicherungen") {
        Zeile("Sicherung herunterladen", icon = Ic.DOWNLOAD,
            unter = if (db != null && zeit(db.backupAt) != null) "Letzte automatische Sicherung: ${datum(db.backupAt)} · ${groesse(db.sizeBytes)}"
            else "Profile, Fortschritt und Einstellungen als eine Datei",
            onClick = {
                runCatching { speichern.launch("flimmer-backup-${SimpleDateFormat("yyyy-MM-dd", Locale.GERMANY).format(Date())}.db") }
                    .onFailure { a.melde("Auf diesem Gerät gibt es keinen Speichern-Dialog.") }
            })
        Zeile("Sicherung einspielen …", icon = AI.SICHERUNG, unter = "Prüft Version und Zustand, bevor etwas ersetzt wird", onClick = {
            if (!laeuft) runCatching { oeffnen.launch(arrayOf("*/*")) }.onFailure { a.melde("Auf diesem Gerät gibt es keine Dateiauswahl.") }
        })
    }
}

private val STUFEN = listOf("" to "Alle", "info" to "INF", "warn" to "WRN", "error" to "ERR")
private val KURZ = mapOf("DEBUG" to "DBG", "INFO" to "INF", "WARN" to "WRN", "ERROR" to "ERR")

/** Eine Protokollzeile wie im Web: „20:01 [ERR] Aufgabe fehlgeschlagen {…}“. */
internal fun logText(l: LogZeile) = uhrzeit(l.time) + " [" + (KURZ[l.level.uppercase()] ?: l.level) + "] " + l.msg + (l.attrs?.let { " $it" } ?: "")

/** /api/logs (level = Mindeststufe); fehlt der Endpunkt, zeigt es die letzten Zeilen aus /api/diagnostics. */
@Composable
private fun Protokoll(api: ApiClient, diagLog: List<String>, a: Aktion) {
    var stufe by remember { mutableStateOf("") }
    var pause by remember { mutableStateOf(false) }
    val z = laden(api, stufe) { api.protokoll(stufe) }
    val ohne = fehlt(z.fehler) && z.daten == null
    Nachladen(5000, !pause && !ohne) { z.neu() }
    val zeilen = if (ohne) diagLog.takeLast(300) else z.daten.orEmpty().map(::logText)
    val tv = LocalTv.current
    Gruppe("Protokoll", zusatz = {
        if (!ohne) Knopf(if (pause) "Fortsetzen" else "Pausieren", { pause = !pause }, icon = if (pause) Ic.ABSPIELEN else Ic.PAUSE)
        Knopf("Kopieren", { a.kopieren(zeilen.joinToString("\n")) }, icon = Ic.TEILEN)
    }) {
        if (!ohne) Wahl(STUFEN, stufe) { stufe = it }
        val f = z.fehler
        if (f != null && !ohne && z.daten == null) Leise(fehlerText(f), Modifier.padding(top = 12.dp), K.AmpelRot)
        else Column(Modifier.padding(top = 12.dp).fillMaxWidth().background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusM)).padding(12.dp)) {
            if (zeilen.isEmpty()) Leise("Keine Einträge.")
            // TV: jede Zeile fokussierbar, damit das D-Pad durch das Protokoll scrollt.
            zeilen.forEach { T(it, if (tv) 20.sp else 12.sp, K.Text2, family = Mono, modifier = if (tv) Modifier.fokusRing(skala = false).focusable() else Modifier) }
        }
    }
}

private val schoen = Json { prettyPrint = true }

@Composable
private fun Diagnose(roh: String, d: Diag, a: Aktion) {
    Kacheln {
        Messwert("Laufende Streams", "${d.active.size}", Modifier.weight(1f))
        Messwert("Hardware-Beschleunigung", d.hw.ifEmpty { "CPU" } + if (d.hwSpeed > 0) " · ${komma(d.hwSpeed)}×" else "", Modifier.weight(1f))
        Messwert("Datenbank", groesse(d.db.sizeBytes) + if (d.db.integrity.isNotEmpty()) " · ${d.db.integrity}" else "", Modifier.weight(1f))
        Messwert("Freier Speicher", groesse(d.diskFree), Modifier.weight(1f), anteil = if (d.diskTotal > 0) 1f - d.diskFree.toFloat() / d.diskTotal else null)
    }
    Unterkopf("Laufende Wiedergaben")
    if (d.active.isEmpty()) Leise("Gerade schaut niemand.", Modifier.padding(bottom = 12.dp))
    d.active.forEach { x ->
        Zeile("„${x.title}“ · ${x.user} · ${x.device}", vorne = { AmpelPunkt(x.light) },
            unter = methode(x.method) + if (x.reasons.isNotEmpty()) ": " + x.reasons.joinToString(" · ") else "")
    }
    Zeile(d.ffmpeg.ifEmpty { "ffmpeg –" } + " · " + d.hw.ifEmpty { "CPU" }, icon = if (d.ffmpeg.isNotEmpty()) Ic.HAKEN else AI.FEHLER,
        unter = "${d.os} · ${d.cpus} CPUs · ${d.version}")
    val ok = d.db.integrity == "ok" || d.db.integrity.isEmpty()
    Zeile("Datenbank", icon = if (ok) Ic.HAKEN else AI.FEHLER, farbe = if (ok) K.Text else K.AmpelRot,
        unter = "Letzte Prüfung ${datum(d.db.checkedAt)} · ${d.db.integrity.ifEmpty { "–" }}", wert = groesse(d.db.sizeBytes))
    Zeile("Diagnose kopieren", icon = AI.PROTOKOLL, unter = "Kopiert Version, Hardware, Zustand und Protokoll als Text – zum Einfügen in eine Fehlermeldung.", onClick = {
        val ohneLog = runCatching { schoen.encodeToString(JsonObject.serializer(), JsonObject(json.parseToJsonElement(roh).jsonObject - "log")) }.getOrDefault(roh)
        a.kopieren(ohneLog + "\n\n" + d.log.joinToString("\n"))
    })
}
