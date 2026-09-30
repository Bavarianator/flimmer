package io.flimmer.app.admin

import android.content.Context
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import io.flimmer.app.ApiClient
import io.flimmer.app.Profile
import io.flimmer.app.deviceProfile
import io.flimmer.app.ui.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.add
import kotlinx.serialization.json.putJsonArray

// ---------- Bibliotheken ----------

/** Medienordner (hinzufügen per Ordner-Browser, entfernen) und neu einlesen. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun Bibliotheken(api: ApiClient) {
    val s = laden(api) { api.einstellungen() }
    val a = rememberAktion()
    var status by remember { mutableStateOf<ScanStatus?>(null) }
    var ordner by remember { mutableStateOf<Ordner?>(null) }
    var anstoss by remember { mutableIntStateOf(0) }
    // Scan-Stand nachfragen, solange eingelesen wird.
    LaunchedEffect(s.daten, anstoss) {
        if (anstoss > 0) delay(1500)
        while (true) {
            val st = runCatching { api.hol<ScanStatus>("/api/status") }.getOrNull() ?: break
            status = st
            if (!st.scanning) break
            delay(1500)
        }
    }
    fun blaettern(p: String) = a.los { ordner = api.hol("/api/setup/dirs?path=${k(p)}") }
    fun gestartet() { status = ScanStatus(true, status?.found ?: 0); anstoss++ }
    fun setDirs(dirs: List<String>) = a.los("Gespeichert", danach = { s.neu(); gestartet() }) {
        api.einstellungenSetzen { putJsonArray("dirs") { dirs.forEach { add(it) } } }
    }
    Seite {
        Geladen(s) { d ->
            Leise("Ordner, aus denen Flimmer Medien einliest. Filme und Serien erkennt Flimmer am Ordner- und Dateinamen.", Modifier.padding(top = 16.dp))
            Gruppe("Medienordner") {
                if (d.dirs.isEmpty()) Leise("Noch kein Ordner. Füge unten einen hinzu.", Modifier.padding(bottom = 12.dp))
                d.dirs.forEach { p ->
                    Zeile(ordnerName(p), icon = AI.BIBLIOTHEK, unter = p, rechts = { Knopf("Entfernen", { setDirs(d.dirs - p) }) })
                }
                val o = ordner
                if (o == null) Knopf("Ordner hinzufügen", { blaettern("") }, Modifier.padding(top = 12.dp), icon = AI.PLUS)
                else Column(Modifier.padding(top = 12.dp).fillMaxWidth().background(K.Flaeche2, RoundedCornerShape(Tokens.Radius.RadiusM)).padding(12.dp)) {
                    T(o.path.ifEmpty { "…" }, LocalTypo.current.klein, K.Text, family = Mono)
                    FlowRow(Modifier.padding(vertical = 12.dp), horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        if (o.path.isNotEmpty()) {
                            Knopf("Diesen Ordner nehmen", { ordner = null; setDirs(d.dirs + o.path) }, primary = true)
                            Knopf("Eine Ebene höher", { blaettern(o.parent) }, icon = Ic.HOCH)
                        }
                        Knopf("Abbrechen", { ordner = null })
                    }
                    o.dirs.forEach { x -> Zeile(x.name, icon = Ic.WEITER, unter = if (o.path.isEmpty()) x.path else null, onClick = { blaettern(x.path) }) }
                }
            }
            Gruppe {
                Zeile("Jetzt neu einlesen", icon = Ic.NEUSTART,
                    unter = status?.let { if (it.scanning) "Liest ein … ${it.found} Titel gefunden" else "${it.found} Titel in der Bibliothek" },
                    onClick = { a.los(danach = ::gestartet) { api.tu("POST", "/api/rescan") } })
            }
        }
    }
}

// ---------- Metadaten-Manager ----------

private const val GRENZE = 200

/** /api/library mit dem Geräteprofil; große Listen außerhalb des UI-Threads lesen. */
private suspend fun bibliothek(api: ApiClient, ctx: Context): List<MItem> {
    val text = api.roh("POST", "/api/library", json.encodeToString(Profile.serializer(), deviceProfile(ctx)))
    return withContext(Dispatchers.Default) { json.decodeFromString<List<MItem>?>(text) ?: emptyList() }
}

