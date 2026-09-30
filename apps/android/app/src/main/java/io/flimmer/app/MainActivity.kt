package io.flimmer.app

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.*
import androidx.mediarouter.media.MediaRouter
import com.google.android.gms.cast.framework.CastContext
import io.flimmer.app.admin.Vpn
import io.flimmer.app.admin.hol
import io.flimmer.app.ui.*
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

sealed interface Screen {
    data object Connect : Screen
    data object Login : Screen
    data class Haupt(val b: Bereich) : Screen
    data class Detail(val item: Item) : Screen
    data class Series(val name: String, val staffel: Int? = null) : Screen
    data class PersonS(val name: String) : Screen
    data class ListeS(val liste: Liste, val sammlung: Boolean) : Screen
    data class Dashboard(val bereich: String) : Screen // Paket admin, nur Admins
    data object Lobby : Screen // Gemeinsam schauen (Paket gemeinsam)
    data class Registrieren(val invite: String) : Screen // Einladung einlösen (Link oder QR)
}

class MainActivity : ComponentActivity() {
    private val refresh = mutableIntStateOf(0) // nach dem Player: „Weiterschauen“ neu laden

    private val player = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { refresh.intValue++ }
    private lateinit var downloads: Downloads
    private lateinit var store: Store
    private val dlTick = mutableIntStateOf(0) // nach einem gestarteten Download: Knöpfe neu beschriften

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        store = Store(this)
        downloads = Downloads(this)
        val tv = isTv(this)
        val profile = deviceProfile(this)
        setContent { FlimmerTheme(tv) { App(profile, tv) } }
    }

    /**
     * Heimnetz nicht erreichbar: über die gespeicherte VPN-Adresse versuchen. Antwortet sie nicht, NetBird öffnen
     * (dort verbinden) und bis 30 s warten. Ganz ohne Zutun geht es mit „Durchgehend aktives VPN“ für NetBird.
     */
    private suspend fun ueberVpn(c: ApiClient, e: Throwable): ApiClient? {
        val vpn = store.vpn
        if ((e as? ApiException)?.code == 401 || vpn.isEmpty() || c.base == vpn) return null
        val v = ApiClient(vpn, store.token)
        suspend fun geht() = runCatching { v.me() }.isSuccess
        if (geht()) return v
        startActivity(packageManager.getLaunchIntentForPackage("io.netbird.client") ?: return null)
        toast("In NetBird verbinden – Flimmer wartet", true)
        repeat(15) { delay(2000); if (geht()) return v }
        return null
    }

    private fun toast(text: String, lang: Boolean = false) = Toast.makeText(this, text, if (lang) Toast.LENGTH_LONG else Toast.LENGTH_SHORT).show()

    /**
     * Startet den Player. Heruntergeladene Titel laufen aus der lokalen Datei (spart Netz, geht offline).
     * [queue]: danach der Reihe nach (nächste Folgen, Wiedergabeliste), wenn „Nächste Folge automatisch“ an ist.
     */
    fun play(api: ApiClient, item: Item, start: Double?, party: String? = null, queue: List<Item> = emptyList(), spur: Spur? = null) {
        val local = downloads.get(item.id)?.takeIf { party == null && it.status == Downloads.Status.Fertig }
        val i = Intent(this, PlayerActivity::class.java)
            .putExtra("server", api.base).putExtra("token", api.token).putExtra("id", item.id)
            .putExtra("titel", item.series ?: item.displayTitle)
            .putExtra("zeile", if (item.series != null) "${folge(item)} · ${item.displayTitle}" else jahr(item))
            .putExtra("queue", queue.map { it.id }.toTypedArray())
            .putExtra("queueTitel", queue.map { it.series ?: it.displayTitle }.toTypedArray())
            .putExtra("queueZeile", queue.map { if (it.series != null) "${folge(it)} · ${it.displayTitle}" else jahr(it) }.toTypedArray())
            .putExtra("tonNr", spur?.ton ?: -2).putExtra("utNr", spur?.ut ?: -2)
        if (local != null) i.putExtra("file", local.file.path).putExtra("start", start ?: downloads.pendingPos(item.id) ?: item.progress)
        else i.putExtra("start", start ?: -1.0).putExtra("party", party)
        player.launch(i)
    }

    private suspend fun download(api: ApiClient, items: List<Item>) {
        runCatching { items.forEach { downloads.start(api, it) } }
            .onSuccess { toast(if (items.size == 1) "Download gestartet" else "${items.size} Downloads gestartet") }
            .onFailure { toast("Download geht gerade nicht: ${it.message}", true) }
        dlTick.intValue++
    }

    fun partyFailed(e: Throwable) = toast("Gemeinsam schauen geht gerade nicht: ${e.message ?: "Server nicht erreichbar"}", true)

    /** Neuen Raum für „Gemeinsam schauen“ öffnen und selbst beitreten. */
    suspend fun startParty(api: ApiClient, item: Item) {
        val room = PartyClient(api).create(item.id)
        play(api, item, 0.0, room.id)
    }

    /** Raum per Code beitreten: der Raum bestimmt den Titel. */
    suspend fun joinParty(api: ApiClient, code: String, library: List<Item>) {
        val room = PartyClient(api).get(code.trim().lowercase())
        val item = library.firstOrNull { it.id == room.state.mediaId } ?: Item(room.state.mediaId, "Gemeinsam schauen")
        play(api, item, null, room.id)
    }

    /** Live-TV: fertige HLS-URL, der Player spielt ohne Zeitleiste und Fortschritt. */
    private fun live(api: ApiClient, url: String, titel: String) = player.launch(
        Intent(this, PlayerActivity::class.java).putExtra("server", api.base).putExtra("token", api.token).putExtra("live_url", url).putExtra("live_titel", titel),
    )

    private fun oeffneLink(url: String) = runCatching { startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url))) }.onFailure { toast("Kein Browser gefunden") }

    @Composable
    private fun App(profile: Profile, tv: Boolean) {
        val scope = rememberCoroutineScope()
        var api by remember { mutableStateOf(store.server.takeIf { it.isNotEmpty() }?.let { ApiClient(it, store.token) }) }
        val stack = remember { mutableStateListOf<Screen>(if (api == null) Screen.Connect else Screen.Haupt(Bereich.Start)) }
        var error by remember { mutableStateOf("") }
        var found by remember { mutableStateOf<List<String>?>(null) }
        var users by remember { mutableStateOf<List<User>?>(null) }
        var pairCode by remember { mutableStateOf<String?>(null) }
        var me by remember { mutableStateOf<User?>(null) }
        var rows by remember { mutableStateOf<List<HomeRow>?>(null) }
        var library by remember { mutableStateOf<List<Item>>(emptyList()) }
        val favoriten = remember { Favoriten(store) }
        var favs by remember { mutableStateOf(store.favoriten) }
        var sammlungen by remember { mutableStateOf<List<Liste>?>(null) }
        var listen by remember { mutableStateOf<List<Liste>?>(null) }
        var verbunden by remember { mutableStateOf(true) }
        var aktionen by remember { mutableStateOf<Item?>(null) }
        var castWahl by remember { mutableStateOf(false) }
        var castZiel by remember { mutableStateOf<MediaRouter.RouteInfo?>(null) }
        var editor by remember { mutableStateOf<Item?>(null) } // Metadaten-Editor (Admin)
        var einstellung by remember { mutableStateOf<String?>(null) } // Bereich unter Einstellungen
        val spuren = remember { mutableStateMapOf<String, Spur>() }
        var verlauf by remember { mutableStateOf(store.suchverlauf) }
        var gruppen by remember { mutableStateOf<List<OffeneGruppe>>(emptyList()) }
        val weg = remember { mutableStateListOf<String>() } // ausgeblendete bzw. schon beigetretene Gruppen
        val screen = stack.last()

        fun go(s: Screen) = stack.add(s)
        fun reset(s: Screen) { stack.clear(); stack.add(s) }
        fun bereich(b: Bereich) {
            if (b == Bereich.Suche) { go(Screen.Haupt(b)); return }
            reset(Screen.Haupt(Bereich.Start))
            if (b != Bereich.Start) go(Screen.Haupt(b))
        }
        fun onAuthError(e: Throwable) {
            if (e is ApiException && e.code == 401) { store.token = ""; reset(Screen.Login); return }
            verbunden = false
            error = "Server nicht erreichbar: ${e.message}"
            // Offline: gleich zu den Downloads, wenn es welche gibt
            if (!tv && screen != Screen.Haupt(Bereich.Downloads) && downloads.list().any { it.status == Downloads.Status.Fertig }) go(Screen.Haupt(Bereich.Downloads))
        }
        fun abmelden() = scope.launch { api?.logout(); store.token = ""; me = null; rows = null; library = emptyList(); reset(Screen.Login) }
        fun serverWechseln() { store.server = ""; api = null; reset(Screen.Connect) }
        // Adresse, Server-QR oder Einladungslink: verbinden, bei einer Einladung gleich registrieren.
        fun verbinden(input: String) {
            val link = parseLink(input)
            scope.launch {
                error = ""
                val c = ApiClient(link.server)
                runCatching { c.users() }
                    .onSuccess { store.server = link.server; store.token = ""; api = c; reset(Screen.Login); link.invite?.let { go(Screen.Registrieren(it)) } }
                    .onFailure { error = "Kein Flimmer-Server unter ${link.server}" }
            }
        }
        fun scannen() = scanQr(this@MainActivity, { toast(it, true) }, ::verbinden)

        BackHandler(enabled = stack.size > 1) { stack.removeAt(stack.lastIndex) }

        if (screen == Screen.Connect) {
            LaunchedEffect(Unit) { found = null; found = discover(this@MainActivity) }
            ConnectScreen(found, error, if (tv) null else ::scannen, ::verbinden)
            return
        }
        val c = api ?: return reset(Screen.Connect)

        if (screen is Screen.Registrieren) {
            RegistrierenScreen(error, onRegister = { name, pw ->
                scope.launch {
                    error = ""
                    runCatching { c.redeem(screen.invite, name, pw) }
                        .onSuccess { store.token = it; reset(Screen.Haupt(Bereich.Start)) }
                        .onFailure {
                            error = when ((it as? ApiException)?.code) {
                                409 -> "Diesen Namen gibt es schon. Bitte wähle einen anderen."
                                410 -> "Diese Einladung gilt nicht mehr. Frag nach einem neuen Link."
                                429 -> "Zu viele Versuche – bitte eine Minute warten."
                                else -> "Das hat nicht geklappt: ${it.message}"
                            }
                        }
                }
            }, onBack = { error = ""; stack.removeAt(stack.lastIndex) })
            return
        }

        if (screen == Screen.Login) {
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
                    if (token != null) { store.token = token; reset(Screen.Haupt(Bereich.Start)); break }
                }
            }
            LoginScreen(users, pairCode, pairCode?.let { "${c.base}/api/pair/$it/qr" }, error, onLogin = { u, pw ->
                scope.launch {
                    error = ""
                    runCatching { c.login(u.id, pw, profile.name) }
                        .onSuccess { store.token = it.token; reset(Screen.Haupt(Bereich.Start)) }
                        .onFailure { error = if ((it as? ApiException)?.code == 429) "Zu viele Versuche – bitte kurz warten." else "Das hat nicht geklappt." }
                }
            }, onScan = ::scannen, onChangeServer = ::serverWechseln)
            return
        }

        // ponytail: fragt auch im Hintergrund (Player offen) weiter – 1 kleine Anfrage alle 20 s; Lifecycle-Bindung, falls das stört
        LaunchedEffect(c) {
            while (true) {
                gruppen = runCatching { optional { c.offeneGruppen() } }.getOrNull().orEmpty()
                delay(20_000)
            }
        }

        // Angemeldet: Bibliothek, Startseite, Favoriten und die neuen Listen laden (einmal und nach jedem Player-Ende).
        LaunchedEffect(c, refresh.intValue) {
            error = ""
            runCatching {
                me = runCatching { c.me() }.getOrElse { e -> api = ueberVpn(c, e) ?: throw e; return@LaunchedEffect }
                verbunden = true
                // NetBird-/Tailscale-Adresse des Servers für unterwegs merken (Gäste bekommen 403)
                runCatching { c.hol<Vpn>("/api/vpn") }.getOrNull()?.addrs?.let { l -> (l.firstOrNull { it.provider == "netbird" } ?: l.firstOrNull())?.let { store.vpn = it.url } }
                downloads.flush(c) // offline Geschautes nachreichen, bevor „Weiterschauen“ lädt
                runCatching { c.putProfile(store.deviceId, profile) } // Diagnose/Einstellungen
                library = c.library(profile)
                rows = runCatching { c.home(profile) }.getOrDefault(emptyList())
                favs = favoriten.laden(c)
                sammlungen = optional { c.collections() }
                listen = optional { c.playlists() }
            }.onFailure(::onAuthError)
        }

        dlTick.intValue // Lesen abonniert die Neuberechnung nach gestarteten Downloads
        // Startseiten-Reihen nach Einstellungen › Startseite sortieren; was dort aus ist, zeigt StartSeite auch nicht aus der Bibliothek
        val zeilen = rows?.let { io.flimmer.app.einstellungen.startReihen(this, it) }
        val versteckt = io.flimmer.app.einstellungen.startVorlieben(this).aus.toSet()
        val d = Daten(library, zeilen, favs, sammlungen, listen, c::abs, me?.admin == true, versteckt)
        fun abspielen(item: Item, start: Double?, queue: List<Item>? = null) {
            val route = castZiel
            if (route != null) {
                scope.launch {
                    runCatching { Cast.play(this@MainActivity, route, c, item, start) }
                        .onSuccess { toast("Läuft auf ${route.name}") }.onFailure { toast("Chromecast: ${it.message}", true) }
                }
                return
            }
            // Folgen: danach die nächsten der Serie
            val q = queue ?: item.series?.let { s -> d.serien[s].orEmpty().dropWhile { it.id != item.id }.drop(1) }.orEmpty()
            play(c, item, start, queue = q, spur = spuren[item.id])
        }
        fun listenLaden(sammlung: Boolean) = scope.launch { if (sammlung) sammlungen = optional { c.collections() } else listen = optional { c.playlists() } }
        /** Liste sofort lokal ändern (flüssiges Umsortieren), dann Server, dann neu laden. */
        fun aendern(sammlung: Boolean, id: String?, lokal: ((Liste) -> Liste?)?, meldung: String?, aufruf: suspend () -> Unit) {
            if (id != null && lokal != null) {
                val neu: (List<Liste>?) -> List<Liste>? = { l -> l?.mapNotNull { if (it.id == id) lokal(it) else it } }
                if (sammlung) sammlungen = neu(sammlungen) else listen = neu(listen)
            }
            scope.launch {
                runCatching { aufruf() }.onSuccess { meldung?.let { toast(it) } }.onFailure { toast("Ging nicht: ${it.message}") }
                listenLaden(sammlung)
            }
        }
        val pflege = ListenPflege(
            neu = { s, name, keys -> aendern(s, null, null, "„$name“ angelegt") { c.listeAnlegen(s, name, keys) } },
            hinzu = { l, s, keys -> aendern(s, l.id, { it.copy(items = it.items + keys.filter { k -> k !in it.items }) }, "Zu „${l.name}“ hinzugefügt") { c.listeAendern(s, l.id, add = keys) } },
            umbenennen = { l, s, name -> aendern(s, l.id, { it.copy(name = name) }, null) { c.listeAendern(s, l.id, name = name) } },
            loeschen = { l, s -> aendern(s, l.id, { null }, "„${l.name}“ gelöscht") { c.listeLoeschen(s, l.id) } },
            entfernen = { l, s, key -> aendern(s, l.id, { it.copy(items = it.items - key) }, null) { c.listeAendern(s, l.id, remove = listOf(key)) } },
            ordnen = { l, s, items -> aendern(s, l.id, { it.copy(items = items) }, null) { c.listeAendern(s, l.id, items = items) } },
        )
        val h = Handlungen(
            oeffnen = { if (it.series != null) go(Screen.Series(it.series)) else go(Screen.Detail(it)) },
            abspielen = { item, start -> abspielen(item, start) },
            alle = { l -> l.firstOrNull()?.let { abspielen(it, 0.0, l.drop(1)) } },
            favorit = { key ->
                val an = key !in favs
                favs = if (an) favs + key else favs - key
                scope.launch {
                    runCatching { favoriten.setzen(c, key, an) }.onFailure { favs = if (an) favs - key else favs + key; toast("Favorit ging nicht: ${it.message}") }
                }
            },
            mehr = { aktionen = it },
            gesehen = { l, an ->
                val ids = l.map { it.id }.toSet()
                library = library.map { if (it.id in ids) it.copy(watched = an, progress = 0.0) else it }
                scope.launch {
                    runCatching { l.forEach { c.watched(it.id, an) } }.onFailure { toast("Ging nicht: ${it.message}") }
                    refresh.intValue++
                }
            },
            person = { go(Screen.PersonS(it)) },
            liste = { l, s -> go(Screen.ListeS(l, s)) },
            bereich = ::bereich,
            zurueck = { if (stack.size > 1) stack.removeAt(stack.lastIndex) },
            cast = if (tv) null else ({ castWahl = true }),
            party = { item -> scope.launch { runCatching { startParty(c, item) }.onFailure { partyFailed(it) } } },
            download = if (tv) null else ({ l -> scope.launch { download(c, l) } }),
            dlStatus = { id -> dlTick.intValue; downloads.get(id)?.status },
            offenerLink = ::oeffneLink,
            pflege = pflege,
            metadaten = if (me?.admin == true) ({ editor = it }) else null,
        )
        // Offene Gruppen der anderen: Angebot zum Beitreten über dem Inhalt, bis man es wegklickt
        val angebot = gruppen.firstOrNull { g -> g.id !in weg && g.host != me?.name && me?.name !in g.members }?.let { g ->
            val x = library.firstOrNull { it.id == g.mediaId }
            PartyAngebot("${g.host.ifEmpty { "Jemand" }} schaut ${x?.let { "„${it.series ?: it.displayTitle}“" } ?: "gerade"} · ${g.members.size} dabei",
                onBeitreten = { weg.add(g.id); scope.launch { runCatching { joinParty(c, g.id, library) }.onFailure { partyFailed(it) } } },
                onZu = { weg.add(g.id) })
        }
        val medien = listOf(Bereich.Filme, Bereich.Serien, Bereich.LiveTv) + listOfNotNull(sammlungen?.let { Bereich.Sammlungen }, listen?.let { Bereich.Listen })
        val admin = me?.admin == true
        val k = Kopf(
            me, c.base, medien, verbunden, castZiel?.name,
            onBereich = ::bereich, onCast = h.cast, onParty = { go(Screen.Lobby) },
            onProfil = { abmelden() }, onAbmelden = { abmelden() }, onServer = ::serverWechseln,
            onDashboard = if (admin) ({ go(Screen.Dashboard("")) }) else null,
            onMetadaten = if (admin) ({ go(Screen.Dashboard("metadaten")) }) else null,
            angebot = angebot,
        )

        when (val s = screen) {
            is Screen.Haupt -> when (s.b) {
                Bereich.Start -> Rahmen(k, s.b, if (tv) "" else "Startseite", if (tv) emptyList() else listOf("Startseite", "Favoriten"), error) { tab ->
                    if (tab == 0) StartSeite(d, h) else FavoritenSeite(d, h)
                }
                Bereich.Favoriten -> Rahmen(k, s.b, "Favoriten", emptyList(), error) { FavoritenSeite(d, h) }
                Bereich.Filme, Bereich.Serien -> {
                    val serien = s.b == Bereich.Serien
                    Rahmen(k, s.b, s.b.label, bibliothekTabs(serien), error) { tab -> BibliothekSeite(serien, tab, d, h) }
                }
                Bereich.LiveTv -> Rahmen(k, s.b, s.b.label, emptyList(), error) {
                    io.flimmer.app.livetv.LiveTVScreen(c, tv) { url, titel -> live(c, url, titel) }
                }
                Bereich.Sammlungen -> Rahmen(k, s.b, s.b.label, emptyList(), error) { ListenSeite(sammlungen, true, d, h) }
                Bereich.Listen -> Rahmen(k, s.b, s.b.label, emptyList(), error) { ListenSeite(listen, false, d, h) }
                Bereich.Downloads -> Rahmen(k, s.b, s.b.label, emptyList(), error) { DownloadsScreen(downloads) { play(c, it, null) } }
                Bereich.Suche -> SucheSeite(d, h, verlauf, { verlauf = it; store.suchverlauf = it }) { q ->
                    runCatching { optional { c.search(q, store.deviceId) } }.getOrNull() // ohne Server: lokale Treffer bleiben
                }
                Bereich.Einstellungen -> {
                    // Handy: aus einem Bereich zurück zur Übersicht; Tablet/TV zeigen Liste und Inhalt nebeneinander
                    BackHandler(enabled = einstellung != null && !tv && !LocalBreit.current) { einstellung = null }
                    Rahmen(k, s.b, s.b.label, emptyList(), error) {
                        io.flimmer.app.einstellungen.EinstellungenScreen(c, einstellung, tv) { einstellung = it }
                    }
                }
            }

            is Screen.Detail -> {
                val fresh = library.firstOrNull { it.id == s.item.id } ?: s.item
                var det by remember(fresh.id) { mutableStateOf<Details?>(null) }
                LaunchedEffect(fresh.id) { det = runCatching { optional { c.details(fresh.id, store.deviceId) } }.getOrNull() }
                FilmDetail(fresh, det, d, h, spuren[fresh.id] ?: Spur()) { spuren[fresh.id] = it }
            }

            is Screen.Series -> {
                val eps = d.serien[s.name].orEmpty()
                var info by remember(s.name) { mutableStateOf<SeriesInfo?>(null) }
                LaunchedEffect(s.name) { info = runCatching { optional { c.seriesInfo(s.name) } }.getOrNull() }
                if (eps.isEmpty()) UnterSeite(s.name, h.zurueck) { T(if (library.isEmpty()) "Lade …" else "Diese Serie gibt es nicht mehr.", color = K.Text2) }
                else SerienDetail(s.name, eps, info, d, h, s.staffel)
            }

            is Screen.PersonS -> {
                var p by remember(s.name) { mutableStateOf<PersonPage?>(null) }
                var fehlt by remember(s.name) { mutableStateOf(false) }
                LaunchedEffect(s.name) { p = runCatching { optional { c.person(s.name, store.deviceId) } }.getOrNull(); fehlt = p == null }
                UnterSeite(s.name, h.zurueck) {
                    val items = p?.items?.map { i -> library.firstOrNull { it.id == i.id } ?: i }
                    PersonSeite(s.name, p?.image?.let(c::abs) ?: "", p?.bio, if (fehlt) emptyList() else items, d, h)
                }
            }

            is Screen.ListeS -> {
                val aktuell = (if (s.sammlung) sammlungen else listen)?.firstOrNull { it.id == s.liste.id } ?: s.liste
                UnterSeite(aktuell.name, h.zurueck) { ListeSeite(aktuell, s.sammlung, d, h) }
            }

            Screen.Lobby -> UnterSeite("Gemeinsam schauen", h.zurueck) {
                io.flimmer.app.gemeinsam.GemeinsamLobby(c, tv) { raum ->
                    scope.launch { runCatching { joinParty(c, raum, library) }.onFailure { partyFailed(it) } }
                }
            }

            is Screen.Dashboard -> io.flimmer.app.admin.DashboardScreen(c, s.bereich, tv,
                onBereich = { b -> stack[stack.lastIndex] = Screen.Dashboard(b) }, // Bereich ersetzen, nicht stapeln
                onZurueck = { if (stack.size > 1) stack.removeAt(stack.lastIndex) else bereich(Bereich.Start) })

            else -> Unit
        }

        aktionen?.let { item ->
            AktionenBlatt(item, d, h, onTeilen = if (tv) null else ({ teilen(c, item) }), onZu = { aktionen = null })
        }

        if (castWahl) {
            var geraete by remember { mutableStateOf<List<MediaRouter.RouteInfo>?>(emptyList()) }
            LaunchedEffect(Unit) { runCatching { Cast.devices(this@MainActivity).collect { geraete = it } }.onFailure { geraete = null } }
            CastWahl(geraete?.map { it.name }, castZiel?.name, onWahl = { name ->
                val route = geraete?.firstOrNull { it.name == name }
                if (route == null && castZiel != null) runCatching { CastContext.getSharedInstance(this@MainActivity).sessionManager.endCurrentSession(true) }
                castZiel = route
                castWahl = false
                if (route != null) toast("Wiedergabe jetzt auf ${route.name}")
            }, onZu = { castWahl = false })
        }

        editor?.let { e -> io.flimmer.app.admin.MetadatenEditor(c, e.id) { editor = null; refresh.intValue++ } }
    }

    private fun teilen(c: ApiClient, item: Item) {
        val url = c.base + if (item.series != null) "/serie/" + Uri.encode(item.series) else "/film/" + item.id
        startActivity(Intent.createChooser(Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, url), "Link teilen"))
    }

}
