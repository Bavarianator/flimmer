package io.flimmer.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import io.flimmer.app.Item
import io.flimmer.app.User
import io.flimmer.app.search
import kotlinx.coroutines.delay

@Composable
private fun Page(content: @Composable () -> Unit) {
    Box(Modifier.fillMaxSize().background(K.Saal)) { content() }
}

@Composable
private fun Feld(value: String, onChange: (String) -> Unit, placeholder: String, modifier: Modifier = Modifier,
                 password: Boolean = false, type: KeyboardType = KeyboardType.Text, onGo: () -> Unit = {}) {
    OutlinedTextField(
        value, onChange, modifier, singleLine = true,
        placeholder = { T(placeholder, color = K.Text3) },
        textStyle = TextStyle(fontFamily = Sans, fontSize = LocalTypo.current.text, color = K.Text),
        visualTransformation = if (password) PasswordVisualTransformation() else androidx.compose.ui.text.input.VisualTransformation.None,
        keyboardOptions = KeyboardOptions(keyboardType = if (password) KeyboardType.Password else type, imeAction = ImeAction.Go),
        keyboardActions = KeyboardActions(onGo = { onGo() }),
        shape = RoundedCornerShape(Tokens.Radius.RadiusM),
        colors = OutlinedTextFieldDefaults.colors(
            focusedBorderColor = K.Text, unfocusedBorderColor = K.LinieStark, cursorColor = K.Text,
            focusedContainerColor = K.Flaeche2, unfocusedContainerColor = K.Flaeche2,
        ),
    )
}

@Composable
private fun Fehler(text: String) = T(text, LocalTypo.current.klein, K.AmpelRot, modifier = Modifier.padding(top = 12.dp))