/** Titel suchen und filtern (ohne Poster, unsicher erkannt). Breit: Liste links, Editor rechts; sonst Editor als Dialog. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun MetadatenManager(api: ApiClient) {
    val ctx = LocalContext.current
    val a = rememberAktion()
    val bib = laden(api) { bibliothek(api, ctx) }
    val rev = laden(api) { api.liste<Unsicher>("/api/settings/review") }
    var q by remember { mutableStateOf("") }
    var ohnePoster by remember { mutableStateOf(false) }
    var nurUnsicher by remember { mutableStateOf(false) }
    var art by remember { mutableStateOf("") } // "" | film | serie
    var wahl by remember { mutableStateOf<Pair<String, Boolean>?>(null) } // ID, gleich „Identifizieren“
    val unsichere = rev.daten.orEmpty().map { it.id }.toSet()
    val neuLaden = { bib.neu(); rev.neu() }
    val tv = LocalTv.current
    BoxWithConstraints(Modifier.fillMaxSize()) {
        val inline = !tv && maxWidth >= 900.dp
        Seite {
            if (unsichere.isNotEmpty() && !nurUnsicher) Hinweis("${unsichere.size} Titel wurden unsicher zugeordnet. Bitte prüfen.", icon = AI.FEHLER, farbe = K.AmpelGelb)
            Spacer(Modifier.height(16.dp))
            FormZeile({ Eingabe(q, { q = it }, "Titel suchen …", it, zeigeLabel = false) }) {
                Knopf("Metadaten aktualisieren", { a.los("Läuft im Hintergrund. Den Stand zeigt „Geplante Aufgaben“.") { api.aufgabeStarten("meta") } }, icon = Ic.NEUSTART)
            }
            Leise("Holt Personen, Studios, Leitsatz, Länder und Filmreihen für alle Titel nach. Gesperrte Felder bleiben unberührt.", Modifier.padding(vertical = 8.dp))
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Chip("Filme", art == "film", { art = if (art == "film") "" else "film" })
                Chip("Serien", art == "serie", { art = if (art == "serie") "" else "serie" })
                Chip("Nur ohne Poster", ohnePoster, { ohnePoster = !ohnePoster })
                Chip("Nur unsichere Treffer (${unsichere.size})", nurUnsicher, { nurUnsicher = !nurUnsicher })
            }
            Spacer(Modifier.height(16.dp))
            Geladen(bib) { items ->
                val such = q.trim().lowercase()
                val treffer = remember(items, such, art, ohnePoster, nurUnsicher, unsichere) {
                    items.filter {
                        (art.isEmpty() || (art == "serie") == (it.series != null)) && (!ohnePoster || it.poster.isEmpty()) &&
                            (!nurUnsicher || it.id in unsichere || it.meta?.uncertain == true) && (such.isEmpty() || such in it.name.lowercase())
                    }
                }
                val liste: @Composable ColumnScope.() -> Unit = {
                    if (treffer.isEmpty()) Leise("Nichts gefunden.")
                    treffer.take(GRENZE).forEach { x ->
                        val u = x.id in unsichere || x.meta?.uncertain == true
                        Zeile(x.name, icon = if (u) AI.FEHLER else if (x.series != null) Ic.SERIE else Ic.FILM, farbe = if (u) K.AmpelGelb else K.Text,
                            an = wahl?.first == x.id, onClick = { wahl = x.id to u })
                    }
                    if (treffer.size > GRENZE) Leise("… ${treffer.size - GRENZE} weitere Titel. Grenze die Suche ein.", Modifier.padding(top = 8.dp))
                }
                if (inline) Row(horizontalArrangement = Arrangement.spacedBy(32.dp)) {
                    Column(Modifier.weight(0.42f), content = liste)
                    Column(Modifier.weight(0.58f)) {
                        val w = wahl
                        if (w == null) Leise("Wähle links einen Titel, um seine Metadaten zu bearbeiten.")
                        else key(w.first) { Editor(api, w.first, if (w.second) "ident" else "meta", onGespeichert = neuLaden) }
                    }
                } else Column(content = liste)
            }
        }
        if (!inline) wahl?.let { w -> EditorDialog(api, w.first, if (w.second) "ident" else "meta", onZu = { wahl = null; neuLaden() }) }
    }
}
