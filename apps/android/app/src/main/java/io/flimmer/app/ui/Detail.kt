package io.flimmer.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.layout
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil3.compose.AsyncImage
import io.flimmer.app.Chapter
import io.flimmer.app.Details
import io.flimmer.app.Downloads
import io.flimmer.app.Item
import io.flimmer.app.Person
import io.flimmer.app.SeriesInfo
import io.flimmer.app.Stream
import io.flimmer.app.mehrWieDieses
import io.flimmer.app.szenenName
import kotlinx.coroutines.delay

/** Gewählte Ton-/Untertitelspur für die nächste Wiedergabe: Position unter den Spuren der Datei, -1 = Untertitel aus. */
data class Spur(val ton: Int? = null, val ut: Int? = null)

private class Aktion(val label: String, val icon: String, val an: Boolean = false, val voll: Boolean = false, val onClick: () -> Unit)

/** Zieht ein Element um [dp] nach oben, ohne Platz zu lassen (negativer Rand wie im Entwurf). */
private fun Modifier.hoch(dp: Dp) = layout { m, c ->
    val p = m.measure(c)
    val y = dp.roundToPx()
    layout(p.width, maxOf(0, p.height - y)) { p.place(0, -y) }
}

/** Hintergrundbild mit den drei Verläufen aus dem Entwurf (links dunkel, unten Saal, oben für die Kopfzeile). */
@Composable
private fun Hintergrund(bild: String, farbe: String, modifier: Modifier) {
    Box(modifier) {
        Art(bild, "", Modifier.fillMaxSize(), ton(farbe))
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(listOf(K.Saal.copy(alpha = 0.92f), K.Saal.copy(alpha = 0.55f), K.Saal.copy(alpha = 0.15f)))))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.6f to Color.Transparent, 1f to K.Saal)))
        Box(Modifier.fillMaxWidth().height(96.dp).background(Brush.verticalGradient(listOf(K.Saal.copy(alpha = 0.7f), Color.Transparent))))
    }
}

