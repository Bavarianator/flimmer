package io.flimmer.app.gemeinsam

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import io.flimmer.app.ApiClient
import io.flimmer.app.Item
import io.flimmer.app.OffeneGruppe
import io.flimmer.app.deviceProfile
import io.flimmer.app.ui.*
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonPrimitive

// Lobby „Gemeinsam schauen“ wie in web/src/screens/Party.tsx: neue Gruppe mit einem Titel aus der Startseite
// (Weiterschauen, Als Nächstes, Neu) erstellen (POST /api/party), einer offenen Gruppe (GET /api/party) oder per Code/Link
// beitreten. Den Raum selbst hat Party.kt.

private val json = Json { ignoreUnknownKeys = true }

@Serializable
private data class Raum(val id: String = "")

/** Raum-IDs sind 12 Hex-Zeichen; aus Code („3F9A 0C21 B7E4“) oder Einladungslink. Leer, wenn keiner drinsteht. */
fun codeAus(s: String): String = Regex("[0-9a-f]{12}").find(s.lowercase().replace(Regex("[\\s-]"), ""))?.value ?: ""

@Composable
fun GemeinsamLobby(api: ApiClient, tv: Boolean, onRaum: (raumId: String) -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val typo = LocalTypo.current
    var titel by remember { mutableStateOf<List<Item>?>(null) }
    var code by remember { mutableStateOf("") }
    var fehler by remember { mutableStateOf("") }
    var offen by remember { mutableStateOf<List<OffeneGruppe>>(emptyList()) }
    var bibliothek by remember { mutableStateOf<Map<String, Item>>(emptyMap()) }
    LaunchedEffect(api) { // offene Gruppen der anderen, alle 15 s neu; die Bibliothek liefert Titel und Bild
        bibliothek = runCatching { api.library(deviceProfile(ctx)) }.getOrDefault(emptyList()).associateBy { it.id }
        while (true) {
            offen = runCatching { api.offeneGruppen() }.getOrDefault(offen)
            delay(15_000)
        }
    }
    val breite = if (tv) Tokens.Masse.KarteBreitTv else if (LocalBreit.current) Tokens.Masse.KarteBreitDt else Tokens.Masse.KarteBreitHd
    LaunchedEffect(api) {
        // Kandidaten: alle Reihen der Startseite, ohne Doppelte, höchstens 16
        titel = runCatching { api.home(deviceProfile(ctx)) }.getOrDefault(emptyList()).flatMap { it.items }.distinctBy { it.id }.take(16)
    }
    fun erstellen(it: Item) {
        fehler = ""
        scope.launch {
            runCatching { json.decodeFromString<Raum>(api.roh("POST", "/api/party", """{"mediaId":${JsonPrimitive(it.id)}}""")) }
                .onSuccess { r -> onRaum(r.id) }
                .onFailure { e -> fehler = "Gemeinsam schauen geht gerade nicht: ${e.message ?: "Server nicht erreichbar"}" }
        }
    }
    fun beitreten() {
        val c = codeAus(code)
        if (c.isEmpty()) fehler = "Das ist kein gültiger Gruppencode. Er hat 12 Zeichen, z. B. 3F9A 0C21 B7E4."
        else { fehler = ""; onRaum(c) }
    }

    Column(Modifier.fillMaxSize().background(K.Saal).verticalScroll(rememberScrollState()).padding(top = 16.dp, bottom = 48.dp)) {
        T("Alle sehen dasselbe – synchron auf jedem Gerät. Pausieren, Spulen und Reaktionen gelten für die ganze Gruppe.",
            typo.text, K.Text2, modifier = Modifier.padding(horizontal = Pad).widthIn(max = 760.dp))

        if (offen.isNotEmpty()) Column(Modifier.padding(top = 28.dp)) {
            T("Gemeinsam schauen – jetzt offen", typo.reihe, K.Text, FontWeight.SemiBold, maxLines = 1, modifier = Modifier.padding(horizontal = Pad))
            LazyRow(Modifier.padding(top = 12.dp), contentPadding = PaddingValues(horizontal = Pad), horizontalArrangement = Arrangement.spacedBy(Luecke)) {
                items(offen, key = { it.id }) { g ->
                    val x = bibliothek[g.mediaId]
                    BreitKarte(
                        titel = x?.let { it.series ?: it.displayTitle } ?: "Gemeinsam schauen", unter = "${g.host} · ${g.members.size} dabei",
                        bild = x?.let { api.abs(it.backdrop) } ?: "", farbe = x?.color ?: "", onClick = { onRaum(g.id) }, modifier = Modifier.width(breite),
                    )
                }
            }
        }

        val l = titel
        Column(Modifier.padding(top = 28.dp)) {
            Row(Modifier.padding(horizontal = Pad, vertical = 0.dp), verticalAlignment = Alignment.Bottom) {
                T("Neue Gruppe erstellen", typo.reihe, K.Text, FontWeight.SemiBold, maxLines = 1)
                if (!l.isNullOrEmpty()) T("Was wollt ihr schauen?", typo.klein, K.Text2, maxLines = 1, modifier = Modifier.padding(start = 12.dp))
            }
            when {
                l == null -> T("Lade …", typo.klein, K.Text2, modifier = Modifier.padding(horizontal = Pad, vertical = 12.dp))
                l.isEmpty() -> T("Öffne einen Film oder eine Folge und wähle im Player „Gemeinsam schauen“.", typo.klein, K.Text2,
                    modifier = Modifier.padding(horizontal = Pad, vertical = 8.dp))
                else -> LazyRow(Modifier.padding(top = 12.dp), contentPadding = PaddingValues(horizontal = Pad), horizontalArrangement = Arrangement.spacedBy(Luecke)) {
                    items(l, key = { it.id }) { x ->
                        BreitKarte(
                            titel = x.series ?: x.displayTitle,
                            unter = if (x.series != null) "${folge(x)} · ${x.displayTitle}" else jahr(x),
                            bild = api.abs(x.backdrop), farbe = x.color, onClick = { erstellen(x) },
                            modifier = Modifier.width(breite),
                            ampel = x.light, fortschritt = x.anteil(),
                        )
                    }
                }
            }
        }

        Column(Modifier.padding(horizontal = Pad).padding(top = 36.dp).widthIn(max = 760.dp)) {
            T("Gruppe beitreten", typo.reihe, K.Text, FontWeight.SemiBold)
            T("Gib den Gruppencode oder den Einladungslink ein, den du bekommen hast.", typo.klein, K.Text2, modifier = Modifier.padding(top = 4.dp, bottom = 12.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                Feld(code, { code = it; fehler = "" }, "Gruppencode oder Link", Modifier.weight(1f), onGo = ::beitreten)
                Knopf("Beitreten", ::beitreten, Modifier.padding(start = 12.dp), primary = true)
            }
            if (fehler.isNotEmpty()) T(fehler, typo.klein, K.AmpelRot, modifier = Modifier.padding(top = 8.dp))
        }
    }
}
