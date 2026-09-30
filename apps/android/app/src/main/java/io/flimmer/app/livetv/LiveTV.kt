package io.flimmer.app.livetv

import android.widget.Toast
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import coil3.compose.AsyncImage
import io.flimmer.app.ApiClient
import io.flimmer.app.ApiException
import io.flimmer.app.ui.*
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

// Live-TV wie web/src/screens/LiveTV.tsx: Tabs Programme, Programmführer, Kanäle. Kein DVR, also keine Aufnahme-Knöpfe.

private val TABS = listOf("Programme", "Programmführer", "Kanäle")

@Composable
fun LiveTVScreen(api: ApiClient, tv: Boolean, onKanal: (url: String, titel: String) -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var tab by rememberSaveable { mutableIntStateOf(0) }
    var liste by remember { mutableStateOf<List<Kanal>?>(null) }
    var fehler by remember { mutableStateOf<Throwable?>(null) }
    var admin by remember { mutableStateOf(false) }
    var neu by remember { mutableIntStateOf(0) }
    LaunchedEffect(neu) {
        fehler = null
        runCatching { kanaele(api.roh("GET", "/api/livetv/channels")) }.onSuccess { liste = it }.onFailure { fehler = it }
        if (liste.isNullOrEmpty()) admin = runCatching { api.me().admin }.getOrDefault(false)
    }
    val einschalten: (Kanal) -> Unit = { k ->
        scope.launch {
            runCatching { json.decodeFromString<Einschalten>(api.roh("POST", "/api/livetv/channels/${enc(k.id)}/play", "{}")) }
                .onSuccess { onKanal(api.abs(it.url), it.title.ifEmpty { k.name }) }
                .onFailure {
                    val text = when ((it as? ApiException)?.code) {
                        503 -> "Alle Plätze belegt – höchstens drei Sender laufen gleichzeitig."
                        403 -> "Live-TV ist für dieses Konto nicht freigegeben"
                        else -> "Der Sender lässt sich gerade nicht öffnen."
                    }
                    Toast.makeText(ctx, text, Toast.LENGTH_LONG).show()
                }
        }
    }

    Column(Modifier.fillMaxSize().background(K.Saal)) {
        Tabs(TABS, tab, tv) { tab = it }
        val f = fehler
        val l = liste
        when {
            (f as? ApiException)?.code == 403 -> Hinweis("Live-TV ist für dieses Konto nicht freigegeben", "Gastkonten sehen kein Live-TV.")
            (f as? ApiException)?.code == 404 || (f == null && l?.isEmpty() == true) -> Hinweis(
                "Noch keine Sender eingerichtet",
                if (admin) "Trage im Dashboard eine M3U-Liste oder einen HDHomeRun ein, dazu ein XMLTV-Programm."
                else "Bitte den Admin, im Dashboard Sender einzurichten.",
            )
            f != null -> ServerFehler(f) { neu++ }
            l == null -> Laden()
            tab == 1 -> Programmfuehrer(api, l, tv, einschalten)
            tab == 2 -> KanalListe(api, l, tv, einschalten)
            else -> Programme(api, l, tv, einschalten)
        }
    }
}

// ---------- Programme: was jetzt läuft und was danach kommt ----------

@Composable
private fun Programme(api: ApiClient, liste: List<Kanal>, tv: Boolean, einschalten: (Kanal) -> Unit) {
    val jetzt = System.currentTimeMillis()
    val laufen = liste.filter { it.now != null }
    val danach = liste.filter { it.next != null }
    val breite = if (tv) Tokens.Masse.KarteBreitTv else if (LocalBreit.current) Tokens.Masse.KarteBreitDt else Tokens.Masse.KarteBreitHd
    @Composable
    fun karte(k: Kanal, s: Sendung?, jetztLaeuft: Boolean) = BreitKarte(
        titel = s?.title ?: k.name,
        unter = listOfNotNull(k.number, k.name, s?.let { if (jetztLaeuft) "bis ${hm(it.bis)}" else "ab ${hm(it.von)}" })
            .filter { it.isNotEmpty() }.joinToString(" · "),
        bild = k.logo?.let(api::abs).orEmpty(), farbe = "", onClick = { einschalten(k) }, modifier = Modifier.width(breite),
        plakat = k.name, fortschritt = if (s != null && jetztLaeuft) s.anteil(jetzt) else 0f,
    )
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(bottom = 32.dp)) {
        if (laufen.isNotEmpty()) Reihe("Läuft jetzt") { items(laufen, key = { it.id }) { karte(it, it.now, true) } }
        if (danach.isNotEmpty()) Reihe("Gleich danach") { items(danach, key = { it.id }) { karte(it, it.next, false) } }
        if (laufen.isEmpty()) Reihe("Kanäle") { items(liste, key = { it.id }) { karte(it, null, false) } }
    }
}

