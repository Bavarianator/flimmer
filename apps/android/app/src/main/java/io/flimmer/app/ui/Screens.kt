package io.flimmer.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.flimmer.app.Item
import io.flimmer.app.User
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

@Composable
private fun Page(content: @Composable () -> Unit) {
    Box(Modifier.fillMaxSize().background(Bg)) { content() }
}

@Composable
private fun Title(text: String, size: Int = 32) = Text(text, color = TextColor, fontSize = size.sp, fontWeight = FontWeight.Bold)

/** Erststart: Server im Heimnetz finden (SSDP) oder Adresse eingeben. */
@Composable
fun ConnectScreen(found: List<String>?, error: String, onConnect: (String) -> Unit) {
    var input by remember { mutableStateOf("") }
    val first = remember { FocusRequester() }
    Page {
        Column(Modifier.fillMaxSize().padding(32.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
            Title("Flimmer", 48)
            Spacer(Modifier.height(24.dp))
            when {
                found == null -> Text("Suche Server im Heimnetz …", color = Muted)
                found.isEmpty() -> Text("Kein Server gefunden. Adresse eingeben:", color = Muted)
                else -> {
                    Text("Gefundene Server:", color = Muted)
                    found.forEachIndexed { i, url ->
                        FButton(url.removePrefix("http://"), { onConnect(url) }, Modifier.padding(top = 8.dp).then(if (i == 0) Modifier.focusRequester(first) else Modifier), primary = true)
                    }
                    LaunchedEffect(found) { runCatching { first.requestFocus() } }
                    Spacer(Modifier.height(16.dp))
                    Text("oder Adresse eingeben:", color = Muted)
                }
            }
            OutlinedTextField(
                input, { input = it }, Modifier.width(420.dp).padding(top = 8.dp),
                placeholder = { Text("192.168.178.20") }, singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go),
                keyboardActions = KeyboardActions(onGo = { if (input.isNotBlank()) onConnect(input) }),
            )
            FButton("Verbinden", { if (input.isNotBlank()) onConnect(input) }, Modifier.padding(top = 12.dp), primary = true)
            if (error.isNotEmpty()) Text(error, color = Color(0xFFFF5D5D), modifier = Modifier.padding(top = 12.dp))
        }
    }
}

/** Profilauswahl im Netflix-Stil; auf dem TV zusätzlich Kopplungscode (kein Tippen mit der Fernbedienung). */
@Composable
fun LoginScreen(users: List<User>?, pairCode: String?, error: String, onLogin: (User, String?) -> Unit, onChangeServer: () -> Unit) {
    var pick by remember { mutableStateOf<User?>(null) }
    var pw by remember { mutableStateOf("") }
    val first = remember { FocusRequester() }
    Page {
        Column(Modifier.fillMaxSize().padding(32.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
            val p = pick
            if (p != null) {
                Title("Hallo ${p.name}")
                OutlinedTextField(
                    pw, { pw = it }, Modifier.width(360.dp).padding(top = 24.dp).focusRequester(first),
                    placeholder = { Text("Passwort") }, singleLine = true, visualTransformation = PasswordVisualTransformation(),
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = ImeAction.Go),
                    keyboardActions = KeyboardActions(onGo = { onLogin(p, pw) }),
                )
                Row(Modifier.padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    FButton("Anmelden", { onLogin(p, pw) }, primary = true)
                    FButton("Zurück", { pick = null; pw = "" })
                }
                LaunchedEffect(p) { runCatching { first.requestFocus() } }
            } else {
                Title("Wer schaut?")
                Spacer(Modifier.height(24.dp))
                if (users == null) Text("Lade …", color = Muted)
                LazyRow(horizontalArrangement = Arrangement.spacedBy(20.dp)) {
                    items(users.orEmpty(), key = { it.id }) { u ->
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            ClickCard({ if (u.hasPassword) pick = u else onLogin(u, null) }, Modifier.size(112.dp).then(if (u == users?.first()) Modifier.focusRequester(first) else Modifier)) {
                                Box(Modifier.fillMaxSize().background(Color.hsl(u.color.toFloat().mod(360f), 0.55f, 0.45f)), contentAlignment = Alignment.Center) {
                                    Text(u.name.take(1).uppercase(), fontSize = 44.sp, fontWeight = FontWeight.Bold, color = Color.White)
                                }
                            }
                            Text(u.name, color = TextColor, modifier = Modifier.padding(top = 8.dp))
                        }
                    }
                }
                LaunchedEffect(users) { if (!users.isNullOrEmpty()) runCatching { first.requestFocus() } }
                if (pairCode != null) {
                    Text("Oder am Handy anmelden und unter „Fernseher koppeln“ eingeben:", color = Muted, modifier = Modifier.padding(top = 40.dp))
                    Text(pairCode.chunked(3).joinToString(" "), color = TextColor, fontSize = 52.sp, fontWeight = FontWeight.Bold, letterSpacing = 8.sp)
                }
                FButton("Anderer Server", onChangeServer, Modifier.padding(top = 32.dp))
            }
            if (error.isNotEmpty()) Text(error, color = Color(0xFFFF5D5D), modifier = Modifier.padding(top = 12.dp))
        }
    }
}

