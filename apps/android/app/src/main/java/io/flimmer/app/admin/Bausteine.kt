package io.flimmer.app.admin

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.widget.Toast
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.repeatOnLifecycle
import io.flimmer.app.ui.*
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** Icons des Dashboards, die ui/Icons.kt nicht hat (Pfade aus web/src/components/Icon.tsx). */
internal object AI {
    const val EINLADUNG = "M3.5 6h17v12h-17zM3.5 6l8.5 7 8.5-7"
    const val BIBLIOTHEK = "M3.5 6h6l2 2.5h9v10h-17z"
    const val NETZWERK = "M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM3 12h18M12 3c2.4 2.5 3.5 5.5 3.5 9s-1.1 6.5-3.5 9c-2.4-2.5-3.5-5.5-3.5-9s1.1-6.5 3.5-9z"
    const val AUFGABEN = "M4.5 5.5h15v14.5h-15zM8.5 3v4M15.5 3v4M8.5 13.5l2.5 2.5 4.5-5"
    const val SICHERUNG = "M12 3.5l7.5 3v5.5c0 4.5-3.2 7.7-7.5 9-4.3-1.3-7.5-4.5-7.5-9V6.5zM8.5 12l2.5 2.5 4.5-5"
    const val PROTOKOLL = "M6 3.5h8.5l4 4v13H6zM9 11.5h6.5M9 15h6.5M9 18.5h4"
    const val FEHLER = "M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 7.5v6M12 16.5v.5"
    const val PLUS = "M12 5v14M5 12h14"
    const val SCHLOSS = "M7 10.5h10a2 2 0 0 1 2 2v6a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2v-6a2 2 0 0 1 2-2zM8 10.5V8a4 4 0 0 1 8 0v2.5"
    const val FERNSEHER = "M3.5 5.5h17v11h-17zM8 20h8"
    const val FERNZUGRIFF = "M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM8 12h8M13 8.5l3.5 3.5-3.5 3.5"
}

// ---------- Laden ----------

class Laden<T>(val daten: T?, val fehler: Throwable?, val neu: () -> Unit)

/** Lädt einmal (und bei neu()); alte Daten bleiben beim Nachladen stehen. */
@Composable
internal fun <T> laden(vararg keys: Any?, block: suspend () -> T): Laden<T> {
    var daten by remember(*keys) { mutableStateOf<T?>(null) }
    var fehler by remember(*keys) { mutableStateOf<Throwable?>(null) }
    var tick by remember(*keys) { mutableIntStateOf(0) }
    val b by rememberUpdatedState(block)
    LaunchedEffect(*keys, tick) {
        try {
            daten = b(); fehler = null
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            fehler = e
        }
    }
    return Laden(daten, fehler) { tick++ }
}

/** Wiederholt neu(), solange [an] gilt und die Seite sichtbar ist (App im Vordergrund). */
@Composable
internal fun Nachladen(ms: Long, an: Boolean = true, neu: () -> Unit) {
    val lc = LocalLifecycleOwner.current
    val n by rememberUpdatedState(neu)
    LaunchedEffect(an, ms) {
        if (an) lc.lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) { while (true) { delay(ms); n() } }
    }
}

/** Ladezustand: fehlt der Endpunkt (404), ein ruhiger Hinweis statt Fehler. */
@Composable
internal fun <T> Geladen(z: Laden<T>, inhalt: @Composable (T) -> Unit) {
    val d = z.daten
    val f = z.fehler
    when {
        d != null -> inhalt(d)
        f != null && fehlt(f) -> Hinweis("Der Server liefert diesen Bereich noch nicht. Nach einem Update erscheint er hier.", "Noch nicht verfügbar")
        f != null -> Column(Modifier.padding(vertical = 12.dp)) {
            T(fehlerText(f), LocalTypo.current.klein, K.AmpelRot)
            Spacer(Modifier.height(8.dp))
            Knopf("Nochmal versuchen", z.neu, icon = Ic.NEUSTART)
        }
        else -> Box(Modifier.fillMaxWidth().height(96.dp), contentAlignment = Alignment.CenterStart) { T("Lade …", LocalTypo.current.klein, K.Text3) }
    }
}

