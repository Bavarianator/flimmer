package io.flimmer.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.focusable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyGridScope
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.FilterQuality
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil3.compose.AsyncImage
import io.flimmer.app.Downloads
import io.flimmer.app.HomeRow
import io.flimmer.app.Item
import io.flimmer.app.Liste
import io.flimmer.app.User
import io.flimmer.app.norm
import io.flimmer.app.search
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

// ---------- Daten und Handlungen, die alle Seiten brauchen ----------

/** Was die Seiten anzeigen. [sammlungen]/[listen] sind null, solange der Server die Endpunkte nicht hat. */
class Daten(
    val library: List<Item>,
    val rows: List<HomeRow>?,
    val favs: Set<String>,
    val sammlungen: List<Liste>?,
    val listen: List<Liste>?,
    val abs: (String) -> String,
    val admin: Boolean,
    val versteckt: Set<String> = emptySet(), // Startseiten-Reihen, die unter Einstellungen › Startseite aus sind
) {
    val filme = library.filter { it.series == null }
    /** Folgen je Serie, sortiert nach Staffel und Folge. */
    val serien: Map<String, List<Item>> = library.filter { it.series != null }.groupBy { it.series!! }
        .mapValues { (_, e) -> e.sortedWith(compareBy({ it.season ?: 0 }, { it.episode ?: 0 })) }
    fun row(id: String) = rows?.firstOrNull { it.id == id }?.items.orEmpty()
    fun resolve(key: String): Item? = key.removePrefix("serie:").let { n -> if (key.startsWith("serie:")) serien[n]?.firstOrNull() else library.firstOrNull { it.id == key } }
}

/** Wohin Klicks führen; MainActivity füllt das. */
class Handlungen(
    val oeffnen: (Item) -> Unit, // Film → Detail, Folge → Serie
    val abspielen: (Item, Double?) -> Unit,
    val alle: (List<Item>) -> Unit, // Liste der Reihe nach abspielen
    val favorit: (String) -> Unit,
    val mehr: (Item) -> Unit, // Aktionen-Blatt
    val gesehen: (List<Item>, Boolean) -> Unit,
    val person: ((String) -> Unit)?,
    val liste: (Liste, Boolean) -> Unit, // true = Sammlung
    val bereich: (Bereich) -> Unit,
    val zurueck: () -> Unit,
    val cast: (() -> Unit)?,
    val party: (Item) -> Unit,
    val download: ((List<Item>) -> Unit)?, // null auf dem TV
    val dlStatus: (String) -> Downloads.Status?,
    val offenerLink: (String) -> Unit,
    val pflege: ListenPflege,
    val metadaten: ((Item) -> Unit)? = null, // Admin: Metadaten-Editor (Paket admin), null solange es ihn nicht gibt
)

/** Sammlungen und Wiedergabelisten verwalten; MainActivity spricht den Server an und lädt die Listen neu. */
class ListenPflege(
    val neu: (sammlung: Boolean, name: String, keys: List<String>) -> Unit,
    val hinzu: (Liste, Boolean, List<String>) -> Unit,
    val umbenennen: (Liste, Boolean, String) -> Unit,
    val loeschen: (Liste, Boolean) -> Unit,
    val entfernen: (Liste, Boolean, String) -> Unit,
    val ordnen: (Liste, Boolean, List<String>) -> Unit,
)

// ---------- Kleine Helfer ----------

/** Mono-Zeile auf Kacheln: „1927 · 2:28“ bzw. „2021– · 3 ST.“ */
fun monoFilm(it: Item) = listOfNotNull((it.meta?.year ?: it.year)?.toString(), if (it.duration > 0) fmtKurz(it.duration) else null).joinToString(" · ")

fun monoSerie(eps: List<Item>): String {
    val jahre = eps.mapNotNull { it.meta?.year ?: it.year }
    val staffeln = eps.mapNotNull { it.season }.distinct().size
    val von = jahre.minOrNull()?.toString() ?: ""
    return listOf(von, if (staffeln > 1) "$staffeln ST." else if (staffeln == 1) "1 ST." else "").filter { it.isNotEmpty() }.joinToString(" · ")
}

/** „Serie · 2021 · 3 Staffeln“ */
fun serienZeile(eps: List<Item>): String {
    val n = eps.mapNotNull { it.season }.filter { it > 0 }.distinct().size
    return listOfNotNull("Serie", eps.mapNotNull { it.meta?.year ?: it.year }.minOrNull()?.toString(), if (n == 1) "1 Staffel" else if (n > 1) "$n Staffeln" else null).joinToString(" · ")
}

fun jahr(it: Item) = (it.meta?.year ?: it.year)?.toString() ?: ""

fun folge(it: Item) = "S${it.season ?: 0} E${it.episode ?: 0}"

fun sub(it: Item): String = when {
    it.series != null -> "${folge(it)} · ${it.displayTitle}"
    it.progress > 0 && it.duration > 0 && !it.watched -> listOfNotNull(jahr(it).ifEmpty { null }, "Noch ${fmtDauer(it.duration - it.progress)}").joinToString(" · ")
    else -> listOfNotNull(jahr(it).ifEmpty { null }, if (it.duration > 0) fmtDauer(it.duration) else null).joinToString(" · ")
}

/** Nächste Folge einer Serie: nach der zuletzt gesehenen, sonst die angefangene, sonst die erste. */
fun naechsteFolge(eps: List<Item>): Item? {
    val last = eps.indexOfLast { it.progress > 0 || it.watched }
    return when {
        last < 0 -> eps.firstOrNull()
        eps[last].watched && last + 1 < eps.size -> eps[last + 1]
        eps[last].watched -> eps.firstOrNull { !it.watched } ?: eps[last]
        else -> eps[last]
    }
}

/**
 * Kachel für einen Titel: Filme als Poster, Serien (auch über eine Folge) als Serienposter mit ungesehenen Folgen.
 * [markiert] != null: Mehrfachauswahl, ein Tipp wählt ([onMarkieren]) statt zu öffnen.
 */
@Composable
fun Kachel(it: Item, d: Daten, h: Handlungen, modifier: Modifier = Modifier, ampelLinks: Boolean = false, markiert: Boolean? = null, onMarkieren: () -> Unit = {}) {
    val s = it.series
    val klick: () -> Unit = if (markiert != null) onMarkieren else ({ h.oeffnen(it) })
    val lang: (() -> Unit)? = if (markiert != null) null else ({ h.mehr(it) })
    Box(modifier.alpha(if (markiert == false) 0.6f else 1f)) {
        if (s != null) {
            val eps = d.serien[s].orEmpty()
            val neu = eps.count { e -> !e.watched }
            PosterKarte(s, jahr(eps.firstOrNull() ?: it), monoSerie(eps), d.abs((eps.firstOrNull() ?: it).poster), it.color, klick,
                gesehen = eps.isNotEmpty() && neu == 0, neu = neu, onLang = lang)
        } else {
            PosterKarte(it.displayTitle, jahr(it), monoFilm(it), d.abs(it.poster), it.color, klick,
                ampel = if (ampelLinks) it.light else null, gesehen = it.watched, fortschritt = it.anteil(), onLang = lang)
        }
        if (markiert == true) Box(Modifier.align(Alignment.TopEnd).padding(6.dp).size(28.dp).background(K.Marke, RoundedCornerShape(Tokens.Radius.RadiusS)),
            contentAlignment = Alignment.Center) { Ico(Ic.HAKEN, 18.dp, K.AufMarke, strich = 2f) }
    }
}

val posterBreite: Dp @Composable get() = if (LocalTv.current) Tokens.Masse.KartePosterTv else if (LocalBreit.current) 176.dp else Tokens.Masse.KartePosterHd

/** Reihe mit Überschrift und waagerechter Liste. */
@Composable
fun Reihe(titel: String, onMehr: (() -> Unit)? = null, luecke: Dp = Luecke, inhalt: LazyListScope.() -> Unit) {
    Column(Modifier.padding(top = if (LocalTv.current) 28.dp else 24.dp)) {
        Abschnitt(titel, onMehr, Modifier.padding(start = Pad, end = Pad, bottom = 12.dp))
        LazyRow(contentPadding = PaddingValues(horizontal = Pad), horizontalArrangement = Arrangement.spacedBy(luecke)) { inhalt() }
    }
}

@Composable
private fun Leer(text: String) = T(text, LocalTypo.current.text, K.Text2, modifier = Modifier.padding(Pad))

