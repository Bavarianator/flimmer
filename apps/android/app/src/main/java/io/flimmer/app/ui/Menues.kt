package io.flimmer.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.flimmer.app.Item
import io.flimmer.app.User

/** Kontextmenü eines Titels (Handy-Aktionen im Entwurf): „…“ oder langes Drücken. */
@Composable
fun AktionenBlatt(it: Item, d: Daten, h: Handlungen, onTeilen: (() -> Unit)?, onZu: () -> Unit) {
    var wahl by remember { mutableStateOf<Boolean?>(null) } // true = Sammlung wählen, false = Wiedergabeliste wählen
    val resume = it.progress > 5 && !it.watched
    val fav = it.key in d.favs
    val titel = if (it.series != null) "${it.series} · ${folge(it)}" else it.displayTitle
    val fakten = if (it.series != null) it.displayTitle else sub(it)
    val schliessend: (() -> Unit) -> () -> Unit = { f -> { onZu(); f() } }
    Blatt(onZu, { BlattKopf(titel, fakten, d.abs(it.poster), it.color) }) {
        when (val w = wahl) {
            null -> {
                MenueZeile(Ic.ABSPIELEN, if (resume) "Fortsetzen ab ${fmtTime(it.progress)}" else "Abspielen", onClick = schliessend { h.abspielen(it, null) })
                if (resume) MenueZeile(Ic.NEUSTART, "Von vorn abspielen", onClick = schliessend { h.abspielen(it, 0.0) })
                if (it.series != null) {
                    MenueZeile(Ic.ZUFALL, "Serie zufällig abspielen", onClick = schliessend { h.alle(d.serien[it.series].orEmpty().shuffled()) })
                    MenueZeile(Ic.SERIE, "Zur Serie", onClick = schliessend { h.oeffnen(it) })
                } else MenueZeile(Ic.INFO, "Details", onClick = schliessend { h.oeffnen(it) })
                MenueZeile(Ic.HERZ, if (fav) "Aus Favoriten entfernen" else "Zu Favoriten hinzufügen", trenner = true, onClick = schliessend { h.favorit(it.key) })
                MenueZeile(Ic.HAKEN, if (it.watched) "Als ungesehen markieren" else "Als gesehen markieren", onClick = schliessend { h.gesehen(listOf(it), !it.watched) })
                if (d.admin && d.sammlungen != null) MenueZeile(Ic.SAMMLUNG, "Zu Sammlung hinzufügen") { wahl = true }
                if (d.listen != null) MenueZeile(Ic.LISTE, "Zu Wiedergabeliste hinzufügen") { wahl = false }
                h.download?.let { dl -> MenueZeile(Ic.DOWNLOAD, "Herunterladen", trenner = true, onClick = schliessend { dl(listOf(it)) }) }
                MenueZeile(Ic.GEMEINSAM, "Gemeinsam schauen starten", trenner = h.download == null, onClick = schliessend { h.party(it) })
                onTeilen?.let { t -> MenueZeile(Ic.TEILEN, "Link teilen", onClick = schliessend(t)) }
                if (d.admin) h.metadaten?.let { m ->
                    MenueZeile(Ic.BEARBEITEN, "Metadaten bearbeiten", trenner = true, onClick = schliessend { m(it) })
                    MenueZeile(Ic.SUCHE, "Identifizieren", onClick = schliessend { m(it) })
                }
            }
            else -> {
                MenueZeile(Ic.ZURUECK, if (w) "Sammlung wählen" else "Wiedergabeliste wählen") { wahl = null }
                // Sammlungen fassen Serien als Ganzes, Wiedergabelisten einzelne Folgen
                ListenWahl(w, d, h, listOf(if (w) it.key else it.id), onZu)
            }
        }
    }
}
