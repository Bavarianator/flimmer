package io.flimmer.app.admin

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.flimmer.app.ApiClient
import io.flimmer.app.ui.*

/** Bereich des Dashboards; [kopf] = Zwischenüberschrift davor (wie im Web-Menü). */
internal data class DashBereich(val id: String, val label: String, val icon: String, val kopf: String? = null)

internal val BEREICHE = listOf(
    DashBereich("uebersicht", "Übersicht", Ic.DASHBOARD),
    DashBereich("allgemein", "Allgemein & Branding", Ic.EINSTELLUNGEN, "SERVER"),
    DashBereich("benutzer", "Benutzer", Ic.GEMEINSAM),
    DashBereich("einladungen", "Einladungen", AI.EINLADUNG),
    DashBereich("bibliotheken", "Bibliotheken", AI.BIBLIOTHEK, "BIBLIOTHEKEN"),
    DashBereich("metadaten", "Metadaten-Manager", Ic.BEARBEITEN),
    DashBereich("umwandlung", "Umwandlung & Trickplay", Ic.ABSPIELEN, "WIEDERGABE"),
    DashBereich("geraete", "Geräte & Aktivitäten", AI.FERNSEHER, "GERÄTE"),
    DashBereich("livetv", "Live-TV & Aufnahmen", Ic.LIVE),
    DashBereich("netzwerk", "Netzwerk & Fernzugriff", AI.NETZWERK, "ERWEITERT"),
    DashBereich("aufgaben", "Geplante Aufgaben", AI.AUFGABEN),
    DashBereich("sicherung", "Sicherung & Protokoll", AI.SICHERUNG),
)

/**
 * Dashboard (nur Admins). [bereich]: "" = nichts gewählt (Handy: Bereichsliste, Tablet/TV: Übersicht), sonst eine ID aus [BEREICHE].
 * Handy: Liste der Bereiche, dann die Detailseite (Zurück führt zur Liste). Tablet und TV: Liste links, Inhalt rechts.
 */
@Composable
fun DashboardScreen(api: ApiClient, bereich: String, tv: Boolean, onBereich: (String) -> Unit, onZurueck: () -> Unit) {
    val b = BEREICHE.firstOrNull { it.id == bereich }
    val a = rememberAktion()
    val scannen = { a.los("Die Bibliothek wird neu eingelesen.") { api.tu("POST", "/api/rescan") } }
    Box(Modifier.fillMaxSize().background(K.Saal)) {
        if (tv || LocalBreit.current) {
            val aktiv = b ?: BEREICHE[0]
            Row(Modifier.fillMaxSize().systemBarsPadding()) {
                BereichListe(aktiv.id, onBereich, onZurueck, Modifier.width(if (tv) 440.dp else 280.dp).fillMaxHeight().background(K.Flaeche1))
                Box(Modifier.width(1.dp).fillMaxHeight().background(K.Linie))
                Column(Modifier.weight(1f)) {
                    Kopf(aktiv.label, null, scannen)
                    key(aktiv.id) { Inhalt(api, aktiv.id, onBereich) }
                }
            }
        } else if (b == null) {
            Column(Modifier.fillMaxSize().systemBarsPadding()) {
                Kopf("Dashboard", onZurueck, null)
                BereichListe("", onBereich, null, Modifier.weight(1f))
            }
        } else {
            BackHandler { onBereich("") }
            Column(Modifier.fillMaxSize().systemBarsPadding()) {
                Kopf(b.label, { onBereich("") }, scannen)
                key(b.id) { Inhalt(api, b.id, onBereich) }
            }
        }
    }
}

@Composable
private fun Inhalt(api: ApiClient, id: String, onBereich: (String) -> Unit) {
    Box(Modifier.fillMaxSize()) {
        when (id) {
            "uebersicht" -> Uebersicht(api, onBereich)
            "allgemein" -> Allgemein(api)
            "benutzer" -> Benutzer(api)
            "einladungen" -> Einladungen(api)
            "bibliotheken" -> Bibliotheken(api)
            "metadaten" -> MetadatenManager(api)
            "umwandlung" -> Umwandlung(api)
            "geraete" -> Geraete(api)
            "livetv" -> LiveTVAdmin(api)
            "netzwerk" -> Netzwerk(api)
            "aufgaben" -> Aufgaben(api)
            "sicherung" -> Sicherung(api)
        }
    }
}

