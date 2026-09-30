package io.flimmer.app.admin

import android.graphics.BitmapFactory
import android.util.Base64
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.FilterQuality
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import io.flimmer.app.ApiClient
import io.flimmer.app.User
import io.flimmer.app.ui.*
import kotlinx.serialization.json.add
import kotlinx.serialization.json.put
import kotlinx.serialization.json.putJsonArray

/** Liste links, Bearbeiten rechts; schmal untereinander. */
@Composable
internal fun Spalten(ab: Dp = 720.dp, links: @Composable ColumnScope.() -> Unit, rechts: @Composable ColumnScope.() -> Unit) {
    BoxWithConstraints(Modifier.fillMaxWidth()) {
        if (maxWidth >= ab) Row(horizontalArrangement = Arrangement.spacedBy(if (LocalTv.current) 48.dp else 32.dp)) {
            Column(Modifier.weight(0.42f), content = links)
            Column(Modifier.weight(0.58f), content = rechts)
        } else Column {
            links()
            rechts()
        }
    }
}

// ---------- Allgemein ----------

/** Servername, Metadatensprache, TMDB-Schlüssel, Updates, ffmpeg. Branding und Pfade kennt der Server nicht. */
@Composable
internal fun Allgemein(api: ApiClient) {
    val z = laden(api) { api.einstellungen() }
    val a = rememberAktion()
    var name by remember { mutableStateOf("") }
    var tmdb by remember { mutableStateOf("") }
    LaunchedEffect(z.daten?.serverName) { z.daten?.let { name = it.serverName } }
    Nachladen(2000, z.daten?.ffmpeg?.download?.running == true) { z.neu() }
    fun setzen(b: kotlinx.serialization.json.JsonObjectBuilder.() -> Unit, danach: () -> Unit = {}) =
        a.los("Gespeichert", danach = { danach(); z.neu() }) { api.einstellungenSetzen(b) }
    Seite {
        Geladen(z) { d ->
            Gruppe("Server") {
                FormZeile({ Eingabe(name, { name = it }, "Servername", it, onEnter = { setzen({ put("serverName", name) }) }) }) {
                    AKnopf("Speichern", { setzen({ put("serverName", name) }) }, aus = name == d.serverName)
                }
                Leise("Erscheint bei der Anmeldung, in den Apps und im Netzwerk.", Modifier.padding(top = 8.dp, bottom = 12.dp))
                Zeile("Adresse im Heimnetz", icon = AI.NETZWERK, unter = d.lanUrl)
            }
            Gruppe("Metadatensprache", "Sprache für Titel, Inhaltsangaben und Bilder.") {
                Wahl(listOf("de" to "Deutsch", "en" to "English"), d.language) { x -> setzen({ put("language", x) }) }
            }
            Gruppe("Metadaten-Dienste", if (d.tmdbKey) "Eigener Schlüssel ist gesetzt." else "Ohne eigenen Schlüssel nutzt Flimmer den eingebauten, sonst TVmaze und Wikidata.") {
                FormZeile({ Eingabe(tmdb, { tmdb = it }, "TMDB-Schlüssel", it, zeigeLabel = false, onEnter = { if (tmdb.isNotEmpty()) setzen({ put("tmdbKey", tmdb) }) { tmdb = "" } }) }) {
                    AKnopf("Speichern", { setzen({ put("tmdbKey", tmdb) }) { tmdb = "" } }, aus = tmdb.isEmpty())
                    if (d.tmdbKey) Knopf("Entfernen", { setzen({ put("tmdbKey", "") }) })
                }
            }
            Gruppe("Updates") {
                Zeile("Version ${d.version}", icon = Ic.INFO, unter = d.update?.let { "Version ${it.version} ist erschienen" } ?: "Aktuell",
                    rechts = if (d.update != null) ({ Knopf("Herunterladen", { a.oeffnen(d.update.url) }, icon = Ic.DOWNLOAD) }) else null)
                Zeile("Nach Updates suchen", icon = Ic.NEUSTART, unter = "Fragt einmal am Tag nach einer neuen Version. Installiert nichts von selbst.",
                    schalter = d.updateCheck, onClick = { setzen({ put("updateCheck", !d.updateCheck) }) })
            }
            Gruppe("ffmpeg") {
                val ff = d.ffmpeg
                Zeile("ffmpeg", icon = if (ff.ok) Ic.HAKEN else AI.FEHLER, farbe = if (ff.ok) K.Text else K.AmpelRot,
                    unter = when {
                        ff.ok -> "Installiert – Umwandeln ist möglich"
                        ff.download.running -> "Wird geladen … ${Math.round(ff.download.percent)} %"
                        else -> ff.download.error?.takeIf { it.isNotEmpty() } ?: ff.hint.ifEmpty { "Fehlt – ohne ffmpeg läuft nur, was das Gerät direkt kann" }
                    },
                    rechts = if (!ff.ok && ff.canDownload && !ff.download.running) ({
                        Knopf("ffmpeg herunterladen", { a.los(danach = z.neu) { api.tu("POST", "/api/setup/ffmpeg") } }, primary = true)
                    }) else null)
            }
        }
    }
}