data class Shelf(val title: String, val items: List<Item>, val wide: Boolean = false)

private fun sub(it: Item) = when {
    it.series != null -> "S${it.season} E${it.episode} · ${it.displayTitle}"
    else -> listOfNotNull(it.meta?.year ?: it.year, if (it.duration > 0) "${(it.duration / 60).toInt()} min" else null).joinToString(" · ")
}

@Composable
fun HomeScreen(
    shelves: List<Shelf>?, error: String, userName: String, abs: (String) -> String,
    onOpen: (Item) -> Unit, onOpenSeries: (String) -> Unit, onLogout: () -> Unit, onRefresh: () -> Unit,
) {
    val first = remember { FocusRequester() }
    Page {
        LazyColumn(Modifier.fillMaxSize(), contentPadding = androidx.compose.foundation.layout.PaddingValues(vertical = 24.dp)) {
            item {
                Row(Modifier.fillMaxWidth().padding(horizontal = 24.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text("Flimmer", color = Accent, fontSize = 30.sp, fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
                    FButton("Aktualisieren", onRefresh)
                    Spacer(Modifier.width(8.dp))
                    FButton("$userName: Profil wechseln", onLogout)
                }
            }
            if (error.isNotEmpty()) item { Text(error, color = Color(0xFFFF5D5D), modifier = Modifier.padding(24.dp)) }
            if (shelves == null) item { Text("Lade Bibliothek …", color = Muted, modifier = Modifier.padding(24.dp)) }
            shelves?.forEachIndexed { si, shelf ->
                item(key = "shelf-" + shelf.title) {
                    Text(shelf.title, color = TextColor, fontSize = 20.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(start = 24.dp, top = 24.dp, bottom = 10.dp))
                    LazyRow(contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 24.dp), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                        items(shelf.items, key = { shelf.title + it.id }) { it ->
                            val seriesCard = shelf.title == "Serien"
                            Box(if (si == 0 && it == shelf.items.first()) Modifier.focusRequester(first) else Modifier) {
                                PosterCard(
                                    it, abs(if (shelf.wide) it.backdrop else it.poster), shelf.wide,
                                    title = it.series ?: it.displayTitle, sub = if (seriesCard) "" else sub(it),
                                ) { if (it.series != null) onOpenSeries(it.series) else onOpen(it) }
                            }
                        }
                    }
                }
            }
        }
        LaunchedEffect(shelves) { if (!shelves.isNullOrEmpty()) { delay(50); runCatching { first.requestFocus() } } }
    }
}

@Composable
private fun Hero(item: Item, title: String, abs: (String) -> String, extra: @Composable () -> Unit) {
    Box(Modifier.fillMaxWidth().height(420.dp)) {
        Art(abs(item.backdrop), title, Modifier.fillMaxSize())
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(listOf(Bg, Bg.copy(alpha = 0.85f), Bg.copy(alpha = 0.2f)))))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Color.Transparent, Bg))))
        Column(Modifier.align(Alignment.BottomStart).padding(32.dp).width(620.dp)) {
            Title(title, 36)
            val m = item.meta
            val facts = listOfNotNull(
                (m?.year ?: item.year)?.toString(),
                if (item.duration > 0) "${(item.duration / 60).toInt()} min" else null,
                m?.rating?.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
                m?.genres?.take(3)?.joinToString(", ")?.takeIf { it.isNotEmpty() },
            )
            Text(facts.joinToString(" · "), color = Muted, modifier = Modifier.padding(top = 6.dp))
            Row(Modifier.padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.size(10.dp).clip(CircleShape).background(lightColor(item.light)))
                Text(lightText(item.light), color = TextColor, modifier = Modifier.padding(start = 8.dp))
            }
            m?.overview?.let { Text(it, color = TextColor, maxLines = 5, modifier = Modifier.padding(top = 10.dp)) }
            extra()
        }
    }
}