/** Erststart: Server im Heimnetz finden (SSDP) oder Adresse eingeben. */
@Composable
fun ConnectScreen(found: List<String>?, error: String, onConnect: (String) -> Unit) {
    var input by remember { mutableStateOf("") }
    val first = remember { FocusRequester() }
    Page {
        Column(Modifier.fillMaxSize().padding(24.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
            PlakatTitel("Flimmer", LocalTypo.current.plakat)
            Spacer(Modifier.height(32.dp))
            when {
                found == null -> Label("Suche Server im Heimnetz …")
                found.isEmpty() -> Label("Kein Server gefunden")
                else -> {
                    Label("Gefundene Server")
                    found.forEachIndexed { i, url ->
                        FButton(url.removePrefix("http://"), { onConnect(url) },
                            Modifier.padding(top = 12.dp).then(if (i == 0) Modifier.focusRequester(first) else Modifier), primary = true)
                    }
                    LaunchedEffect(found) { runCatching { first.requestFocus() } }
                }
            }
            Spacer(Modifier.height(32.dp))
            Label("Adresse eingeben")
            Feld(input, { input = it }, "192.168.178.20", Modifier.width(420.dp).padding(top = 8.dp), type = KeyboardType.Uri,
                onGo = { if (input.isNotBlank()) onConnect(input) })
            FButton("Verbinden", { if (input.isNotBlank()) onConnect(input) }, Modifier.padding(top = 12.dp))
            if (error.isNotEmpty()) Fehler(error)
        }
    }
}

/** „Wer schaut?“ – Profilauswahl; auf dem TV zusätzlich Kopplungscode (kein Tippen mit der Fernbedienung). */
@Composable
fun LoginScreen(users: List<User>?, pairCode: String?, error: String, onLogin: (User, String?) -> Unit, onChangeServer: () -> Unit) {
    var pick by remember { mutableStateOf<User?>(null) }
    var pw by remember { mutableStateOf("") }
    val first = remember { FocusRequester() }
    val tv = LocalTv.current
    Page {
        Column(Modifier.fillMaxSize().padding(24.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
            val p = pick
            if (p != null) {
                Avatar(p.name, p.color, if (tv) 128.dp else 88.dp)
                T(p.name, LocalTypo.current.titel, weight = FontWeight.SemiBold, modifier = Modifier.padding(top = 16.dp))
                Feld(pw, { pw = it }, "Passwort", Modifier.width(360.dp).padding(top = 24.dp).focusRequester(first), password = true, onGo = { onLogin(p, pw) })
                Row(Modifier.padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    FButton("Anmelden", { onLogin(p, pw) }, primary = true)
                    FButton("Zurück", { pick = null; pw = "" })
                }
                LaunchedEffect(p) { runCatching { first.requestFocus() } }
            } else {
                T("Wer schaut?", LocalTypo.current.titel, weight = FontWeight.SemiBold)
                Spacer(Modifier.height(32.dp))
                if (users == null) Label("Lade …")
                LazyRow(horizontalArrangement = Arrangement.spacedBy(if (tv) 40.dp else 20.dp)) {
                    items(users.orEmpty(), key = { it.id }) { u ->
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            val sz = if (tv) 160.dp else 96.dp
                            ClickCard({ if (u.hasPassword) pick = u else onLogin(u, null) },
                                Modifier.size(sz).then(if (u == users?.first()) Modifier.focusRequester(first) else Modifier)) {
                                Avatar(u.name, u.color, sz)
                            }
                            T(u.name, LocalTypo.current.karte, K.Text2, modifier = Modifier.padding(top = 12.dp))
                        }
                    }
                }
                LaunchedEffect(users) { if (!users.isNullOrEmpty()) runCatching { first.requestFocus() } }
                if (pairCode != null) {
                    Spacer(Modifier.height(48.dp))
                    Label("Oder am Handy unter „Fernseher koppeln“ eingeben")
                    T(pairCode.chunked(3).joinToString(" "), LocalTypo.current.code, family = Mono, modifier = Modifier.padding(top = 8.dp))
                }
                FButton("Anderer Server", onChangeServer, Modifier.padding(top = 40.dp))
            }
            if (error.isNotEmpty()) Fehler(error)
        }
    }
}

enum class Tab(val label: String) { Start("Start"), Filme("Filme"), Serien("Serien"), Merkliste("Merkliste"), Suche("Suche"), Bibliothek("Bibliothek"), Profil("Profil") }

/** TV: Kopfleiste wie im Entwurf. Handy: untere Leiste Start · Suche · Bibliothek · Profil. */
@Composable
fun Shell(tab: Tab, userName: String, userColor: Int, error: String, onTab: (Tab) -> Unit, content: @Composable () -> Unit) {
    val tv = LocalTv.current
    Page {
        if (tv) {
            Column(Modifier.fillMaxSize()) {
                Row(Modifier.fillMaxWidth().padding(start = Pad, end = Pad, top = Tokens.Abstand.TvRandOben, bottom = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                    T("FLIMMER", LocalTypo.current.titel, family = Plakat, modifier = Modifier.padding(end = 48.dp))
                    listOf(Tab.Start, Tab.Filme, Tab.Serien, Tab.Merkliste, Tab.Suche).forEach { NavTab(it.label, it == tab) { onTab(it) } }
                    Spacer(Modifier.weight(1f))
                    T(userName, LocalTypo.current.text, K.Text2, modifier = Modifier.padding(end = 16.dp))
                    // tv-Surface statt clickable: nur so ist der Avatar mit dem D-Pad erreichbar
                    androidx.tv.material3.Surface(
                        onClick = { onTab(Tab.Profil) },
                        shape = androidx.tv.material3.ClickableSurfaceDefaults.shape(RoundedCornerShape(Tokens.Radius.RadiusS)),
                        border = androidx.tv.material3.ClickableSurfaceDefaults.border(
                            focusedBorder = androidx.tv.material3.Border(androidx.compose.foundation.BorderStroke(Tokens.Masse.FokusRingTv, K.Text), inset = 3.dp),
                        ),
                        colors = androidx.tv.material3.ClickableSurfaceDefaults.colors(containerColor = Color.Transparent, focusedContainerColor = Color.Transparent),
                    ) { Avatar(userName.ifEmpty { "?" }, userColor, 56.dp) }
                }
                if (error.isNotEmpty()) Box(Modifier.padding(horizontal = Pad)) { Fehler(error) }
                Box(Modifier.weight(1f)) { content() }
            }
        } else {
            Column(Modifier.fillMaxSize().statusBarsPadding()) {
                if (error.isNotEmpty()) Box(Modifier.padding(horizontal = Pad)) { Fehler(error) }
                Box(Modifier.weight(1f)) { content() }
                Row(Modifier.fillMaxWidth().background(K.Saal).navigationBarsPadding().padding(vertical = 8.dp), horizontalArrangement = Arrangement.SpaceEvenly) {
                    listOf(Tab.Start, Tab.Suche, Tab.Bibliothek, Tab.Profil).forEach { t ->
                        Column(Modifier.clickable { onTab(t) }.padding(horizontal = 12.dp, vertical = 6.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                            T(t.label, LocalTypo.current.klein, if (t == tab) K.Text else K.Text3, if (t == tab) FontWeight.SemiBold else FontWeight.Normal)
                            Box(Modifier.padding(top = 4.dp).width(24.dp).height(2.dp).background(if (t == tab) K.Text else Color.Transparent))
                        }
                    }
                }
            }
        }
    }
}

data class Shelf(val title: String, val items: List<Item>, val wide: Boolean = false)

fun sub(it: Item): String = when {
    it.series != null -> "S${it.season} E${it.episode} · ${it.displayTitle}"
    it.progress > 0 && it.duration > 0 && !it.watched -> listOfNotNull(it.meta?.year ?: it.year, "Noch ${fmtDauer(it.duration - it.progress)}").joinToString(" · ")
    else -> listOfNotNull(it.meta?.year ?: it.year, if (it.duration > 0) fmtDauer(it.duration) else null).joinToString(" · ")
}

/** Großer Kopfbereich: Label, Plakat-Titel, Fakten, Ampel, Beschreibung, Knöpfe. */
@Composable
private fun Hero(item: Item, title: String, label: String, abs: (String) -> String, extra: @Composable () -> Unit) {
    val tv = LocalTv.current
    Box(Modifier.fillMaxWidth().height(if (tv) 520.dp else 480.dp)) {
        Art(abs(item.backdrop), title, Modifier.fillMaxSize())
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(listOf(K.Saal, K.Saal.copy(alpha = 0.7f), Color.Transparent))))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Color.Transparent, K.Saal))))
        Column(Modifier.align(Alignment.BottomStart).padding(horizontal = Pad, vertical = 24.dp).width(if (tv) 900.dp else 600.dp)) {
            if (label.isNotEmpty()) Label(label)
            PlakatTitel(title, modifier = Modifier.padding(top = 8.dp))
            val m = item.meta
            val facts = listOfNotNull(
                if (item.series != null) "S${item.season} E${item.episode} · ${item.displayTitle}" else (m?.year ?: item.year)?.toString(),
                if (item.duration > 0) fmtDauer(item.duration) else null,
                m?.rating?.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
            )
            Row(Modifier.padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                T(facts.joinToString(" · "), LocalTypo.current.klein, K.Text2, maxLines = 1)
                Spacer(Modifier.width(16.dp))
                Ampel(item.light)
            }
            if (tv || label.isEmpty()) m?.overview?.let { T(it, LocalTypo.current.text, K.Text, maxLines = if (tv) 3 else 6, modifier = Modifier.padding(top = 12.dp)) }
            extra()
        }
    }
}

