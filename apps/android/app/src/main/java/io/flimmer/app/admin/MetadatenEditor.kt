package io.flimmer.app.admin

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import io.flimmer.app.ApiClient
import io.flimmer.app.Person
import io.flimmer.app.Store
import io.flimmer.app.ui.*
import kotlinx.serialization.json.put

// Metadaten-Editor wie web/src/screens/admin/MetadatenEditor.tsx: Tabs „Metadaten“ und „Identifizieren“.
// Speichern über PUT /api/items/{id}/meta, Identifizieren über /api/items/{id}/search|identify.

internal val SPERRBAR = listOf("title", "overview", "genres", "people", "studios", "tags", "age", "year", "tagline")
private val FELD = mapOf(
    "title" to "Titel", "originalTitle" to "Originaltitel", "sortTitle" to "Sortiertitel", "year" to "Jahr", "rating" to "Community-Bewertung",
    "age" to "Altersfreigabe", "tagline" to "Leitsatz", "overview" to "Übersicht", "genres" to "Genres", "studios" to "Studios", "tags" to "Tags",
    "people" to "Personen",
)
private val ARTEN = listOf("actor" to "Darsteller", "director" to "Regie", "writer" to "Drehbuch", "producer" to "Produktion")
private fun artName(kind: String) = ARTEN.firstOrNull { it.first == kind }?.second ?: if (kind == "composer") "Musik" else kind
private val FSK = listOf(-1 to "–", 0 to "FSK 0", 6 to "FSK 6", 12 to "FSK 12", 16 to "FSK 16", 18 to "FSK 18")

/** Formularzustand; Zahlen als Text wie im Eingabefeld, age -1 = keine Freigabe. */
internal data class Form(
    val title: String, val originalTitle: String, val sortTitle: String, val year: String, val rating: String, val age: Int,
    val tagline: String, val overview: String, val genres: List<String>, val studios: List<String>, val tags: List<String>,
    val people: List<Person>, val locked: List<String>,
)

internal fun formAus(d: MDetails): Form {
    val m = d.meta ?: MMeta()
    val r = m.rating ?: 0.0
    return Form(
        title = m.title ?: d.title, originalTitle = m.originalTitle ?: "", sortTitle = m.sortTitle ?: "",
        year = (m.year ?: d.year)?.toString() ?: "",
        rating = if (r <= 0) "" else if (r % 1.0 == 0.0) r.toInt().toString() else r.toString().replace('.', ','),
        age = m.age ?: -1, tagline = d.tagline ?: m.tagline ?: "", overview = m.overview ?: "", genres = m.genres,
        studios = d.studios.ifEmpty { m.studios }, tags = d.tags.ifEmpty { m.tags }, people = d.people.ifEmpty { m.people }, locked = d.locked,
    )
}

internal fun Form.aenderung() = MetaAenderung(
    title = title.trim(), originalTitle = originalTitle.trim(), sortTitle = sortTitle.trim(), year = year.trim().toIntOrNull() ?: 0,
    rating = rating.trim().replace(',', '.').toDoubleOrNull() ?: 0.0, age = age.takeIf { it >= 0 }, tagline = tagline.trim(),
    overview = overview.trim(), genres = genres, studios = studios, tags = tags, people = people, locked = locked,
)

/** Dialog „Metadaten bearbeiten“ (Kontextmenü eines Titels). */
@Composable
fun MetadatenEditor(api: ApiClient, id: String, onZu: () -> Unit) = EditorDialog(api, id, "meta", onZu)