// ---------- Kanäle: Liste mit Nummer, Logo, Name und laufender Sendung ----------

@Composable
private fun KanalListe(api: ApiClient, liste: List<Kanal>, tv: Boolean, einschalten: (Kanal) -> Unit) {
    val jetzt = System.currentTimeMillis()
    val handy = !tv && !LocalBreit.current
    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(start = Pad, end = Pad, top = 8.dp, bottom = 48.dp)) {
        items(liste, key = { it.id }) { k ->
            Row(
                Modifier.fillMaxWidth().heightIn(min = if (tv) 112.dp else 72.dp).linieUnten().klick({ einschalten(k) }, skala = false)
                    .padding(horizontal = if (tv) 24.dp else 12.dp, vertical = if (tv) 12.dp else 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                T(k.number.orEmpty(), if (tv) 22.sp else 13.sp, K.Text3, family = Mono, maxLines = 1,
                    modifier = Modifier.width(if (tv) 56.dp else 32.dp).padding(end = 4.dp))
                Box(Modifier.padding(start = 8.dp, end = 16.dp).size(if (tv) 128.dp else 72.dp, if (tv) 72.dp else 44.dp)
                    .clip(RoundedCornerShape(Tokens.Radius.RadiusS)).background(K.Flaeche2), contentAlignment = Alignment.Center) {
                    if (k.logo.isNullOrEmpty()) T(k.name.take(3).uppercase(), if (tv) 22.sp else 13.sp, K.Text2, family = Mono, maxLines = 1)
                    else AsyncImage(api.abs(k.logo), null, Modifier.fillMaxSize(), contentScale = ContentScale.Fit)
                }
                Column(Modifier.weight(1f)) {
                    T(k.name, if (tv) 28.sp else 16.sp, K.Text, FontWeight.SemiBold, maxLines = 1)
                    val zeile = (k.now?.let { "${it.title} · bis ${hm(it.bis)}" } ?: "Kein Programm") +
                        (k.next?.let { " · Gleich danach: ${it.title}" } ?: "")
                    T(zeile, if (tv) 24.sp else 14.sp, K.Text2, maxLines = 1)
                    k.now?.let { Fortschritt(it.anteil(jetzt), Modifier.widthIn(max = 320.dp).padding(top = 6.dp), 3.dp, K.Text, K.Linie) }
                }
                if (!handy && !k.group.isNullOrEmpty()) Label(k.group, Modifier.padding(start = 16.dp))
            }
        }
    }
}

// ---------- Programmführer: Kanäle links, Zeitraster rechts, Linie für „jetzt“ ----------