// ---------- Benutzer ----------

/** Liste und Bearbeiten. Der Server kennt Name, Admin und PIN. */
@Composable
internal fun Benutzer(api: ApiClient) {
    val z = laden(api) { api.benutzer() }
    var wahl by remember { mutableStateOf("") }
    val liste = z.daten.orEmpty()
    LaunchedEffect(liste.size) { if (wahl.isEmpty() && liste.isNotEmpty()) wahl = liste[0].id }
    Seite {
        Geladen(z) { l ->
            Spalten(links = {
                Spacer(Modifier.height(16.dp))
                Knopf("Benutzer hinzufügen", { wahl = "neu" }, icon = AI.PLUS)
                Spacer(Modifier.height(12.dp))
                l.forEach { u ->
                    Zeile(u.name, an = u.id == wahl, onClick = { wahl = u.id }, vorne = { Avatar(u.name, u.color, if (LocalTv.current) 48.dp else 32.dp) },
                        unter = (if (u.admin) "Administrator" else "Benutzer") + " · " + (if (u.hasPassword) "mit PIN" else "ohne PIN (nur im Heimnetz)"))
                }
                Leise("${l.size} Benutzer · ${l.count { it.admin }} Administrator. Gäste aus Einladungen erscheinen nicht in dieser Liste.", Modifier.padding(top = 12.dp))
            }, rechts = {
                if (wahl == "neu") NeuerBenutzer(api) { id -> z.neu(); wahl = id }
                else l.firstOrNull { it.id == wahl }?.let { u -> key(u.id) { BenutzerBearbeiten(api, u, z.neu) { wahl = ""; z.neu() } } }
            })
        }
    }
}

private const val PIN_TEXT = "Mindestens 4 Zeichen. Admins brauchen immer eine PIN. Ohne PIN kommt das Profil nur im Heimnetz an."

@Composable
private fun NeuerBenutzer(api: ApiClient, fertig: (String) -> Unit) {
    val a = rememberAktion()
    var name by remember { mutableStateOf("") }
    var pin by remember { mutableStateOf("") }
    var admin by remember { mutableStateOf(false) }
    Gruppe("Neuer Benutzer") {
        Eingabe(name, { name = it }, "Name", Modifier.fillMaxWidth())
        Spacer(Modifier.height(12.dp))
        Eingabe(pin, { pin = it }, "PIN oder Passwort", Modifier.fillMaxWidth(), passwort = true)
        Leise(PIN_TEXT, Modifier.padding(vertical = 8.dp))
        Zeile("Administrator", icon = Ic.SCHLUESSEL, unter = "Darf das Dashboard öffnen und alle Einstellungen ändern.", schalter = admin, onClick = { admin = !admin })
        AKnopf("Anlegen", {
            a.los("Gespeichert") {
                val u: User = api.schick("POST", "/api/users", obj {
                    put("name", name); if (pin.isNotEmpty()) put("password", pin); put("admin", admin); put("color", (0 until 360).random())
                })
                fertig(u.id)
            }
        }, Modifier.padding(top = 16.dp), primary = true, aus = name.isBlank())
    }
}