@OptIn(androidx.compose.foundation.layout.ExperimentalLayoutApi::class)
@Composable
private fun PlayButtons(item: Item, onPlay: (Item, Double?) -> Unit, onInfo: (() -> Unit)?, merk: Pair<Boolean, () -> Unit>?, onParty: ((Item) -> Unit)? = null) {
    val play = remember { FocusRequester() }
    // FlowRow: auf schmalen Handys brechen die Knöpfe um statt abgeschnitten zu werden.
    androidx.compose.foundation.layout.FlowRow(Modifier.padding(top = 20.dp), horizontalArrangement = Arrangement.spacedBy(12.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        val resume = item.progress > 5 && !item.watched
        FButton(if (resume) "▶  Fortsetzen" else "▶  Abspielen", { onPlay(item, null) }, Modifier.focusRequester(play), primary = true)
        if (resume) FButton("Von vorn", { onPlay(item, 0.0) })
        if (merk != null) FButton(if (merk.first) "✓ Merkliste" else "+ Merkliste", merk.second)
        if (onParty != null) FButton("Gemeinsam schauen", { onParty(item) })
        if (onInfo != null) FButton("Info", onInfo)
    }
    if (item.progress > 0 && item.duration > 0 && !item.watched) Fortschritt((item.progress / item.duration).toFloat(), Modifier.padding(top = 16.dp).width(360.dp))
    LaunchedEffect(item.id) { delay(80); runCatching { play.requestFocus() } }
}

@Composable
private fun Reihe(shelf: Shelf, abs: (String) -> String, onOpen: (Item) -> Unit) {
    T(shelf.title, LocalTypo.current.reihe, weight = FontWeight.SemiBold, modifier = Modifier.padding(start = Pad, top = 28.dp, bottom = 12.dp))
    LazyRow(contentPadding = PaddingValues(horizontal = Pad), horizontalArrangement = Arrangement.spacedBy(if (LocalTv.current) 24.dp else 12.dp)) {
        items(shelf.items, key = { shelf.title + it.id }) { it ->
            PosterCard(it, abs(if (shelf.wide) it.backdrop else it.poster), shelf.wide, it.series ?: it.displayTitle,
                if (shelf.title == "Serien") "" else sub(it)) { onOpen(it) }
        }
    }
}

@Composable
fun HomeScreen(shelves: List<Shelf>?, error: String, abs: (String) -> String, onOpen: (Item) -> Unit, onPlay: (Item, Double?) -> Unit) {
    val hero = shelves?.firstOrNull()?.items?.firstOrNull()
    val heroLabel = shelves?.firstOrNull()?.title ?: ""
    val tv = LocalTv.current
    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = 48.dp)) {
        if (!tv) item { T("FLIMMER", LocalTypo.current.titel, family = Plakat, modifier = Modifier.padding(start = Pad, top = 8.dp, bottom = 8.dp)) }
        if (shelves == null) item { Box(Modifier.padding(Pad)) { Label("Lade Bibliothek …") } }
        if (hero != null) item {
            Hero(hero, hero.series ?: hero.displayTitle, heroLabel, abs) { PlayButtons(hero, onPlay, { onOpen(hero) }, null) }
        }
        shelves?.forEach { shelf -> item(key = "shelf-" + shelf.title) { Reihe(shelf, abs, onOpen) } }
    }
}

