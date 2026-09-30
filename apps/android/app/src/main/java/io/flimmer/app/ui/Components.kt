package io.flimmer.app.ui

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontVariation
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import coil3.compose.AsyncImage
import io.flimmer.app.Item
import io.flimmer.app.R

// Design „Mischung“ aus docs/ui-rework: warmes Schwarz, keine Akzentfarbe – Hervorhebung ist Helligkeit.
// Farben, Maße und Schriftgrößen kommen aus Tokens.kt (generiert aus tokens.json).
val K = Tokens.Kino

val Plakat = FontFamily(Font(R.font.flimmer_plakat))
val Mono = FontFamily(Font(R.font.plex_mono))

private fun plex(w: Int) = Font(R.font.plex_sans, FontWeight(w), variationSettings = FontVariation.Settings(FontVariation.weight(w)))
val Sans = FontFamily(plex(400), plex(500), plex(600), plex(700))

/** Schriftgrößen je Gerät: TV (10-Fuß), Tablet oder Handy. */
data class Typo(val plakat: TextUnit, val titel: TextUnit, val reihe: TextUnit, val karte: TextUnit, val text: TextUnit, val klein: TextUnit, val label: TextUnit, val code: TextUnit)

private val TypoTv = Typo(Tokens.TypoTv.Plakat, Tokens.TypoTv.Titel, Tokens.TypoTv.Reihe, Tokens.TypoTv.Karte, Tokens.TypoTv.Text, Tokens.TypoTv.Klein, Tokens.TypoTv.Label, Tokens.TypoTv.Code)
private val TypoDt = Typo(Tokens.TypoDt.Plakat, Tokens.TypoDt.Titel, Tokens.TypoDt.Reihe, Tokens.TypoDt.Karte, Tokens.TypoDt.Text, Tokens.TypoDt.Klein, Tokens.TypoDt.Label, Tokens.TypoDt.Code)
private val TypoHd = Typo(Tokens.TypoHd.Plakat, Tokens.TypoHd.Titel, Tokens.TypoHd.Reihe, Tokens.TypoHd.Karte, Tokens.TypoHd.Text, Tokens.TypoHd.Klein, Tokens.TypoHd.Label, Tokens.TypoHd.Code)

/** true auf Android TV / Fire TV: D-Pad-Fokus, Schiene links, große Kacheln. */
val LocalTv = staticCompositionLocalOf { false }
/** true auf Tablets (ab 600 dp Breite): Seitenleiste statt Schublade, mehr Spalten. */
val LocalBreit = staticCompositionLocalOf { false }
val LocalTypo = staticCompositionLocalOf { TypoHd }

@Composable
fun FlimmerTheme(tv: Boolean, content: @Composable () -> Unit) {
    val scheme = darkColorScheme(
        primary = K.Marke, onPrimary = K.AufMarke, background = K.Saal, surface = K.Flaeche1,
        surfaceVariant = K.Flaeche2, onSurface = K.Text, onBackground = K.Text, outline = K.LinieStark,
        surfaceContainer = K.Flaeche1, surfaceContainerHigh = K.Flaeche1, surfaceContainerLow = K.Flaeche1,
    )
    // TV: Der Entwurf rechnet in 1920 × 1080 px. Eine eigene Dichte bildet genau das auf den Bildschirm ab,
    // egal ob das Gerät 960 oder 1280 dp breit meldet.
    val m = LocalContext.current.resources.displayMetrics
    val dichte = if (tv) Density(maxOf(m.widthPixels, m.heightPixels) / 1920f, 1f) else LocalDensity.current
    val breit = !tv && LocalConfiguration.current.screenWidthDp >= 600
    MaterialTheme(colorScheme = scheme, typography = MaterialTheme.typography.let { t ->
        t.copy(bodyLarge = t.bodyLarge.copy(fontFamily = Sans), bodyMedium = t.bodyMedium.copy(fontFamily = Sans), labelLarge = t.labelLarge.copy(fontFamily = Sans))
    }) {
        CompositionLocalProvider(
            LocalTv provides tv, LocalBreit provides breit, LocalDensity provides dichte,
            LocalTypo provides if (tv) TypoTv else if (breit) TypoDt else TypoHd, content = content,
        )
    }
}

/** Seitenrand: Handy 16, Tablet 24, TV 48 (neben der 72-px-Schiene). */
val Pad: Dp @Composable get() = if (LocalTv.current) 48.dp else if (LocalBreit.current) 24.dp else 16.dp

