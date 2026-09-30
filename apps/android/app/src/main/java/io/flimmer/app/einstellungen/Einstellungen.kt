package io.flimmer.app.einstellungen

import android.app.Activity
import android.content.Context
import android.content.ContextWrapper
import android.widget.Toast
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.flimmer.app.ApiClient
import io.flimmer.app.ApiException
import io.flimmer.app.BildModus
import io.flimmer.app.Store
import io.flimmer.app.User
import io.flimmer.app.deviceProfile
import io.flimmer.app.parseLink
import io.flimmer.app.scanQr
import io.flimmer.app.ui.*
import kotlinx.coroutines.launch

// Benutzer-Einstellungen wie web/src/screens/Einstellungen.tsx: Profil, Schnellverbindung, Anzeige, Startseite,
// Wiedergabe, Untertitel. Handy: ohne Bereich die Übersicht, mit Bereich die Seite. Tablet/TV: Menü links, Inhalt rechts.
// Admin-Teile liegen im Dashboard (eigenes Modul).

private const val ANZEIGE = "M3.5 5.5h17v11h-17zM8 20h8" // Icon „fernseher“ aus dem Entwurf

private class Bereich(val id: String, val label: String, val icon: String)

private val BEREICHE = listOf(
    Bereich("profil", "Profil", Ic.PROFIL),
    Bereich("schnellverbindung", "Schnellverbindung", Ic.SCHLUESSEL),
    Bereich("anzeige", "Anzeige", ANZEIGE),
    Bereich("startseite", "Startseite", Ic.START),
    Bereich("wiedergabe", "Wiedergabe", Ic.ABSPIELEN),
    Bereich("untertitel", "Untertitel", Ic.UNTERTITEL),
)
private val HOCHLADEN = Bereich("hochladen", "Videos hochladen", Ic.HOCHLADEN) // nur mit Recht „Hochladen“, nicht am TV

private val TONSPRACHEN = listOf("de" to "Deutsch", "en" to "English", "fr" to "Français", "es" to "Español", "it" to "Italiano", "ja" to "日本語", "tr" to "Türkçe")

/**
 * [bereich]: profil | schnellverbindung | anzeige | startseite | wiedergabe | untertitel, null = Übersicht (Handy)
 * bzw. Profil (Tablet/TV). [onBereich] wechselt den Bereich; auf dem Handy heißt null „zurück zur Übersicht“.
 */
@Composable
fun EinstellungenScreen(api: ApiClient, bereich: String?, tv: Boolean, onBereich: (String?) -> Unit) {
    var me by remember { mutableStateOf<User?>(null) }
    LaunchedEffect(api) { me = runCatching { api.me() }.getOrNull() }
    val handy = !tv && !LocalBreit.current
    val bereiche = if (me?.upload == true && !tv) BEREICHE + HOCHLADEN else BEREICHE
    val aktiv = bereiche.firstOrNull { it.id == bereich } ?: if (handy) null else BEREICHE[0]

    Box(Modifier.fillMaxSize().background(K.Saal)) {
        if (handy) {
            if (aktiv == null) Seite { Uebersicht(api, me, onBereich) }
            else Column {
                Row(Modifier.fillMaxWidth().height(56.dp).padding(start = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                    IconKnopf(Ic.ZURUECK, { onBereich(null) })
                    T(aktiv.label, 18.sp, K.Text, FontWeight.SemiBold, maxLines = 1, modifier = Modifier.padding(start = 4.dp))
                }
                Box(Modifier.weight(1f)) { Seite { Inhalt(api, aktiv.id, me, tv, onBereich) } }
            }
        } else Row(Modifier.fillMaxSize()) {
            Column(Modifier.width(if (tv) 480.dp else 280.dp).fillMaxHeight().verticalScroll(rememberScrollState()).padding(start = Pad, top = 16.dp, end = 8.dp)) {
                Wer(me)
                bereiche.forEach { b -> NavPunkt(b, b == aktiv, tv) { onBereich(b.id) } }
            }
            Box(Modifier.weight(1f)) {
                Seite {
                    T(aktiv!!.label, LocalTypo.current.titel, K.Text, FontWeight.SemiBold, modifier = Modifier.padding(bottom = 8.dp))
                    Inhalt(api, aktiv.id, me, tv, onBereich)
                }
            }
        }
    }
}

@Composable
private fun Seite(inhalt: @Composable ColumnScope.() -> Unit) =
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = Pad).padding(top = 16.dp, bottom = 48.dp).widthIn(max = 760.dp), content = inhalt)