/** Raster für Filme, Serien, Merkliste und Suchergebnisse. */
@Composable
fun Raster(items: List<Item>, abs: (String) -> String, empty: String, onOpen: (Item) -> Unit, header: @Composable () -> Unit = {}) {
    val tv = LocalTv.current
    Column(Modifier.fillMaxSize()) {
        header()
        if (items.isEmpty()) Box(Modifier.padding(Pad)) { T(empty, color = K.Text2) }
        LazyVerticalGrid(
            GridCells.Adaptive(if (tv) Tokens.Masse.KartePosterTv else Tokens.Masse.KartePosterHd),
            contentPadding = PaddingValues(horizontal = Pad, vertical = 16.dp),
            horizontalArrangement = Arrangement.spacedBy(if (tv) 24.dp else 12.dp),
            verticalArrangement = Arrangement.spacedBy(if (tv) 32.dp else 20.dp),
        ) {
            items(items, key = { it.id }) { PosterCard(it, abs(it.poster), false, it.series ?: it.displayTitle, if (it.series != null) "Serie" else sub(it)) { onOpen(it) } }
        }
    }
}

@Composable
fun SearchScreen(library: List<Item>, abs: (String) -> String, onOpen: (Item) -> Unit) {
    var q by remember { mutableStateOf("") }
    val focus = remember { FocusRequester() }
    Raster(if (q.isBlank()) emptyList() else search(q, library), abs, if (q.isBlank()) "Titel, Serie …" else "Nichts gefunden für „$q“", onOpen) {
        Feld(q, { q = it }, "Suchen", Modifier.fillMaxWidth().padding(horizontal = Pad).padding(top = 16.dp).focusRequester(focus))
    }
    LaunchedEffect(Unit) { runCatching { focus.requestFocus() } }
}