@Composable
private fun Programmfuehrer(api: ApiClient, liste: List<Kanal>, tv: Boolean, einschalten: (Kanal) -> Unit) {
    var fuehrer by remember { mutableStateOf<Fuehrer?>(null) }
    var fehler by remember { mutableStateOf<Throwable?>(null) }
    var neu by remember { mutableIntStateOf(0) }
    var gruppe by rememberSaveable { mutableStateOf("") }
    var wahl by remember { mutableStateOf<Pair<Kanal, Sendung>?>(null) }
    var jetzt by remember { mutableLongStateOf(System.currentTimeMillis()) }
    LaunchedEffect(neu) {
        fehler = null
        runCatching { fuehrer(api.roh("GET", "/api/livetv/guide?hours=12")) }.onSuccess { fuehrer = it }.onFailure { fehler = it }
    }
    LaunchedEffect(Unit) { while (true) { delay(60_000); jetzt = System.currentTimeMillis() } }
    val g = fuehrer
    fehler?.let { return ServerFehler(it) { neu++ } }
    if (g == null) return Laden()
    if (g.channels.isNullOrEmpty()) return Hinweis("Kein Programm geladen", "Die Sender laufen, aber es gibt keine XMLTV-Daten für die nächsten Stunden.", Ic.UHR)

    val handy = !tv && !LocalBreit.current
    val r = remember(g) { Raster(g.from, g.to, if (tv) 8 else 5) }
    val sendungen = remember(g) { g.channels.associate { it.id to it.programs.orEmpty() } }
    val gruppen = liste.mapNotNull { it.group?.takeIf(String::isNotEmpty) }.distinct()
    val zeilen = liste.filter { gruppe.isEmpty() || it.group == gruppe }
    val hoehe = if (tv) 96.dp else 64.dp
    val spalte = if (tv) 300.dp else if (handy) 96.dp else 220.dp
    val scroll = rememberScrollState() // ein Zustand für alle Zeilen: Kopf und Kanäle fahren gemeinsam waagerecht

    Column(Modifier.fillMaxSize()) {
        Row(Modifier.fillMaxWidth().padding(start = Pad, end = Pad, top = 8.dp, bottom = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            Row(Modifier.weight(1f).horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Chip("Alle", gruppe.isEmpty(), { gruppe = "" })
                gruppen.forEach { x -> Chip(x, gruppe == x, { gruppe = x }) }
            }
            if (!handy) Label("${zeilen.size} Kanäle · ${hm(r.von)}–${hm(r.bis)}", Modifier.padding(start = 16.dp))
        }
        LazyColumn(
            Modifier.padding(horizontal = Pad).padding(bottom = 16.dp).clip(RoundedCornerShape(Tokens.Radius.RadiusM))
                .border(1.dp, K.Linie, RoundedCornerShape(Tokens.Radius.RadiusM)),
        ) {
            item(key = "kopf") {
                Row(Modifier.height(40.dp).linieUnten()) {
                    Box(Modifier.width(spalte).fillMaxHeight().linieRechts().padding(horizontal = 16.dp), contentAlignment = Alignment.CenterStart) { Label("Kanal") }
                    Box(Modifier.weight(1f).horizontalScroll(scroll)) {
                        Box(Modifier.size(r.breite.dp, 40.dp)) {
                            r.marken.forEach { x ->
                                Box(Modifier.offset(x = r.px(x).dp).width(1.dp).fillMaxHeight().background(K.Linie))
                                T(hm(x), if (tv) 20.sp else 12.sp, K.Text3, family = Mono, maxLines = 1, modifier = Modifier.offset(x = (r.px(x) + 8).dp, y = 12.dp))
                            }
                            if (jetzt in r.von until r.bis) {
                                Box(Modifier.offset(x = r.px(jetzt).dp - 1.dp).width(2.dp).fillMaxHeight().background(K.Marke))
                                T(hm(jetzt), if (tv) 20.sp else 12.sp, K.AufMarke, family = Mono, maxLines = 1,
                                    modifier = Modifier.offset(x = r.px(jetzt).dp, y = 8.dp).background(K.Marke).padding(horizontal = 8.dp))
                            }
                        }
                    }
                }
            }
            items(zeilen, key = { it.id }) { k ->
                Row(Modifier.height(hoehe).linieUnten()) {
                    Row(Modifier.width(spalte).fillMaxHeight().linieRechts().padding(start = if (handy) 8.dp else 16.dp, end = 8.dp),
                        verticalAlignment = Alignment.CenterVertically) {
                        if (!handy && !k.number.isNullOrEmpty()) T(k.number, if (tv) 22.sp else 13.sp, K.Text3, family = Mono, maxLines = 1, modifier = Modifier.width(if (tv) 56.dp else 32.dp))
                        T(k.name, if (tv) 26.sp else if (handy) 13.sp else 15.sp, K.Text, FontWeight.Medium, maxLines = 1)
                    }
                    Box(Modifier.weight(1f).fillMaxHeight().horizontalScroll(scroll)) {
                        Box(Modifier.size(r.breite.dp, hoehe)) {
                            r.marken.forEach { x -> Box(Modifier.offset(x = r.px(x).dp).width(1.dp).fillMaxHeight().background(K.Linie)) }
                            sendungen[k.id].orEmpty().filter { it.bis > r.von && it.von < r.bis }.forEach { s ->
                                val (links, breite) = r.block(s)
                                Block(s, links, breite, s.laeuft(jetzt), tv) { wahl = k to s }
                            }
                            if (jetzt in r.von until r.bis) Box(Modifier.offset(x = r.px(jetzt).dp - 1.dp).width(2.dp).fillMaxHeight().background(K.Marke))
                        }
                    }
                }
            }
        }
    }
    wahl?.let { (k, s) -> SendungDialog(k, s, tv, { wahl = null }) { wahl = null; einschalten(k) } }
}

@Composable
private fun Block(s: Sendung, links: Int, breite: Int, laeuft: Boolean, tv: Boolean, onClick: () -> Unit) {
    Column(
        Modifier.offset(x = links.dp, y = 6.dp).size(breite.dp, if (tv) 84.dp else 52.dp).klick(onClick, skala = false)
            .background(if (laeuft) K.Flaeche2 else K.Flaeche1).border(1.dp, K.Linie, RoundedCornerShape(Tokens.Radius.RadiusS))
            .padding(horizontal = if (tv) 16.dp else 10.dp),
        verticalArrangement = Arrangement.Center,
    ) {
        T(s.title, if (tv) 24.sp else 14.sp, K.Text, FontWeight.Medium, maxLines = 1)
        T("${hm(s.von)}–${hm(s.bis)}", if (tv) 20.sp else 12.sp, K.Text2, family = Mono, maxLines = 1)
    }
}

@Composable
private fun SendungDialog(k: Kanal, s: Sendung, tv: Boolean, onZu: () -> Unit, onEinschalten: () -> Unit) {
    val jetzt = System.currentTimeMillis()
    val laeuft = s.laeuft(jetzt)
    val typo = LocalTypo.current
    val erster = remember { FocusRequester() }
    Dialog(onZu) {
        Column(Modifier.width(if (tv) 760.dp else 360.dp).background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusM))
            .border(1.dp, K.Linie, RoundedCornerShape(Tokens.Radius.RadiusM)).padding(if (tv) 32.dp else 20.dp)) {
            T(s.title, typo.reihe, K.Text, FontWeight.SemiBold, maxLines = 3)
            T("${hm(s.von)}–${hm(s.bis)} · " + listOfNotNull(k.number, k.name).filter { it.isNotEmpty() }.joinToString(" "), typo.klein, K.Text2,
                modifier = Modifier.padding(top = 4.dp))
            s.desc?.takeIf { it.isNotBlank() }?.let { T(it, typo.text, K.Text, maxLines = 10, modifier = Modifier.padding(top = 12.dp)) }
            if (laeuft) Row(Modifier.padding(top = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                Fortschritt(s.anteil(jetzt), Modifier.weight(1f).padding(end = 12.dp), 4.dp, K.Text, K.Linie)
                Label("noch " + fmtDauer((s.bis - jetzt) / 1000.0))
            }
            Row(Modifier.padding(top = 20.dp).align(Alignment.End), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Knopf("Schließen", onZu)
                if (laeuft) Knopf("Einschalten", onEinschalten, Modifier.focusRequester(erster), primary = true, icon = Ic.ABSPIELEN)
            }
        }
    }
    if (laeuft) LaunchedEffect(Unit) { runCatching { erster.requestFocus() } }
}