@Composable
private fun Wer(me: User?) {
    if (me == null) return
    val tv = LocalTv.current
    Row(Modifier.padding(bottom = 16.dp), verticalAlignment = Alignment.CenterVertically) {
        Avatar(me.name, me.color, if (tv) 72.dp else 48.dp)
        Column(Modifier.padding(start = 14.dp)) {
            T(me.name, LocalTypo.current.reihe, K.Text, FontWeight.SemiBold, maxLines = 1)
            T(if (me.admin) "Admin" else "Benutzer", LocalTypo.current.klein, K.Text2)
        }
    }
}

/** Menüpunkt links (Tablet/TV). TV: schon der Fokus zeigt den Bereich, wie im Web. */
@Composable
private fun NavPunkt(b: Bereich, an: Boolean, tv: Boolean, onClick: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().padding(bottom = if (tv) 8.dp else 2.dp).height(if (tv) 72.dp else 48.dp)
            .onFocusChanged { if (tv && it.isFocused && !an) onClick() }
            .klick(onClick, skala = false).background(if (an) K.Flaeche3 else K.Saal).padding(horizontal = if (tv) 24.dp else 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Ico(b.icon, if (tv) 32.dp else 22.dp, if (an) K.Text else K.Text2)
        T(b.label, if (tv) LocalTypo.current.text else 15.sp, if (an) K.Text else K.Text2, if (an) FontWeight.SemiBold else FontWeight.Medium,
            maxLines = 1, modifier = Modifier.padding(start = if (tv) 20.dp else 14.dp))
    }
}

// ---------- Handy: Übersicht (Handy-Einstellungen im Entwurf) ----------

@Composable
private fun ColumnScope.Uebersicht(api: ApiClient, me: User?, onBereich: (String?) -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    Wer(me)
    Gruppe("Konto") {
        Zeile(Ic.PROFIL, "Profil", null, linie = false) { onBereich("profil") }
        Zeile(Ic.SCHLUESSEL, "Schnellverbindung", "Ein anderes Gerät zeigt einen 6-stelligen Code? Gib ihn hier ein") { onBereich("schnellverbindung") }
        Zeile(Ic.WECHSEL, "Profil wechseln", null) { scope.launch { abmelden(ctx, api) } }
    }
    Gruppe("App") {
        (BEREICHE.drop(2) + listOfNotNull(HOCHLADEN.takeIf { me?.upload == true })).forEachIndexed { i, b -> Zeile(b.icon, b.label, null, linie = i > 0) { onBereich(b.id) } }
    }
    Gruppe(null) { Zeile(Ic.ABMELDEN, "Abmelden", null, linie = false, gefahr = true) { scope.launch { abmelden(ctx, api) } } }
    val version = remember { runCatching { ctx.packageManager.getPackageInfo(ctx.packageName, 0).versionName }.getOrNull().orEmpty() }
    T("Flimmer für Android $version", 12.sp, K.Text3, family = Mono, modifier = Modifier.align(Alignment.CenterHorizontally).padding(top = 20.dp))
}

@Composable
private fun ColumnScope.Inhalt(api: ApiClient, id: String, me: User?, tv: Boolean, onBereich: (String?) -> Unit) {
    when (id) {
        "profil" -> Profil(api, me, tv, onBereich)
        "schnellverbindung" -> Schnellverbindung(api)
        "anzeige" -> Anzeige()
        "startseite" -> Startseite()
        "wiedergabe" -> Wiedergabe(api)
        "untertitel" -> Untertitel(onBereich)
        "hochladen" -> Hochladen(api)
    }
}

// ---------- Profil ----------

