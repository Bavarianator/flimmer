package io.flimmer.app.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
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
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontVariation
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
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

/** Schriftgrößen je Gerät: TV (10-Fuß) oder Handy. */
data class Typo(val plakat: TextUnit, val titel: TextUnit, val reihe: TextUnit, val karte: TextUnit, val text: TextUnit, val klein: TextUnit, val label: TextUnit, val code: TextUnit)

private val TypoTv = Typo(Tokens.TypoTv.Plakat, Tokens.TypoTv.Titel, Tokens.TypoTv.Reihe, Tokens.TypoTv.Karte, Tokens.TypoTv.Text, Tokens.TypoTv.Klein, Tokens.TypoTv.Label, Tokens.TypoTv.Code)
private val TypoHd = Typo(Tokens.TypoHd.Plakat, Tokens.TypoHd.Titel, Tokens.TypoHd.Reihe, Tokens.TypoHd.Karte, Tokens.TypoHd.Text, Tokens.TypoHd.Klein, Tokens.TypoHd.Label, Tokens.TypoHd.Code)

/** true auf Android TV / Fire TV: Bausteine mit D-Pad-Fokus (androidx.tv) statt Touch-Material. */
val LocalTv = staticCompositionLocalOf { false }
val LocalTypo = staticCompositionLocalOf { TypoHd }

@Composable
fun FlimmerTheme(tv: Boolean, content: @Composable () -> Unit) {
    val scheme = darkColorScheme(
        primary = K.Marke, onPrimary = K.AufMarke, background = K.Saal, surface = K.Flaeche1,
        surfaceVariant = K.Flaeche2, onSurface = K.Text, onBackground = K.Text, outline = K.LinieStark,
    )
    MaterialTheme(colorScheme = scheme, typography = MaterialTheme.typography.let { t ->
        t.copy(bodyLarge = t.bodyLarge.copy(fontFamily = Sans), bodyMedium = t.bodyMedium.copy(fontFamily = Sans), labelLarge = t.labelLarge.copy(fontFamily = Sans))
    }) {
        androidx.tv.material3.MaterialTheme(
            colorScheme = androidx.tv.material3.darkColorScheme(primary = K.Marke, onPrimary = K.AufMarke, background = K.Saal, surface = K.Flaeche1, onSurface = K.Text),
        ) {
            CompositionLocalProvider(LocalTv provides tv, LocalTypo provides if (tv) TypoTv else TypoHd, content = content)
        }
    }
}

val Pad: androidx.compose.ui.unit.Dp @Composable get() = if (LocalTv.current) 96.dp else 16.dp

@Composable
fun T(text: String, size: TextUnit = LocalTypo.current.text, color: Color = K.Text, weight: FontWeight = FontWeight.Normal,
      family: FontFamily = Sans, maxLines: Int = Int.MAX_VALUE, modifier: Modifier = Modifier, spacing: TextUnit = TextUnit.Unspecified) =
    Text(text, modifier, color = color, maxLines = maxLines, overflow = TextOverflow.Ellipsis,
        style = TextStyle(fontFamily = family, fontSize = size, fontWeight = weight, letterSpacing = spacing))

/** Kleines Versal-Label in Plex Mono, z. B. „WEITERSCHAUEN“. */
@Composable
fun Label(text: String, modifier: Modifier = Modifier) =
    T(text.uppercase(), LocalTypo.current.label, K.Text2, family = Mono, spacing = 0.18.em, modifier = modifier)

/** Plakat-Titel (Flimmer Plakat, Versalien). */
@Composable
fun PlakatTitel(text: String, size: TextUnit = LocalTypo.current.plakat, modifier: Modifier = Modifier) =
    T(text.uppercase(), size, K.Text, family = Plakat, maxLines = 2, modifier = modifier)

fun lightColor(light: String) = when (light) {
    "green" -> K.AmpelGruen
    "yellow" -> K.AmpelGelb
    else -> K.AmpelRot
}

fun lightText(light: String) = when (light) {
    "green" -> "Läuft direkt"
    "yellow" -> "Server wandelt teilweise um"
    else -> "Server ist knapp"
}