// ---------- Aktionen ----------

/** Startet Server-Aufrufe aus Knöpfen: Erfolg optional als Meldung, Fehler immer als Meldung. */
class Aktion(private val scope: CoroutineScope, private val ctx: Context) {
    fun melde(text: String) = Toast.makeText(ctx, text, Toast.LENGTH_SHORT).show()
    fun los(ok: String? = null, fehler: ((Throwable) -> String)? = null, danach: () -> Unit = {}, block: suspend () -> Unit) {
        scope.launch {
            try {
                block(); ok?.let(::melde); danach()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                melde(fehler?.invoke(e) ?: fehlerText(e))
            }
        }
    }
    fun kopieren(text: String) {
        (ctx.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager).setPrimaryClip(ClipData.newPlainText("Flimmer", text))
        melde("In die Zwischenablage kopiert")
    }
    fun teilen(text: String) = runCatching {
        ctx.startActivity(Intent.createChooser(Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, text), "Link teilen"))
    }.onFailure { kopieren(text) }
    fun oeffnen(url: String) = runCatching { ctx.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url))) }.onFailure { melde("Kein Browser gefunden") }
}

@Composable
internal fun rememberAktion(): Aktion {
    val s = rememberCoroutineScope()
    val c = LocalContext.current
    return remember(s, c) { Aktion(s, c) }
}

// ---------- Layout ----------

internal val kompakt: Boolean @Composable get() = !LocalTv.current && !LocalBreit.current

/** Scrollbarer Inhalt eines Bereichs. */
@Composable
internal fun Seite(inhalt: @Composable ColumnScope.() -> Unit) =
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = Pad).padding(bottom = 48.dp), content = inhalt)

