package io.flimmer.app.einstellungen

import android.net.Uri
import android.provider.OpenableColumns
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import io.flimmer.app.ApiClient
import io.flimmer.app.ui.*
import kotlinx.coroutines.launch

// Videos hochladen wie web/src/screens/Hochladen.tsx: nur mit Recht „Hochladen“ (vergibt der Admin unter Dashboard ›
// Benutzer). System-Dateiwahl, dann nacheinander POST /api/upload; der Server legt sie in seinen Upload-Ordner und scannt.
// ponytail: läuft nur, solange die Seite offen ist; für Hintergrund-Uploads bräuchte es einen Foreground-Service.

private class Datei(val name: String) {
    var anteil by mutableFloatStateOf(0f)
    var status by mutableStateOf("")
    var fehler by mutableStateOf(false)
}

@Composable
internal fun ColumnScope.Hochladen(api: ApiClient) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val typo = LocalTypo.current
    val liste = remember { mutableStateListOf<Datei>() }
    val waehlen = rememberLauncherForActivityResult(ActivityResultContracts.GetMultipleContents()) { uris: List<Uri> ->
        val neu = uris.map { u -> u to Datei(ctx.contentResolver.query(u, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)
            ?.use { c -> if (c.moveToFirst()) c.getString(0) else null } ?: "video.mp4") }
        liste.addAll(neu.map { it.second })
        scope.launch {
            for ((u, d) in neu) { // nacheinander: das NAS schreibt eine Datei zur Zeit
                val groesse = ctx.contentResolver.query(u, arrayOf(OpenableColumns.SIZE), null, null, null)
                    ?.use { c -> if (c.moveToFirst() && !c.isNull(0)) c.getLong(0) else -1L } ?: -1L
                runCatching { api.upload(d.name, groesse, { ctx.contentResolver.openInputStream(u)!! }) { d.anteil = it } }
                    .onSuccess { d.anteil = 1f; d.status = "fertig" }
                    .onFailure { e -> d.fehler = true; d.status = e.message ?: "Fehler" }
            }
        }
    }
    T("Filme und Videos kommen in den Upload-Ordner von Flimmer und erscheinen nach dem Scan in der Bibliothek. " +
        "Serienfolgen erkennt Flimmer am Namen, z. B. „Serie S01E02.mkv“. Lass diese Seite offen, bis alles hochgeladen ist.",
        typo.klein, K.Text2, modifier = Modifier.padding(bottom = 16.dp))
    Knopf("Dateien wählen", { waehlen.launch("video/*") }, primary = true, icon = Ic.HOCHLADEN)
    liste.forEach { d ->
        Column(Modifier.padding(top = 16.dp)) {
            Row {
                T(d.name, typo.klein, K.Text, maxLines = 1, modifier = Modifier.weight(1f))
                T(d.status.ifEmpty { "${(d.anteil * 100).toInt()} %" }, typo.klein, if (d.fehler) K.AmpelRot else K.Text2,
                    maxLines = 2, modifier = Modifier.padding(start = 12.dp))
            }
            Fortschritt(d.anteil, Modifier.fillMaxWidth().padding(top = 6.dp))
        }
    }
}