/** Abstand zwischen Kacheln. */
val Luecke: Dp @Composable get() = if (LocalTv.current) 24.dp else if (LocalBreit.current) 16.dp else 8.dp

@Composable
fun T(text: String, size: TextUnit = LocalTypo.current.text, color: Color = K.Text, weight: FontWeight = FontWeight.Normal,
      family: FontFamily = Sans, maxLines: Int = Int.MAX_VALUE, modifier: Modifier = Modifier, spacing: TextUnit = TextUnit.Unspecified,
      lineHeight: TextUnit = TextUnit.Unspecified, italic: Boolean = false) =
    Text(text, modifier, color = color, maxLines = maxLines, overflow = TextOverflow.Ellipsis,
        style = TextStyle(fontFamily = family, fontSize = size, fontWeight = weight, letterSpacing = spacing, lineHeight = lineHeight,
            fontStyle = if (italic) androidx.compose.ui.text.font.FontStyle.Italic else null))

/** Kleines Versal-Label in Plex Mono, z. B. „GENRES“. */
@Composable
fun Label(text: String, modifier: Modifier = Modifier, color: Color = K.Text3) =
    T(text.uppercase(), LocalTypo.current.label, color, family = Mono, spacing = 0.15.em, modifier = modifier)

/** Plakat-Titel (Flimmer Plakat, Versalien). */
@Composable
fun PlakatTitel(text: String, size: TextUnit = LocalTypo.current.plakat, modifier: Modifier = Modifier, maxLines: Int = 2) =
    T(text.uppercase(), size, K.Text, family = Plakat, maxLines = maxLines, modifier = modifier, lineHeight = size * 0.95f)

fun lightColor(light: String) = when (light) {
    "green" -> K.AmpelGruen
    "yellow" -> K.AmpelGelb
    else -> K.AmpelRot
}

fun lightText(light: String) = when (light) {
    "green" -> "Läuft direkt"
    "yellow" -> "Server wandelt teilweise um"
    else -> "Wird umgewandelt"
}

/** Ampelpunkt wie im Entwurf: grün voll, gelb halb gefüllt mit Ring, rot nur Ring (auch ohne Farbe lesbar). */
@Composable
fun AmpelPunkt(light: String, size: Dp = if (LocalTv.current) 12.dp else 9.dp) {
    val c = lightColor(light)
    Canvas(Modifier.size(size)) {
        val w = size.toPx() / 6
        when (light) {
            "green" -> drawCircle(c)
            "yellow" -> {
                drawArc(c, 90f, 180f, useCenter = true)
                drawCircle(c, this.size.minDimension / 2 - w / 2, style = Stroke(w))
            }
            else -> drawCircle(c, this.size.minDimension / 2 - w / 2, style = Stroke(w))
        }
    }
}

@Composable
fun Ampel(light: String, withText: Boolean = true, text: String = lightText(light)) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        AmpelPunkt(light)
        if (withText) T(text, LocalTypo.current.klein, K.Text2, modifier = Modifier.padding(start = 8.dp))
    }
}

/** Farbe aus dem Server ("#rrggbb"), sonst Fläche 2. */
fun ton(hex: String): Color = runCatching { Color(android.graphics.Color.parseColor(hex)) }.getOrDefault(K.Flaeche2)