@Composable
private fun ColumnScope.Profil(api: ApiClient, me: User?, tv: Boolean, onBereich: (String?) -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    Gruppe(null) {
        Zeile(Ic.WECHSEL, "Profil wechseln", (me?.name?.let { "$it · " } ?: "") + "Anderes Profil auf diesem Gerät", linie = false) { scope.launch { abmelden(ctx, api) } }
        if (me?.admin == true) Zeile(Ic.GEMEINSAM, "Profile und PINs verwalten", "Im Dashboard unter „Benutzer“")
        else Zeile(Ic.SCHLUESSEL, "PIN ändern", "Das darf nur ein Admin. Frag den, der Flimmer eingerichtet hat.")
        if (!tv) Zeile(ANZEIGE, "Fernseher koppeln", "Einen Fernseher mit dem Code auf seinem Bildschirm anmelden") { onBereich("schnellverbindung") }
        Zeile(Ic.SERVER, api.base.substringAfter("://"), "Verbunden · Server wechseln") { Store(ctx).server = ""; ctx.activity()?.recreate() }
        Zeile(Ic.ABMELDEN, "Abmelden", null, gefahr = true) { scope.launch { abmelden(ctx, api) } }
    }
}

/** Wie im Web (abmelden, dann neu laden): Token vergessen und die Activity neu aufbauen, sie startet dann bei der Anmeldung. */
private suspend fun abmelden(ctx: Context, api: ApiClient) {
    api.logout()
    Store(ctx).token = ""
    ctx.activity()?.recreate()
}

private fun Context.activity(): Activity? = this as? Activity ?: (this as? ContextWrapper)?.baseContext?.activity()

// ---------- Anzeige ----------

@Composable
private fun ColumnScope.Anzeige() {
    val ctx = LocalContext.current
    val store = remember { Store(ctx) }
    var modus by remember { mutableStateOf(store.bildModus) }
    Gruppe("Bildanpassung", "„Automatisch“ schneidet eingebrannte schwarze Balken weg. Gilt nur auf diesem Gerät.", flaeche = false) {
        Wahl(BildModus.entries.map { it to it.label }, modus) { modus = it; store.bildModus = it; gespeichert(ctx) }
    }
}

// ---------- Startseite ----------

@Composable
private fun ColumnScope.Startseite() {
    val ctx = LocalContext.current
    val tv = LocalTv.current
    var v by remember { mutableStateOf(startVorlieben(ctx)) }
    fun setze(neu: StartVorlieben) { setStartVorlieben(ctx, neu); v = neu }
    val liste = v.liste
    fun schiebe(i: Int, d: Int) = setze(v.copy(reihenfolge = liste.toMutableList().apply { add(i + d, removeAt(i)) }))
    Gruppe("Reihen der Startseite", "Wähle, welche Reihen du auf der Startseite siehst – und in welcher Reihenfolge. Gilt auf diesem Gerät.") {
        liste.forEachIndexed { i, id ->
            val aus = id in v.aus
            val name = REIHEN_NAMEN[id] ?: id
            Row(Modifier.fillMaxWidth().linieOben(i > 0).heightIn(min = if (tv) 88.dp else 60.dp).padding(start = 16.dp, end = 8.dp),
                verticalAlignment = Alignment.CenterVertically) {
                Ico(if (aus) Ic.SCHLIESSEN else Ic.HAKEN, if (tv) 32.dp else 22.dp, if (aus) K.Text3 else K.Text)
                T("%02d · %s".format(i + 1, name), if (tv) LocalTypo.current.text else 15.sp, if (aus) K.Text3 else K.Text, FontWeight.Medium,
                    maxLines = 2, modifier = Modifier.weight(1f).padding(horizontal = 14.dp))
                Knopf(if (aus) "Ausgeblendet" else "Sichtbar", { setze(v.copy(aus = if (aus) v.aus - id else v.aus + id)) }, an = !aus)
                val g = if (tv) 72.dp else 44.dp
                if (i > 0) IconKnopf(Ic.HOCH, { schiebe(i, -1) }, groesse = g) else Spacer(Modifier.size(g))
                if (i < liste.lastIndex) IconKnopf(Ic.RUNTER, { schiebe(i, 1) }, groesse = g) else Spacer(Modifier.size(g))
            }
        }
    }
}