/** Kopfzeile über dem Bild: Zurück, Cast, Mehr. */
@Composable
private fun DetailKopfzeile(h: Handlungen, onMehr: () -> Unit, modifier: Modifier = Modifier) {
    Row(modifier.fillMaxWidth().statusBarsPadding().height(if (LocalBreit.current) 64.dp else 56.dp).padding(horizontal = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        IconKnopf(Ic.ZURUECK, h.zurueck)
        Spacer(Modifier.weight(1f))
        h.cast?.let { IconKnopf(Ic.CAST, it) }
        IconKnopf(Ic.MEHR_V, onMehr)
    }
}

/** Großer Abspielknopf mit Fortschritt am unteren Rand. */
@Composable
private fun SpielKnopf(text: String, anteil: Float, onClick: () -> Unit, modifier: Modifier = Modifier) {
    val tv = LocalTv.current
    Box(modifier.height(if (tv) 72.dp else if (LocalBreit.current) 48.dp else 52.dp).klick(onClick).background(K.Marke)) {
        Row(Modifier.align(Alignment.Center).padding(horizontal = if (tv) 40.dp else 24.dp), verticalAlignment = Alignment.CenterVertically) {
            Ico(Ic.ABSPIELEN, if (tv) 32.dp else 24.dp, K.AufMarke, voll = true)
            T(text, if (tv) LocalTypo.current.text else 16.sp, K.AufMarke, FontWeight.SemiBold, maxLines = 1, modifier = Modifier.padding(start = if (tv) 16.dp else 10.dp))
        }
        if (anteil > 0f) Fortschritt(anteil, Modifier.align(Alignment.BottomStart), if (tv) 5.dp else 4.dp, K.AufMarke, K.AufMarke.copy(alpha = 0.22f))
    }
}

/** Handy: Icon mit Beschriftung, gleich breit verteilt. Tablet/TV: umrandete Icon-Knöpfe. */
@Composable
private fun AktionsLeiste(liste: List<Aktion>, modifier: Modifier = Modifier) {
    if (LocalTv.current || LocalBreit.current) {
        Row(modifier, horizontalArrangement = Arrangement.spacedBy(if (LocalTv.current) 16.dp else 8.dp)) {
            liste.forEach { a -> IconKnopf(a.icon, a.onClick, rahmen = true, voll = a.voll, farbe = K.Text) }
        }
        return
    }
    Row(modifier.fillMaxWidth()) {
        liste.forEach { a ->
            Column(Modifier.weight(1f).height(56.dp).klick(a.onClick, skala = false), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
                Ico(a.icon, 24.dp, if (a.an) K.Text else K.Text2, voll = a.voll)
                T(a.label, 12.sp, if (a.an) K.Text else K.Text2, maxLines = 1, modifier = Modifier.padding(top = 2.dp))
            }
        }
    }
}

private fun dlAktion(it: List<Item>, h: Handlungen, label: String = "Herunterladen"): Aktion? {
    val dl = h.download ?: return null
    val st = it.map { e -> h.dlStatus(e.id) }
    return when {
        st.all { s -> s == Downloads.Status.Fertig } -> Aktion("Offline", Ic.HAKEN, an = true) { h.bereich(Bereich.Downloads) }
        st.none { s -> s == null } -> Aktion("Lädt …", Ic.DOWNLOAD, an = true) { h.bereich(Bereich.Downloads) }
        else -> Aktion(label, Ic.DOWNLOAD) { dl(it.filter { e -> h.dlStatus(e.id) == null }) }
    }
}

/** Überschrift der Detailseite: Poster links, Plakattitel und Fakten daneben (Handy). */
@Composable
private fun HandyKopf(titel: String, poster: String, mono: String, farbe: String, zeile: String, ueberBild: Boolean = true, fakten: @Composable () -> Unit) {
    Row((if (ueberBild) Modifier.hoch(90.dp) else Modifier.padding(top = 8.dp)).padding(horizontal = 16.dp), verticalAlignment = Alignment.Bottom) {
        Box(Modifier.size(120.dp, 180.dp).clip(RoundedCornerShape(Tokens.Radius.RadiusS)).background(ton(farbe))
            .border(1.dp, K.Text.copy(alpha = 0.2f), RoundedCornerShape(Tokens.Radius.RadiusS))) {
            if (poster.isNotEmpty()) AsyncImage(poster, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
            else Column(Modifier.fillMaxSize().padding(10.dp), verticalArrangement = Arrangement.SpaceBetween) {
                PlakatTitel(titel, 22.sp, maxLines = 5)
                T(mono.uppercase(), 10.sp, K.Text2, family = Mono)
            }
        }
        Column(Modifier.padding(start = 16.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            PlakatTitel(titel, if (titel.length > 14) 32.sp else 44.sp, maxLines = 3)
            if (zeile.isNotEmpty()) T(zeile, 13.sp, K.Text2)
            fakten()
        }
    }
}

@Composable
private fun FskStern(age: Int?, rating: Double?, rest: String?) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        age?.let { Fsk(it) }
        rating?.takeIf { it > 0 }?.let { r ->
            Row(verticalAlignment = Alignment.CenterVertically) {
                Ico(Ic.STERN, if (LocalTv.current) 22.dp else 14.dp, K.Text2, voll = true)
                T("%.1f".format(java.util.Locale.GERMANY, r), LocalTypo.current.klein, K.Text2, modifier = Modifier.padding(start = 3.dp))
            }
        }
        rest?.let { T(it, LocalTypo.current.klein, K.Text2) }
    }
}

/** Inhalt mit „Mehr anzeigen“. */
@Composable
private fun Inhalt(tagline: String?, text: String?, zeilen: Int) {
    var offen by remember { mutableStateOf(false) }
    if (!tagline.isNullOrBlank()) T("„$tagline“", LocalTypo.current.text, K.Text, italic = true, modifier = Modifier.padding(top = 20.dp))
    if (text.isNullOrBlank()) return
    T(text, LocalTypo.current.text, K.Text, maxLines = if (offen) Int.MAX_VALUE else zeilen, lineHeight = LocalTypo.current.text * 1.5f,
        modifier = Modifier.padding(top = if (tagline.isNullOrBlank()) 16.dp else 8.dp))
    if (!LocalTv.current && text.length > 160) T(if (offen) "Weniger anzeigen" else "Mehr anzeigen", 15.sp, K.Text, FontWeight.Medium,
        modifier = Modifier.height(44.dp).klick({ offen = !offen }, skala = false).wrapContentHeight(Alignment.CenterVertically))
}

/** Zeile „GENRES   Krimi, Drama“; mit [onClick] unterstrichen (Person). */
@Composable
private fun MetaZeile(label: String, wert: String, onClick: (() -> Unit)? = null) {
    Row(Modifier.heightIn(min = if (LocalTv.current) 48.dp else 32.dp), verticalAlignment = Alignment.CenterVertically) {
        T(label.uppercase(), if (LocalTv.current) 22.sp else 12.sp, K.Text3, family = Mono, spacing = 1.5.sp, modifier = Modifier.width(if (LocalTv.current) 200.dp else 88.dp))
        T(wert, if (LocalTv.current) LocalTypo.current.klein else 15.sp, K.Text, maxLines = 2,
            modifier = if (onClick != null) Modifier.klick(onClick, skala = false) else Modifier)
    }
}

private fun personen(p: List<Person>, kind: String) = p.filter { it.kind == kind }.joinToString(", ") { it.name }

/** Auswahl Ton/Untertitel wie im Entwurf (Handy: Liste, Tablet: 2 Spalten). Öffnet ein Blatt mit den Spuren. */
@Composable
private fun Spurwahl(det: Details, spur: Spur, onSpur: (Spur) -> Unit) {
    var offen by remember { mutableStateOf<String?>(null) }
    fun name(s: Stream) = listOfNotNull(s.title ?: s.lang?.let(::sprache), s.codec.uppercase().ifEmpty { null },
        s.channels?.let { c -> if (c >= 6) "${c - 1}.1" else if (c == 2) "Stereo" else "Mono" }).joinToString(" · ")
    val tonText = det.audio.getOrNull(spur.ton ?: det.audio.indexOfFirst { it.default }.coerceAtLeast(0))?.let(::name) ?: "Standard"
    val utText = when (val u = spur.ut) {
        null -> det.subs.firstOrNull { it.default || it.forced }?.let(::name) ?: "Aus"
        -1 -> "Aus"
        else -> det.subs.getOrNull(u)?.let(::name) ?: "Aus"
    }
    Column(Modifier.padding(top = 12.dp).fillMaxWidth().background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusM))) {
        det.video?.let { v -> SpurZeile(Ic.SAMMLUNG, "Video", listOfNotNull(if (v.height > 0) "${v.height}p" else null, v.codec.uppercase(), v.hdr?.uppercase()).joinToString(" · "), null, false) }
        if (det.audio.isNotEmpty()) SpurZeile(Ic.TON, "Ton", tonText, { offen = "ton" }, det.video != null)
        SpurZeile(Ic.UNTERTITEL, "Untertitel", utText, if (det.subs.isEmpty()) null else ({ offen = "ut" }), true)
    }
    when (offen) {
        "ton" -> Blatt({ offen = null }, { BlattKopf("Tonspur", "", "", "") }) {
            det.audio.forEachIndexed { i, s -> MenueZeile(Ic.TON, name(s), an = i == (spur.ton ?: det.audio.indexOfFirst { it.default }.coerceAtLeast(0))) { onSpur(spur.copy(ton = i)); offen = null } }
        }
        "ut" -> Blatt({ offen = null }, { BlattKopf("Untertitel", "", "", "") }) {
            MenueZeile(Ic.SCHLIESSEN, "Aus", an = spur.ut == -1) { onSpur(spur.copy(ut = -1)); offen = null }
            det.subs.forEachIndexed { i, s -> MenueZeile(Ic.UNTERTITEL, name(s) + if (s.forced) " (erzwungen)" else "", an = spur.ut == i) { onSpur(spur.copy(ut = i)); offen = null } }
        }
    }
}