@Composable
internal fun EditorDialog(api: ApiClient, id: String, start: String, onZu: () -> Unit, onGespeichert: () -> Unit = {}) {
    val tv = LocalTv.current
    val breit = LocalBreit.current
    val form = RoundedCornerShape(if (tv || breit) Tokens.Radius.RadiusM else 0.dp)
    Dialog(onZu, DialogProperties(usePlatformDefaultWidth = false)) {
        Column(
            Modifier.then(
                when {
                    tv -> Modifier.width(1400.dp).fillMaxHeight(0.92f)
                    breit -> Modifier.widthIn(max = 1000.dp).fillMaxWidth(0.94f).fillMaxHeight(0.94f)
                    else -> Modifier.fillMaxSize()
                },
            ).background(K.Flaeche1, form).border(1.dp, K.Linie, form).imePadding(),
        ) {
            Row(Modifier.fillMaxWidth().padding(start = if (tv) 32.dp else 20.dp, end = 8.dp, top = 8.dp, bottom = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                T("Metadaten bearbeiten", LocalTypo.current.reihe, K.Text, FontWeight.SemiBold, maxLines = 1, modifier = Modifier.weight(1f))
                IconKnopf(Ic.SCHLIESSEN, onZu)
            }
            Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
            Column(Modifier.weight(1f).verticalScroll(rememberScrollState()).padding(horizontal = if (tv) 32.dp else 20.dp).padding(bottom = 24.dp)) {
                Editor(api, id, start, fertig = onZu, onGespeichert = onGespeichert)
            }
        }
    }
}

/** Inhalt des Editors; [start] = "ident" öffnet gleich die Zuordnung (unsichere Titel). */
@Composable
internal fun Editor(api: ApiClient, id: String, start: String = "meta", fertig: (() -> Unit)? = null, onGespeichert: () -> Unit = {}) {
    val ctx = LocalContext.current
    val geraet = remember { Store(ctx).deviceId }
    val z = laden(api, id) { api.hol<MDetails>("/api/items/${k(id)}?device=${k(geraet)}") }
    var tab by remember(id) { mutableStateOf(start) }
    Geladen(z) { d ->
        Column(Modifier.padding(top = 12.dp)) {
            Leise(d.title + (d.year?.let { " ($it)" } ?: ""))
            Reiter(listOf("meta" to "Metadaten", "ident" to "Identifizieren"), tab) { tab = it }
            if (tab == "meta") key(d) { Formular(api, d, fertig, onGespeichert) }
            else Identifizieren(api, d) { z.neu(); tab = "meta"; onGespeichert() }
        }
    }
}

@Composable
private fun Reiter(tabs: List<Pair<String, String>>, aktiv: String, onWahl: (String) -> Unit) {
    val tv = LocalTv.current
    Row(Modifier.padding(top = 8.dp).fillMaxWidth().height(if (tv) 72.dp else 48.dp)
        .drawBehind { drawRect(K.Linie, Offset(0f, size.height - 1.dp.toPx()), Size(size.width, 1.dp.toPx())) }) {
        tabs.forEach { (id, label) ->
            val an = id == aktiv
            Box(Modifier.fillMaxHeight().klick({ onWahl(id) }, skala = false).padding(horizontal = 12.dp), contentAlignment = Alignment.Center) {
                T(label, if (tv) LocalTypo.current.text else 15.sp, if (an) K.Text else K.Text2, if (an) FontWeight.SemiBold else FontWeight.Medium, maxLines = 1)
                if (an) Box(Modifier.align(Alignment.BottomCenter).fillMaxWidth().height(2.dp).background(K.Text))
            }
        }
    }
}

/** Zwei Felder nebeneinander, auf dem Handy untereinander. */
@Composable
private fun Paar(a: @Composable (Modifier) -> Unit, b: @Composable (Modifier) -> Unit) {
    if (kompakt) {
        a(Modifier.fillMaxWidth().padding(top = 12.dp))
        b(Modifier.fillMaxWidth().padding(top = 12.dp))
    } else Row(Modifier.padding(top = 12.dp), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
        a(Modifier.weight(1f))
        b(Modifier.weight(1f))
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Formular(api: ApiClient, d: MDetails, fertig: (() -> Unit)?, onGespeichert: () -> Unit) {
    val a = rememberAktion()
    var f by remember(d) { mutableStateOf(formAus(d)) }
    var laeuft by remember { mutableStateOf(false) }
    // Geänderte Felder gleich sperren, wie es der Server ohne locked täte – hier sichtbar und abwählbar.
    fun setze(feld: String, neu: Form) { f = if (feld in SPERRBAR && feld !in neu.locked) neu.copy(locked = neu.locked + feld) else neu }
    Column {
        d.path?.let { p ->
            Label("Pfad", Modifier.padding(top = 16.dp))
            T(p, LocalTypo.current.klein, K.Text2, family = Mono, modifier = Modifier.padding(top = 4.dp))
        }
        Eingabe(f.title, { setze("title", f.copy(title = it)) }, "Titel", Modifier.fillMaxWidth().padding(top = 12.dp))
        Paar({ Eingabe(f.originalTitle, { v -> setze("originalTitle", f.copy(originalTitle = v)) }, "Originaltitel", it) },
            { Eingabe(f.sortTitle, { v -> setze("sortTitle", f.copy(sortTitle = v)) }, "Sortiertitel", it) })
        Paar({ Eingabe(f.year, { v -> setze("year", f.copy(year = v)) }, "Jahr", it, zahl = true) },
            { Eingabe(f.rating, { v -> setze("rating", f.copy(rating = v)) }, "Community-Bewertung", it) })
        Unterkopf("Altersfreigabe")
        Wahl(FSK, f.age) { setze("age", f.copy(age = it)) }
        Eingabe(f.tagline, { setze("tagline", f.copy(tagline = it)) }, "Leitsatz", Modifier.fillMaxWidth().padding(top = 12.dp))
        Eingabe(f.overview, { setze("overview", f.copy(overview = it)) }, "Übersicht", Modifier.fillMaxWidth().padding(top = 12.dp), mehrzeilig = true)
        ListeFeld("Genres", f.genres) { setze("genres", f.copy(genres = it)) }
        ListeFeld("Studios", f.studios) { setze("studios", f.copy(studios = it)) }
        ListeFeld("Tags", f.tags) { setze("tags", f.copy(tags = it)) }
        val ids = listOfNotNull(d.meta?.tmdbId?.let { "TMDB $it" }, d.meta?.imdbId?.takeIf { it.isNotEmpty() }?.let { "IMDb $it" })
        if (ids.isNotEmpty()) Leise(ids.joinToString(" · "), Modifier.padding(top = 12.dp))
        Personen(f.people) { setze("people", f.copy(people = it)) }
        Gruppe("Felder sperren", "Gesperrte Felder überschreibt Flimmer beim Aktualisieren nicht.") {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                SPERRBAR.forEach { kf -> Chip(FELD[kf] ?: kf, kf in f.locked, { f = f.copy(locked = if (kf in f.locked) f.locked - kf else f.locked + kf) }) }
            }
            val alle = SPERRBAR.all { it in f.locked }
            Spacer(Modifier.height(12.dp))
            Zeile("Metadaten sperren", icon = AI.SCHLOSS, schalter = alle, onClick = { f = f.copy(locked = if (alle) emptyList() else SPERRBAR) })
        }
        val quelle = d.meta?.source?.takeIf { it.isNotEmpty() }?.let { "Quelle: ${it.uppercase()}" + (zeit(d.added)?.let { _ -> " · " + datum(d.added, false) } ?: "") + " · " } ?: ""
        Leise(quelle + "Änderungen speichert Flimmer in der Datenbank.", Modifier.padding(top = 24.dp))
        Row(Modifier.padding(top = 12.dp).align(Alignment.End), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            if (fertig != null) Knopf("Abbrechen", fertig)
            AKnopf("Speichern", {
                laeuft = true
                a.los("Gespeichert", fehler = { if (fehlt(it)) "Der Server kann Metadaten noch nicht speichern." else fehlerText(it) },
                    danach = { onGespeichert(); fertig?.invoke() }) {
                    try { api.metaSpeichern(d.id, f.aenderung()) } finally { laeuft = false }
                }
            }, primary = true, aus = laeuft || f.title.isBlank())
        }
    }
}

/** Liste als Chips (Drücken entfernt) mit Eingabe zum Ergänzen (Enter oder +). */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun ListeFeld(label: String, werte: List<String>, setWerte: (List<String>) -> Unit) {
    var neu by remember { mutableStateOf("") }
    val hinzu = {
        val w = neu.trim()
        if (w.isNotEmpty() && w !in werte) setWerte(werte + w)
        neu = ""
    }
    Column(Modifier.padding(top = 16.dp)) {
        T(label, LocalTypo.current.klein, K.Text2, FontWeight.Medium, modifier = Modifier.padding(bottom = 6.dp))
        if (werte.isNotEmpty()) FlowRow(Modifier.padding(bottom = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            werte.forEach { w -> EntfernChip(w) { setWerte(werte - w) } }
        }
        FormZeile({ Eingabe(neu, { neu = it }, "Hinzufügen …", it, zeigeLabel = false, onEnter = hinzu) }) {
            AKnopf("Hinzufügen", hinzu, icon = AI.PLUS, aus = neu.isBlank())
        }
    }
}

@Composable
private fun EntfernChip(text: String, onClick: () -> Unit) {
    val tv = LocalTv.current
    Row(Modifier.height(if (tv) 56.dp else 36.dp).klick(onClick).semantics { contentDescription = "$text entfernen" }
        .border(1.dp, K.LinieStark, RoundedCornerShape(Tokens.Radius.RadiusS)).padding(horizontal = if (tv) 20.dp else 12.dp),
        verticalAlignment = Alignment.CenterVertically) {
        T(text, if (tv) LocalTypo.current.klein else 14.sp, K.Text, maxLines = 1)
        Spacer(Modifier.width(6.dp))
        Ico(Ic.SCHLIESSEN, if (tv) 24.dp else 16.dp, K.Text2)
    }
}

@Composable
private fun Personen(werte: List<Person>, setWerte: (List<Person>) -> Unit) {
    var name by remember { mutableStateOf("") }
    var rolle by remember { mutableStateOf("") }
    var art by remember { mutableStateOf("actor") }
    val hinzu = {
        if (name.isNotBlank()) {
            setWerte(werte + Person(name.trim(), rolle.trim().ifEmpty { null }, art))
            name = ""; rolle = ""
        }
    }
    Gruppe("Personen · ${werte.size}") {
        werte.forEachIndexed { i, p ->
            Zeile(p.name, icon = Ic.PROFIL, unter = artName(p.kind) + (p.role?.takeIf { it.isNotEmpty() }?.let { " · „$it“" } ?: ""),
                rechts = { IconKnopf(Ic.SCHLIESSEN, { setWerte(werte.filterIndexed { j, _ -> j != i }) }, rahmen = true) })
        }
        Paar({ Eingabe(name, { name = it }, "Name", it, zeigeLabel = false, onEnter = hinzu) },
            { Eingabe(rolle, { rolle = it }, "Rolle", it, zeigeLabel = false, onEnter = hinzu) })
        Spacer(Modifier.height(12.dp))
        Wahl(ARTEN, art) { art = it }
        AKnopf("Hinzufügen", hinzu, Modifier.padding(top = 12.dp), icon = AI.PLUS, aus = name.isBlank())
    }
}

@Composable
private fun Identifizieren(api: ApiClient, d: MDetails, fertig: () -> Unit) {
    val a = rememberAktion()
    var q by remember { mutableStateOf(d.meta?.title ?: d.title) }
    var liste by remember { mutableStateOf<List<Kandidat>?>(null) }
    val suche = { a.los { liste = api.liste("/api/items/${k(d.id)}/search?q=${k(q)}") } }
    LaunchedEffect(Unit) { suche() }
    Column {
        Leise("Wähle den passenden Treffer. Flimmer lädt danach Titel, Beschreibung und Bilder neu.", Modifier.padding(vertical = 12.dp))
        FormZeile({ Eingabe(q, { q = it }, "Suchen", it, zeigeLabel = false, onEnter = suche) }) { Knopf("Suchen", suche, icon = Ic.SUCHE) }
        Spacer(Modifier.height(12.dp))
        if (liste?.isEmpty() == true) Leise("Keine Treffer. Versuch einen anderen Suchbegriff.")
        liste.orEmpty().forEach { kd ->
            Zeile(kd.title + (kd.year?.let { " ($it)" } ?: ""), icon = Ic.HAKEN, wert = "TMDB ${kd.tmdbId}", onClick = {
                a.los("Neu zugeordnet", danach = fertig) { api.tu("POST", "/api/items/${k(d.id)}/identify", obj { put("tmdbId", kd.tmdbId) }) }
            })
        }
    }
}