@Composable
fun Ampel(light: String, withText: Boolean = true) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(if (LocalTv.current) 14.dp else 10.dp).clip(CircleShape).background(lightColor(light)))
        if (withText) T(lightText(light), LocalTypo.current.klein, K.Text2, modifier = Modifier.padding(start = 10.dp))
    }
}

/** Bild vom Server; ohne Bild eine ruhige Tonfläche mit Initiale in der Plakatschrift. */
@Composable
fun Art(url: String, name: String, modifier: Modifier = Modifier) {
    Box(modifier.background(K.Flaeche2), contentAlignment = Alignment.Center) {
        T(name.take(1).uppercase(), LocalTypo.current.titel, K.Text3, family = Plakat)
        if (url.isNotEmpty()) AsyncImage(url, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
    }
}

/** Fokus wie im Web: papierweißer Rahmen mit dunklem Abstand, Skalierung 1.04, kein Glow. */
@Composable
fun ClickCard(onClick: () -> Unit, modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    val shape = RoundedCornerShape(Tokens.Radius.RadiusS)
    if (LocalTv.current) {
        androidx.tv.material3.Card(
            onClick = onClick,
            modifier = modifier,
            shape = androidx.tv.material3.CardDefaults.shape(shape),
            colors = androidx.tv.material3.CardDefaults.colors(containerColor = K.Flaeche2),
            scale = androidx.tv.material3.CardDefaults.scale(focusedScale = Tokens.Dauer.FokusScale),
            border = androidx.tv.material3.CardDefaults.border(
                focusedBorder = androidx.tv.material3.Border(BorderStroke(Tokens.Masse.FokusRingTv, K.Text), inset = 3.dp, shape = shape),
            ),
            glow = androidx.tv.material3.CardDefaults.glow(),
        ) { content() }
    } else {
        androidx.compose.material3.Card(onClick = onClick, modifier = modifier, shape = shape,
            colors = androidx.compose.material3.CardDefaults.cardColors(containerColor = K.Flaeche2)) { content() }
    }
}

@Composable
fun FButton(text: String, onClick: () -> Unit, modifier: Modifier = Modifier, primary: Boolean = false) {
    val shape = RoundedCornerShape(Tokens.Radius.RadiusM)
    val size = LocalTypo.current.text
    if (LocalTv.current) {
        androidx.tv.material3.Button(
            onClick = onClick, modifier = modifier, shape = androidx.tv.material3.ButtonDefaults.shape(shape),
            colors = if (primary) androidx.tv.material3.ButtonDefaults.colors(containerColor = K.Marke, contentColor = K.AufMarke, focusedContainerColor = K.Marke, focusedContentColor = K.AufMarke)
            else androidx.tv.material3.ButtonDefaults.colors(containerColor = Color.Transparent, contentColor = K.Text, focusedContainerColor = K.Flaeche3, focusedContentColor = K.Text),
            border = androidx.tv.material3.ButtonDefaults.border(
                border = androidx.tv.material3.Border(BorderStroke(1.dp, if (primary) Color.Transparent else K.LinieStark), shape = shape),
                focusedBorder = androidx.tv.material3.Border(BorderStroke(Tokens.Masse.FokusRingTv, K.Text), inset = 3.dp, shape = shape),
            ),
            scale = androidx.tv.material3.ButtonDefaults.scale(focusedScale = Tokens.Dauer.FokusScale),
            glow = androidx.tv.material3.ButtonDefaults.glow(),
        ) { androidx.tv.material3.Text(text, style = TextStyle(fontFamily = Sans, fontSize = size, fontWeight = FontWeight.SemiBold)) }
    } else if (primary) {
        androidx.compose.material3.Button(onClick, modifier.height(Tokens.Masse.ZielTouch), shape = shape,
            colors = androidx.compose.material3.ButtonDefaults.buttonColors(containerColor = K.Marke, contentColor = K.AufMarke)) {
            T(text, size, K.AufMarke, FontWeight.SemiBold)
        }
    } else {
        androidx.compose.material3.OutlinedButton(onClick, modifier.height(Tokens.Masse.ZielTouch), shape = shape, border = BorderStroke(1.dp, K.LinieStark)) {
            T(text, size, K.Text, FontWeight.Medium)
        }
    }
}

/** Nur Fortschrittslinie wie im Entwurf: papierweiß auf Linie. */
@Composable
fun Fortschritt(frac: Float, modifier: Modifier = Modifier) {
    Box(modifier.fillMaxWidth().height(3.dp).background(K.Linie)) {
        Box(Modifier.fillMaxWidth(frac.coerceIn(0f, 1f)).height(3.dp).background(K.Gesehen))
    }
}

@Composable
fun PosterCard(item: Item, url: String, wide: Boolean, title: String, sub: String, onClick: () -> Unit) {
    val tv = LocalTv.current
    val w = if (wide) (if (tv) Tokens.Masse.KarteBreitTv else Tokens.Masse.KarteBreitHd) else (if (tv) Tokens.Masse.KartePosterTv else Tokens.Masse.KartePosterHd)
    Column(Modifier.width(w)) {
        ClickCard(onClick, Modifier.fillMaxWidth().aspectRatio(if (wide) 16f / 9f else 2f / 3f)) {
            Box(Modifier.fillMaxSize()) {
                Art(url, item.series ?: item.displayTitle, Modifier.fillMaxSize())
                Box(Modifier.align(Alignment.TopEnd).padding(8.dp).background(K.BadgeGrund, RoundedCornerShape(Tokens.Radius.RadiusS)).padding(6.dp)) {
                    Box(Modifier.size(if (tv) 12.dp else 8.dp).clip(CircleShape).background(lightColor(item.light)))
                }
            }
        }
        if (item.progress > 0 && item.duration > 0 && !item.watched) Fortschritt((item.progress / item.duration).toFloat(), Modifier.padding(top = 4.dp))
        T(title, LocalTypo.current.karte, K.Text, FontWeight.Medium, maxLines = 1, modifier = Modifier.padding(top = 8.dp))
        if (sub.isNotEmpty()) T(sub, LocalTypo.current.klein, K.Text2, maxLines = 1)
    }
}

/** Reiter-Eintrag der TV-Kopfleiste: fokussierbar, aktiver Reiter unterstrichen. */
@Composable
fun NavTab(text: String, active: Boolean, onClick: () -> Unit) {
    var focused by remember { mutableStateOf(false) }
    androidx.tv.material3.Surface(
        onClick = onClick,
        modifier = Modifier.onFocusChanged { focused = it.isFocused },
        shape = androidx.tv.material3.ClickableSurfaceDefaults.shape(RoundedCornerShape(Tokens.Radius.RadiusS)),
        colors = androidx.tv.material3.ClickableSurfaceDefaults.colors(containerColor = Color.Transparent, focusedContainerColor = K.Marke),
    ) {
        Column(Modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
            T(text, LocalTypo.current.text, if (focused) K.AufMarke else if (active) K.Text else K.Text2, FontWeight.Medium)
            if (active && !focused) Box(Modifier.padding(top = 4.dp).fillMaxWidth().height(3.dp).background(K.Text))
        }
    }
}

@Composable
fun Avatar(name: String, color: Int, size: androidx.compose.ui.unit.Dp) {
    val palette = listOf(K.Avatar1, K.Avatar2, K.Avatar3, K.Avatar4, K.Avatar5, K.Avatar6)
    Box(Modifier.size(size).background(palette[(color / 60).mod(palette.size)], RoundedCornerShape(Tokens.Radius.RadiusS)), contentAlignment = Alignment.Center) {
        T(name.take(1).uppercase(), (size.value * 0.5f).let { androidx.compose.ui.unit.TextUnit(it, androidx.compose.ui.unit.TextUnitType.Sp) }, K.Saal, family = Plakat)
    }
}

@Composable
fun Zeile(content: @Composable () -> Unit) = Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) { content() }

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