/** Kopfzeile: Zurück (Handy), Titel, „Bibliothek scannen“. */
@Composable
private fun Kopf(titel: String, zurueck: (() -> Unit)?, scannen: (() -> Unit)?) {
    val tv = LocalTv.current
    val breit = LocalBreit.current
    Column {
        Row(Modifier.fillMaxWidth().height(if (tv) 72.dp + Tokens.Abstand.TvRandOben else if (breit) 64.dp else 56.dp)
            .padding(start = if (tv) Pad else if (breit) 24.dp else 4.dp, end = if (tv) Pad else 12.dp, top = if (tv) Tokens.Abstand.TvRandOben else 0.dp),
            verticalAlignment = Alignment.CenterVertically) {
            if (zurueck != null) IconKnopf(Ic.ZURUECK, zurueck)
            T(titel, if (tv) LocalTypo.current.titel else if (breit) 20.sp else 18.sp, K.Text, FontWeight.SemiBold, maxLines = 1,
                modifier = Modifier.weight(1f).padding(start = 4.dp))
            if (scannen != null) {
                if (tv || breit) Knopf("Bibliothek scannen", scannen, icon = Ic.NEUSTART)
                else IconKnopf(Ic.NEUSTART, scannen)
            }
        }
        if (!tv) Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
    }
}

/** Menü des Dashboards; TV: Fokus startet auf dem aktiven Eintrag. */
@Composable
private fun BereichListe(aktiv: String, onBereich: (String) -> Unit, onZurueck: (() -> Unit)?, modifier: Modifier) {
    val tv = LocalTv.current
    val erster = remember { FocusRequester() }
    if (tv) LaunchedEffect(Unit) { runCatching { erster.requestFocus() } }
    Column(modifier.verticalScroll(rememberScrollState()).padding(horizontal = if (tv) 16.dp else 12.dp, vertical = if (tv) Tokens.Abstand.TvRandOben else 12.dp)) {
        if (onZurueck != null) {
            T("DASHBOARD", if (tv) 22.sp else 12.sp, K.Text3, family = Mono, spacing = 2.sp, modifier = Modifier.padding(start = 12.dp, bottom = 4.dp))
            Eintrag(Ic.ZURUECK, "Zurück zu Flimmer", false, onZurueck, leise = true)
        }
        BEREICHE.forEach { b ->
            if (b.kopf != null) T(b.kopf, if (tv) 22.sp else 12.sp, K.Text3, family = Mono, spacing = 2.sp, modifier = Modifier.padding(start = 12.dp, top = 20.dp, bottom = 6.dp))
            Eintrag(b.icon, b.label, b.id == aktiv, { onBereich(b.id) }, Modifier.then(if (b.id == aktiv || (aktiv.isEmpty() && b == BEREICHE[0])) Modifier.focusRequester(erster) else Modifier))
        }
    }
}

@Composable
private fun Eintrag(icon: String, text: String, an: Boolean, onClick: () -> Unit, modifier: Modifier = Modifier, leise: Boolean = false) {
    val tv = LocalTv.current
    val farbe = if (an) K.Text else K.Text2
    Row(modifier.fillMaxWidth().height(if (tv) 72.dp else if (LocalBreit.current) 44.dp else 52.dp).klick(onClick, skala = false)
        .background(if (an) K.Flaeche3 else Color.Transparent, RoundedCornerShape(Tokens.Radius.RadiusS)).padding(horizontal = 12.dp),
        verticalAlignment = Alignment.CenterVertically) {
        Ico(icon, if (tv) 32.dp else if (leise) 20.dp else 24.dp, farbe)
        T(text, if (tv) LocalTypo.current.text else if (leise) 14.sp else 15.sp, farbe, if (an) FontWeight.SemiBold else FontWeight.Medium, maxLines = 1,
            modifier = Modifier.padding(start = 14.dp).weight(1f))
        if (!tv && !LocalBreit.current && !leise) Ico(Ic.WEITER, 20.dp, K.Text3)
    }
}

// ---------- Übersicht ----------

internal fun aktivitaetIcon(kind: String) = when (kind) {
    "login" -> Ic.PROFIL
    "play" -> Ic.ABSPIELEN
    "scan" -> Ic.NEUSTART
    "error" -> AI.FEHLER
    "invite" -> AI.EINLADUNG
    "backup" -> AI.SICHERUNG
    "task" -> AI.AUFGABEN
    "user" -> Ic.GEMEINSAM
    else -> Ic.INFO
}

