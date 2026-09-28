package io.flimmer.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil3.compose.AsyncImage
import io.flimmer.app.Item

val Bg = Color(0xFF0B0D12)
val Surface = Color(0xFF151923)
val TextColor = Color(0xFFEEF1F7)
val Muted = Color(0xFF8B93A7)
val Accent = Color(0xFF7C9CFF)

/** true auf Android TV / Fire TV: Bausteine mit D-Pad-Fokus (androidx.tv) statt Touch-Material. */
val LocalTv = staticCompositionLocalOf { false }

@Composable
fun FlimmerTheme(tv: Boolean, content: @Composable () -> Unit) {
    val scheme = darkColorScheme(primary = Accent, background = Bg, surface = Surface, onSurface = TextColor, onBackground = TextColor)
    MaterialTheme(colorScheme = scheme) {
        androidx.tv.material3.MaterialTheme(
            colorScheme = androidx.tv.material3.darkColorScheme(primary = Accent, background = Bg, surface = Surface, onSurface = TextColor),
        ) {
            CompositionLocalProvider(LocalTv provides tv, content = content)
        }
    }
}

fun lightColor(light: String) = when (light) {
    "green" -> Color(0xFF35C878)
    "yellow" -> Color(0xFFF5C043)
    else -> Color(0xFFFF5D5D)
}

fun lightText(light: String) = when (light) {
    "green" -> "Läuft direkt – ohne Umwandlung"
    "yellow" -> "Läuft flüssig – der Server wandelt einen Teil um"
    else -> "Muss umgewandelt werden – der Server ist dafür knapp"
}

private fun hue(s: String) = s.fold(0) { h, c -> (h * 31 + c.code) % 360 }.toFloat()

/** Bild vom Server; ohne Bild ein aus dem Namen erzeugter Farbverlauf mit Initiale (wie in der Web-UI). */
@Composable
fun Art(url: String, name: String, modifier: Modifier = Modifier) {
    val h = hue(name)
    Box(
        modifier.background(Brush.linearGradient(listOf(Color.hsl(h, 0.55f, 0.38f), Color.hsl((h + 60) % 360, 0.6f, 0.16f)))),
        contentAlignment = Alignment.Center,
    ) {
        Text(name.take(1).uppercase(), fontSize = 48.sp, fontWeight = FontWeight.Bold, color = TextColor.copy(alpha = 0.85f))
        if (url.isNotEmpty()) AsyncImage(url, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
    }
}

@Composable
fun ClickCard(onClick: () -> Unit, modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    if (LocalTv.current) {
        androidx.tv.material3.Card(
            onClick = onClick,
            modifier = modifier,
            shape = androidx.tv.material3.CardDefaults.shape(RoundedCornerShape(12.dp)),
            scale = androidx.tv.material3.CardDefaults.scale(focusedScale = 1.06f),
            border = androidx.tv.material3.CardDefaults.border(
                focusedBorder = androidx.tv.material3.Border(androidx.compose.foundation.BorderStroke(3.dp, Accent)),
            ),
        ) { content() }
    } else {
        androidx.compose.material3.Card(onClick = onClick, modifier = modifier, shape = RoundedCornerShape(12.dp)) { content() }
    }
}

@Composable
fun FButton(text: String, onClick: () -> Unit, modifier: Modifier = Modifier, primary: Boolean = false) {
    if (LocalTv.current) {
        androidx.tv.material3.Button(onClick = onClick, modifier = modifier) { androidx.tv.material3.Text(text) }
    } else if (primary) {
        androidx.compose.material3.Button(onClick = onClick, modifier = modifier) { Text(text) }
    } else {
        androidx.compose.material3.OutlinedButton(onClick = onClick, modifier = modifier) { Text(text) }
    }
}

@Composable
fun PosterCard(item: Item, url: String, wide: Boolean, title: String, sub: String, onClick: () -> Unit) {
    val w = if (wide) (if (LocalTv.current) 300.dp else 240.dp) else (if (LocalTv.current) 180.dp else 130.dp)
    Column(Modifier.width(w)) {
        ClickCard(onClick, Modifier.fillMaxWidth().aspectRatio(if (wide) 16f / 9f else 2f / 3f)) {
            Box(Modifier.fillMaxSize()) {
                Art(url, item.series ?: item.displayTitle, Modifier.fillMaxSize().clip(RoundedCornerShape(12.dp)))
                Box(Modifier.align(Alignment.TopEnd).padding(8.dp).size(10.dp).clip(CircleShape).background(lightColor(item.light)))
                if (item.progress > 0 && item.duration > 0 && !item.watched) {
                    LinearProgressIndicator(
                        progress = { (item.progress / item.duration).toFloat().coerceIn(0f, 1f) },
                        modifier = Modifier.align(Alignment.BottomCenter).fillMaxWidth().padding(8.dp),
                        color = Accent,
                        trackColor = Color.White.copy(alpha = 0.25f),
                    )
                }
            }
        }
        Text(title, Modifier.padding(top = 6.dp), color = TextColor, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
        if (sub.isNotEmpty()) Text(sub, color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

fun fmtTime(sec: Double): String {
    val s = sec.toLong()
    val h = s / 3600
    val m = (s % 3600) / 60
    return if (h > 0) "%d:%02d:%02d".format(h, m, s % 60) else "%d:%02d".format(m, s % 60)
}