@Composable
private fun Page(content: @Composable () -> Unit) {
    Box(Modifier.fillMaxSize().background(K.Saal)) { content() }
}

@Composable
fun Feld(value: String, onChange: (String) -> Unit, placeholder: String, modifier: Modifier = Modifier,
         password: Boolean = false, type: KeyboardType = KeyboardType.Text, onGo: () -> Unit = {}) {
    OutlinedTextField(
        value, onChange, modifier, singleLine = true,
        placeholder = { T(placeholder, color = K.Text3) },
        textStyle = TextStyle(fontFamily = Sans, fontSize = LocalTypo.current.text, color = K.Text),
        visualTransformation = if (password) PasswordVisualTransformation() else androidx.compose.ui.text.input.VisualTransformation.None,
        keyboardOptions = KeyboardOptions(keyboardType = if (password) KeyboardType.Password else type, imeAction = ImeAction.Go),
        keyboardActions = KeyboardActions(onGo = { onGo() }),
        shape = RoundedCornerShape(Tokens.Radius.RadiusS),
        colors = OutlinedTextFieldDefaults.colors(
            focusedBorderColor = K.Text, unfocusedBorderColor = K.LinieStark, cursorColor = K.Text,
            focusedContainerColor = K.Flaeche2, unfocusedContainerColor = K.Flaeche2,
        ),
    )
}

@Composable
private fun Fehler(text: String) = T(text, LocalTypo.current.klein, K.AmpelRot, modifier = Modifier.padding(top = 12.dp))

// ---------- Anmeldung ----------

/** Erststart: Server im Heimnetz finden (SSDP), Adresse bzw. Einladungslink eingeben oder QR-Code scannen ([onScan], nicht auf dem TV). */
@Composable
fun ConnectScreen(found: List<String>?, error: String, onScan: (() -> Unit)?, onConnect: (String) -> Unit) {
    var input by remember { mutableStateOf("") }
    val first = remember { FocusRequester() }
    Page {
        Column(Modifier.fillMaxSize().systemBarsPadding().padding(24.dp).verticalScroll(rememberScrollState()), horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center) {
            Wortmarke(if (LocalTv.current) 72.dp else 36.dp)
            Spacer(Modifier.height(32.dp))
            when {
                found == null -> Label("Suche Server im Heimnetz …")
                found.isEmpty() -> Label("Kein Server gefunden")
                else -> {
                    Label("Gefundene Server")
                    found.forEachIndexed { i, url ->
                        Knopf(url.removePrefix("http://"), { onConnect(url) },
                            Modifier.padding(top = 12.dp).then(if (i == 0) Modifier.focusRequester(first) else Modifier), primary = true, icon = Ic.SERVER)
                    }
                    LaunchedEffect(found) { runCatching { first.requestFocus() } }
                }
            }
            Spacer(Modifier.height(32.dp))
            Label("Adresse oder Einladungslink eingeben")
            Feld(input, { input = it }, "192.168.178.20", Modifier.widthIn(max = 420.dp).fillMaxWidth().padding(top = 8.dp), type = KeyboardType.Uri,
                onGo = { if (input.isNotBlank()) onConnect(input) })
            Knopf("Verbinden", { if (input.isNotBlank()) onConnect(input) }, Modifier.padding(top = 12.dp))
            onScan?.let { Knopf("QR-Code scannen", it, Modifier.padding(top = 12.dp), icon = Ic.QR) }
            if (error.isNotEmpty()) Fehler(error)
        }
    }
}

/**
 * „Wer schaut?“ – Profilauswahl; auf dem TV zusätzlich Kopplungscode mit QR (kein Tippen mit der Fernbedienung).
 * „Mit Name anmelden“ für Konten ohne Kachel (Gäste mit Passwort, z. B. von außerhalb); [onScan] liest eine Einladung.
 */
@Composable
fun LoginScreen(users: List<User>?, pairCode: String?, pairQr: String?, error: String, onLogin: (User, String?) -> Unit, onScan: () -> Unit,
                onChangeServer: () -> Unit) {
    var pick by remember { mutableStateOf<User?>(null) }
    var manuell by remember { mutableStateOf(false) }
    var name by remember { mutableStateOf("") }
    var pw by remember { mutableStateOf("") }
    val first = remember { FocusRequester() }
    val tv = LocalTv.current
    Page {
        Column(Modifier.fillMaxSize().systemBarsPadding().padding(24.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
            val p = pick
            if (manuell) {
                T("Mit Name anmelden", LocalTypo.current.titel, weight = FontWeight.SemiBold)
                Feld(name, { name = it }, "Name", Modifier.widthIn(max = 360.dp).fillMaxWidth().padding(top = 24.dp).focusRequester(first))
                Feld(pw, { pw = it }, "Passwort", Modifier.widthIn(max = 360.dp).fillMaxWidth().padding(top = 12.dp), password = true,
                    onGo = { if (name.isNotBlank()) onLogin(User(name.trim(), name.trim()), pw) })
                Row(Modifier.padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Knopf("Anmelden", { if (name.isNotBlank()) onLogin(User(name.trim(), name.trim()), pw) }, primary = true) // Server nimmt auch den Namen
                    Knopf("Zurück", { manuell = false; pw = "" })
                }
                LaunchedEffect(Unit) { runCatching { first.requestFocus() } }
            } else if (p != null) {
                Avatar(p.name, p.color, if (tv) 128.dp else 88.dp)
                T(p.name, LocalTypo.current.titel, weight = FontWeight.SemiBold, modifier = Modifier.padding(top = 16.dp))
                Feld(pw, { pw = it }, "Passwort", Modifier.widthIn(max = 360.dp).fillMaxWidth().padding(top = 24.dp).focusRequester(first), password = true, onGo = { onLogin(p, pw) })
                Row(Modifier.padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Knopf("Anmelden", { onLogin(p, pw) }, primary = true)
                    Knopf("Zurück", { pick = null; pw = "" })
                }
                LaunchedEffect(p) { runCatching { first.requestFocus() } }
            } else {
                T("Wer schaut?", LocalTypo.current.titel, weight = FontWeight.SemiBold)
                Spacer(Modifier.height(32.dp))
                if (users == null) Label("Lade …")
                LazyRow(horizontalArrangement = Arrangement.spacedBy(if (tv) 40.dp else 20.dp)) {
                    items(users.orEmpty(), key = { it.id }) { u ->
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            val sz = if (tv) 160.dp else 96.dp
                            Box(Modifier.size(sz).then(if (u == users?.first()) Modifier.focusRequester(first) else Modifier)
                                .klick({ if (u.hasPassword) pick = u else onLogin(u, null) })) { Avatar(u.name, u.color, sz) }
                            T(u.name, LocalTypo.current.karte, K.Text2, modifier = Modifier.padding(top = 12.dp))
                        }
                    }
                }
                LaunchedEffect(users) { if (!users.isNullOrEmpty()) runCatching { first.requestFocus() } }
                if (pairCode != null) {
                    Row(Modifier.padding(top = 48.dp), verticalAlignment = Alignment.CenterVertically) {
                        Column(horizontalAlignment = Alignment.End) {
                            Label("Mit dem Handy scannen oder unter „Einstellungen › Schnellverbindung“ eingeben")
                            T(pairCode.chunked(3).joinToString(" "), LocalTypo.current.code, family = Mono, modifier = Modifier.padding(top = 8.dp))
                        }
                        // PNG mit weißem Rand; ohne Filter skaliert, sonst verschwimmen die Module
                        if (pairQr != null) AsyncImage(pairQr, "QR-Code zum Koppeln", Modifier.padding(start = 32.dp).size(200.dp), filterQuality = FilterQuality.None)
                    }
                }
                FlowRow(Modifier.padding(top = 40.dp), horizontalArrangement = Arrangement.spacedBy(12.dp, Alignment.CenterHorizontally),
                    verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    if (!tv) Knopf("Einladung scannen", onScan, icon = Ic.QR)
                    Knopf("Mit Name anmelden", { manuell = true }, icon = Ic.PROFIL)
                    Knopf("Anderer Server", onChangeServer, icon = Ic.SERVER)
                }
            }
            if (error.isNotEmpty()) Fehler(error)
        }
    }
}

