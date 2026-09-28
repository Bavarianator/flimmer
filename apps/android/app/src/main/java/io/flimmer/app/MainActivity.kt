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
import io.flimmer.app.ui.Shelf
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

    fun play(api: ApiClient, item: Item, start: Double?) {
        player.launch(
            Intent(this, PlayerActivity::class.java)
                .putExtra("server", api.base).putExtra("token", api.token)
                .putExtra("id", item.id).putExtra("start", start ?: -1.0),
        )
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
                        shelves = rows.map { Shelf(it.title, it.items, wide = true) } +
                            listOf(Shelf("Filme", lib.filter { it.series == null }), Shelf("Serien", lib.filter { it.series != null }.distinctBy { it.series }))
                                .filter { it.items.isNotEmpty() }
                    }.onFailure(::onAuthError)
                }
                HomeScreen(shelves, error, me?.name ?: "", c::abs,
                    onOpen = { go(Screen.Detail(it)) },
                    onOpenSeries = { go(Screen.Series(it)) },
                    onLogout = { scope.launch { c.logout(); store.token = ""; me = null; shelves = null; reset(Screen.Login) } },
                    onRefresh = { refresh.intValue++ })
            }

            is Screen.Detail -> {
                val c = api ?: return reset(Screen.Connect)
                val fresh = library.firstOrNull { it.id == s.item.id } ?: s.item
                DetailScreen(fresh, c::abs) { item, start -> play(c, item, start) }
            }

            is Screen.Series -> {
                val c = api ?: return reset(Screen.Connect)
                SeriesScreen(s.name, library.filter { it.series == s.name }, c::abs) { item, start -> play(c, item, start) }
            }
        }
    }
}