@Composable
private fun BenutzerBearbeiten(api: ApiClient, u: User, neu: () -> Unit, weg: () -> Unit) {
    val a = rememberAktion()
    var name by remember { mutableStateOf(u.name) }
    var pin by remember { mutableStateOf("") }
    var sicher by remember { mutableStateOf(false) }
    fun setzen(b: kotlinx.serialization.json.JsonObjectBuilder.() -> Unit) =
        a.los("Gespeichert", danach = { pin = ""; neu() }) { api.roh("PUT", "/api/users/${k(u.id)}", obj(b)) }
    Gruppe("${u.name} bearbeiten", zusatz = { Avatar(u.name, u.color, if (LocalTv.current) 64.dp else 40.dp) }) {
        FormZeile({ Eingabe(name, { name = it }, "Name", it, onEnter = { setzen { put("name", name) } }) }) {
            AKnopf("Speichern", { setzen { put("name", name) } }, aus = name.isBlank() || name == u.name)
        }
        Spacer(Modifier.height(12.dp))
        Zeile("Administrator", icon = Ic.SCHLUESSEL, unter = "Darf das Dashboard öffnen und alle Einstellungen ändern.", schalter = u.admin,
            onClick = { setzen { put("admin", !u.admin) } })
        Spacer(Modifier.height(12.dp))
        FormZeile({ Eingabe(pin, { pin = it }, "Neue PIN", it, passwort = true, onEnter = { if (pin.isNotEmpty()) setzen { put("password", pin) } }) }) {
            AKnopf("Speichern", { setzen { put("password", pin) } }, aus = pin.isEmpty())
            if (u.hasPassword && !u.admin) Knopf("PIN entfernen", { setzen { put("password", "") } })
        }
        Leise(PIN_TEXT, Modifier.padding(vertical = 8.dp))
        Knopf(if (sicher) "Wirklich löschen?" else "Benutzer löschen", {
            if (sicher) a.los(danach = weg) { api.tu("DELETE", "/api/users/${k(u.id)}") } else sicher = true
        }, Modifier.padding(top = 8.dp), icon = Ic.LOESCHEN)
    }
}

// ---------- Einladungen ----------