@Composable
private fun SpurZeile(icon: String, label: String, wert: String, onClick: (() -> Unit)?, linie: Boolean) {
    if (linie) Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
    Row(Modifier.fillMaxWidth().height(if (LocalTv.current) 88.dp else 56.dp).then(if (onClick != null) Modifier.klick(onClick, skala = false) else Modifier)
        .padding(start = 16.dp, end = 12.dp), verticalAlignment = Alignment.CenterVertically) {
        Ico(icon, 20.dp, K.Text2)
        Column(Modifier.weight(1f).padding(start = 12.dp)) {
            T(label.uppercase(), if (LocalTv.current) 22.sp else 11.sp, K.Text3, family = Mono, spacing = 1.5.sp)
            T(wert, if (LocalTv.current) LocalTypo.current.klein else 15.sp, K.Text, maxLines = 1)
        }
        if (onClick != null) Ico(Ic.WEITER, 20.dp, K.Text2)
    }
}

/** ISO-639 → deutscher Sprachname (die häufigen), sonst der Code. */
fun sprache(code: String): String = when (code.lowercase().take(3)) {
    "de", "deu", "ger" -> "Deutsch"; "en", "eng" -> "Englisch"; "fr", "fra", "fre" -> "Französisch"; "es", "spa" -> "Spanisch"
    "it", "ita" -> "Italienisch"; "ja", "jpn" -> "Japanisch"; "nl", "nld", "dut" -> "Niederländisch"; "und" -> "Unbekannt"
    else -> code
}