@Composable
private fun PlayButtons(item: Item, onPlay: (Item, Double?) -> Unit) {
    val play = remember { FocusRequester() }
    Row(Modifier.padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        if (item.progress > 5 && !item.watched) {
            FButton("▶ Fortsetzen ab ${fmtTime(item.progress)}", { onPlay(item, null) }, Modifier.focusRequester(play), primary = true)
            FButton("Von vorn", { onPlay(item, 0.0) })
        } else {
            FButton("▶ Abspielen", { onPlay(item, 0.0) }, Modifier.focusRequester(play), primary = true)
        }
    }
    LaunchedEffect(item.id) { delay(50); runCatching { play.requestFocus() } }
}

@Composable
fun DetailScreen(item: Item, abs: (String) -> String, onPlay: (Item, Double?) -> Unit) {
    Page { LazyColumn { item { Hero(item, item.displayTitle, abs) { PlayButtons(item, onPlay) } } } }
}

@Composable
fun SeriesScreen(name: String, episodes: List<Item>, abs: (String) -> String, onPlay: (Item, Double?) -> Unit) {
    val seasons = episodes.groupBy { it.season ?: 0 }.toSortedMap()
    val all = seasons.values.flatten()
    val last = all.indexOfLast { it.progress > 0 }
    val next = when {
        last < 0 -> all.firstOrNull()
        all[last].watched && last + 1 < all.size -> all[last + 1]
        else -> all[last]
    } ?: return
    Page {
        LazyColumn(contentPadding = androidx.compose.foundation.layout.PaddingValues(bottom = 32.dp)) {
            item {
                Hero(next, name, abs) {
                    Text("Als Nächstes: S${next.season} E${next.episode} · ${next.displayTitle}", color = Muted, modifier = Modifier.padding(top = 8.dp))
                    PlayButtons(next, onPlay)
                }
            }
            seasons.forEach { (season, eps) ->
                item(key = "season-$season") {
                    Text(if (season == 0) "Specials" else "Staffel $season", color = TextColor, fontSize = 20.sp, fontWeight = FontWeight.SemiBold,
                        modifier = Modifier.padding(start = 24.dp, top = 20.dp, bottom = 10.dp))
                    LazyRow(contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 24.dp), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                        items(eps.sortedBy { it.episode ?: 0 }, key = { it.id }) { e ->
                            PosterCard(e, abs(e.backdrop), true, "${e.episode}. ${e.displayTitle}", if (e.duration > 0) "${(e.duration / 60).toInt()} min" else "") {
                                onPlay(e, null)
                            }
                        }
                    }
                }
            }
        }
    }
}

/** Kurz sichtbare Meldung, z. B. „Server nicht erreichbar“. */
@Composable
fun Toastish(text: String) {
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.BottomCenter) {
        Text(text, color = TextColor, modifier = Modifier.padding(32.dp).clip(RoundedCornerShape(12.dp)).background(Surface).padding(16.dp))
    }
}