/** Abschnitt mit Überschrift; [zusatz] = Knöpfe rechts (Handy: darunter). */
@Composable
internal fun Gruppe(titel: String? = null, text: String? = null, zusatz: (@Composable () -> Unit)? = null, inhalt: @Composable ColumnScope.() -> Unit = {}) {
    val typo = LocalTypo.current
    Column(Modifier.fillMaxWidth().padding(top = if (LocalTv.current) 32.dp else 24.dp)) {
        if (titel != null || zusatz != null) {
            if (kompakt) {
                if (titel != null) T(titel, typo.reihe, K.Text, FontWeight.SemiBold)
                if (zusatz != null) Row(Modifier.padding(top = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) { zusatz() }
                Spacer(Modifier.height(12.dp))
            } else Row(Modifier.padding(bottom = 12.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                T(titel ?: "", typo.reihe, K.Text, FontWeight.SemiBold, modifier = Modifier.weight(1f))
                zusatz?.invoke()
            }
        }
        if (text != null) Leise(text, Modifier.padding(bottom = 12.dp))
        inhalt()
    }
}

@Composable
internal fun Leise(text: String, modifier: Modifier = Modifier, farbe: Color = K.Text2) = T(text, LocalTypo.current.klein, farbe, modifier = modifier)

@Composable
internal fun Unterkopf(text: String) = T(text, if (LocalTv.current) LocalTypo.current.text else 16.sp, K.Text, FontWeight.SemiBold, modifier = Modifier.padding(top = 16.dp, bottom = 8.dp))

private fun Modifier.linieUnten(): Modifier = drawBehind { drawRect(K.Linie, Offset(0f, size.height - 1.dp.toPx()), Size(size.width, 1.dp.toPx())) }

/**
 * Zeile wie .fl-zeile: Icon, Titel mit Unterzeile, rechts Wert, Schalter oder Knöpfe. Mit [onClick] ist die ganze Zeile bedienbar.
 * Auf dem TV ist jede Zeile ohne eigene Knöpfe fokussierbar, sonst käme das D-Pad nicht an ihr vorbei.
 */
@Composable
internal fun Zeile(
    titel: String, modifier: Modifier = Modifier, icon: String? = null, unter: String? = null, wert: String? = null, schalter: Boolean? = null,
    an: Boolean = false, farbe: Color = K.Text, onClick: (() -> Unit)? = null, vorne: (@Composable () -> Unit)? = null,
    unten: (@Composable () -> Unit)? = null, rechts: (@Composable RowScope.() -> Unit)? = null,
) {
    val tv = LocalTv.current
    val typo = LocalTypo.current
    val schmal = !tv && !LocalBreit.current
    val bedienen = when {
        onClick != null -> Modifier.klick(onClick, skala = false)
        tv && rechts == null -> Modifier.fokusRing(skala = false).focusable()
        else -> Modifier
    }
    Row(
        modifier.fillMaxWidth().then(bedienen).background(if (an) K.Flaeche3 else K.Flaeche1).linieUnten()
            .heightIn(min = if (tv) 88.dp else 56.dp).padding(horizontal = if (tv) 24.dp else 16.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (icon != null) { Ico(icon, if (tv) 32.dp else 24.dp, farbe); Spacer(Modifier.width(16.dp)) }
        if (vorne != null) { vorne(); Spacer(Modifier.width(12.dp)) }
        Column(Modifier.weight(1f)) {
            T(titel, if (tv) typo.text else 15.sp, K.Text, FontWeight.Medium)
            if (!unter.isNullOrEmpty()) T(unter, typo.klein, K.Text2)
            unten?.invoke()
            // Handy: Knöpfe unter den Text, sonst drücken sie ihn auf eine Wortspalte zusammen
            if (rechts != null && schmal) Row(Modifier.padding(top = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) { rechts() }
        }
        if (wert != null) T(wert, typo.klein, K.Text2, maxLines = 1, modifier = Modifier.padding(start = 12.dp))
        if (schalter != null) { Spacer(Modifier.width(12.dp)); Schalter(schalter) }
        if (rechts != null && !schmal) Row(Modifier.padding(start = 12.dp), horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) { rechts() }
    }
}

/** Knopf mit „aus“ (gedimmt, ohne Wirkung). */
@Composable
internal fun AKnopf(text: String, onClick: () -> Unit, modifier: Modifier = Modifier, primary: Boolean = false, icon: String? = null, aus: Boolean = false) =
    Knopf(text, { if (!aus) onClick() }, modifier.alpha(if (aus) 0.4f else 1f), primary, icon)

/** Kennzahl: Label in Mono, großer Wert, optional Unterzeile und Balken. */
@Composable
internal fun Messwert(label: String, wert: String, modifier: Modifier = Modifier, unter: String? = null, anteil: Float? = null) {
    val tv = LocalTv.current
    Column(modifier.background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusM)).padding(if (tv) 24.dp else 16.dp)) {
        Label(label)
        T(wert, if (tv) 40.sp else if (LocalBreit.current) 26.sp else 22.sp, K.Text, FontWeight.SemiBold, maxLines = 1, modifier = Modifier.padding(top = 4.dp))
        if (unter != null) T(unter, LocalTypo.current.klein, K.Text2, maxLines = 1)
        if (anteil != null) Balken(anteil, Modifier.padding(top = 8.dp))
    }
}

@Composable
internal fun Balken(anteil: Float, modifier: Modifier = Modifier) = Fortschritt(anteil, modifier)

/** Kacheln in Spalten (Handy 2, Tablet 3, TV 4). */
@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun Kacheln(spalten: Int = if (LocalTv.current) 4 else if (LocalBreit.current) 3 else 2, inhalt: @Composable FlowRowScope.() -> Unit) {
    val l = Luecke
    FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(l), verticalArrangement = Arrangement.spacedBy(l),
        maxItemsInEachRow = spalten, content = inhalt)
}

@Composable
internal fun Hinweis(text: String, titel: String? = null, icon: String = Ic.INFO, farbe: Color = K.Text) {
    Row(Modifier.fillMaxWidth().padding(top = 16.dp).background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusM))
        .border(1.dp, K.Linie, RoundedCornerShape(Tokens.Radius.RadiusM)).padding(16.dp)) {
        Ico(icon, if (LocalTv.current) 32.dp else 24.dp, farbe)
        Column(Modifier.padding(start = 12.dp)) {
            if (titel != null) T(titel, if (LocalTv.current) LocalTypo.current.text else 15.sp, K.Text, FontWeight.SemiBold)
            T(text, LocalTypo.current.klein, K.Text2)
        }
    }
}