private val artLabel = mapOf("director" to "Regie", "writer" to "Drehbuch", "composer" to "Musik", "producer" to "Produktion")

/** Besetzung & Mitwirkende: Fotos (ohne Foto Initialen), ein Tipp öffnet die Personenseite. */
private fun LazyListScope.besetzung(people: List<Person>, d: Daten, h: Handlungen, titel: String) {
    if (people.isEmpty()) return
    item(key = "besetzung") {
        Reihe(titel, luecke = if (LocalTv.current) 24.dp else 12.dp) {
            items(people.take(40)) { p ->
                PersonKarte(p.name, artLabel[p.kind] ?: p.role.orEmpty(), p.image?.let(d.abs) ?: "") { h.person?.invoke(p.name) }
            }
        }
    }
}

/** Szenen mit Vorschaubild; Allerweltsnamen werden „Szene N“. Tippen bzw. OK startet dort. */
private fun LazyListScope.szenen(it: Item, kapitel: List<Chapter>, d: Daten, h: Handlungen) {
    if (kapitel.size <= 1) return
    item(key = "szenen") {
        val w = if (LocalTv.current) 416.dp else if (LocalBreit.current) 256.dp else 176.dp
        Reihe("Szenen") {
            items(kapitel.size) { i ->
                val k = kapitel[i]
                val name = szenenName(k.name, i + 1)
                BreitKarte(name, "", k.image?.let(d.abs) ?: "", it.color, { h.abspielen(it, k.start) }, Modifier.width(w), plakat = name, rest = fmtTime(k.start))
            }
        }
    }
}

/** „Mehr wie dieses“ (Auswahl in mehrWieDieses(), mit Rückfall bis „zuletzt hinzugefügt“). */
private fun LazyListScope.mehrWie(liste: List<Item>, d: Daten, h: Handlungen) {
    if (liste.isEmpty()) return
    item(key = "mehr") {
        Reihe("Mehr wie dieses") { items(liste, key = { "mw" + it.id }) { Kachel(it, d, h, Modifier.width(posterBreite)) } }
    }
}

/** Fakten: Genres, Regie, Drehbuch, Musik, Studio/Sender, Land, Tags. Leere Felder fallen weg, ganz leer gibt es keinen Block. */
@Composable
private fun FaktenBlock(genres: List<String>, people: List<Person>, studios: List<String>, countries: List<String>, tags: List<String>, serie: Boolean,
                        h: Handlungen, modifier: Modifier = Modifier) {
    val zeilen = listOf(
        "Genres" to genres.joinToString(", "),
        "Regie" to personen(people, "director"),
        "Drehbuch" to personen(people, "writer"),
        "Musik" to personen(people, "composer"),
        (if (serie) "Sender" else "Studio") to studios.joinToString(", "),
        "Land" to countries.joinToString(", "),
        "Tags" to tags.joinToString(", "),
    ).filter { it.second.isNotBlank() }
    if (zeilen.isEmpty()) return
    val art = artLabel.entries.associate { (k, v) -> v to k }
    Column(modifier) {
        zeilen.forEach { (label, wert) ->
            val erste = art[label]?.let { k -> people.firstOrNull { it.kind == k } }
            MetaZeile(label, wert, if (erste != null && h.person != null) ({ h.person.invoke(erste.name) }) else null)
        }
    }
}

private fun technik(det: Details): String = listOfNotNull(
    det.container.uppercase().ifEmpty { null },
    det.video?.codec?.uppercase(),
    det.video?.takeIf { it.width > 0 }?.let { "${it.width}×${it.height}" },
    det.audio.firstOrNull()?.let { a -> a.codec.uppercase() + (a.channels?.let { c -> if (c >= 6) " ${c - 1}.1" else " $c.0" } ?: "") },
    if (det.subs.isNotEmpty()) "${det.subs.size} Untertitel" else null,
).joinToString(" · ").uppercase()

// ---------- Film ----------