/** Einladung einlösen: Name und Passwort wählen – danach klappt die Anmeldung überall, auch von unterwegs. */
@Composable
fun RegistrierenScreen(error: String, onRegister: (name: String, password: String) -> Unit, onBack: () -> Unit) {
    var name by remember { mutableStateOf("") }
    var pw by remember { mutableStateOf("") }
    val first = remember { FocusRequester() }
    val los = { if (name.isNotBlank() && pw.length >= 4) onRegister(name.trim(), pw) }
    Page {
        Column(Modifier.fillMaxSize().systemBarsPadding().padding(24.dp).verticalScroll(rememberScrollState()), horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center) {
            T("Du bist eingeladen", LocalTypo.current.titel, weight = FontWeight.SemiBold)
            T("Wie sollen dich die anderen sehen? Mit Name und Passwort meldest du dich später wieder an – in der App, im Browser und von unterwegs.",
                LocalTypo.current.text, K.Text2, modifier = Modifier.widthIn(max = 420.dp).padding(top = 12.dp))
            Feld(name, { name = it.take(40) }, "Dein Name", Modifier.widthIn(max = 360.dp).fillMaxWidth().padding(top = 24.dp).focusRequester(first))
            Feld(pw, { pw = it }, "Passwort (mindestens 4 Zeichen)", Modifier.widthIn(max = 360.dp).fillMaxWidth().padding(top = 12.dp), password = true, onGo = los)
            Row(Modifier.padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Knopf("Los geht’s", los, primary = name.isNotBlank() && pw.length >= 4)
                Knopf("Zurück", onBack)
            }
            if (error.isNotEmpty()) Fehler(error)
            LaunchedEffect(Unit) { runCatching { first.requestFocus() } }
        }
    }
}

// ---------- Startseite ----------

/** Startseite: Meine Medien, Weiterschauen, Als Nächstes, Kürzlich hinzugefügt je Bibliothek. TV mit Kopfbereich zum fokussierten Titel. */
@Composable
fun StartSeite(d: Daten, h: Handlungen) {
    val tv = LocalTv.current
    val breit = LocalBreit.current
    // Reihenfolge und Auswahl der Reihen aus Einstellungen › Startseite (d.rows ist schon sortiert, versteckte fehlen)
    val standard = listOf("continue", "nextup", "recent-movies", "recent-series")
    val reihenfolge = (d.rows.orEmpty().map { it.id } + standard).distinct().filter { it !in d.versteckt }
    val weiter = d.row("continue")
    val naechste = d.row("nextup")
    // Ältere Server liefern „recent“ statt der zwei Reihen: dann aus der Bibliothek
    val neuFilme = d.row("recent-movies").ifEmpty { d.filme.sortedByDescending { it.added }.take(16) }
    val neuSerien = d.row("recent-series").ifEmpty {
        d.serien.values.mapNotNull { eps -> eps.maxByOrNull { it.added } }.sortedByDescending { it.added }.take(16)
    }
    var fokus by remember { mutableStateOf<Item?>(null) }
    val erster = remember { FocusRequester() } // TV: Fokus beim Start auf die erste Kachel
    LaunchedEffect(d.rows != null && d.library.isNotEmpty()) { if (tv) { delay(150); runCatching { erster.requestFocus() } } }
    if (d.rows == null && d.library.isEmpty()) { Leer("Lade Bibliothek …"); return }
    if (d.library.isEmpty()) { Leer("Noch keine Titel – im Web unter Einstellungen › Bibliotheken einen Ordner hinzufügen."); return }

    Column(Modifier.fillMaxSize()) {
        if (tv) TvKopfbereich(fokus ?: weiter.firstOrNull() ?: naechste.firstOrNull() ?: neuFilme.firstOrNull(), d, if (fokus != null && fokus in weiter) "Weiterschauen" else "")
        LazyColumn(Modifier.weight(1f), contentPadding = PaddingValues(bottom = 48.dp)) {
            item(key = "medien") {
                Column(Modifier.padding(top = if (tv) 8.dp else 16.dp)) {
                    Abschnitt("Meine Medien", modifier = Modifier.padding(start = Pad, end = Pad, bottom = 12.dp))
                    MeineMedien(d, h)
                }
            }
            val weiterUndNaechste = if (tv) weiter + naechste.filter { n -> weiter.none { it.series != null && it.series == n.series } } else weiter
            val inhalt = mapOf("continue" to weiterUndNaechste, "nextup" to if (tv) emptyList() else naechste, "recent-movies" to neuFilme, "recent-series" to neuSerien)
            val erste = reihenfolge.firstOrNull { inhalt[it].orEmpty().isNotEmpty() } // TV: dort sitzt der Fokus am Anfang
            val fokusStart: (String, Item) -> Modifier = { id, e -> if (id == erste && e == inhalt[id]?.first()) Modifier.focusRequester(erster) else Modifier }
            for (id in reihenfolge) when (id) {
                "continue" -> if (weiterUndNaechste.isNotEmpty()) item(key = "weiter") {
                    Reihe(if (tv) "Weiterschauen & Als Nächstes" else "Weiterschauen") {
                        items(weiterUndNaechste, key = { "w" + it.id }) { e ->
                            BreitKarte(e.series ?: e.displayTitle, if (e.series != null) "${folge(e)} · ${e.displayTitle}" else "${jahr(e)} · Fortsetzen ab ${fmtTime(e.progress)}",
                                d.abs(e.backdrop), e.color, { h.abspielen(e, null) }, Modifier.width(if (tv) 416.dp else if (breit) 300.dp else 256.dp).tvFokus { fokus = e }
                                    .then(fokusStart(id, e)),
                                ampel = e.light, rest = if (e.progress > 0 && e.duration > 0) "Noch ${fmtDauer(e.duration - e.progress)}" else null,
                                nr = if (e.progress <= 0 && e.series != null) folge(e) else null, fortschritt = e.anteil(), onLang = { h.mehr(e) })
                        }
                    }
                }
                "nextup" -> if (!tv && naechste.isNotEmpty()) item(key = "naechste") {
                    Reihe("Als Nächstes", { h.bereich(Bereich.Serien) }) {
                        items(naechste, key = { "n" + it.id }) { e ->
                            BreitKarte(e.series ?: e.displayTitle, "${folge(e)} · ${e.displayTitle}", d.abs(e.backdrop), e.color, { h.abspielen(e, null) },
                                Modifier.width(if (breit) 300.dp else 208.dp).then(fokusStart(id, e)), ampel = e.light, nr = folge(e), onLang = { h.mehr(e) })
                        }
                    }
                }
                "recent-movies" -> if (neuFilme.isNotEmpty()) item(key = "filme") {
                    Reihe("Kürzlich hinzugefügt in Filme", { h.bereich(Bereich.Filme) }) {
                        items(neuFilme, key = { "f" + it.id }) { Kachel(it, d, h, Modifier.width(posterBreite).tvFokus { fokus = it }.then(fokusStart(id, it))) }
                    }
                }
                "recent-series" -> if (neuSerien.isNotEmpty()) item(key = "serien") {
                    Reihe("Kürzlich hinzugefügt in Serien", { h.bereich(Bereich.Serien) }) {
                        items(neuSerien, key = { "s" + it.id }) { Kachel(it, d, h, Modifier.width(posterBreite).tvFokus { fokus = it }.then(fokusStart(id, it))) }
                    }
                }
            }
        }
    }
}

/** Nur TV: meldet, welcher Titel gerade den Fokus hat (für den Kopfbereich). */
@Composable
private fun Modifier.tvFokus(onFokus: () -> Unit): Modifier =
    if (LocalTv.current) onFocusChanged { if (it.hasFocus) onFokus() } else this

/** TV-Start: Kopfbereich mit Hintergrundbild rechts oben, Plakattitel, Fakten und Inhalt des fokussierten Titels. */
@Composable
private fun TvKopfbereich(it: Item?, d: Daten, label: String) {
    if (it == null) return
    val eps = it.series?.let { s -> d.serien[s] }
    Box(Modifier.fillMaxWidth().height(400.dp)) {
        Box(Modifier.align(Alignment.TopEnd).fillMaxWidth(0.6f).fillMaxHeight()) {
            Art(d.abs(it.backdrop), "", Modifier.fillMaxSize(), ton(it.color))
            Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(listOf(K.Saal.copy(alpha = 0.92f), K.Saal.copy(alpha = 0.55f), K.Saal.copy(alpha = 0.15f)))))
            Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.6f to Color.Transparent, 1f to K.Saal)))
        }
        Column(Modifier.padding(start = Pad, top = Tokens.Abstand.TvRandOben).width(1000.dp)) {
            val l = listOfNotNull(label.ifEmpty { null }, it.series?.let { _ -> folge(it) }).joinToString(" · ")
            if (l.isNotEmpty()) Label(l, color = K.Text2)
            PlakatTitel(it.series ?: it.displayTitle, 96.sp, Modifier.padding(top = 4.dp), maxLines = 1)
            Fakten(it, eps, Modifier.padding(top = 12.dp))
            (it.meta?.overview)?.let { o -> T(o, LocalTypo.current.text, K.Text, maxLines = 2, modifier = Modifier.padding(top = 12.dp)) }
        }
    }
}

