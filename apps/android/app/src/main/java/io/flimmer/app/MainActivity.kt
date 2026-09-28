package io.flimmer.app

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import io.flimmer.app.ui.ConnectScreen
import io.flimmer.app.ui.DetailScreen
import io.flimmer.app.ui.FlimmerTheme
import io.flimmer.app.ui.HomeScreen
import io.flimmer.app.ui.LoginScreen
import io.flimmer.app.ui.SeriesScreen
import io.flimmer.app.ui.BibliothekScreen
import io.flimmer.app.ui.ProfilScreen
import io.flimmer.app.ui.Raster
import io.flimmer.app.ui.SearchScreen
import io.flimmer.app.ui.Shelf
import io.flimmer.app.ui.Shell
import io.flimmer.app.ui.Tab
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

sealed interface Screen {
    data object Connect : Screen
    data object Login : Screen
    data object Home : Screen
    data class Detail(val item: Item) : Screen
    data class Series(val name: String) : Screen
}

class MainActivity : ComponentActivity() {
    private val refresh = mutableIntStateOf(0) // nach dem Player: „Weiterschauen“ neu laden

    private val player = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { refresh.intValue++ }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val store = Store(this)
        val tv = isTv(this)
        val profile = deviceProfile(this)
        setContent { FlimmerTheme(tv) { App(store, profile, tv) } }
    }

    fun play(api: ApiClient, item: Item, start: Double?, party: String? = null) = play(api, item.id, start, party)

    fun play(api: ApiClient, itemId: String, start: Double?, party: String?) {
        player.launch(
            Intent(this, PlayerActivity::class.java)
                .putExtra("server", api.base).putExtra("token", api.token)
                .putExtra("id", itemId).putExtra("start", start ?: -1.0).putExtra("party", party),
        )
    }

    fun partyFailed(e: Throwable) =
        android.widget.Toast.makeText(this, "Gemeinsam schauen geht gerade nicht: ${e.message ?: "Server nicht erreichbar"}", android.widget.Toast.LENGTH_LONG).show()

    /** Neuen Raum für „Gemeinsam schauen“ öffnen und selbst beitreten. */
    suspend fun startParty(api: ApiClient, item: Item) {
        val room = PartyClient(api).create(item.id)
        play(api, item.id, 0.0, room.id)
    }

    /** Raum per Code beitreten: der Raum bestimmt den Titel. */
    suspend fun joinParty(api: ApiClient, code: String) {
        val room = PartyClient(api).get(code.trim().lowercase())
        play(api, room.state.mediaId, null, room.id)
    }

    @Composable
    private fun App(store: Store, profile: Profile, tv: Boolean) {
        val scope = rememberCoroutineScope()
        var api by remember { mutableStateOf(store.server.takeIf { it.isNotEmpty() }?.let { ApiClient(it, store.token) }) }
        val stack = remember { mutableStateListOf<Screen>(if (api == null) Screen.Connect else Screen.Home) }
        var error by remember { mutableStateOf("") }
        var found by remember { mutableStateOf<List<String>?>(null) }
        var users by remember { mutableStateOf<List<User>?>(null) }
        var pairCode by remember { mutableStateOf<String?>(null) }
        var me by remember { mutableStateOf<User?>(null) }
        var shelves by remember { mutableStateOf<List<Shelf>?>(null) }
        var library by remember { mutableStateOf<List<Item>>(emptyList()) }
        var tab by remember { mutableStateOf(Tab.Start) }
        var watch by remember { mutableStateOf(store.watchlist) }
        fun watchKey(it: Item) = it.series?.let { "s:$it" } ?: it.id // Serien als Ganzes merken
        fun toggle(it: Item) {
            val k = watchKey(it)
            watch = if (k in watch) watch - k else watch + k
            store.watchlist = watch
        }
        val screen = stack.last()

        fun go(s: Screen) = stack.add(s)
        fun reset(s: Screen) { stack.clear(); stack.add(s) }
        fun onAuthError(e: Throwable) {
            if (e is ApiException && e.code == 401) { store.token = ""; reset(Screen.Login) } else error = "Server nicht erreichbar: ${e.message}"
        }

        BackHandler(enabled = stack.size > 1) { stack.removeAt(stack.lastIndex) }

        when (val s = screen) {
            Screen.Connect -> {
                LaunchedEffect(Unit) { found = null; found = discover(this@MainActivity) }
                ConnectScreen(found, error) { input ->
                    val url = normalizeServer(input)
                    scope.launch {
                        error = ""
                        val c = ApiClient(url)
                        runCatching { c.users() }
                            .onSuccess { store.server = url; store.token = ""; api = c; reset(Screen.Login) }
                            .onFailure { error = "Kein Flimmer-Server unter $url" }
                    }
                }
            }

            Screen.Login -> {
                val c = api ?: return reset(Screen.Connect)
                LaunchedEffect(c) {
                    users = runCatching { c.users() }.getOrElse { error = "Server nicht erreichbar"; emptyList() }
                }
                if (tv) LaunchedEffect(c) {
                    // TV-Kopplung: Code anzeigen, pollen bis am Handy bestätigt; abgelaufen → neuer Code.
                    while (true) {
                        val p = runCatching { c.pairStart(profile.name) }.getOrNull() ?: break
                        pairCode = p.code
                        val token = run {
                            while (true) {
                                delay(2000)
                                val r = runCatching { c.pairPoll(p.code, p.secret) }
                                if (r.isFailure) return@run null
                                r.getOrNull()?.let { return@run it }
                            }
                            null
                        }
                        if (token != null) { store.token = token; reset(Screen.Home); break }
                    }
                }
                LoginScreen(users, pairCode, error, onLogin = { u, pw ->
                    scope.launch {
                        error = ""
                        runCatching { c.login(u.id, pw, profile.name) }
                            .onSuccess { store.token = it.token; reset(Screen.Home) }
                            .onFailure { error = if ((it as? ApiException)?.code == 429) "Zu viele Versuche – bitte kurz warten." else "Das hat nicht geklappt." }
                    }
                }, onChangeServer = { store.server = ""; api = null; reset(Screen.Connect) })
            }

            Screen.Home -> {
                val c = api ?: return reset(Screen.Connect)
                LaunchedEffect(c, refresh.intValue) {
                    error = ""
                    runCatching {
                        me = c.me()
                        runCatching { c.putProfile(store.deviceId, profile) } // Diagnose/Einstellungen
                        val lib = c.library(profile)
                        library = lib
                        val rows = runCatching { c.home(profile) }.getOrDefault(emptyList())
                        shelves = rows.filter { it.items.isNotEmpty() }.map { Shelf(it.title, it.items, wide = true) } +
                            listOf(Shelf("Filme", lib.filter { it.series == null }), Shelf("Serien", lib.filter { it.series != null }.distinctBy { it.series }))
                                .filter { it.items.isNotEmpty() }
                    }.onFailure(::onAuthError)
                }
                val open: (Item) -> Unit = { if (it.series != null) go(Screen.Series(it.series)) else go(Screen.Detail(it)) }
                val logout: () -> Unit = { scope.launch { c.logout(); store.token = ""; me = null; shelves = null; reset(Screen.Login) } }
                BackHandler(enabled = tab != Tab.Start) { tab = Tab.Start }
                Shell(tab, me?.name ?: "", me?.color ?: 0, error, onTab = { tab = it; error = "" }) {
                    when (tab) {
                        Tab.Start -> HomeScreen(shelves, error, c::abs, open) { item, start -> play(c, item, start) }
                        Tab.Filme -> Raster(library.filter { it.series == null }, c::abs, "Noch keine Filme", open)
                        Tab.Serien -> Raster(library.filter { it.series != null }.distinctBy { it.series }, c::abs, "Noch keine Serien", open)
                        Tab.Merkliste -> Raster(library.filter { watchKey(it) in watch }.distinctBy { watchKey(it) }, c::abs,
                            "Noch nichts gemerkt – bei einem Titel „+ Merkliste“ wählen.", open)
                        Tab.Suche -> SearchScreen(library, c::abs, open)
                        Tab.Bibliothek -> BibliothekScreen(library, library.filter { watchKey(it) in watch }.distinctBy { watchKey(it) }, c::abs, open)
                        Tab.Profil -> ProfilScreen(me?.name ?: "", me?.color ?: 0, c.base, profile.name + " · " + profile.video.joinToString(", "),
                            logout, onChangeServer = { store.server = ""; api = null; reset(Screen.Connect) }, onRefresh = { refresh.intValue++ },
                            onJoin = { code -> scope.launch { runCatching { joinParty(c, code) }.onFailure { error = "Raum „$code“ nicht gefunden" } } })
                    }
                }
            }

            is Screen.Detail -> {
                val c = api ?: return reset(Screen.Connect)
                val fresh = library.firstOrNull { it.id == s.item.id } ?: s.item
                DetailScreen(fresh, c::abs, watchKey(fresh) in watch, { toggle(fresh) }, onParty = { scope.launch { runCatching { startParty(c, fresh) }.onFailure { partyFailed(it) } } }) { item, start -> play(c, item, start) }
            }

            is Screen.Series -> {
                val c = api ?: return reset(Screen.Connect)
                val eps = library.filter { it.series == s.name }
                val key = eps.firstOrNull()
                SeriesScreen(s.name, eps, c::abs, key != null && watchKey(key) in watch, { key?.let(::toggle) },
                    onParty = { e -> scope.launch { runCatching { startParty(c, e) }.onFailure { partyFailed(it) } } }) { item, start -> play(c, item, start) }
            }
        }
    }
}