@Composable
fun FilmDetail(it: Item, det: Details?, d: Daten, h: Handlungen, spur: Spur, onSpur: (Spur) -> Unit) {
    val tv = LocalTv.current
    val breit = LocalBreit.current
    val fav = it.key in d.favs
    val resume = it.progress > 5 && !it.watched
    val spielText = if (resume) "Fortsetzen ab ${fmtTime(it.progress)}" else "Abspielen"
    val play = remember { FocusRequester() }
    val aktionen = listOfNotNull(
        if (resume) Aktion("Von vorn", Ic.NEUSTART) { h.abspielen(it, 0.0) } else null,
        Aktion("Gemeinsam", Ic.GEMEINSAM) { h.party(it) },
        dlAktion(listOf(it), h),
        Aktion("Gesehen", Ic.HAKEN, an = it.watched) { h.gesehen(listOf(it), !it.watched) },
        Aktion("Favorit", Ic.HERZ, an = fav, voll = fav) { h.favorit(it.key) },
        Aktion("Mehr", Ic.MEHR) { h.mehr(it) },
    )
    val genres = it.meta?.genres.orEmpty()
    val people = det?.people.orEmpty()
    val regie = personen(people, "director")
    val mitBild = it.backdrop.isNotEmpty() // ohne Bild ein kompakter Kopf statt einer großen leeren Fläche
    val fakten: @Composable (Modifier) -> Unit = { m -> FaktenBlock(genres, people, det?.studios.orEmpty(), det?.countries.orEmpty(), det?.tags.orEmpty(), false, h, m) }
    LaunchedEffect(it.id) { if (tv) { delay(80); runCatching { play.requestFocus() } } }

    LazyColumn(Modifier.fillMaxSize().background(K.Saal), contentPadding = PaddingValues(bottom = 48.dp)) {
        item(key = "kopf") {
            when {
                tv -> Box(Modifier.fillMaxWidth().then(if (mitBild) Modifier.height(860.dp) else Modifier)) {
                    Hintergrund(d.abs(it.backdrop), it.color, Modifier.matchParentSize())
                    Column(Modifier.padding(start = 96.dp, top = if (mitBild) 120.dp else 72.dp, end = 96.dp, bottom = 24.dp)) {
                        Label((listOf("Film") + genres.take(2).joinToString(", ").ifEmpty { null }).filterNotNull().joinToString(" · "), color = K.Text2)
                        PlakatTitel(it.displayTitle, 144.sp, Modifier.padding(top = 8.dp))
                        Fakten(it, null, Modifier.padding(top = 16.dp))
                        Box(Modifier.width(1280.dp)) { Inhalt(null, it.meta?.overview, 3) }
                        if (regie.isNotEmpty() || people.isNotEmpty()) T(listOfNotNull(regie.ifEmpty { null }?.let { r -> "Regie $r" },
                            personen(people.filter { p -> p.kind == "actor" }.take(3), "actor").ifEmpty { null }?.let { a -> "Mit $a" }).joinToString(" · "), LocalTypo.current.text, K.Text2, maxLines = 1,
                            modifier = Modifier.padding(top = 12.dp))
                        Row(Modifier.padding(top = 40.dp), verticalAlignment = Alignment.CenterVertically) {
                            SpielKnopf(spielText, it.anteil(), { h.abspielen(it, null) }, Modifier.focusRequester(play).padding(end = 24.dp))
                            AktionsLeiste(aktionen)
                        }
                        det?.let { x -> T(technik(x), LocalTypo.current.klein, K.Text2, modifier = Modifier.padding(top = 20.dp)) }
                    }
                }
                breit -> Box(Modifier.fillMaxWidth().heightIn(min = if (mitBild) 560.dp else 0.dp)) {
                    Hintergrund(d.abs(it.backdrop), it.color, Modifier.matchParentSize())
                    Row(Modifier.padding(start = 24.dp, end = 24.dp, top = 96.dp, bottom = 32.dp)) {
                        Column(Modifier.weight(0.62f)) {
                            Label((listOf("Film") + genres.take(2).joinToString(", ").ifEmpty { null }).filterNotNull().joinToString(" · "), color = K.Text2)
                            PlakatTitel(it.displayTitle, 72.sp, Modifier.padding(top = 8.dp, bottom = 12.dp))
                            Fakten(it, null)
                            Row(Modifier.padding(top = 20.dp), verticalAlignment = Alignment.CenterVertically) {
                                SpielKnopf(spielText, it.anteil(), { h.abspielen(it, null) }, Modifier.padding(end = 12.dp))
                                AktionsLeiste(aktionen)
                            }
                            det?.let { x -> Spurwahl(x, spur, onSpur) }
                            Inhalt(det?.tagline, it.meta?.overview, 3)
                        }
                        fakten(Modifier.weight(0.38f).padding(start = 40.dp))
                    }
                    DetailKopfzeile(h, { h.mehr(it) })
                }
                else -> Column {
                    if (mitBild) Box(Modifier.fillMaxWidth().height(300.dp)) {
                        Hintergrund(d.abs(it.backdrop), it.color, Modifier.fillMaxSize())
                        DetailKopfzeile(h, { h.mehr(it) })
                    } else DetailKopfzeile(h, { h.mehr(it) })
                    HandyKopf(it.displayTitle, d.abs(it.poster), monoFilm(it), it.color, listOfNotNull(jahr(it).ifEmpty { null }, if (it.duration > 0) fmtDauer(it.duration) else null).joinToString(" · "), mitBild) {
                        FskStern(it.meta?.age, it.meta?.rating, null)
                        Ampel(it.light)
                    }
                    Column(Modifier.padding(horizontal = 16.dp)) {
                        SpielKnopf(spielText, it.anteil(), { h.abspielen(it, null) }, Modifier.fillMaxWidth().padding(top = 16.dp))
                        if (it.duration > 0) T(listOfNotNull(if (resume) "Noch ${fmtDauer(it.duration - it.progress)}" else null,
                            "endet um ${endetUm(it.duration - if (resume) it.progress else 0.0)}").joinToString(" · "), 13.sp, K.Text2, modifier = Modifier.padding(top = 8.dp))
                        AktionsLeiste(aktionen, Modifier.padding(top = 8.dp))
                        det?.let { x -> Spurwahl(x, spur, onSpur) }
                        Inhalt(det?.tagline, it.meta?.overview, 4)
                        fakten(Modifier.padding(top = 12.dp))
                    }
                }
            }
        }
        if (tv) item(key = "fakten") { fakten(Modifier.padding(horizontal = 96.dp).padding(top = 16.dp)) }
        szenen(it, det?.chapters.orEmpty(), d, h)
        besetzung(people, d, h, "Besetzung & Mitwirkende")
        mehrWie(mehrWieDieses(genres, it.meta?.year ?: it.year, it.key, d.filme.filter { f -> f.id != it.id }, d.sammlungen), d, h)
        if (!tv) item(key = "links") {
            Column(Modifier.padding(horizontal = Pad).padding(top = 24.dp)) {
                Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    it.meta?.imdbId?.let { id -> Knopf("IMDb", { h.offenerLink("https://www.imdb.com/title/$id/") }) }
                    it.meta?.tmdbId?.let { id -> Knopf("TMDB", { h.offenerLink("https://www.themoviedb.org/movie/$id") }) }
                }
                det?.let { x -> T(technik(x), 11.sp, K.Text3, family = Mono, spacing = 1.sp, modifier = Modifier.padding(top = 12.dp)) }
            }
        }
    }
}