// ---------- Wiedergabe ----------

private val FORMATE = listOf("h264" to "H.264", "hevc" to "HEVC", "hevc10" to "HEVC 10-bit", "av1" to "AV1", "vp9" to "VP9")

@Composable
private fun ColumnScope.Wiedergabe(api: ApiClient) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val store = remember { Store(ctx) }
    var lang by remember { mutableStateOf(store.audioLangs.firstOrNull() ?: "de") }
    var quality by remember { mutableIntStateOf(store.quality) }
    var night by remember { mutableStateOf(store.night) }
    var autoNext by remember { mutableStateOf(store.autoNext) }
    var profil by remember { mutableStateOf(deviceProfile(ctx)) }
    var test by remember { mutableStateOf("") }

    T("Gilt auf diesem Gerät.", LocalTypo.current.klein, K.Text2)
    Gruppe("Bevorzugte Tonsprache", "Flimmer wählt zuerst die Tonspur in dieser Sprache.", flaeche = false) {
        Wahl(TONSPRACHEN, lang) { x -> lang = x; setTonsprachen(ctx, if (x == "en") listOf("en") else listOf(x, "en")); gespeichert(ctx) }
    }
    Gruppe("Maximale Qualität", "Größere Bilder rechnet der Server beim Umwandeln herunter.", flaeche = false) {
        Wahl(listOf(0 to "Auto", 1080 to "1080p", 720 to "720p", 480 to "480p"), quality) { q -> quality = q; store.quality = q; gespeichert(ctx) }
    }
    Gruppe(null) {
        SchalterZeile(Ic.TON, "Nachtmodus", "Laute Stellen leiser, leise Dialoge lauter – für alle Titel", night, linie = false) {
            night = !night; store.night = night; gespeichert(ctx)
        }
        SchalterZeile(Ic.NAECHSTE, "Nächste Folge automatisch", "Startet die nächste Folge nach dem Abspann.", autoNext) {
            autoNext = !autoNext; store.autoNext = autoNext; gespeichert(ctx)
        }
    }
    Gruppe("Dieses Gerät") {
        Zeile(Ic.NEUSTART, "Gerät neu testen", test.ifEmpty { "Fragt die Decoder dieses Geräts ab und meldet sie dem Server" }, linie = false) {
            test = "Wird getestet …"
            scope.launch {
                val p = deviceProfile(ctx)
                profil = p
                test = runCatching { api.putProfile(store.deviceId, p) }
                    .fold({ "${p.video.size + p.audio.size} Formate laufen direkt" }, { "Der Server hat das Ergebnis nicht angenommen: ${it.message}" })
            }
        }
        FORMATE.forEach { (id, name) ->
            val direkt = id in profil.video
            Row(Modifier.fillMaxWidth().linieOben(true).height(if (LocalTv.current) 64.dp else 44.dp).padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                AmpelPunkt(if (direkt) "green" else "red")
                T(name, LocalTypo.current.klein, K.Text, FontWeight.Medium, modifier = Modifier.weight(1f).padding(start = 14.dp))
                T(if (direkt) "direkt" else "wird umgewandelt", LocalTypo.current.klein, K.Text2)
            }
        }
        val hdr = profil.hdr
        Row(Modifier.fillMaxWidth().linieOben(true).height(if (LocalTv.current) 64.dp else 44.dp).padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) {
            AmpelPunkt(if (hdr.isNullOrEmpty()) "yellow" else "green")
            T("HDR", LocalTypo.current.klein, K.Text, FontWeight.Medium, modifier = Modifier.weight(1f).padding(start = 14.dp))
            T(when { hdr == null -> "unbekannt"; hdr.isEmpty() -> "wird umgerechnet (SDR-Bildschirm)"; else -> hdr.joinToString(", ") { it.uppercase() } },
                LocalTypo.current.klein, K.Text2)
        }
    }
}

