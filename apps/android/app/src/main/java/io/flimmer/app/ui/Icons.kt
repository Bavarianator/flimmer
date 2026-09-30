package io.flimmer.app.ui

import androidx.compose.foundation.layout.size
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.addPathNodes
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/** Pfade wie im Entwurf und in web/src/components/Icon.tsx (SPEC 5): 24er-Raster, Strich 1,5, eckige Enden. */
object Ic {
    const val MENUE = "M4 6.5h16M4 12h16M4 17.5h16"
    const val CAST = "M3.5 9V5.5h17v13H14M3.5 12.5a6 6 0 0 1 6 6M3.5 15.5a3 3 0 0 1 3 3M3.5 18.5h.5"
    const val SUCHE = "M11 17.5a6.5 6.5 0 1 0 0-13 6.5 6.5 0 0 0 0 13zM16 16l4.5 4.5"
    const val ZURUECK = "M15 5l-7 7 7 7"
    const val WEITER = "M9 5l7 7-7 7"
    const val HOCH = "M5 15l7-7 7 7"
    const val RUNTER = "M5 9l7 7 7-7"
    const val MEHR_V = "M11 5h2v2h-2zM11 11h2v2h-2zM11 17h2v2h-2z"
    const val MEHR = "M5 11h2v2H5zM11 11h2v2h-2zM17 11h2v2h-2z"
    const val START = "M4 11l8-7 8 7M6 9.5V20h12V9.5"
    const val HERZ = "M12 19.5l-7.2-7.1a4.3 4.3 0 0 1 6.1-6.1l1.1 1.1 1.1-1.1a4.3 4.3 0 0 1 6.1 6.1z"
    const val DOWNLOAD = "M12 4v11M7 10l5 5 5-5M5 20h14"
    const val HOCHLADEN = "M12 15V4M7 9l5-5 5 5M5 20h14"
    const val FILM = "M4 5h16v14H4zM8 5v14M16 5v14M4 9.5h4M4 14.5h4M16 9.5h4M16 14.5h4"
    const val SERIE = "M4 7h16v12H4zM8 3.5l4 3.5 4-3.5"
    const val MUSIK = "M9 17.5V5.5l10-2v12M9 17.5a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0zM19 15.5a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0z"
    const val LIVE = "M3.5 6.5h17v11h-17zM8 20.5h8M10.5 9.5v5l4-2.5z"
    const val SAMMLUNG = "M4 7.5h13v12.5H4zM7 4.5h13V17"
    const val LISTE = "M4 6h11M4 11h11M4 16h6M14 14v6l5-3z"
    const val DASHBOARD = "M4 4h7v9H4zM13 4h7v5h-7zM13 11h7v9h-7zM4 15h7v5H4z"
    const val BEARBEITEN = "M4 20h4.5L19.5 9 15 4.5 4 15.5zM12.5 7l4.5 4.5"
    const val EINSTELLUNGEN = "M4 7h10M18 7h2M4 17h2M10 17h10M16 4.5v5M8 14.5v5"
    const val SERVER = "M4 4.5h16v6H4zM4 13.5h16v6H4zM7.5 7.5h1M7.5 16.5h1"
    const val ABMELDEN = "M10 4H4.5v16H10M15 7.5l4.5 4.5-4.5 4.5M19.5 12H9"
    const val PROFIL = "M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4.5 20a7.5 7.5 0 0 1 15 0"
    const val WECHSEL = "M4 11.5V8h14.5l-3-3M20 12.5V16H5.5l3 3"
    const val SCHLUESSEL = "M8 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7zM11.5 12H20M17.5 12v3M14.5 12v2"
    const val RASTER = "M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z"
    const val HANDY = "M7 3.5h10v17H7zM11 17.5h2"
    const val QR = "M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h2.5v2.5H14zM17.5 17.5H20V20h-2.5zM14 20h1M20 14v1"
    const val HAKEN = "M5 12.5l4.5 4.5L19 7.5"
    const val NEUSTART = "M4 12a8 8 0 1 0 2.5-5.8M4 4v4.5h4.5"
    const val VOR = "M20 12a8 8 0 1 1-2.5-5.8M20 4v4.5h-4.5"
    const val ABSPIELEN = "M8 5.5v13a1 1 0 0 0 1.5.9l10.4-6.5a1 1 0 0 0 0-1.8L9.5 4.6A1 1 0 0 0 8 5.5z"
    const val PAUSE = "M8 5v14M16 5v14"
    const val NAECHSTE = "M18 5v14M6 5.5l9 6.5-9 6.5z"
    const val ZUFALL = "M4 7.5h3l9 9h4M4 16.5h3l2.5-2.5M13.5 10L16 7.5h4M17.5 5l2.5 2.5-2.5 2.5M17.5 14l2.5 2.5-2.5 2.5"
    const val SORTIEREN = "M7.5 4v16M4 7.5L7.5 4 11 7.5M16.5 20V4M13 16.5l3.5 3.5 3.5-3.5"
    const val FILTER = "M4 6h16M7 12h10M10 18h4"
    const val TON = "M4 9.5v5h4l5 4v-13l-5 4zM17 9a4 4 0 0 1 0 6"
    const val UNTERTITEL = "M3.5 5.5h17v13h-17zM7 12.5h4M13 12.5h4M7 15.5h7"
    const val QUALITAET = "M3.5 5.5h17v13h-17zM7.5 9v6M7.5 12h3.5M11 9v6M14 9v6h1.5a3 3 0 0 0 0-6z"
    const val INFO = "M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 11v6M12 7.5v.5"
    const val UHR = "M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 7v5.5l3.5 2"
    const val LOESCHEN = "M4 7h16M9 7V4h6v3M6.5 7l1 13h9l1-13"
    const val GEMEINSAM = "M9 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM3.5 19.5a5.5 5.5 0 0 1 11 0M15.5 5.2a3 3 0 0 1 0 5.6M17 14.2a5.5 5.5 0 0 1 3.5 5.3"
    const val TEILEN = "M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1"
    const val STERN = "M12 3.5l2.6 5.3 5.9.9-4.3 4.1 1 5.8-5.2-2.7-5.2 2.7 1-5.8-4.3-4.1 5.9-.9z"
    const val SCHLIESSEN = "M6 6l12 12M18 6L6 18"
    const val SEITENVERHAELTNIS = "M4 9V4h5M15 4h5v5M20 15v5h-5M9 20H4v-5"
    const val PIP = "M3.5 5.5h17v13h-17zM12 11.5h5.5V16H12z"
}

/** Strich-Icon aus einem SVG-Pfad; [voll] füllt die Form (Herz gesetzt, Abspielen, Stern). */
@Composable
fun Ico(d: String, size: Dp = 24.dp, color: Color = K.Text, voll: Boolean = false, modifier: Modifier = Modifier, strich: Float = 1.5f) {
    val v = remember(d, voll, strich) {
        ImageVector.Builder(defaultWidth = 24.dp, defaultHeight = 24.dp, viewportWidth = 24f, viewportHeight = 24f)
            .addPath(addPathNodes(d), fill = if (voll) SolidColor(Color.Black) else null, stroke = SolidColor(Color.Black),
                strokeLineWidth = strich, strokeLineCap = StrokeCap.Square, strokeLineJoin = StrokeJoin.Miter)
            .build()
    }
    Icon(v, null, modifier.size(size), tint = color)
}