// ---------- Serie ----------

@Composable
fun SerienDetail(name: String, eps: List<Item>, info: SeriesInfo?, d: Daten, h: Handlungen, staffelStart: Int?) {
    val tv = LocalTv.current
    val breit = LocalBreit.current
    val next = naechsteFolge(eps) ?: return
    val key = "serie:$name"
    val fav = key in d.favs
    val staffeln = eps.groupBy { it.season ?: 0 }.toSortedMap(compareBy { if (it == 0) Int.MAX_VALUE else it })
    var staffel by remember(name) { mutableIntStateOf(staffelStart ?: next.season ?: staffeln.keys.first()) }
    val allesGesehen = eps.all { it.watched }
    val genres = info?.genres?.ifEmpty { null } ?: next.meta?.genres.orEmpty()
    val spielText = when {
        next.progress > 5 && !next.watched -> "${folge(next)} fortsetzen"
        eps.any { it.watched } -> "${folge(next)} abspielen"
        else -> "Abspielen"
    }
    val play = remember { FocusRequester() }
    val aktionen = listOfNotNull(
        Aktion("Zufällig", Ic.ZUFALL) { h.alle(eps.shuffled()) },
        Aktion("Gemeinsam", Ic.GEMEINSAM) { h.party(next) },
        Aktion("Gesehen", Ic.HAKEN, an = allesGesehen) { h.gesehen(eps, !allesGesehen) },
        Aktion("Favorit", Ic.HERZ, an = fav, voll = fav) { h.favorit(key) },
        Aktion("Mehr", Ic.MEHR) { h.mehr(next) },
    )
    val jahr0 = info?.year ?: eps.mapNotNull { it.meta?.year ?: it.year }.minOrNull()
    val bis = info?.endYear
    val status = info?.status
    val zeile = listOfNotNull(
        jahr0?.let { j -> if (bis != null && bis != j) "$j – $bis" else if (status != null && status != "Ended") "$j – heute" else "$j" },
        staffeln.keys.count { it > 0 }.let { n -> if (n == 1) "1 Staffel" else "$n Staffeln" },
    ).joinToString(" · ")
    val ueberblick = info?.overview
    val bild = info?.backdrop ?: next.backdrop // Serienbild vor dem Standbild der Folge
    val farbe = info?.color ?: next.color
    val mitBild = bild.isNotEmpty()
    val fakten: @Composable (Modifier) -> Unit = { m -> FaktenBlock(genres, info?.people.orEmpty(), info?.studios.orEmpty(), emptyList(), emptyList(), true, h, m) }
    LaunchedEffect(name) { if (tv) { delay(80); runCatching { play.requestFocus() } } }

    LazyColumn(Modifier.fillMaxSize().background(K.Saal), contentPadding = PaddingValues(bottom = 48.dp)) {
        item(key = "kopf") {
            when {
                tv || breit -> Box(Modifier.fillMaxWidth().heightIn(min = if (!mitBild) 0.dp else if (tv) 720.dp else 480.dp)) {
                    Hintergrund(d.abs(bild), farbe, Modifier.matchParentSize())
                    Column(Modifier.padding(start = if (tv) 96.dp else 24.dp, top = if (tv) (if (mitBild) 120.dp else 72.dp) else if (mitBild) 96.dp else 64.dp,
                        end = if (tv) 96.dp else 24.dp, bottom = 24.dp)
                        .widthIn(max = if (tv) 1400.dp else 760.dp)) {
                        Label((listOf("Serie") + genres.take(2).joinToString(", ").ifEmpty { null }).filterNotNull().joinToString(" · "), color = K.Text2)
                        PlakatTitel(info?.title ?: name, if (tv) 144.sp else 72.sp, Modifier.padding(top = 8.dp, bottom = 12.dp))
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                            T(zeile, LocalTypo.current.klein, K.Text2)
                            FskStern(info?.age ?: next.meta?.age, info?.rating ?: next.meta?.rating, null)
                            Ampel(next.light)
                        }
                        Box(Modifier.widthIn(max = 1280.dp)) { Inhalt(null, ueberblick, 3) }
                        Row(Modifier.padding(top = if (tv) 40.dp else 20.dp), verticalAlignment = Alignment.CenterVertically) {
                            SpielKnopf(spielText, next.anteil(), { h.abspielen(next, null) }, Modifier.focusRequester(play).padding(end = if (tv) 24.dp else 12.dp))
                            AktionsLeiste(aktionen)
                        }
                        T("${next.displayTitle}" + if (next.duration > 0) " · endet um ${endetUm(next.duration - next.progress)}" else "",
                            LocalTypo.current.klein, K.Text2, modifier = Modifier.padding(top = 8.dp))
                    }
                    if (!tv) DetailKopfzeile(h, { h.mehr(next) })
                }
                else -> Column {
                    if (mitBild) Box(Modifier.fillMaxWidth().height(300.dp)) {
                        Hintergrund(d.abs(bild), farbe, Modifier.fillMaxSize())
                        DetailKopfzeile(h, { h.mehr(next) })
                    } else DetailKopfzeile(h, { h.mehr(next) })
                    HandyKopf(info?.title ?: name, d.abs(info?.poster ?: eps.first().poster), monoSerie(eps), farbe, zeile, mitBild) {
                        FskStern(info?.age ?: next.meta?.age, info?.rating ?: next.meta?.rating, info?.status?.let { s -> if (s == "Ended") "Beendet" else "Fortlaufend" })
                        Ampel(next.light)
                    }
                    Column(Modifier.padding(horizontal = 16.dp)) {
                        SpielKnopf(spielText, next.anteil(), { h.abspielen(next, null) }, Modifier.fillMaxWidth().padding(top = 16.dp))
                        T(listOfNotNull(next.displayTitle, if (next.progress > 0 && !next.watched) "noch ${fmtDauer(next.duration - next.progress)}" else null,
                            if (next.duration > 0) "endet um ${endetUm(next.duration - next.progress)}" else null).joinToString(" · "), 13.sp, K.Text2,
                            modifier = Modifier.padding(top = 8.dp))
                        AktionsLeiste(aktionen, Modifier.padding(top = 8.dp))
                        Inhalt(null, ueberblick, 3)
                        fakten(Modifier.padding(top = 12.dp))
                    }
                }
            }
        }
        if (tv || breit) item(key = "fakten") { fakten(Modifier.padding(horizontal = if (tv) 96.dp else Pad).padding(top = 16.dp)) }
        item(key = "staffeln") {
            Row(Modifier.padding(top = 20.dp).horizontalScroll(rememberScrollState()).padding(horizontal = Pad), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                staffeln.keys.forEach { s -> Chip(if (s == 0) "Specials" else "Staffel $s", s == staffel, { staffel = s }) }
            }
        }
        val folgen = staffeln[staffel].orEmpty()
        item(key = "st-kopf-$staffel") {
            Row(Modifier.padding(start = Pad, end = Pad, top = 20.dp, bottom = 8.dp), verticalAlignment = Alignment.Bottom) {
                T(if (staffel == 0) "Specials" else "Staffel $staffel", LocalTypo.current.reihe, K.Text, FontWeight.SemiBold)
                T("${folgen.size} Folgen · ${fmtDauer(folgen.sumOf { it.duration })}", LocalTypo.current.klein, K.Text2, modifier = Modifier.padding(start = 10.dp))
                Spacer(Modifier.weight(1f))
                if (h.download != null) dlAktion(folgen, h, "Staffel laden")?.let { a -> Knopf(a.label, a.onClick, icon = a.icon) }
            }
        }
        items(folgen, key = { "e" + it.id }) { e -> FolgenZeile(e, d, h) }
        besetzung(info?.people.orEmpty(), d, h, "Besetzung")
        mehrWie(mehrWieDieses(genres, jahr0, key, d.serien.filterKeys { it != name }.values.map { it.first() }, d.sammlungen), d, h)
    }
}