/** Faktenzeile: Jahr · Dauer · FSK · ★ · Rest · endet um · Ampel. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun Fakten(it: Item, eps: List<Item>?, modifier: Modifier = Modifier) {
    val typo = LocalTypo.current
    FlowRow(modifier, horizontalArrangement = Arrangement.spacedBy(if (LocalTv.current) 16.dp else 10.dp), verticalArrangement = Arrangement.Center) {
        val m = it.meta
        val teile = listOfNotNull(
            if (eps != null) monoSerie(eps).substringBefore(" ·").ifEmpty { null } else jahr(it).ifEmpty { null },
            if (eps == null && it.duration > 0) fmtDauer(it.duration) else null,
        )
        teile.forEach { t -> T(t, typo.klein, K.Text2) }
        m?.age?.let { a -> Fsk(a) }
        m?.rating?.takeIf { r -> r > 0 }?.let { r ->
            Row(verticalAlignment = Alignment.CenterVertically) {
                Ico(Ic.STERN, if (LocalTv.current) 22.dp else 14.dp, K.Text2, voll = true)
                T("%.1f".format(java.util.Locale.GERMANY, r), typo.klein, K.Text2, modifier = Modifier.padding(start = 3.dp))
            }
        }
        if (it.progress > 0 && it.duration > 0 && !it.watched) T("Noch ${fmtDauer(it.duration - it.progress)}", typo.klein, K.Text2)
        if (it.duration > 0) T("endet um ${endetUm(it.duration - if (it.watched) 0.0 else it.progress)}", typo.klein, K.Text2)
        Ampel(it.light)
    }
}

/** „1 Serie“, „2 Serien“. */
fun anzahl(n: Int, eins: String, viele: String) = "$n ${if (n == 1) eins else viele}"

/**
 * Kacheln unter „Meine Medien“: Handy 2 Spalten, Tablet 6, TV eine Reihe.
 * Vorschau = Hintergrundbild eines Titels der Kategorie; ohne Bild bleibt dessen Tonfläche.
 */
@Composable
private fun MeineMedien(d: Daten, h: Handlungen) {
    data class M(val b: Bereich, val anzahl: String, val titel: Item?)
    fun wahl(items: Sequence<Item>) = items.firstOrNull { it.backdrop.isNotEmpty() } ?: items.firstOrNull { it.color.isNotEmpty() }
    fun wahl(listen: List<Liste>) = wahl(listen.asSequence().flatMap { it.items }.mapNotNull(d::resolve))
    val liste = buildList {
        if (d.filme.isNotEmpty()) add(M(Bereich.Filme, anzahl(d.filme.size, "Film", "Filme"), wahl(d.filme.asSequence())))
        if (d.serien.isNotEmpty()) add(M(Bereich.Serien, anzahl(d.serien.size, "Serie", "Serien"), wahl(d.serien.values.asSequence().flatten())))
        d.sammlungen?.let { add(M(Bereich.Sammlungen, anzahl(it.size, "Sammlung", "Sammlungen"), wahl(it))) }
        d.listen?.let { add(M(Bereich.Listen, anzahl(it.size, "Liste", "Listen"), wahl(it))) }
        add(M(Bereich.Favoriten, anzahl(d.favs.size, "Favorit", "Favoriten"), wahl(d.favs.asSequence().mapNotNull(d::resolve))))
    }
    val tv = LocalTv.current
    val spalten = if (tv) liste.size else if (LocalBreit.current) 6 else 2
    Column(Modifier.padding(horizontal = Pad), verticalArrangement = Arrangement.spacedBy(if (tv) 24.dp else 12.dp)) {
        liste.chunked(spalten).forEach { zeile ->
            Row(horizontalArrangement = Arrangement.spacedBy(if (tv) 24.dp else if (LocalBreit.current) 16.dp else 12.dp)) {
                zeile.forEach { m ->
                    MedienKachel(m.b.label, m.anzahl, m.b.icon, m.titel?.color?.takeIf { it.isNotEmpty() }?.let { ton(it) } ?: K.Flaeche3, { h.bereich(m.b) },
                        if (tv) Modifier.width(296.dp) else Modifier.weight(1f), bild = d.abs(m.titel?.backdrop ?: ""))
                }
                if (!tv) repeat(spalten - zeile.size) { Spacer(Modifier.weight(1f)) }
            }
        }
    }
}

// ---------- Raster (Bibliotheken, Favoriten, Listen, Person) ----------

val rasterSpalten: GridCells @Composable get() = if (LocalTv.current) GridCells.Fixed(6) else if (LocalBreit.current) GridCells.Adaptive(150.dp) else GridCells.Fixed(3)

@Composable
fun Raster(items: List<Item>, d: Daten, h: Handlungen, leer: String, ampelLinks: Boolean = false, kopf: LazyGridScope.() -> Unit = {}) {
    val tv = LocalTv.current
    LazyVerticalGrid(
        rasterSpalten, Modifier.fillMaxSize(),
        contentPadding = PaddingValues(start = Pad, end = Pad, top = 4.dp, bottom = 32.dp),
        horizontalArrangement = Arrangement.spacedBy(if (tv) 56.dp else Luecke),
        verticalArrangement = Arrangement.spacedBy(if (tv) 24.dp else 16.dp),
    ) {
        kopf()
        if (items.isEmpty()) item(span = { GridItemSpan(maxLineSpan) }) { T(leer, LocalTypo.current.text, K.Text2, modifier = Modifier.padding(vertical = 16.dp)) }
        items(items.size) { i -> Kachel(items[i], d, h, ampelLinks = ampelLinks) }
    }
}

// ---------- Favoriten ----------

@Composable
fun FavoritenSeite(d: Daten, h: Handlungen) {
    val favs = d.favs.mapNotNull { d.resolve(it) }.sortedBy { norm(it.series ?: it.displayTitle) }
    Raster(favs, d, h, "Noch keine Favoriten – bei einem Titel auf das Herz tippen.")
}

// ---------- Bibliothek (Filme, Serien) ----------

enum class Sortierung(val label: String) { Name("Name"), Neu("Neu hinzugefügt"), Jahr("Erscheinungsjahr"), Bewertung("Bewertung") }

/** Filme bzw. Serien: Tabs Alle · Vorschläge/Als Nächstes · Favoriten · Genres (· Studios bei Filmen). */
fun bibliothekTabs(serien: Boolean) = if (serien) listOf("Serien", "Als Nächstes", "Favoriten", "Genres") else listOf("Filme", "Vorschläge", "Favoriten", "Genres", "Studios")

@Composable
fun BibliothekSeite(serien: Boolean, tab: Int, d: Daten, h: Handlungen) {
    val alle = if (serien) d.serien.values.map { it.first() } else d.filme
    when (tab) {
        0 -> BibliothekAlle(alle, serien, d, h)
        1 -> if (serien) {
            val n = d.row("nextup")
            BreitRaster(n, d, h, "Keine angefangenen Serien.")
        } else Raster(alle.filter { !it.watched && it.progress <= 0 }.sortedByDescending { it.meta?.rating ?: 0.0 }.take(60), d, h, "Keine Vorschläge – alles gesehen.")
        2 -> Raster(alle.filter { it.key in d.favs }, d, h, "Noch keine Favoriten.")
        3 -> Gruppen(alle, d, h, "Noch keine Genres – die kommen mit den Metadaten.") { it.meta?.genres.orEmpty() }
        else -> Gruppen(alle, d, h, "Noch keine Studios – die kommen mit den Metadaten.") { it.meta?.studios.orEmpty() }
    }
}