@Composable
fun BibliothekScreen(library: List<Item>, watchlist: List<Item>, abs: (String) -> String, onOpen: (Item) -> Unit) {
    var teil by remember { mutableStateOf(0) } // 0 Filme, 1 Serien, 2 Merkliste
    val items = when (teil) {
        1 -> library.filter { it.series != null }.distinctBy { it.series }
        2 -> watchlist
        else -> library.filter { it.series == null }
    }
    Raster(items, abs, if (teil == 2) "Noch nichts gemerkt – bei einem Titel „+ Merkliste“ wählen." else "Noch nichts da", onOpen) {
        Row(Modifier.padding(horizontal = Pad).padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            listOf("Filme", "Serien", "Merkliste").forEachIndexed { i, l -> FButton(l, { teil = i }, primary = teil == i) }
        }
    }
}

@Composable
fun ProfilScreen(userName: String, userColor: Int, server: String, device: String, onLogout: () -> Unit, onChangeServer: () -> Unit, onRefresh: () -> Unit, onJoin: (String) -> Unit) {
    var code by remember { mutableStateOf("") }
    Column(Modifier.fillMaxSize().padding(Pad).padding(top = 16.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Avatar(userName.ifEmpty { "?" }, userColor, 64.dp)
            T(userName, LocalTypo.current.titel, weight = FontWeight.SemiBold, modifier = Modifier.padding(start = 16.dp))
        }
        Spacer(Modifier.height(32.dp))
        Label("Server"); T(server, color = K.Text2, modifier = Modifier.padding(top = 4.dp, bottom = 16.dp))
        Label("Gerät"); T(device, color = K.Text2, modifier = Modifier.padding(top = 4.dp, bottom = 24.dp))
        Label("Gemeinsam schauen – Raumcode")
        Row(Modifier.padding(top = 8.dp, bottom = 24.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Feld(code, { code = it }, "z. B. 3f9a1c0b2d4e", Modifier.width(260.dp), onGo = { if (code.isNotBlank()) onJoin(code) })
            FButton("Beitreten", { if (code.isNotBlank()) onJoin(code) }, primary = true)
        }
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            FButton("Bibliothek aktualisieren", onRefresh)
            FButton("Profil wechseln", onLogout)
            FButton("Anderer Server", onChangeServer)
        }
    }
}

@Composable
fun DetailScreen(item: Item, abs: (String) -> String, inList: Boolean, onToggleList: () -> Unit, onParty: () -> Unit, onPlay: (Item, Double?) -> Unit) {
    Page { LazyColumn { item { Hero(item, item.displayTitle, "", abs) { PlayButtons(item, onPlay, null, inList to onToggleList, { onParty() }) } } } }
}

@Composable
fun SeriesScreen(name: String, episodes: List<Item>, abs: (String) -> String, inList: Boolean, onToggleList: () -> Unit, onParty: (Item) -> Unit, onPlay: (Item, Double?) -> Unit) {
    val seasons = episodes.groupBy { it.season ?: 0 }.toSortedMap()
    val all = seasons.values.flatMap { eps -> eps.sortedBy { it.episode ?: 0 } }
    val last = all.indexOfLast { it.progress > 0 || it.watched }
    val next = when {
        last < 0 -> all.firstOrNull()
        all[last].watched && last + 1 < all.size -> all[last + 1]
        else -> all[last]
    } ?: return
    Page {
        LazyColumn(contentPadding = PaddingValues(bottom = 48.dp)) {
            item { Hero(next, name, if (last < 0) "" else "Als Nächstes", abs) { PlayButtons(next, onPlay, null, inList to onToggleList, onParty) } }
            seasons.forEach { (season, eps) ->
                item(key = "season-$season") {
                    Reihe(Shelf(if (season == 0) "Specials" else "Staffel $season",
                        eps.sortedBy { it.episode ?: 0 }.map { it.copy(series = null, title = "${it.episode}. ${it.displayTitle}", meta = null) }, wide = true), abs) { e ->
                        onPlay(episodes.first { it.id == e.id }, null)
                    }
                }
            }
        }
    }
}