/** Folge wie im Entwurf: Standbild mit Nummer, Titel, Dauer/Rest, Inhalt, Download-Knopf. */
@Composable
private fun FolgenZeile(e: Item, d: Daten, h: Handlungen) {
    val tv = LocalTv.current
    Column {
        Box(Modifier.padding(horizontal = Pad).fillMaxWidth().height(1.dp).background(K.Linie))
        Row(Modifier.padding(start = Pad, end = 4.dp).padding(vertical = 6.dp), verticalAlignment = Alignment.Top) {
            Row(Modifier.weight(1f).klick({ h.abspielen(e, null) }, { h.mehr(e) }, skala = false)) {
                Box(Modifier.size(if (tv) 256.dp else 128.dp, if (tv) 144.dp else 72.dp).clip(RoundedCornerShape(Tokens.Radius.RadiusS))) {
                    Art(d.abs(e.backdrop), "", Modifier.fillMaxSize(), ton(e.color))
                    T("E${e.episode ?: 0}", if (tv) 32.sp else 22.sp, K.Text.copy(alpha = 0.35f), family = Mono, modifier = Modifier.padding(start = 8.dp, top = 6.dp))
                    if (e.watched) Box(Modifier.align(Alignment.TopEnd).padding(6.dp).size(24.dp).background(K.BadgeGrund, RoundedCornerShape(2.dp)), contentAlignment = Alignment.Center) {
                        Ico(Ic.HAKEN, 16.dp, K.Text, strich = 2f)
                    }
                    if (e.anteil() > 0f) Fortschritt(e.anteil(), Modifier.align(Alignment.BottomStart))
                }
                Column(Modifier.padding(start = 12.dp).weight(1f)) {
                    T("${e.episode ?: 0}. ${e.displayTitle}", LocalTypo.current.karte, K.Text, FontWeight.SemiBold, maxLines = 1)
                    T(when {
                        e.watched -> "${fmtDauer(e.duration)} · Gesehen"
                        e.progress > 0 -> "Noch ${fmtDauer(e.duration - e.progress)} · endet um ${endetUm(e.duration - e.progress)}"
                        else -> "${fmtDauer(e.duration)} · endet um ${endetUm(e.duration)}"
                    }, LocalTypo.current.klein, K.Text2, maxLines = 1)
                    e.meta?.overview?.let { T(it, LocalTypo.current.klein, K.Text2, maxLines = 2) }
                }
            }
            if (h.download != null) {
                val st = h.dlStatus(e.id)
                Box(Modifier.size(44.dp).klick({ if (st == null) h.download.invoke(listOf(e)) else h.bereich(Bereich.Downloads) }), contentAlignment = Alignment.Center) {
                    Box(Modifier.size(32.dp).background(if (st != null) K.Flaeche3 else Color.Transparent, RoundedCornerShape(2.dp)), contentAlignment = Alignment.Center) {
                        Ico(if (st == Downloads.Status.Fertig) Ic.HAKEN else Ic.DOWNLOAD, 22.dp, if (st != null) K.Text else K.Text2)
                    }
                }
            }
        }
    }
}