/** Filter aus dem Blatt „Sortieren & Filtern“. */
data class Filter(
    val status: String = "alle", // alle | ungesehen | angefangen | gesehen
    val direkt: Boolean = false,
    val genres: Set<String> = emptySet(),
    val fsk: Set<Int> = emptySet(),
    val jahrzehnte: Set<Int> = emptySet(),
) {
    val aktiv get() = this != Filter()
}

/** Wendet [f] an; Serien zählen über ihre Folgen als gesehen bzw. angefangen. */
fun filtern(l: List<Item>, f: Filter, serien: Map<String, List<Item>>): List<Item> = l.filter { it ->
    val eps = it.series?.let { s -> serien[s] }
    val gesehen = eps?.all { e -> e.watched } ?: it.watched
    val angefangen = !gesehen && (eps?.any { e -> e.watched || e.progress > 0 } ?: (it.progress > 0))
    val jahr = it.meta?.year ?: it.year
    (f.status == "alle" || (f.status == "ungesehen" && !gesehen) || (f.status == "angefangen" && angefangen) || (f.status == "gesehen" && gesehen)) &&
        (!f.direkt || it.light == "green") &&
        (f.genres.isEmpty() || it.meta?.genres.orEmpty().any { g -> g in f.genres }) &&
        (f.fsk.isEmpty() || it.meta?.age in f.fsk) &&
        (f.jahrzehnte.isEmpty() || (jahr != null && jahr / 10 * 10 in f.jahrzehnte))
}