// ---------- Kleinteile ----------

/** Tabs mit 2-dp-Unterstrich wie in der Kopfzeile (Rahmen.kt), hier im Inhalt, weil der Einstieg keine Tabs mitgibt. */
@Composable
private fun Tabs(labels: List<String>, aktiv: Int, tv: Boolean, onTab: (Int) -> Unit) {
    Row(Modifier.fillMaxWidth().height(if (tv) 72.dp else 48.dp).linieUnten().horizontalScroll(rememberScrollState()).padding(horizontal = Pad - 12.dp)) {
        labels.forEachIndexed { i, t ->
            Box(Modifier.fillMaxHeight().klick({ onTab(i) }, skala = false).padding(horizontal = 12.dp), contentAlignment = Alignment.Center) {
                T(t, if (tv) LocalTypo.current.text else 15.sp, if (i == aktiv) K.Text else K.Text2,
                    if (i == aktiv) FontWeight.SemiBold else FontWeight.Medium, maxLines = 1)
                if (i == aktiv) Box(Modifier.align(Alignment.BottomCenter).fillMaxWidth().height(if (tv) 3.dp else 2.dp).background(K.Text))
            }
        }
    }
}

/** Ruhiger Hinweis (leer, gesperrt, nicht eingerichtet). */
@Composable
private fun Hinweis(titel: String, text: String, icon: String = Ic.LIVE, knopf: (@Composable () -> Unit)? = null) {
    val typo = LocalTypo.current
    Column(Modifier.fillMaxWidth().padding(horizontal = Pad, vertical = 48.dp).widthIn(max = 640.dp)) {
        Ico(icon, if (LocalTv.current) 56.dp else 40.dp, K.Text3)
        T(titel, typo.reihe, K.Text, FontWeight.SemiBold, modifier = Modifier.padding(top = 16.dp))
        T(text, typo.klein, K.Text2, modifier = Modifier.padding(top = 4.dp))
        knopf?.let { Box(Modifier.padding(top = 20.dp)) { it() } }
    }
}

@Composable
private fun ServerFehler(e: Throwable, nochmal: () -> Unit) =
    Hinweis("Der Server antwortet nicht", "Läuft der Rechner mit Flimmer, und ist dieses Gerät im selben Netz? (${e.message ?: e.javaClass.simpleName})", Ic.INFO) {
        Knopf("Nochmal versuchen", nochmal, primary = true, icon = Ic.NEUSTART)
    }

@Composable
private fun Laden() = T("Lade …", LocalTypo.current.text, K.Text2, modifier = Modifier.padding(Pad))

private fun Modifier.linieUnten() = drawBehind { drawRect(K.Linie, Offset(0f, size.height - 1.dp.toPx()), androidx.compose.ui.geometry.Size(size.width, 1.dp.toPx())) }
private fun Modifier.linieRechts() = drawBehind { drawRect(K.Linie, Offset(size.width - 1.dp.toPx(), 0f), androidx.compose.ui.geometry.Size(1.dp.toPx(), size.height)) }

private fun enc(s: String) = java.net.URLEncoder.encode(s, "UTF-8").replace("+", "%20")