// ---------- Untertitel ----------

@Composable
private fun ColumnScope.Untertitel(onBereich: (String?) -> Unit) {
    val ctx = LocalContext.current
    val store = remember { Store(ctx) }
    var modus by remember { mutableStateOf(store.subtitleMode) }
    val tv = LocalTv.current
    T("Legt fest, wann Untertitel von selbst erscheinen. Im Player kannst du jederzeit umschalten.", LocalTypo.current.klein, K.Text2)
    Gruppe("Untertitelmodus") {
        listOf(
            Triple("", "Intelligent", "Zeigt Untertitel, wenn der Ton nicht in deiner Sprache ist, sonst nur erzwungene."),
            Triple("always", "Immer", "Zeigt immer Untertitel in deiner Sprache."),
            Triple("off", "Nur erzwungene", "Zeigt nur Untertitel für fremdsprachige Stellen."),
        ).forEachIndexed { i, (w, titel, text) ->
            val an = modus == w
            Row(Modifier.fillMaxWidth().linieOben(i > 0).heightIn(min = if (tv) 88.dp else 60.dp)
                .klick({ modus = w; setUntertitelModus(ctx, w); gespeichert(ctx) }, skala = false).padding(horizontal = 16.dp, vertical = 10.dp),
                verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.size(if (tv) 28.dp else 20.dp).border(1.5.dp, if (an) K.Text else K.LinieStark, CircleShape), contentAlignment = Alignment.Center) {
                    if (an) Box(Modifier.size(if (tv) 14.dp else 10.dp).background(K.Text, CircleShape))
                }
                Column(Modifier.padding(start = 16.dp).weight(1f)) {
                    T(titel, if (tv) LocalTypo.current.text else 15.sp, K.Text, FontWeight.Medium)
                    T(text, LocalTypo.current.klein, K.Text2)
                }
            }
        }
    }
    Gruppe("Sprache der Untertitel") {
        val lang = store.audioLangs.firstOrNull() ?: "de"
        Zeile(Ic.TON, TONSPRACHEN.firstOrNull { it.first == lang }?.second ?: lang, "Folgt der bevorzugten Tonsprache unter „Wiedergabe“.", linie = false) { onBereich("wiedergabe") }
    }
}

// ---------- Schnellverbindung ----------

@Composable
private fun ColumnScope.Schnellverbindung(api: ApiClient) {
    val scope = rememberCoroutineScope()
    var code by remember { mutableStateOf("") }
    var ok by remember { mutableStateOf("") }
    var fehler by remember { mutableStateOf("") }
    val typo = LocalTypo.current
    fun los() {
        if (code.length != 6) return
        ok = ""; fehler = ""
        scope.launch {
            runCatching { api.pairConfirm(code) }
                .onSuccess { ok = "„${it.ifEmpty { "Gerät" }}“ ist jetzt angemeldet."; code = "" }
                .onFailure { fehler = if ((it as? ApiException)?.code == 429) "Zu viele Versuche – bitte eine Minute warten." else "Diesen Code kennt der Server nicht, oder er ist abgelaufen." }
        }
    }
    T("Ein anderes Gerät zeigt einen 6-stelligen Code oder QR-Code? Gib den Code ein oder scanne ihn – es meldet sich dann als dein Profil an, ohne dass du ein Passwort tippst.",
        typo.text, K.Text2)
    Label("Code", Modifier.padding(top = 20.dp, bottom = 8.dp))
    Row(verticalAlignment = Alignment.CenterVertically) {
        Feld(code, { code = it.filter(Char::isDigit).take(6) }, "123456", Modifier.width(if (LocalTv.current) 320.dp else 180.dp), type = KeyboardType.Number, onGo = ::los)
        Knopf("Anmelden", ::los, Modifier.padding(start = 12.dp), primary = code.length == 6)
    }
    if (!LocalTv.current) {
        val ctx = LocalContext.current
        Knopf("QR-Code scannen", {
            scanQr(ctx, { fehler = it }) { text ->
                val c = parseLink(text).pairCode
                if (c == null) fehler = "Das ist kein Kopplungscode von Flimmer." else { code = c; los() }
            }
        }, Modifier.padding(top = 12.dp), icon = Ic.QR)
    }
    if (ok.isNotEmpty()) Meldung(Ic.HAKEN, ok, K.Text)
    if (fehler.isNotEmpty()) Meldung(Ic.INFO, fehler, K.AmpelRot)
}