/** Auswahl aus wenigen Werten als Chips. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun <W> Wahl(werte: List<Pair<W, String>>, an: W, onWahl: (W) -> Unit) {
    FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        werte.forEach { (w, label) -> Chip(label, w == an, { if (w != an) onWahl(w) }) }
    }
}

/** Formularzeile: Feld und Knöpfe nebeneinander, auf dem Handy die Knöpfe darunter. */
@Composable
internal fun FormZeile(feld: @Composable (Modifier) -> Unit, knoepfe: @Composable RowScope.() -> Unit) {
    if (kompakt) Column(Modifier.fillMaxWidth()) {
        feld(Modifier.fillMaxWidth())
        Row(Modifier.padding(top = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp), content = knoepfe)
    } else Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        feld(Modifier.weight(1f))
        knoepfe()
    }
}

/**
 * Eingabefeld mit Beschriftung darüber. Auf dem TV fährt das D-Pad nur über das Feld (Fokusring);
 * erst OK setzt den echten Fokus und öffnet die Tastatur.
 */
@Composable
internal fun Eingabe(
    wert: String, setWert: (String) -> Unit, label: String, modifier: Modifier = Modifier, zeigeLabel: Boolean = true,
    zahl: Boolean = false, passwort: Boolean = false, mehrzeilig: Boolean = false, onEnter: () -> Unit = {},
) {
    val tv = LocalTv.current
    var bearbeiten by remember { mutableStateOf(false) }
    val fr = remember { FocusRequester() }
    LaunchedEffect(bearbeiten) { if (bearbeiten) runCatching { fr.requestFocus() } }
    Column(modifier) {
        if (zeigeLabel) T(label, LocalTypo.current.klein, K.Text2, FontWeight.Medium, modifier = Modifier.padding(bottom = 4.dp))
        val m = Modifier.fillMaxWidth().then(
            if (tv) Modifier.focusRequester(fr).focusProperties { canFocus = bearbeiten }.onFocusChanged { if (!it.isFocused && bearbeiten) bearbeiten = false }
            else Modifier,
        )
        val platz = if (zeigeLabel) "" else label
        val feld = @Composable {
            if (mehrzeilig) OutlinedTextField(
                wert, setWert, m, minLines = 4,
                placeholder = { T(platz, color = K.Text3) },
                textStyle = TextStyle(fontFamily = Sans, fontSize = LocalTypo.current.text, color = K.Text),
                shape = RoundedCornerShape(Tokens.Radius.RadiusS),
                colors = OutlinedTextFieldDefaults.colors(
                    focusedBorderColor = K.Text, unfocusedBorderColor = K.LinieStark, cursorColor = K.Text,
                    focusedContainerColor = K.Flaeche2, unfocusedContainerColor = K.Flaeche2,
                ),
            ) else Feld(wert, setWert, platz, m, passwort, if (zahl) KeyboardType.Number else KeyboardType.Text, onEnter)
        }
        if (tv) Box(Modifier.klick({ bearbeiten = true }, skala = false)) { feld() } else feld()
    }
}