/** Bild vom Server; ohne Bild eine ruhige Tonfläche mit Initiale in der Plakatschrift. */
@Composable
fun Art(url: String, name: String, modifier: Modifier = Modifier, farbe: Color = K.Flaeche2) {
    Box(modifier.background(farbe), contentAlignment = Alignment.Center) {
        if (url.isEmpty()) T(name.take(1).uppercase(), LocalTypo.current.titel, K.Text3, family = Plakat)
        else AsyncImage(url, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
    }
}

/**
 * Fokus wie im Web (Klasse ist-fokus): Doppelring – innen Saal, außen Papierweiß – und auf dem TV Scale 1,04.
 * Auf dem Handy gibt es im Touch-Modus keinen Fokus, dort bleibt es beim Ripple.
 */
@Composable
fun Modifier.fokusRing(radius: Dp = Tokens.Radius.RadiusS, skala: Boolean = true): Modifier {
    var f by remember { mutableStateOf(false) }
    val tv = LocalTv.current
    val s by animateFloatAsState(if (f && skala && tv) Tokens.Dauer.FokusScale else 1f, tween(Tokens.Dauer.DauerSchnell), label = "fokus")
    val ring = if (tv) Tokens.Masse.FokusRingTv else Tokens.Masse.FokusRingDt
    return onFocusChanged { f = it.isFocused }
        .graphicsLayer { scaleX = s; scaleY = s }
        .drawWithContent {
            drawContent()
            if (!f) return@drawWithContent
            val r = ring.toPx()
            for ((abstand, farbe) in listOf(r / 2 to K.FokusInnen, r * 1.5f to K.Text)) {
                drawRoundRect(farbe, Offset(-abstand, -abstand), Size(size.width + 2 * abstand, size.height + 2 * abstand),
                    CornerRadius(radius.toPx() + abstand), style = Stroke(r))
            }
        }
}

/** Klickbar mit Fokusring; [onLang] = langes Drücken (Kontextmenü). */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun Modifier.klick(onClick: () -> Unit, onLang: (() -> Unit)? = null, radius: Dp = Tokens.Radius.RadiusS, skala: Boolean = true): Modifier =
    fokusRing(radius, skala).clip(RoundedCornerShape(radius)).combinedClickable(onLongClick = onLang, onClick = onClick)

/** Knopf wie im Entwurf: primär papierweiß, sonst mit 1-px-Rahmen. Höhe 44 (Handy), 48 (Tablet), 72 (TV). */
@Composable
fun Knopf(text: String, onClick: () -> Unit, modifier: Modifier = Modifier, primary: Boolean = false, icon: String? = null, an: Boolean = false) {
    val tv = LocalTv.current
    val h = if (tv) 72.dp else if (LocalBreit.current) 48.dp else 44.dp
    val fg = if (primary) K.AufMarke else K.Text
    Row(
        modifier.height(h).klick(onClick)
            .background(if (primary) K.Marke else if (an) K.Flaeche3 else Color.Transparent)
            .then(if (primary) Modifier else Modifier.border(1.dp, if (an) K.Text else K.LinieStark, RoundedCornerShape(Tokens.Radius.RadiusS)))
            .padding(horizontal = if (tv) 32.dp else if (text.isEmpty()) 0.dp else 16.dp),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.Center,
    ) {
        if (icon != null) Ico(icon, if (tv) 32.dp else 22.dp, fg, voll = primary && icon == Ic.ABSPIELEN)
        if (icon != null && text.isNotEmpty()) Spacer(Modifier.width(if (tv) 16.dp else 10.dp))
        if (text.isNotEmpty()) T(text, if (tv) LocalTypo.current.text else 16.sp, fg, if (primary) FontWeight.SemiBold else FontWeight.Medium, maxLines = 1)
    }
}

/** Quadratischer Icon-Knopf (Kopfzeile, Aktionen). [rahmen] zeichnet den 1-px-Rahmen wie auf Tablet/TV. */
@Composable
fun IconKnopf(d: String, onClick: () -> Unit, modifier: Modifier = Modifier, groesse: Dp = if (LocalTv.current) 72.dp else 48.dp,
              farbe: Color = K.Text, voll: Boolean = false, rahmen: Boolean = false, an: Boolean = false) {
    Box(
        modifier.size(groesse).klick(onClick)
            .then(if (an) Modifier.background(K.Flaeche3) else Modifier)
            .then(if (rahmen || an) Modifier.border(1.dp, if (an) K.Text else K.LinieStark, RoundedCornerShape(Tokens.Radius.RadiusS)) else Modifier),
        contentAlignment = Alignment.Center,
    ) { Ico(d, if (LocalTv.current) 32.dp else 24.dp, farbe, voll) }
}

/** Filter-Chip: an = Fläche 3 mit hellem Rahmen und Haken. */
@Composable
fun Chip(text: String, an: Boolean, onClick: () -> Unit, punkt: String? = null) {
    val tv = LocalTv.current
    Row(
        Modifier.height(if (tv) 56.dp else 36.dp).klick(onClick).background(if (an) K.Flaeche3 else Color.Transparent)
            .border(1.dp, if (an) K.Text else K.LinieStark, RoundedCornerShape(Tokens.Radius.RadiusS)).padding(horizontal = if (tv) 24.dp else 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (an) { Ico(Ic.HAKEN, if (tv) 24.dp else 16.dp, K.Text, strich = 2f); Spacer(Modifier.width(6.dp)) }
        if (punkt != null) { AmpelPunkt(punkt); Spacer(Modifier.width(6.dp)) }
        T(text, if (tv) LocalTypo.current.klein else 14.sp, if (an) K.Text else K.Text2, maxLines = 1)
    }
}

/** Schalter wie im Entwurf (44 × 24, Knopf 18). */
@Composable
fun Schalter(an: Boolean, modifier: Modifier = Modifier) {
    Box(
        modifier.size(44.dp, 24.dp).clip(RoundedCornerShape(12.dp)).background(if (an) K.Marke else Color.Transparent)
            .then(if (an) Modifier else Modifier.border(1.dp, K.LinieStark, RoundedCornerShape(12.dp))).padding(horizontal = 3.dp),
        contentAlignment = if (an) Alignment.CenterEnd else Alignment.CenterStart,
    ) { Box(Modifier.size(18.dp).clip(CircleShape).background(if (an) K.Saal else K.Text2)) }
}

/** Fortschrittslinie: papierweiß auf 22 % Weiß (4 px, TV 6 px). */
@Composable
fun Fortschritt(frac: Float, modifier: Modifier = Modifier, hoehe: Dp = if (LocalTv.current) 6.dp else 4.dp, farbe: Color = K.Gesehen,
                grund: Color = K.Gesehen.copy(alpha = 0.22f)) {
    Box(modifier.fillMaxWidth().height(hoehe).background(grund)) {
        Box(Modifier.fillMaxWidth(frac.coerceIn(0f, 1f)).height(hoehe).background(farbe))
    }
}

/** Kleines Plättchen oben auf Kacheln (Ampel, Haken, Anzahl neuer Folgen). */
@Composable
private fun Plaettchen(modifier: Modifier, hell: Boolean = false, content: @Composable () -> Unit) {
    val s = if (LocalTv.current) 32.dp else 24.dp
    Box(modifier.defaultMinSize(s, s).height(s).background(if (hell) K.Marke else K.BadgeGrund, RoundedCornerShape(Tokens.Radius.RadiusS))
        .padding(horizontal = if (hell) 6.dp else 0.dp), contentAlignment = Alignment.Center) { content() }
}

/**
 * Hochformat-Kachel (2:3). Ohne Poster: Tonfläche mit Titel in der Plakatschrift und Mono-Zeile unten,
 * wie im Entwurf. [ampelLinks]: Bibliotheksraster zeigt die Ampel oben links.
 */
@Composable
fun PosterKarte(
    titel: String, unter: String, mono: String, bild: String, farbe: String, onClick: () -> Unit, modifier: Modifier = Modifier,
    ampel: String? = null, gesehen: Boolean = false, neu: Int = 0, fortschritt: Float = 0f, onLang: (() -> Unit)? = null,
) {
    val tv = LocalTv.current
    val typo = LocalTypo.current
    Column(modifier) {
        Box(Modifier.fillMaxWidth().aspectRatio(2f / 3f).klick(onClick, onLang).background(ton(farbe))) {
            if (bild.isEmpty()) Column(Modifier.fillMaxSize().padding(if (tv) 16.dp else 10.dp), verticalArrangement = Arrangement.SpaceBetween) {
                PlakatTitel(titel, if (tv) 36.sp else if (LocalBreit.current) 24.sp else 17.sp, Modifier.padding(top = if (ampel != null) 22.dp else 0.dp, end = 16.dp), maxLines = 5)
                if (mono.isNotEmpty()) Column {
                    Box(Modifier.fillMaxWidth().height(1.dp).background(K.Text.copy(alpha = 0.28f)))
                    T(mono.uppercase(), if (tv) 22.sp else 10.sp, K.Text2, family = Mono, maxLines = 1, modifier = Modifier.padding(top = 5.dp))
                }
            } else AsyncImage(bild, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
            val rand = Modifier.padding(if (tv) 12.dp else 8.dp)
            if (ampel != null) Plaettchen(Modifier.align(Alignment.TopStart).then(rand)) { AmpelPunkt(ampel) }
            when {
                neu > 0 -> Plaettchen(Modifier.align(Alignment.TopEnd).then(rand), hell = true) { T("$neu", if (tv) 22.sp else 12.sp, K.AufMarke, family = Mono) }
                gesehen -> Plaettchen(Modifier.align(Alignment.TopEnd).then(rand)) { Ico(Ic.HAKEN, if (tv) 22.dp else 16.dp, K.Text, strich = 2f) }
            }
            if (fortschritt > 0f) Fortschritt(fortschritt, Modifier.align(Alignment.BottomStart))
        }
        T(titel, typo.karte, K.Text, FontWeight.Medium, maxLines = 1, modifier = Modifier.padding(top = if (tv) 12.dp else 6.dp))
        if (unter.isNotEmpty()) T(unter, typo.klein, K.Text2, maxLines = 1)
    }
}

/** Querformat-Kachel (16:9) für Weiterschauen, Als Nächstes und Folgen. */
@Composable
fun BreitKarte(
    titel: String, unter: String, bild: String, farbe: String, onClick: () -> Unit, modifier: Modifier = Modifier,
    plakat: String = titel, ampel: String? = null, rest: String? = null, nr: String? = null, fortschritt: Float = 0f,
    gesehen: Boolean = false, onLang: (() -> Unit)? = null,
) {
    val tv = LocalTv.current
    val typo = LocalTypo.current
    Column(modifier) {
        Box(Modifier.fillMaxWidth().aspectRatio(16f / 9f).klick(onClick, onLang).background(ton(farbe))) {
            if (bild.isNotEmpty()) AsyncImage(bild, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
            else PlakatTitel(plakat, if (tv) 44.sp else if (LocalBreit.current) 30.sp else 26.sp,
                Modifier.padding(start = if (tv) 20.dp else 12.dp, top = if (tv) 16.dp else 10.dp, end = if (tv) 72.dp else 48.dp))
            val rand = Modifier.padding(if (tv) 12.dp else 8.dp)
            if (nr != null) T(nr, if (tv) 44.sp else 32.sp, K.Text.copy(alpha = 0.35f), family = Mono,
                modifier = Modifier.align(Alignment.BottomStart).padding(start = if (tv) 20.dp else 12.dp, bottom = 8.dp))
            if (rest != null) T(rest.uppercase(), if (tv) 22.sp else 11.sp, K.Text, family = Mono, spacing = 0.1.em,
                modifier = Modifier.align(Alignment.BottomStart).padding(start = if (tv) 20.dp else 12.dp, bottom = 14.dp)
                    .background(K.BadgeGrund, RoundedCornerShape(Tokens.Radius.RadiusS)).padding(horizontal = 6.dp, vertical = 1.dp))
            if (ampel != null) Plaettchen(Modifier.align(Alignment.TopEnd).then(rand)) { AmpelPunkt(ampel) }
            else if (gesehen) Plaettchen(Modifier.align(Alignment.TopEnd).then(rand)) { Ico(Ic.HAKEN, if (tv) 22.dp else 16.dp, K.Text, strich = 2f) }
            if (fortschritt > 0f) Fortschritt(fortschritt, Modifier.align(Alignment.BottomStart))
        }
        T(titel, typo.karte, K.Text, FontWeight.Medium, maxLines = 1, modifier = Modifier.padding(top = if (tv) 12.dp else 6.dp))
        if (unter.isNotEmpty()) T(unter, typo.klein, K.Text2, maxLines = 1)
    }
}

/** Kachel unter „Meine Medien“: Vorschaubild ([bild], abgedunkelt) oder Tonfläche, Icon oben rechts, Name in Plakatschrift, Anzahl in Mono. */
@Composable
fun MedienKachel(name: String, anzahl: String, icon: String, farbe: Color, onClick: () -> Unit, modifier: Modifier = Modifier, bild: String = "") {
    val tv = LocalTv.current
    Box(modifier.height(if (tv) 92.dp else if (LocalBreit.current) 88.dp else 80.dp).klick(onClick).background(farbe)) {
        if (bild.isNotEmpty()) {
            AsyncImage(bild, null, Modifier.matchParentSize(), contentScale = ContentScale.Crop)
            Box(Modifier.matchParentSize().background(Brush.verticalGradient(listOf(K.Saal.copy(alpha = 0.25f), K.Saal.copy(alpha = 0.85f)))))
        }
        Box(Modifier.fillMaxSize().padding(horizontal = if (tv) 20.dp else 12.dp, vertical = if (tv) 14.dp else 10.dp)) {
            Ico(icon, if (tv) 32.dp else 20.dp, K.Text.copy(alpha = 0.7f), modifier = Modifier.align(Alignment.TopEnd))
            Column(Modifier.align(Alignment.BottomStart)) {
                T(name.uppercase(), if (tv) 36.sp else 20.sp, K.Text, family = Plakat, maxLines = 1)
                T(anzahl.uppercase(), if (tv) 22.sp else 10.sp, K.Text2, family = Mono, spacing = 0.1.em, maxLines = 1)
            }
        }
    }
}

/** Abschnittsüberschrift; mit [onMehr] klickbar und mit Pfeil (wie „Als Nächstes ›“). */
@Composable
fun Abschnitt(titel: String, onMehr: (() -> Unit)? = null, modifier: Modifier = Modifier, zahl: String? = null) {
    val typo = LocalTypo.current
    Row(modifier.then(if (onMehr != null) Modifier.klick(onMehr, skala = false) else Modifier), verticalAlignment = Alignment.CenterVertically) {
        T(titel, typo.reihe, K.Text, FontWeight.SemiBold, maxLines = 1)
        if (zahl != null) T(zahl, typo.label, K.Text3, family = Mono, spacing = 0.15.em, modifier = Modifier.padding(start = 8.dp))
        if (onMehr != null) Ico(Ic.WEITER, if (LocalTv.current) 28.dp else 20.dp, K.Text2)
    }
}

/** Altersfreigabe als umrandetes Mono-Kästchen. */
@Composable
fun Fsk(age: Int) = T("FSK $age", if (LocalTv.current) 22.sp else 12.sp, K.Text, family = Mono,
    modifier = Modifier.border(1.dp, K.LinieStark, RoundedCornerShape(Tokens.Radius.RadiusS)).padding(horizontal = 6.dp, vertical = 1.dp))

@Composable
fun Avatar(name: String, color: Int, size: Dp) {
    val palette = listOf(K.Avatar1, K.Avatar2, K.Avatar3, K.Avatar4, K.Avatar5, K.Avatar6)
    Box(Modifier.size(size).background(palette[(color / 60).mod(palette.size)], RoundedCornerShape(Tokens.Radius.RadiusS)), contentAlignment = Alignment.Center) {
        T(name.take(1).uppercase(), (size.value * 0.56f).sp, K.Saal, family = Plakat)
    }
}

/** Personen-Kachel (Besetzung): Bild oder Initialen auf Avatarfarbe. */
@Composable
fun PersonKarte(name: String, rolle: String, bild: String, onClick: () -> Unit) {
    val tv = LocalTv.current
    val w = if (tv) 160.dp else 88.dp
    val palette = listOf(K.Avatar1, K.Avatar2, K.Avatar3, K.Avatar4, K.Avatar5, K.Avatar6)
    Column(Modifier.width(w)) {
        Box(Modifier.size(w).klick(onClick).background(palette[name.hashCode().mod(palette.size)]), contentAlignment = Alignment.Center) {
            T(name.split(' ').filter { it.isNotEmpty() }.take(2).joinToString("") { it.take(1) }.uppercase(), if (tv) 64.sp else 40.sp, K.Saal, family = Plakat)
            if (bild.isNotEmpty()) AsyncImage(bild, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
        }
        T(name, if (tv) LocalTypo.current.klein else 14.sp, K.Text, FontWeight.Medium, maxLines = 2, modifier = Modifier.padding(top = 8.dp))
        if (rolle.isNotEmpty()) T(rolle, LocalTypo.current.klein, K.Text2, maxLines = 1)
    }
}

fun fmtTime(sec: Double): String {
    val s = sec.toLong()
    val h = s / 3600
    val m = (s % 3600) / 60
    return if (h > 0) "%d:%02d:%02d".format(h, m, s % 60) else "%d:%02d".format(m, s % 60)
}

/** „2 Std. 29 Min.“ wie im Entwurf */
fun fmtDauer(sec: Double): String {
    val m = (sec / 60).toInt()
    return if (m >= 60) "${m / 60} Std. ${m % 60} Min." else "$m Min."
}

/** „2:28“ für die Mono-Zeile der Kacheln. */
fun fmtKurz(sec: Double): String {
    val m = (sec / 60).toInt()
    return "%d:%02d".format(m / 60, m % 60)
}

/** „endet um 21:14“ ab jetzt. */
fun endetUm(restSec: Double): String =
    java.text.SimpleDateFormat("HH:mm", java.util.Locale.GERMANY).format(java.util.Date(System.currentTimeMillis() + (restSec * 1000).toLong()))

fun Item.anteil(): Float = if (progress > 0 && duration > 0 && !watched) (progress / duration).toFloat() else 0f