@Composable
private fun Meldung(icon: String, text: String, farbe: androidx.compose.ui.graphics.Color) =
    Row(Modifier.padding(top = 16.dp), verticalAlignment = Alignment.CenterVertically) {
        Ico(icon, 22.dp, farbe)
        T(text, LocalTypo.current.klein, farbe, modifier = Modifier.padding(start = 10.dp))
    }

// ---------- Bausteine: Gruppe, Zeile, Schalter, Wahl ----------

private fun gespeichert(ctx: Context) = Toast.makeText(ctx, "Gespeichert", Toast.LENGTH_SHORT).show()

private fun Modifier.linieOben(an: Boolean): Modifier = if (!an) this else drawBehind { drawRect(K.Linie, size = Size(size.width, 1.dp.toPx())) }

/** Abschnitt: Überschrift, Erklärung, darunter die Zeilen auf Fläche 1 (oder frei, [flaeche] = false, für Wahl-Chips). */
@Composable
private fun Gruppe(titel: String?, text: String? = null, flaeche: Boolean = true, inhalt: @Composable ColumnScope.() -> Unit) {
    val typo = LocalTypo.current
    if (titel != null) T(titel, typo.reihe, K.Text, FontWeight.SemiBold, modifier = Modifier.padding(top = 28.dp))
    if (text != null) T(text, typo.klein, K.Text2, modifier = Modifier.padding(top = 4.dp))
    Column(Modifier.padding(top = if (titel == null && text == null) 20.dp else 12.dp).fillMaxWidth()
        .then(if (flaeche) Modifier.background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusM)) else Modifier), content = inhalt)
}

@Composable
private fun Zeile(icon: String, titel: String, unter: String?, linie: Boolean = true, gefahr: Boolean = false, onClick: (() -> Unit)? = null) {
    val tv = LocalTv.current
    val farbe = if (gefahr) K.AmpelRot else K.Text
    Row(Modifier.fillMaxWidth().linieOben(linie).heightIn(min = if (tv) 88.dp else 56.dp)
        .then(if (onClick != null) Modifier.klick(onClick, skala = false) else Modifier).padding(horizontal = 16.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically) {
        Ico(icon, if (tv) 32.dp else 22.dp, farbe)
        Column(Modifier.padding(start = 16.dp).weight(1f)) {
            T(titel, if (tv) LocalTypo.current.text else 15.sp, farbe, FontWeight.Medium, maxLines = 2)
            if (unter != null) T(unter, LocalTypo.current.klein, K.Text2)
        }
        if (onClick != null && !gefahr) Ico(Ic.WEITER, 20.dp, K.Text2)
    }
}

@Composable
private fun SchalterZeile(icon: String, titel: String, unter: String, an: Boolean, linie: Boolean = true, onClick: () -> Unit) {
    val tv = LocalTv.current
    Row(Modifier.fillMaxWidth().linieOben(linie).heightIn(min = if (tv) 88.dp else 60.dp).klick(onClick, skala = false)
        .padding(horizontal = 16.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
        Ico(icon, if (tv) 32.dp else 22.dp, K.Text)
        Column(Modifier.padding(start = 16.dp, end = 12.dp).weight(1f)) {
            T(titel, if (tv) LocalTypo.current.text else 15.sp, K.Text, FontWeight.Medium)
            T(unter, LocalTypo.current.klein, K.Text2)
        }
        Schalter(an)
    }
}

/** Auswahl aus wenigen Werten als Chips (wie Wahl im Web). */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun <W> Wahl(werte: List<Pair<W, String>>, an: W, onWahl: (W) -> Unit) {
    FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        werte.forEach { (w, text) -> Chip(text, w == an, { onWahl(w) }) }
    }
}