/** Server, Kennzahlen, aktive Geräte, laufende Aufgaben, letzte Aktivitäten. Lädt alle 10 s neu, solange sichtbar. */
@Composable
internal fun Uebersicht(api: ApiClient, onBereich: (String) -> Unit) {
    val z = laden(api) { api.uebersicht() }
    val a = rememberAktion()
    Nachladen(10_000) { z.neu() }
    Seite {
        Geladen(z) { o ->
            ServerKarte(o.server) { a.oeffnen(it) }
            Spacer(Modifier.height(Luecke))
            Kacheln {
                Messwert("Filme", "${o.library.movies}", Modifier.weight(1f))
                Messwert("Serien", "${o.library.series}", Modifier.weight(1f))
                Messwert("Folgen", "${o.library.episodes}", Modifier.weight(1f), unter = o.library.lastScan?.let { "Zuletzt gescannt ${vor(it)}" })
                o.disks.forEach { d ->
                    Messwert("Speicher", groesse(d.total - d.free), Modifier.weight(1f), unter = "${groesse(d.free)} frei von ${groesse(d.total)}",
                        anteil = if (d.total > 0) 1f - d.free.toFloat() / d.total else 0f)
                }
            }
            Gruppe("Aktive Geräte", zusatz = { Knopf("Alle anzeigen", { onBereich("geraete") }) }) {
                if (o.sessions.isEmpty()) Leise("Gerade schaut niemand.")
                Kacheln(if (LocalTv.current) 3 else if (LocalBreit.current) 2 else 1) {
                    o.sessions.forEach { SitzungKarte(it, Modifier.weight(1f)) }
                }
            }
            LaufendeAufgaben(api)
            Gruppe("Letzte Aktivitäten", zusatz = { Knopf("Alle anzeigen", { onBereich("geraete") }) }) {
                if (o.activity.isEmpty()) Leise("Noch nichts passiert.")
                o.activity.forEach { x ->
                    Zeile(x.text, icon = aktivitaetIcon(x.kind), farbe = if (x.kind == "error") K.AmpelRot else K.Text, wert = vor(x.time))
                }
            }
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun ServerKarte(s: ServerInfo, oeffnen: (String) -> Unit) {
    val tv = LocalTv.current
    Column(Modifier.padding(top = 16.dp).fillMaxWidth().background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusM)).padding(if (tv) 32.dp else 20.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(if (tv) 80.dp else 56.dp).background(K.Flaeche2, RoundedCornerShape(Tokens.Radius.RadiusS)), contentAlignment = Alignment.Center) {
                Ico(Ic.SERVER, if (tv) 40.dp else 28.dp)
            }
            Column(Modifier.padding(start = 16.dp)) {
                T(s.name.uppercase(), if (tv) 44.sp else 30.sp, K.Text, family = Plakat, maxLines = 1)
                Row(verticalAlignment = Alignment.CenterVertically) {
                    AmpelPunkt("green", 8.dp)
                    Leise("Verbunden", Modifier.padding(start = 8.dp))
                }
            }
        }
        FlowRow(Modifier.padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(32.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            listOf("Version" to s.version, "Betriebssystem" to "${s.os} · ${s.arch}", "Läuft seit" to seit(s.uptime)).forEach { (k, v) ->
                Column { Label(k); T(v, if (tv) LocalTypo.current.text else 15.sp, K.Text, FontWeight.Medium, maxLines = 1) }
            }
        }
        s.update?.let { u ->
            Row(Modifier.padding(top = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                Leise("Update verfügbar: ${u.version}", Modifier.weight(1f, fill = false).padding(end = 12.dp))
                Knopf("Aktualisieren", { oeffnen(u.url) }, icon = Ic.DOWNLOAD)
            }
        }
    }
}

@Composable
private fun SitzungKarte(s: Session, modifier: Modifier) {
    val tv = LocalTv.current
    val typo = LocalTypo.current
    Column(modifier.background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusM)).padding(if (tv) 24.dp else 16.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Avatar(s.user, s.userColor, if (tv) 48.dp else 32.dp)
            Column(Modifier.padding(start = 12.dp)) {
                T(s.user, typo.klein, K.Text, FontWeight.SemiBold, maxLines = 1)
                T(s.device + if (s.client.isNotEmpty()) " · ${s.client}" else "", typo.klein, K.Text2, maxLines = 1)
            }
        }
        T(s.title, if (tv) typo.text else 16.sp, K.Text, FontWeight.SemiBold, maxLines = 1, modifier = Modifier.padding(top = 12.dp))
        Row(Modifier.padding(top = 4.dp), verticalAlignment = Alignment.CenterVertically) {
            AmpelPunkt(s.light)
            T(methode(s.method) + (s.reason?.let { " · $it" } ?: ""), typo.klein, K.Text2, maxLines = 1, modifier = Modifier.padding(start = 8.dp))
        }
        Balken(if (s.duration > 0) (s.position / s.duration).toFloat() else 0f, Modifier.padding(top = 12.dp))
        T(if (s.paused) "pausiert" else "noch ${dauer(maxOf(0.0, s.duration - s.position))}", typo.klein, K.Text3, modifier = Modifier.padding(top = 6.dp))
    }
}

/** Laufende Aufgaben aus /api/tasks; fehlt der Endpunkt, bleibt der Abschnitt weg. */
@Composable
private fun LaufendeAufgaben(api: ApiClient) {
    val z = laden(api) { api.aufgaben() }
    val laufend = z.daten.orEmpty().filter { it.running }
    Nachladen(3000, laufend.isNotEmpty()) { z.neu() }
    if (laufend.isEmpty()) return
    Gruppe("Laufende Aufgaben") {
        laufend.forEach { t ->
            val p = t.progress ?: 0.0
            Zeile(t.name, unter = t.description, wert = "${Math.round(p * 100)} %", unten = { Balken(p.toFloat(), Modifier.padding(top = 8.dp)) })
        }
    }
}