@Composable
private fun BibliothekAlle(alle: List<Item>, serien: Boolean, d: Daten, h: Handlungen) {
    var sort by remember { mutableStateOf(Sortierung.Name) }
    var filter by remember { mutableStateOf(Filter()) }
    var blatt by remember { mutableStateOf(false) }
    var auswahl by remember { mutableStateOf<Set<String>?>(null) } // Mehrfachauswahl: IDs, null = aus
    var listenWahl by remember { mutableStateOf<Boolean?>(null) } // true = Sammlung, false = Wiedergabeliste
    val name: (Item) -> String = { norm(it.series ?: it.displayTitle).removePrefix("der ").removePrefix("die ").removePrefix("das ").removePrefix("the ") }
    val liste = remember(alle, sort, filter, d.serien) {
        filtern(alle, filter, d.serien).let { l ->
            when (sort) {
                Sortierung.Name -> l.sortedBy(name)
                Sortierung.Neu -> l.sortedByDescending { if (serien) d.serien[it.series].orEmpty().maxOf { e -> e.added } else it.added }
                Sortierung.Jahr -> l.sortedByDescending { it.meta?.year ?: it.year ?: 0 }
                Sortierung.Bewertung -> l.sortedByDescending { it.meta?.rating ?: 0.0 }
            }
        }
    }
    val grid = rememberLazyGridState()
    val scope = rememberCoroutineScope()
    val tv = LocalTv.current
    val gewaehlt = auswahl?.let { a -> liste.filter { it.id in a } }.orEmpty()
    // Serien in der Auswahl stehen für alle ihre Folgen (gesehen, herunterladen)
    val folgen = gewaehlt.flatMap { if (it.series != null) d.serien[it.series].orEmpty() else listOf(it) }
    Box(Modifier.fillMaxSize()) {
        LazyVerticalGrid(
            rasterSpalten, Modifier.fillMaxSize(), grid,
            contentPadding = PaddingValues(start = Pad, end = if (!tv && !LocalBreit.current && sort == Sortierung.Name) 26.dp else Pad, bottom = if (auswahl != null) 96.dp else 32.dp),
            horizontalArrangement = Arrangement.spacedBy(if (tv) 56.dp else Luecke),
            verticalArrangement = Arrangement.spacedBy(if (tv) 24.dp else 16.dp),
        ) {
            item(span = { GridItemSpan(maxLineSpan) }) {
                Column {
                    Row(Modifier.height(if (tv) 72.dp else 52.dp), verticalAlignment = Alignment.CenterVertically) {
                        T(if (liste.size == alle.size) anzahl(alle.size, if (serien) "Serie" else "Film", if (serien) "Serien" else "Filme") else "${liste.size} von ${anzahl(alle.size, if (serien) "Serie" else "Film", if (serien) "Serien" else "Filme")}",
                            LocalTypo.current.text, K.Text, FontWeight.Medium, modifier = Modifier.weight(1f))
                        val g = if (tv) 72.dp else 44.dp
                        IconKnopf(Ic.ZUFALL, {
                            val z = liste.randomOrNull() ?: return@IconKnopf
                            if (serien) h.abspielen(d.serien[z.series]?.let(::naechsteFolge) ?: z, null) else h.abspielen(z, null)
                        }, groesse = g, rahmen = tv)
                        IconKnopf(Ic.HAKEN, { auswahl = if (auswahl == null) emptySet() else null }, groesse = g, rahmen = tv, an = auswahl != null)
                        Box {
                            IconKnopf(Ic.FILTER, { blatt = true }, groesse = g, rahmen = tv, an = filter.aktiv || sort != Sortierung.Name)
                            if (filter.aktiv) Box(Modifier.align(Alignment.TopEnd).padding(8.dp).size(8.dp).background(K.Text, androidx.compose.foundation.shape.CircleShape))
                        }
                    }
                    Row(Modifier.padding(bottom = 12.dp).horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Chip("Ungesehen", filter.status == "ungesehen", { filter = filter.copy(status = if (filter.status == "ungesehen") "alle" else "ungesehen") })
                        Chip("Direkt abspielbar", filter.direkt, { filter = filter.copy(direkt = !filter.direkt) }, punkt = "green")
                        filter.genres.forEach { g -> Chip(g, true, { filter = filter.copy(genres = filter.genres - g) }) }
                    }
                }
            }
            if (liste.isEmpty()) item(span = { GridItemSpan(maxLineSpan) }) { T("Nichts gefunden.", LocalTypo.current.text, K.Text2) }
            items(liste.size, key = { liste[it].id }) { i ->
                val it = liste[i]
                Kachel(it, d, h, ampelLinks = !serien, markiert = auswahl?.let { a -> it.id in a }) {
                    auswahl = auswahl?.let { a -> if (it.id in a) a - it.id else a + it.id }
                }
            }
        }
        // Alphabet-Leiste zum Springen (nur Handy, nur nach Name sortiert)
        if (!tv && !LocalBreit.current && sort == Sortierung.Name && liste.size > 30 && auswahl == null) {
            Column(Modifier.align(Alignment.TopEnd).padding(top = 8.dp, end = 2.dp).width(22.dp).fillMaxHeight(0.8f), verticalArrangement = Arrangement.SpaceBetween,
                horizontalAlignment = Alignment.CenterHorizontally) {
                ("#ABCDEFGHIJKLMNOPQRSTUVWXYZ").forEach { z ->
                    T(z.toString(), 10.sp, K.Text3, family = Mono, modifier = Modifier.clickable {
                        val i = liste.indexOfFirst { val c = name(it).firstOrNull() ?: '#'; if (z == '#') !c.isLetter() else c.uppercaseChar() >= z }
                        if (i >= 0) scope.launch { grid.scrollToItem(i + 1) }
                    })
                }
            }
        }
        // Leiste der Mehrfachauswahl
        if (auswahl != null) Row(Modifier.align(Alignment.BottomCenter).fillMaxWidth().background(K.Flaeche1).navigationBarsPadding()
            .padding(horizontal = Pad, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            T("${gewaehlt.size} ausgewählt", LocalTypo.current.text, K.Text, FontWeight.Medium, modifier = Modifier.weight(1f))
            if (gewaehlt.isNotEmpty()) {
                IconKnopf(Ic.HERZ, { gewaehlt.map { it.key }.filter { k -> k !in d.favs }.forEach(h.favorit); auswahl = null })
                IconKnopf(Ic.HAKEN, { h.gesehen(folgen, true); auswahl = null })
                if (d.listen != null) IconKnopf(Ic.LISTE, { listenWahl = false })
                if (d.sammlungen != null && d.admin) IconKnopf(Ic.SAMMLUNG, { listenWahl = true })
                h.download?.let { dl -> IconKnopf(Ic.DOWNLOAD, { dl(folgen); auswahl = null }) }
            }
            IconKnopf(Ic.SCHLIESSEN, { auswahl = null })
        }
    }
    if (blatt) FilterBlatt(sort, { sort = it }, filter, { filter = it }, alle, { blatt = false })
    listenWahl?.let { s ->
        Blatt({ listenWahl = null }, { BlattKopf(if (s) "Zu Sammlung hinzufügen" else "Zu Wiedergabeliste hinzufügen", "${gewaehlt.size} Titel", "", "") }) {
            ListenWahl(s, d, h, gewaehlt.map { it.key }) { listenWahl = null; auswahl = null }
        }
    }
}

/** Blatt „Sortieren & Filtern“: Sortierung, Status, Läuft direkt, Genres, FSK, Jahrzehnte. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun FilterBlatt(sort: Sortierung, onSort: (Sortierung) -> Unit, f: Filter, onFilter: (Filter) -> Unit, alle: List<Item>, onZu: () -> Unit) {
    val genres = remember(alle) { alle.flatMap { it.meta?.genres.orEmpty() }.groupingBy { it }.eachCount().entries.sortedByDescending { it.value }.map { it.key } }
    val jahrzehnte = remember(alle) { alle.mapNotNull { (it.meta?.year ?: it.year)?.let { j -> j / 10 * 10 } }.distinct().sortedDescending() }
    val fsk = remember(alle) { alle.mapNotNull { it.meta?.age }.distinct().sorted() }
    fun <T> Set<T>.um(x: T) = if (x in this) this - x else this + x
    Blatt(onZu, { BlattKopf("Sortieren & Filtern", "", "", "") }) {
        Column(Modifier.heightIn(max = 560.dp).verticalScroll(rememberScrollState()).padding(horizontal = 16.dp)) {
            @Composable fun Gruppe(titel: String, inhalt: @Composable () -> Unit) {
                Label(titel, Modifier.padding(top = 16.dp, bottom = 8.dp))
                FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) { inhalt() }
            }
            Gruppe("Sortieren") { Sortierung.entries.forEach { s -> Chip(s.label, s == sort, { onSort(s) }) } }
            Gruppe("Status") {
                listOf("alle" to "Alle", "ungesehen" to "Ungesehen", "angefangen" to "Angefangen", "gesehen" to "Gesehen")
                    .forEach { (k, l) -> Chip(l, f.status == k, { onFilter(f.copy(status = k)) }) }
            }
            Gruppe("Wiedergabe") { Chip("Läuft direkt", f.direkt, { onFilter(f.copy(direkt = !f.direkt)) }, punkt = "green") }
            if (genres.isNotEmpty()) Gruppe("Genres") { genres.forEach { g -> Chip(g, g in f.genres, { onFilter(f.copy(genres = f.genres.um(g))) }) } }
            if (fsk.isNotEmpty()) Gruppe("FSK") { fsk.forEach { a -> Chip("FSK $a", a in f.fsk, { onFilter(f.copy(fsk = f.fsk.um(a))) }) } }
            if (jahrzehnte.isNotEmpty()) Gruppe("Jahrzehnte") { jahrzehnte.forEach { j -> Chip("${j}er", j in f.jahrzehnte, { onFilter(f.copy(jahrzehnte = f.jahrzehnte.um(j))) }) } }
            Row(Modifier.padding(vertical = 16.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Knopf("Zurücksetzen", { onFilter(Filter()); onSort(Sortierung.Name) })
                Knopf("Fertig", onZu, primary = true)
            }
        }
    }
}

/** Gruppen mit Sprung-Chips und je Gruppe eine Reihe (Genres, Studios; Tablet-Bibliothek im Entwurf). */
@Composable
private fun Gruppen(alle: List<Item>, d: Daten, h: Handlungen, leer: String, schluessel: (Item) -> List<String>) {
    val gruppen = remember(alle) {
        alle.flatMap(schluessel).groupingBy { it }.eachCount().entries.sortedByDescending { it.value }.map { it.key to it.value }
    }
    val liste = androidx.compose.foundation.lazy.rememberLazyListState()
    val scope = rememberCoroutineScope()
    if (gruppen.isEmpty()) { Leer(leer); return }
    LazyColumn(Modifier.fillMaxSize(), liste, contentPadding = PaddingValues(bottom = 32.dp)) {
        item {
            Row(Modifier.horizontalScroll(rememberScrollState()).padding(horizontal = Pad, vertical = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically) {
                Label("Springen zu", Modifier.padding(end = 8.dp))
                gruppen.forEachIndexed { i, (g, n) -> Chip("$g · $n", false, { scope.launch { liste.animateScrollToItem(i + 1) } }) }
            }
        }
        gruppen.forEach { (g, n) ->
            item(key = g) {
                Reihe("$g · $n") {
                    items(alle.filter { g in schluessel(it) }, key = { g + it.id }) { Kachel(it, d, h, Modifier.width(posterBreite)) }
                }
            }
        }
    }
}

/** Raster mit Querformat-Kacheln (Als Nächstes). */
@Composable
private fun BreitRaster(items: List<Item>, d: Daten, h: Handlungen, leer: String) {
    val tv = LocalTv.current
    LazyVerticalGrid(
        if (tv) GridCells.Fixed(4) else GridCells.Adaptive(if (LocalBreit.current) 280.dp else 170.dp), Modifier.fillMaxSize(),
        contentPadding = PaddingValues(Pad), horizontalArrangement = Arrangement.spacedBy(Luecke), verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        if (items.isEmpty()) item(span = { GridItemSpan(maxLineSpan) }) { T(leer, LocalTypo.current.text, K.Text2) }
        items(items.size, key = { items[it].id }) { i ->
            val e = items[i]
            BreitKarte(e.series ?: e.displayTitle, "${folge(e)} · ${e.displayTitle}", d.abs(e.backdrop), e.color, { h.abspielen(e, null) },
                ampel = e.light, nr = folge(e), fortschritt = e.anteil(), onLang = { h.mehr(e) })
        }
    }
}

// ---------- Sammlungen und Wiedergabelisten ----------

/** Sammlungen bearbeiten nur Admins, automatische (auto-*) gar nicht; Wiedergabelisten gehören dem Profil. */
fun darfBearbeiten(l: Liste?, sammlung: Boolean, admin: Boolean) = !sammlung || (admin && (l == null || (!l.auto && !l.id.startsWith("auto-"))))

@Composable
fun ListenSeite(listen: List<Liste>?, sammlung: Boolean, d: Daten, h: Handlungen) {
    val tv = LocalTv.current
    var neu by remember { mutableStateOf(false) }
    if (listen == null) { Leer("Dieser Server kennt noch keine ${if (sammlung) "Sammlungen" else "Wiedergabelisten"}."); return }
    LazyVerticalGrid(
        if (tv) GridCells.Fixed(4) else GridCells.Adaptive(if (LocalBreit.current) 280.dp else 170.dp), Modifier.fillMaxSize(),
        contentPadding = PaddingValues(Pad), horizontalArrangement = Arrangement.spacedBy(Luecke), verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        item(span = { GridItemSpan(maxLineSpan) }) {
            Column {
                if (listen.isEmpty()) T(if (sammlung) "Noch keine Sammlungen." else "Noch keine Wiedergabelisten.", LocalTypo.current.text, K.Text2,
                    modifier = Modifier.padding(bottom = 12.dp))
                if (darfBearbeiten(null, sammlung, d.admin)) Row {
                    Knopf(if (sammlung) "Neue Sammlung" else "Neue Wiedergabeliste", { neu = true }, icon = if (sammlung) Ic.SAMMLUNG else Ic.LISTE)
                }
            }
        }
        items(listen.size, key = { listen[it].id }) { i ->
            val l = listen[i]
            val erster = d.resolve(l.items.firstOrNull() ?: "")
            BreitKarte(l.name, "${l.items.size} Titel" + if (l.auto) " · Filmreihe" else "",
                d.abs(erster?.backdrop ?: ""), erster?.color ?: "", { h.liste(l, sammlung) })
        }
    }
    if (neu) EingabeDialog(if (sammlung) "Neue Sammlung" else "Neue Wiedergabeliste", "Name der Liste.", "z. B. Filmabend", "Anlegen", "",
        onOk = { name -> h.pflege.neu(sammlung, name.trim(), emptyList()); neu = false }, onZu = { neu = false })
}

/** Eine Sammlung bzw. Wiedergabeliste; wer darf, kann umbenennen, löschen, Titel entfernen und die Reihenfolge ändern. */
@Composable
fun ListeSeite(l: Liste, sammlung: Boolean, d: Daten, h: Handlungen) {
    val items = l.items.mapNotNull { d.resolve(it) }
    val darf = darfBearbeiten(l, sammlung, d.admin)
    var bearbeiten by remember { mutableStateOf(false) }
    var dialog by remember { mutableStateOf<String?>(null) } // name | loeschen
    if (bearbeiten) ListeBearbeiten(l, sammlung, d, h) { bearbeiten = false }
    else Raster(items, d, h, "Diese Liste ist leer.") {
        item(span = { GridItemSpan(maxLineSpan) }) {
            Column(Modifier.padding(vertical = 12.dp)) {
                l.overview?.let { T(it, LocalTypo.current.text, K.Text, maxLines = 4, modifier = Modifier.padding(bottom = 12.dp)) }
                Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    if (items.isNotEmpty()) {
                        Knopf("Alle abspielen", { h.alle(items.flatMap { if (it.series != null) d.serien[it.series].orEmpty() else listOf(it) }) }, primary = true, icon = Ic.ABSPIELEN)
                        Knopf("Zufällig", { h.alle(items.shuffled()) }, icon = Ic.ZUFALL)
                    }
                    if (darf) {
                        if (l.items.isNotEmpty()) Knopf("Bearbeiten", { bearbeiten = true }, icon = Ic.SORTIEREN)
                        Knopf("Umbenennen", { dialog = "name" }, icon = Ic.BEARBEITEN)
                        Knopf("Löschen", { dialog = "loeschen" }, icon = Ic.LOESCHEN)
                    } else if (sammlung && l.auto) Label("Automatische Filmreihe", Modifier.align(Alignment.CenterVertically))
                }
            }
        }
    }
    when (dialog) {
        "name" -> EingabeDialog("Umbenennen", "Neuer Name für „${l.name}“.", l.name, "Speichern", "", start = l.name,
            onOk = { n -> h.pflege.umbenennen(l, sammlung, n.trim()); dialog = null }, onZu = { dialog = null })
        "loeschen" -> FrageDialog("„${l.name}“ löschen?", if (sammlung) "Die Sammlung verschwindet für alle. Die Titel bleiben erhalten." else "Die Titel bleiben erhalten.",
            "Löschen", onJa = { h.pflege.loeschen(l, sammlung); dialog = null; h.zurueck() }, onZu = { dialog = null })
    }
}

/** Bearbeiten: Titel entfernen und mit ↑/↓ umsortieren (auch per D-Pad). Jede Änderung geht sofort an den Server. */
@Composable
private fun ListeBearbeiten(l: Liste, sammlung: Boolean, d: Daten, h: Handlungen, onFertig: () -> Unit) {
    val tv = LocalTv.current
    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(start = Pad, end = Pad, bottom = 32.dp)) {
        item {
            Row(Modifier.padding(vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                T("${l.items.size} Titel", LocalTypo.current.text, K.Text, FontWeight.Medium, modifier = Modifier.weight(1f))
                Knopf("Fertig", onFertig, primary = true, icon = Ic.HAKEN)
            }
        }
        items(l.items.size) { i ->
            val key = l.items[i]
            val it = d.resolve(key)
            Row(Modifier.fillMaxWidth().heightIn(min = if (tv) 120.dp else 72.dp).drawBehind { drawRect(K.Linie, size = androidx.compose.ui.geometry.Size(size.width, 1.dp.toPx())) }
                .padding(vertical = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                Art(d.abs(it?.poster ?: ""), it?.series ?: it?.displayTitle ?: key, Modifier.size(if (tv) 72.dp else 40.dp, if (tv) 108.dp else 60.dp), ton(it?.color ?: ""))
                Column(Modifier.weight(1f).padding(start = 12.dp)) {
                    T(it?.series ?: it?.displayTitle ?: "Nicht mehr vorhanden", LocalTypo.current.karte, K.Text, FontWeight.Medium, maxLines = 1)
                    if (it != null) T(if (it.series != null && !key.startsWith("serie:")) "${folge(it)} · ${it.displayTitle}" else sub(it), LocalTypo.current.klein, K.Text2, maxLines = 1)
                }
                val neu = l.items.toMutableList()
                IconKnopf(Ic.HOCH, { if (i > 0) { neu.add(i - 1, neu.removeAt(i)); h.pflege.ordnen(l, sammlung, neu) } }, farbe = if (i > 0) K.Text else K.Text3)
                IconKnopf(Ic.RUNTER, { if (i < neu.size - 1) { neu.add(i + 1, neu.removeAt(i)); h.pflege.ordnen(l, sammlung, neu) } }, farbe = if (i < l.items.size - 1) K.Text else K.Text3)
                IconKnopf(Ic.SCHLIESSEN, { h.pflege.entfernen(l, sammlung, key) }, farbe = K.AmpelRot)
            }
        }
    }
}

/** Liste wählen (oder neu anlegen) und [keys] hinzufügen – Aktionen-Blatt und Mehrfachauswahl. */
@Composable
fun ColumnScope.ListenWahl(sammlung: Boolean, d: Daten, h: Handlungen, keys: List<String>, onFertig: () -> Unit) {
    var neu by remember { mutableStateOf(false) }
    val listen = (if (sammlung) d.sammlungen else d.listen).orEmpty().filter { darfBearbeiten(it, sammlung, d.admin) }
    MenueZeile(Ic.BEARBEITEN, if (sammlung) "Neue Sammlung …" else "Neue Wiedergabeliste …") { neu = true }
    listen.forEach { l -> MenueZeile(if (sammlung) Ic.SAMMLUNG else Ic.LISTE, l.name, an = keys.isNotEmpty() && keys.all { it in l.items }) { h.pflege.hinzu(l, sammlung, keys); onFertig() } }
    if (neu) EingabeDialog(if (sammlung) "Neue Sammlung" else "Neue Wiedergabeliste", "Die gewählten Titel kommen gleich hinein.", "Name", "Anlegen", "",
        onOk = { name -> h.pflege.neu(sammlung, name.trim(), keys); neu = false; onFertig() }, onZu = { neu = false })
}

// ---------- Person ----------

@Composable
fun PersonSeite(name: String, bild: String, bio: String?, items: List<Item>?, d: Daten, h: Handlungen) {
    Raster(items.orEmpty(), d, h, if (items == null) "Lade …" else "Keine Titel in dieser Bibliothek.") {
        item(span = { GridItemSpan(maxLineSpan) }) {
            Row(Modifier.padding(vertical = 16.dp), verticalAlignment = Alignment.Top) {
                PersonKarte(name, "", bild) {}
                if (!bio.isNullOrBlank()) T(bio, LocalTypo.current.text, K.Text, maxLines = 8, modifier = Modifier.padding(start = 16.dp))
            }
        }
        item(span = { GridItemSpan(maxLineSpan) }) { Abschnitt("Bekannt aus", modifier = Modifier.padding(bottom = 4.dp)) }
    }
}

// ---------- Suche ----------

/** Suche mit Filter-Chips; Treffer vom Server (fehlertolerant), ohne Server lokal über die Bibliothek. */
@Composable
fun SucheSeite(d: Daten, h: Handlungen, verlauf: List<String>, onVerlauf: (List<String>) -> Unit, serverSuche: suspend (String) -> List<Item>?) {
    var q by remember { mutableStateOf("") }
    var filter by remember { mutableStateOf("Alles") }
    var treffer by remember { mutableStateOf<List<Item>>(emptyList()) }
    val focus = remember { FocusRequester() }
    val tv = LocalTv.current
    LaunchedEffect(q) {
        if (q.isBlank()) { treffer = emptyList(); return@LaunchedEffect }
        treffer = search(q, d.library) // sofort lokal
        delay(250)
        serverSuche(q)?.let { treffer = it }
    }
    // Folgen, deren Titel passt (der Server liefert je Serie nur einen Treffer)
    val folgen = remember(q, d.library) { if (q.length < 2) emptyList() else d.library.filter { it.series != null && io.flimmer.app.score(q, it.displayTitle) > 0 }.take(20) }
    val filme = treffer.filter { it.series == null }
    val serien = treffer.filter { it.series != null }
    Column(Modifier.fillMaxSize().background(K.Saal).systemBarsPadding()) {
        Row(Modifier.fillMaxWidth().height(if (tv) 96.dp else 56.dp).padding(start = if (tv) 96.dp else 4.dp, end = if (tv) 96.dp else 12.dp, top = if (tv) 24.dp else 0.dp),
            verticalAlignment = Alignment.CenterVertically) {
            if (!tv) IconKnopf(Ic.ZURUECK, h.zurueck)
            var fokus by remember { mutableStateOf(false) }
            Row(Modifier.weight(1f).height(if (tv) 72.dp else 44.dp).fokusRingAn(fokus).background(K.Flaeche2, RoundedCornerShape(Tokens.Radius.RadiusS))
                .border(1.dp, K.LinieStark, RoundedCornerShape(Tokens.Radius.RadiusS)).padding(start = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                Ico(Ic.SUCHE, if (tv) 32.dp else 20.dp, K.Text2)
                BasicTextField(q, { q = it }, Modifier.weight(1f).padding(start = 8.dp).focusRequester(focus).onFocusChanged { fokus = it.isFocused }, singleLine = true,
                    textStyle = TextStyle(fontFamily = Sans, fontSize = if (tv) LocalTypo.current.text else 16.sp, color = K.Text), cursorBrush = SolidColor(K.Text),
                    keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                    keyboardActions = KeyboardActions(onSearch = { if (q.isNotBlank()) onVerlauf((listOf(q.trim()) + verlauf.filter { it != q.trim() }).take(8)) }),
                    decorationBox = { inner -> Box { if (q.isEmpty()) T("Filme, Serien, Folgen …", if (tv) LocalTypo.current.text else 16.sp, K.Text3, maxLines = 1); inner() } })
                if (q.isNotEmpty()) IconKnopf(Ic.SCHLIESSEN, { q = "" }, groesse = if (tv) 72.dp else 44.dp, farbe = K.Text2)
            }
        }
        LaunchedEffect(Unit) { runCatching { focus.requestFocus() } }
        Row(Modifier.fillMaxWidth().height(if (tv) 88.dp else 56.dp).horizontalScroll(rememberScrollState()).padding(start = if (tv) 96.dp else Pad, end = Pad),
            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            listOf("Alles", "Filme", "Serien", "Folgen").forEach { f -> Chip(f, filter == f, { filter = f }) }
            Chip("Direkt abspielbar", filter == "Direkt", { filter = if (filter == "Direkt") "Alles" else "Direkt" }, punkt = "green")
        }
        Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
        val zeig: (String) -> Boolean = { filter == "Alles" || filter == "Direkt" || filter == it }
        val gruen: (List<Item>) -> List<Item> = { l -> if (filter == "Direkt") l.filter { it.light == "green" } else l }
        LazyColumn(Modifier.weight(1f), contentPadding = PaddingValues(start = if (tv) 96.dp else Pad, end = if (tv) 96.dp else Pad, bottom = 32.dp)) {
            if (q.isBlank()) {
                if (verlauf.isNotEmpty()) {
                    item {
                        Row(Modifier.height(48.dp).padding(top = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                            T("Zuletzt gesucht", LocalTypo.current.reihe, K.Text, FontWeight.SemiBold, modifier = Modifier.weight(1f))
                            T("Verlauf löschen", 14.sp, K.Text2, modifier = Modifier.klick({ onVerlauf(emptyList()) }, skala = false).padding(8.dp))
                        }
                    }
                    items(verlauf) { v ->
                        Row(Modifier.fillMaxWidth().height(48.dp), verticalAlignment = Alignment.CenterVertically) {
                            Row(Modifier.weight(1f).fillMaxHeight().klick({ q = v }, skala = false), verticalAlignment = Alignment.CenterVertically) {
                                Ico(Ic.UHR, 20.dp, K.Text3); T(v, 15.sp, K.Text, modifier = Modifier.padding(start = 14.dp))
                            }
                            IconKnopf(Ic.SCHLIESSEN, { onVerlauf(verlauf - v) }, groesse = 44.dp, farbe = K.Text3)
                        }
                    }
                } else item { T("Titel, Serien oder Folgen suchen.", LocalTypo.current.text, K.Text2, modifier = Modifier.padding(vertical = 16.dp)) }
                return@LazyColumn
            }
            val s = gruen(serien); val f = gruen(filme); val e = gruen(folgen)
            if (zeig("Serien") && s.isNotEmpty()) trefferGruppe("Serien", s) { TrefferZeile(it, d, h) }
            if (zeig("Filme") && f.isNotEmpty()) trefferGruppe("Filme", f) { TrefferZeile(it, d, h) }
            if (zeig("Folgen") && e.isNotEmpty()) trefferGruppe("Folgen", e) { FolgenTreffer(it, d, h) }
            if (s.isEmpty() && f.isEmpty() && e.isEmpty()) item {
                Row(Modifier.padding(top = 24.dp).fillMaxWidth().background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusM)).padding(16.dp)) {
                    Ico(Ic.INFO, 20.dp, K.Text3)
                    T("Keine Treffer für „$q“. Tippfehler verzeiht die Suche, aber nicht alles.", 13.sp, K.Text2, modifier = Modifier.padding(start = 12.dp))
                }
            }
        }
    }
}

/** Fokusring ohne Klick (Suchfeld). */
@Composable
private fun Modifier.fokusRingAn(an: Boolean): Modifier = if (an && LocalTv.current) border(3.dp, K.Text, RoundedCornerShape(Tokens.Radius.RadiusS)).padding(3.dp) else this

private fun LazyListScope.trefferGruppe(titel: String, l: List<Item>, zeile: @Composable (Item) -> Unit) {
    item(key = "k$titel") {
        Column {
            Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
            Row(Modifier.height(48.dp).padding(top = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                T(titel, LocalTypo.current.reihe, K.Text, FontWeight.SemiBold)
                T("${l.size}", LocalTypo.current.label, K.Text3, family = Mono, modifier = Modifier.padding(start = 8.dp))
            }
        }
    }
    items(l, key = { titel + it.id }) { zeile(it) }
}

@Composable
private fun TrefferZeile(it: Item, d: Daten, h: Handlungen) {
    val tv = LocalTv.current
    val eps = it.series?.let { s -> d.serien[s] }
    Row(Modifier.fillMaxWidth().heightIn(min = if (tv) 150.dp else 96.dp).klick({ h.oeffnen(it) }, { h.mehr(it) }, skala = false).padding(vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically) {
        Art(d.abs((eps?.firstOrNull() ?: it).poster), it.series ?: it.displayTitle, Modifier.size(if (tv) 92.dp else 56.dp, if (tv) 138.dp else 84.dp), ton(it.color))
        Column(Modifier.weight(1f).padding(start = 14.dp)) {
            T(it.series ?: it.displayTitle, LocalTypo.current.karte, K.Text, FontWeight.Medium, maxLines = 1)
            T(if (eps != null) serienZeile(eps) else "Film · ${sub(it)}", LocalTypo.current.klein, K.Text2, maxLines = 1)
            Ampel(it.light)
        }
        Ico(Ic.WEITER, 20.dp, K.Text3)
    }
}

@Composable
private fun FolgenTreffer(it: Item, d: Daten, h: Handlungen) {
    val tv = LocalTv.current
    Row(Modifier.fillMaxWidth().heightIn(min = 70.dp).klick({ h.abspielen(it, null) }, { h.mehr(it) }, skala = false).padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(if (tv) 192.dp else 96.dp, if (tv) 108.dp else 54.dp)) {
            Art(d.abs(it.backdrop), "", Modifier.fillMaxSize(), ton(it.color))
            T(folge(it), 11.sp, K.Text, family = Mono, modifier = Modifier.align(Alignment.BottomStart).padding(start = 6.dp, bottom = 8.dp)
                .background(K.BadgeGrund, RoundedCornerShape(2.dp)).padding(horizontal = 4.dp))
            if (it.anteil() > 0f || it.watched) Fortschritt(if (it.watched) 1f else it.anteil(), Modifier.align(Alignment.BottomStart))
        }
        Column(Modifier.weight(1f).padding(start = 14.dp)) {
            T(it.displayTitle, LocalTypo.current.karte, K.Text, FontWeight.Medium, maxLines = 1)
            T("${it.series} · Staffel ${it.season ?: 0}", LocalTypo.current.klein, K.Text2, maxLines = 1)
            Ampel(it.light, text = (if (it.progress > 0 && !it.watched) "Noch ${fmtDauer(it.duration - it.progress)}" else fmtDauer(it.duration)) + " · " + lightText(it.light))
        }
    }
}