/** Neue Einladung (Link teilen, QR) und aktive Einladungen. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun Einladungen(api: ApiClient) {
    val liste = laden(api) { api.liste<Einladung>("/api/invites") }
    val s = laden(api) { api.einstellungen() }
    val a = rememberAktion()
    var notiz by remember { mutableStateOf("") }
    var tage by remember { mutableIntStateOf(14) }
    var max by remember { mutableStateOf("1") }
    var libs by remember { mutableStateOf(listOf<String>()) }
    var neu by remember { mutableStateOf<NeueEinladung?>(null) }
    val dirs = s.daten?.dirs.orEmpty()
    Seite {
        Spalten(links = {
            Gruppe("Neue Einladung") {
                Eingabe(notiz, { notiz = it }, "Notiz", Modifier.fillMaxWidth())
                Leise("Nur für dich sichtbar – so erkennst du die Einladung in der Liste wieder.", Modifier.padding(vertical = 8.dp))
                if (dirs.size > 1) {
                    Unterkopf("Bibliotheken")
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Chip("Alle Bibliotheken", libs.isEmpty(), { libs = emptyList() })
                        dirs.forEach { d -> Chip(ordnerName(d), d in libs, { libs = if (d in libs) libs - d else libs + d }) }
                    }
                }
                Unterkopf("Gültig für")
                Wahl(listOf(1, 7, 14, 30).map { it to if (it == 1) "1 Tag" else "$it Tage" }, tage) { tage = it }
                Spacer(Modifier.height(12.dp))
                Eingabe(max, { max = it }, "Max. Einlösungen (0 = unbegrenzt)", Modifier.fillMaxWidth(), zahl = true)
                Knopf("Einladung erstellen", {
                    a.los {
                        neu = api.schick("POST", "/api/invites", obj {
                            put("note", notiz); putJsonArray("libraries") { libs.forEach { add(it) } }; put("hours", tage * 24)
                            put("maxUses", maxOf(0, max.toIntOrNull() ?: 0))
                        })
                        notiz = ""
                        liste.neu()
                    }
                }, Modifier.padding(top = 16.dp), primary = true, icon = AI.EINLADUNG)
            }
        }, rechts = {
            neu?.let { n ->
                Gruppe("Einladung erstellt", "Schick diesen Link weiter. Er wird nur jetzt vollständig angezeigt – später siehst du in der Liste nur noch die Notiz.") {
                    T(n.url, LocalTypo.current.klein, K.Text, family = Mono, modifier = Modifier.fillMaxWidth()
                        .background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusS)).padding(12.dp))
                    Row(Modifier.padding(top = 12.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        if (!LocalTv.current) Knopf("Link teilen", { a.teilen(n.url) }, primary = true, icon = Ic.TEILEN)
                        Knopf("Kopieren", { a.kopieren(n.url) }, icon = Ic.TEILEN)
                    }
                    n.qr?.let { qr -> Qr(qr) }
                    n.hint?.takeIf { it.isNotEmpty() }?.let { Leise(it, Modifier.padding(top = 8.dp)) }
                }
            }
        })
        Gruppe("Aktive Einladungen") {
            Geladen(liste) { l ->
                if (l.isEmpty()) Leise("Noch keine Einladungen.")
                l.forEach { e ->
                    val was = e.scope.libraries.map(::ordnerName) + (if (e.scope.items.isNotEmpty()) listOf("${e.scope.items.size} Titel") else emptyList())
                    Zeile(e.note.ifEmpty { e.id }, icon = AI.EINLADUNG, unter = listOf(
                        if (was.isNotEmpty()) "Nur ${was.joinToString(", ")}" else "Alle Bibliotheken",
                        e.created?.let { "erstellt am ${datum(it, false)}" } ?: "",
                        "läuft ab am ${datum(e.expires, false)}",
                        if (e.maxUses > 0) "${e.uses} / ${e.maxUses} eingelöst" else "${e.uses}-mal eingelöst",
                        if (e.guests > 0) "${e.guests} Gäste" else "",
                    ).filter { it.isNotEmpty() }.joinToString(" · "),
                        rechts = { Knopf("Widerrufen", { a.los(danach = liste.neu) { api.tu("DELETE", "/api/invites/${k(e.id)}") } }) })
                }
            }
        }
    }
}

/** QR-Code aus der Data-URL des Servers (PNG, Base64). */
@Composable
private fun Qr(dataUrl: String) {
    val bild = remember(dataUrl) {
        runCatching { Base64.decode(dataUrl.substringAfter("base64,"), Base64.DEFAULT) }.getOrNull()
            ?.let { BitmapFactory.decodeByteArray(it, 0, it.size) }?.asImageBitmap()
    } ?: return
    val g = if (LocalTv.current) 320.dp else 180.dp
    Box(Modifier.padding(top = 16.dp).size(g + 16.dp).background(K.Text, RoundedCornerShape(Tokens.Radius.RadiusS)).padding(8.dp)) {
        Image(bild, "QR-Code der Einladung", Modifier.size(g), filterQuality = FilterQuality.None) // sonst verschwimmen die Module
    }
}
