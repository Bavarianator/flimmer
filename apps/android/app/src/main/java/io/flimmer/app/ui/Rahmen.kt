package io.flimmer.app.ui

import androidx.activity.compose.BackHandler
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.DrawerValue
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.ModalNavigationDrawer
import androidx.compose.material3.rememberDrawerState
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.zIndex
import io.flimmer.app.User
import kotlinx.coroutines.launch

/** Hauptmenü aus docs/umbau-jellyfin.md. Musik folgt in Phase 2. */
enum class Bereich(val label: String, val icon: String) {
    Start("Startseite", Ic.START), Favoriten("Favoriten", Ic.HERZ), Downloads("Downloads", Ic.DOWNLOAD),
    Filme("Filme", Ic.FILM), Serien("Serien", Ic.SERIE), LiveTv("Live-TV", Ic.LIVE), Sammlungen("Sammlungen", Ic.SAMMLUNG), Listen("Wiedergabelisten", Ic.LISTE),
    Suche("Suche", Ic.SUCHE), Einstellungen("Einstellungen", Ic.EINSTELLUNGEN),
}

/** Was der Rahmen über Benutzer und Server wissen muss, und wohin seine Knöpfe führen. */
class Kopf(
    val me: User?,
    val server: String,
    val medien: List<Bereich>, // sichtbare Einträge unter MEDIEN
    val verbunden: Boolean,
    val castZiel: String?, // Name des Cast-Geräts, null = dieses Gerät
    val onBereich: (Bereich) -> Unit,
    val onCast: (() -> Unit)?, // null = kein Cast (TV, ohne Play-Dienste)
    val onParty: () -> Unit,
    val onProfil: () -> Unit,
    val onAbmelden: () -> Unit,
    val onServer: () -> Unit,
    val onDashboard: (() -> Unit)?, // nur Admins
    val onMetadaten: (() -> Unit)? = null,
    val angebot: PartyAngebot? = null, // offene Gruppe eines anderen: Balken über dem Inhalt
)

/** „Anna schaut „Heat“ · 2 dabei“ mit Beitreten und Ausblenden. */
class PartyAngebot(val text: String, val onBeitreten: () -> Unit, val onZu: () -> Unit)

@Composable
private fun Angebot(a: PartyAngebot?) {
    if (a == null) return
    val tv = LocalTv.current
    Row(Modifier.fillMaxWidth().padding(horizontal = Pad, vertical = 8.dp).background(K.Flaeche2, RoundedCornerShape(Tokens.Radius.RadiusS))
        .padding(start = 16.dp, end = 4.dp, top = 6.dp, bottom = 6.dp), verticalAlignment = Alignment.CenterVertically) {
        Ico(Ic.GEMEINSAM, if (tv) 32.dp else 22.dp, K.Text)
        T(a.text, LocalTypo.current.klein, K.Text, FontWeight.Medium, maxLines = 2, modifier = Modifier.weight(1f).padding(horizontal = 12.dp))
        Knopf("Beitreten", a.onBeitreten, primary = true)
        IconKnopf(Ic.SCHLIESSEN, a.onZu, farbe = K.Text2)
    }
}

/**
 * Rahmen jeder Hauptseite. Handy: Kopfzeile 56 dp + Schublade 312 dp + wischbare Tabs.
 * Tablet: Seitenleiste 88 dp + Kopfzeile 64 dp mit Tabs. TV: Icon-Schiene links, die beim Fokus aufklappt.
 */
@Composable
fun Rahmen(k: Kopf, bereich: Bereich, titel: String, tabs: List<String>, fehler: String, startTab: Int = 0, inhalt: @Composable (Int) -> Unit) {
    val tv = LocalTv.current
    val breit = LocalBreit.current
    var tab by rememberSaveable(bereich, tabs.size) { mutableIntStateOf(startTab.coerceIn(0, maxOf(0, tabs.size - 1))) }
    Box(Modifier.fillMaxSize().background(K.Saal)) {
        when {
            tv -> TvRahmen(k, bereich, titel, tabs, tab, { tab = it }, fehler) { inhalt(tab) }
            breit -> Row(Modifier.fillMaxSize().systemBarsPadding()) {
                Seitenleiste(k, bereich)
                Column(Modifier.weight(1f)) {
                    Kopfzeile(k, titel, tabs, tab, { tab = it }, menue = null)
                    Angebot(k.angebot)
                    Seiten(tabs.size, tab, { tab = it }, fehler, inhalt)
                }
            }
            else -> {
                val drawer = rememberDrawerState(DrawerValue.Closed)
                val scope = rememberCoroutineScope()
                BackHandler(drawer.isOpen) { scope.launch { drawer.close() } }
                ModalNavigationDrawer(
                    drawerContent = { Schublade(k, bereich) { b -> scope.launch { drawer.close() }; k.onBereich(b) } },
                    drawerState = drawer, scrimColor = K.Scrim,
                ) {
                    Column(Modifier.fillMaxSize().background(K.Saal).systemBarsPadding()) {
                        Kopfzeile(k, titel, tabs, tab, { tab = it }, menue = { scope.launch { drawer.open() } })
                        Angebot(k.angebot)
                        Seiten(tabs.size, tab, { tab = it }, fehler, inhalt)
                    }
                }
            }
        }
    }
}

/** Seiten zum Wischen (Handy/Tablet); der Tab oben und die Seite bleiben gekoppelt. */
@Composable
private fun ColumnScope.Seiten(n: Int, tab: Int, onTab: (Int) -> Unit, fehler: String, inhalt: @Composable (Int) -> Unit) {
    if (fehler.isNotEmpty()) T(fehler, LocalTypo.current.klein, K.AmpelRot, modifier = Modifier.padding(horizontal = Pad, vertical = 8.dp))
    if (n <= 1) { Box(Modifier.weight(1f)) { inhalt(0) }; return }
    val pager = rememberPagerState(tab) { n }
    LaunchedEffect(tab) { if (pager.currentPage != tab) pager.animateScrollToPage(tab) }
    LaunchedEffect(pager.settledPage) { onTab(pager.settledPage) } // erst nach dem Einrasten, sonst bricht ein Sprung über zwei Tabs ab
    HorizontalPager(pager, Modifier.weight(1f), key = { it }) { inhalt(it) }
}

@Composable
private fun Kopfzeile(k: Kopf, titel: String, tabs: List<String>, tab: Int, onTab: (Int) -> Unit, menue: (() -> Unit)?) {
    val breit = menue == null
    Column(Modifier.fillMaxWidth().background(K.Saal)) {
        Row(Modifier.fillMaxWidth().height(if (breit) 64.dp else 56.dp).padding(start = if (breit) 24.dp else 4.dp, end = if (breit) 20.dp else 4.dp),
            verticalAlignment = Alignment.CenterVertically) {
            if (menue != null) IconKnopf(Ic.MENUE, menue)
            T(titel, if (breit) 20.sp else 18.sp, K.Text, FontWeight.SemiBold, maxLines = 1, modifier = Modifier.padding(start = 4.dp).then(if (breit) Modifier else Modifier.weight(1f)))
            if (breit) {
                // Tablet: Tabs sitzen in der Kopfzeile
                Row(Modifier.padding(start = 24.dp).fillMaxHeight().weight(1f).horizontalScroll(rememberScrollState())) {
                    tabs.forEachIndexed { i, t -> Reiter(t, i == tab) { onTab(i) } }
                }
                IconKnopf(Ic.GEMEINSAM, k.onParty)
            }
            k.onCast?.let { IconKnopf(Ic.CAST, it, groesse = if (breit) 48.dp else 44.dp, farbe = if (k.castZiel != null) K.AmpelGruen else K.Text) }
            IconKnopf(Ic.SUCHE, { k.onBereich(Bereich.Suche) }, groesse = if (breit) 48.dp else 44.dp)
            if (!breit) AvatarMenue(k, 30.dp)
        }
        Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
        if (!breit && tabs.size > 1) {
            Row(Modifier.fillMaxWidth().height(48.dp).horizontalScroll(rememberScrollState()).padding(horizontal = 12.dp)) {
                tabs.forEachIndexed { i, t -> Reiter(t, i == tab) { onTab(i) } }
            }
            Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
        }
    }
}

/** Tab mit 2-px-Unterstrich; aktiv = Text und halbfett. */
@Composable
private fun Reiter(text: String, aktiv: Boolean, onClick: () -> Unit) {
    Box(Modifier.fillMaxHeight().klick(onClick, skala = false).padding(horizontal = 12.dp), contentAlignment = Alignment.Center) {
        T(text, 15.sp, if (aktiv) K.Text else K.Text2, if (aktiv) FontWeight.SemiBold else FontWeight.Medium, maxLines = 1)
        if (aktiv) Box(Modifier.align(Alignment.BottomCenter).fillMaxWidth().height(2.dp).background(K.Text))
    }
}

/** Avatar mit Benutzermenü (Profil wechseln, Einstellungen, Abmelden). */
@Composable
fun AvatarMenue(k: Kopf, groesse: Dp) {
    var offen by remember { mutableStateOf(false) }
    Box {
        Box(Modifier.size(48.dp).klick({ offen = true }), contentAlignment = Alignment.Center) {
            Avatar(k.me?.name ?: "?", k.me?.color ?: 0, groesse)
        }
        DropdownMenu(offen, { offen = false }, containerColor = K.Flaeche1, shape = RoundedCornerShape(Tokens.Radius.RadiusM),
            modifier = Modifier.border(1.dp, K.Linie, RoundedCornerShape(Tokens.Radius.RadiusM))) {
            MenueZeile(Ic.WECHSEL, "Profil wechseln") { offen = false; k.onProfil() }
            MenueZeile(Ic.EINSTELLUNGEN, "Einstellungen") { offen = false; k.onBereich(Bereich.Einstellungen) }
            MenueZeile(Ic.ABMELDEN, "Abmelden", gefahr = true) { offen = false; k.onAbmelden() }
        }
    }
}

/** Eintrag in Kontextmenü, Aktionen-Blatt und Dropdown: Icon 22, Text 15, Höhe 48. */
@Composable
fun MenueZeile(icon: String, text: String, gefahr: Boolean = false, trenner: Boolean = false, an: Boolean = false, onClick: () -> Unit) {
    val tv = LocalTv.current
    if (trenner) Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
    Row(Modifier.fillMaxWidth().defaultMinSize(minWidth = 240.dp).height(if (tv) 72.dp else 48.dp).klick(onClick, skala = false)
        .background(if (an) K.Flaeche3 else Color.Transparent).padding(horizontal = if (tv) 32.dp else 16.dp),
        verticalAlignment = Alignment.CenterVertically) {
        Ico(icon, if (tv) 32.dp else 22.dp, if (gefahr) K.AmpelRot else K.Text)
        T(text, if (tv) LocalTypo.current.text else 15.sp, if (gefahr) K.AmpelRot else K.Text, maxLines = 1,
            modifier = Modifier.padding(start = if (tv) 24.dp else 16.dp).weight(1f))
        if (an) Ico(Ic.HAKEN, if (tv) 32.dp else 22.dp, K.Text)
    }
}

/** Das Block-F-Zeichen (brand/block-f): heller Block, F ausgestanzt, untere Hälfte des F grau (Seitenleiste Tablet). */
@Composable
private fun Zeichen(size: Dp) {
    androidx.compose.foundation.Canvas(Modifier.size(size)) {
        val s = this.size.width / 640f
        val f = listOf(160 to 80, 480 to 80, 480 to 160, 240 to 160, 240 to 240, 400 to 240, 400 to 320, 240 to 320, 240 to 560, 160 to 560)
        val block = Path().apply {
            fillType = androidx.compose.ui.graphics.PathFillType.EvenOdd
            addRect(androidx.compose.ui.geometry.Rect(0f, 0f, 640f * s, 640f * s))
            f.forEachIndexed { i, (x, y) -> if (i == 0) moveTo(x * s, y * s) else lineTo(x * s, y * s) }
            close()
        }
        drawPath(block, K.Text)
        drawRect(K.LinieStark, androidx.compose.ui.geometry.Offset(160 * s, 320 * s), androidx.compose.ui.geometry.Size(80 * s, 240 * s))
    }
}

/** Die Flimmer-Wortmarke (brand/block-f): Pixelblöcke, „flim“ grau, „mer“ hell. Höhe = 7 Zellen, Breite = 34 Zellen. */
@Composable
fun Wortmarke(hoehe: Dp, modifier: Modifier = Modifier) {
    val glyphen = listOf(
        listOf("XXX.", "X...", "XXX.", "X...", "X...", "X...", "X..."),
        listOf("XX.", ".X.", ".X.", ".X.", ".X.", ".X.", ".XX"),
        listOf(".X.", "...", "XX.", ".X.", ".X.", ".X.", ".XX"),
        listOf(".....", ".....", "XXXXX", "X.X.X", "X.X.X", "X.X.X", "X.X.X"),
        listOf(".....", ".....", "XXXXX", "X.X.X", "X.X.X", "X.X.X", "X.X.X"),
        listOf("....", "....", "XXXX", "X..X", "XXXX", "X...", "XXXX"),
        listOf("....", "....", "X.XX", "XX..", "X...", "X...", "X..."),
    )
    val zellen = glyphen.sumOf { it[0].length } + glyphen.size - 1
    androidx.compose.foundation.Canvas(modifier.size(hoehe * zellen / 7f, hoehe)) {
        val u = this.size.height / 7f
        var x = 0f
        glyphen.forEachIndexed { i, g ->
            val farbe = if (i < 4) K.LinieStark else K.Text
            g.forEachIndexed { r, zeile ->
                var c = 0
                while (c < zeile.length) {
                    if (zeile[c] != 'X') { c++; continue }
                    var n = 1
                    while (c + n < zeile.length && zeile[c + n] == 'X') n++
                    // 0,5 px Überlappung, damit zwischen den Blöcken keine Haarlinien entstehen
                    drawRect(farbe, androidx.compose.ui.geometry.Offset(x + c * u - .5f, r * u - .5f),
                        androidx.compose.ui.geometry.Size(n * u + 1f, u + 1f))
                    c += n
                }
            }
            x += (g[0].length + 1) * u
        }
    }
}

/** Handy: Schublade 312 dp mit Benutzer, Hauptmenü und Serverstatus. */
@Composable
private fun Schublade(k: Kopf, aktiv: Bereich, onBereich: (Bereich) -> Unit) {
    Column(Modifier.width(312.dp).fillMaxHeight().background(K.Flaeche1).systemBarsPadding()) {
        Column(Modifier.padding(start = 16.dp, end = 16.dp, top = 12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Avatar(k.me?.name ?: "?", k.me?.color ?: 0, 48.dp)
                Column(Modifier.padding(start = 12.dp)) {
                    T(k.me?.name ?: "", 17.sp, K.Text, FontWeight.SemiBold, maxLines = 1)
                    T(listOfNotNull(k.server.substringAfter("://").substringBefore(':'), if (k.me?.admin == true) "Admin" else null).joinToString(" · "),
                        13.sp, K.Text2, maxLines = 1)
                }
            }
            Row(Modifier.padding(top = 4.dp).fillMaxWidth().height(44.dp).klick(k.onProfil, skala = false), verticalAlignment = Alignment.CenterVertically) {
                Ico(Ic.PROFIL, 20.dp, K.Text2)
                T("Profil wechseln", 14.sp, K.Text2, FontWeight.Medium, modifier = Modifier.padding(start = 10.dp))
            }
        }
        Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
        LazyColumn(Modifier.weight(1f)) {
            item { MenuePunkt(Bereich.Start, aktiv, onBereich) }
            item { MenuePunkt(Bereich.Favoriten, aktiv, onBereich) }
            item { MenuePunkt(Bereich.Downloads, aktiv, onBereich) }
            item { MenuePunkt(Ic.GEMEINSAM, "Gemeinsam schauen", false, k.onParty) }
            item { Kopfzeile2("MEDIEN") }
            items(k.medien) { MenuePunkt(it, aktiv, onBereich) }
            if (k.onDashboard != null) {
                item { Kopfzeile2("ADMINISTRATION") }
                item { MenuePunkt(Ic.DASHBOARD, "Dashboard", false, k.onDashboard) }
                k.onMetadaten?.let { m -> item { MenuePunkt(Ic.BEARBEITEN, "Metadaten-Manager", false, m) } }
            }
            item { Kopfzeile2("BENUTZER") }
            item { MenuePunkt(Bereich.Einstellungen, aktiv, onBereich) }
            item { MenuePunkt(Ic.SERVER, "Server auswählen", false, k.onServer) }
            item { MenuePunkt(Ic.ABMELDEN, "Abmelden", false, k.onAbmelden) }
        }
        Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
        Row(Modifier.height(44.dp).padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) {
            AmpelPunkt(if (k.verbunden) "green" else "red", 8.dp)
            T(k.server.substringAfter("://") + if (k.verbunden) " · verbunden" else " · nicht erreichbar", 13.sp, K.Text3, maxLines = 1,
                modifier = Modifier.padding(start = 12.dp))
        }
    }
}

@Composable
private fun Kopfzeile2(text: String) = T(text, 12.sp, K.Text3, family = Mono, spacing = 2.sp, modifier = Modifier.padding(start = 24.dp, top = 8.dp, bottom = 4.dp))

@Composable
private fun MenuePunkt(b: Bereich, aktiv: Bereich, onBereich: (Bereich) -> Unit) = MenuePunkt(b.icon, b.label, b == aktiv) { onBereich(b) }

@Composable
private fun MenuePunkt(icon: String, text: String, an: Boolean, onClick: () -> Unit) {
    Row(Modifier.padding(horizontal = 8.dp).fillMaxWidth().height(44.dp).klick(onClick, skala = false).background(if (an) K.Flaeche3 else Color.Transparent)
        .padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) {
        Ico(icon, 24.dp, if (an) K.Text else K.Text2)
        T(text, 15.sp, if (an) K.Text else K.Text2, if (an) FontWeight.SemiBold else FontWeight.Medium, maxLines = 1, modifier = Modifier.padding(start = 16.dp))
    }
}

/** Tablet: Seitenleiste 88 dp mit Zeichen, Icon + Beschriftung, unten Einstellungen und Avatar. */
@Composable
private fun Seitenleiste(k: Kopf, aktiv: Bereich) {
    Row {
        Column(Modifier.width(88.dp).fillMaxHeight().background(K.Flaeche1), horizontalAlignment = Alignment.CenterHorizontally) {
            Box(Modifier.fillMaxWidth().height(64.dp).clickable { k.onBereich(Bereich.Start) }, contentAlignment = Alignment.Center) { Zeichen(28.dp) }
            Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
            Column(Modifier.weight(1f).padding(vertical = 12.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(4.dp)) {
                (listOf(Bereich.Start, Bereich.Favoriten) + k.medien + Bereich.Downloads + Bereich.Suche).forEach { b ->
                    val an = b == aktiv
                    Column(Modifier.size(72.dp, 60.dp).klick({ k.onBereich(b) }, skala = false).background(if (an) K.Flaeche3 else Color.Transparent),
                        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
                        Ico(b.icon, 24.dp, if (an) K.Text else K.Text2)
                        T(if (b == Bereich.Start) "Start" else if (b == Bereich.Listen) "Listen" else b.label, 12.sp, if (an) K.Text else K.Text2,
                            if (an) FontWeight.SemiBold else FontWeight.Medium, maxLines = 1, modifier = Modifier.padding(top = 4.dp))
                    }
                }
            }
            Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
            Column(Modifier.padding(top = 12.dp, bottom = 16.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                k.onDashboard?.let { IconKnopf(Ic.DASHBOARD, it, farbe = K.Text2) }
                IconKnopf(Ic.EINSTELLUNGEN, { k.onBereich(Bereich.Einstellungen) }, farbe = if (aktiv == Bereich.Einstellungen) K.Text else K.Text2)
                AvatarMenue(k, 34.dp)
            }
        }
        Box(Modifier.width(1.dp).fillMaxHeight().background(K.Linie))
    }
}

/** TV: Schiene links nur mit Icons (72 px); hat sie den Fokus, klappt sie mit Beschriftung über den Inhalt auf. */
@Composable
private fun TvRahmen(k: Kopf, bereich: Bereich, titel: String, tabs: List<String>, tab: Int, onTab: (Int) -> Unit, fehler: String, inhalt: @Composable () -> Unit) {
    var offen by remember { mutableStateOf(false) }
    val breite by animateDpAsState(if (offen) 320.dp else 72.dp, label = "schiene")
    Box(Modifier.fillMaxSize()) {
        Column(Modifier.fillMaxSize().padding(start = 72.dp)) {
            if (tabs.size > 1 || titel.isNotEmpty()) Row(Modifier.padding(start = Pad, end = 96.dp, top = Tokens.Abstand.TvRandOben).height(72.dp),
                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                T(titel, LocalTypo.current.titel, K.Text, FontWeight.SemiBold, maxLines = 1, modifier = Modifier.padding(end = 16.dp))
                if (tabs.size > 1) tabs.forEachIndexed { i, t -> Chip(t, i == tab, { onTab(i) }) }
            }
            if (fehler.isNotEmpty()) T(fehler, LocalTypo.current.klein, K.AmpelRot, modifier = Modifier.padding(horizontal = Pad))
            Angebot(k.angebot)
            Box(Modifier.weight(1f)) { inhalt() }
        }
        Column(
            Modifier.zIndex(1f).width(breite).fillMaxHeight()
                .background(if (offen) Brush.horizontalGradient(listOf(K.Flaeche1, K.Flaeche1, K.Flaeche1.copy(alpha = 0.96f))) else Brush.horizontalGradient(listOf(K.Saal, K.Saal)))
                .onFocusChanged { offen = it.hasFocus }
                .padding(top = Tokens.Abstand.TvRandOben, bottom = 32.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Schienenpunkt(null, k.me?.name ?: "", offen, false, { k.onProfil() }) { Avatar(k.me?.name ?: "?", k.me?.color ?: 0, 40.dp) }
            Spacer(Modifier.height(16.dp))
            (listOf(Bereich.Suche, Bereich.Start, Bereich.Favoriten) + k.medien).forEach { b ->
                Schienenpunkt(b.icon, b.label, offen, b == bereich, { k.onBereich(b) })
            }
            Schienenpunkt(Ic.GEMEINSAM, "Gemeinsam schauen", offen, false, k.onParty)
            Spacer(Modifier.weight(1f))
            k.onDashboard?.let { Schienenpunkt(Ic.DASHBOARD, "Dashboard", offen, false, it) }
            Schienenpunkt(Bereich.Einstellungen.icon, Bereich.Einstellungen.label, offen, bereich == Bereich.Einstellungen, { k.onBereich(Bereich.Einstellungen) })
        }
    }
}

@Composable
private fun Schienenpunkt(icon: String?, text: String, offen: Boolean, an: Boolean, onClick: () -> Unit, bild: @Composable () -> Unit = {}) {
    var f by remember { mutableStateOf(false) }
    Row(Modifier.padding(horizontal = 8.dp).fillMaxWidth().height(64.dp).onFocusChanged { f = it.isFocused }.klick(onClick, skala = false)
        .background(if (f) K.Marke else if (an) K.Flaeche3 else Color.Transparent).padding(start = 8.dp),
        verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(40.dp), contentAlignment = Alignment.Center) {
            if (icon != null) Ico(icon, 32.dp, if (f) K.AufMarke else if (an) K.Text else K.Text2) else bild()
        }
        if (offen) T(text, LocalTypo.current.text, if (f) K.AufMarke else if (an) K.Text else K.Text2, if (an) FontWeight.SemiBold else FontWeight.Medium,
            maxLines = 1, modifier = Modifier.padding(start = 20.dp))
    }
}

/** Kontextmenü: Handy/Tablet als Blatt von unten, TV als Dialog in der Mitte. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun Blatt(onZu: () -> Unit, kopf: @Composable () -> Unit, inhalt: @Composable ColumnScope.() -> Unit) {
    if (LocalTv.current) {
        Dialog(onZu) {
            Column(Modifier.width(640.dp).background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusM)).border(1.dp, K.Linie, RoundedCornerShape(Tokens.Radius.RadiusM))
                .padding(vertical = 16.dp)) {
                kopf()
                Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
                inhalt()
            }
        }
    } else {
        ModalBottomSheet(onZu, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = K.Flaeche1, scrimColor = K.Scrim,
            shape = RoundedCornerShape(topStart = Tokens.Radius.RadiusM, topEnd = Tokens.Radius.RadiusM),
            dragHandle = { Box(Modifier.height(24.dp), contentAlignment = Alignment.Center) { Box(Modifier.size(32.dp, 4.dp).clip(RoundedCornerShape(2.dp)).background(K.LinieStark)) } }) {
            kopf()
            Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
            Column(Modifier.navigationBarsPadding().padding(bottom = 8.dp)) { inhalt() }
        }
    }
}

/** Kopf eines Blatts: kleines Poster, Titel, Fakten. */
@Composable
fun BlattKopf(titel: String, fakten: String, bild: String, farbe: String) {
    Row(Modifier.padding(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 16.dp), verticalAlignment = Alignment.CenterVertically) {
        Art(bild, titel, Modifier.size(48.dp, 72.dp).clip(RoundedCornerShape(Tokens.Radius.RadiusS)), ton(farbe))
        Column(Modifier.padding(start = 14.dp)) {
            T(titel, if (LocalTv.current) LocalTypo.current.reihe else 18.sp, K.Text, FontWeight.SemiBold, maxLines = 2)
            if (fakten.isNotEmpty()) T(fakten, LocalTypo.current.klein, K.Text2, maxLines = 1)
        }
    }
}

/** Gerätewahl „Wiedergabe auf anderem Gerät“: dieses Gerät oder ein Chromecast im Heimnetz. */
@Composable
fun CastWahl(geraete: List<String>?, aktiv: String?, onWahl: (String?) -> Unit, onZu: () -> Unit) {
    Blatt(onZu, { T("Wiedergabe auf anderem Gerät", 18.sp, K.Text, FontWeight.SemiBold, modifier = Modifier.padding(start = 16.dp, bottom = 12.dp)) }) {
        MenueZeile(Ic.HANDY, "Dieses Gerät", an = aktiv == null) { onWahl(null) }
        when {
            geraete == null -> T("Chromecast ist auf diesem Gerät nicht verfügbar.", 14.sp, K.Text2, modifier = Modifier.padding(16.dp))
            geraete.isEmpty() -> T("Suche Geräte im Heimnetz …", 14.sp, K.Text2, modifier = Modifier.padding(16.dp))
            else -> geraete.forEach { g -> MenueZeile(Ic.CAST, g, an = g == aktiv) { onWahl(g) } }
        }
    }
}

/** Kleiner Dialog mit einem Eingabefeld (Raumcode, Schnellverbindung). */
@Composable
fun EingabeDialog(titel: String, hinweis: String, platzhalter: String, knopf: String, fehler: String, onOk: (String) -> Unit, onZu: () -> Unit, start: String = "") {
    var text by remember { mutableStateOf(start) }
    Dialog(onZu) {
        Column(Modifier.width(if (LocalTv.current) 720.dp else 340.dp).background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusM))
            .border(1.dp, K.Linie, RoundedCornerShape(Tokens.Radius.RadiusM)).padding(20.dp)) {
            T(titel, LocalTypo.current.reihe, K.Text, FontWeight.SemiBold)
            T(hinweis, LocalTypo.current.klein, K.Text2, modifier = Modifier.padding(top = 4.dp, bottom = 12.dp))
            Feld(text, { text = it }, platzhalter, Modifier.fillMaxWidth(), onGo = { if (text.isNotBlank()) onOk(text) })
            if (fehler.isNotEmpty()) T(fehler, LocalTypo.current.klein, K.AmpelRot, modifier = Modifier.padding(top = 8.dp))
            Row(Modifier.padding(top = 16.dp).align(Alignment.End), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Knopf("Abbrechen", onZu)
                Knopf(knopf, { if (text.isNotBlank()) onOk(text) }, primary = true)
            }
        }
    }
}

/** Kopf für Unterseiten (Liste, Person): Zurück und Titel; TV nur Titel. */
@Composable
fun UnterSeite(titel: String, zurueck: () -> Unit, inhalt: @Composable () -> Unit) {
    val tv = LocalTv.current
    Column(Modifier.fillMaxSize().background(K.Saal).systemBarsPadding()) {
        Row(Modifier.fillMaxWidth().height(if (tv) 72.dp else 56.dp).padding(start = if (tv) 96.dp else 4.dp, end = 16.dp)
            .then(if (tv) Modifier.padding(top = 8.dp) else Modifier), verticalAlignment = Alignment.CenterVertically) {
            if (!tv) IconKnopf(Ic.ZURUECK, zurueck)
            T(titel, if (tv) LocalTypo.current.titel else 18.sp, K.Text, FontWeight.SemiBold, maxLines = 1, modifier = Modifier.padding(start = 4.dp))
        }
        if (!tv) Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
        Box(Modifier.weight(1f).then(if (tv) Modifier.padding(horizontal = 48.dp) else Modifier)) { inhalt() }
    }
}

/** Rückfrage vor Gefährlichem (Löschen). */
@Composable
fun FrageDialog(titel: String, text: String, ja: String, onJa: () -> Unit, onZu: () -> Unit) {
    Dialog(onZu) {
        Column(Modifier.width(if (LocalTv.current) 720.dp else 340.dp).background(K.Flaeche1, RoundedCornerShape(Tokens.Radius.RadiusM))
            .border(1.dp, K.Linie, RoundedCornerShape(Tokens.Radius.RadiusM)).padding(20.dp)) {
            T(titel, LocalTypo.current.reihe, K.Text, FontWeight.SemiBold)
            T(text, LocalTypo.current.klein, K.Text2, modifier = Modifier.padding(top = 4.dp))
            Row(Modifier.padding(top = 16.dp).align(Alignment.End), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Knopf("Abbrechen", onZu)
                Knopf(ja, onJa, primary = true)
            }
        }
    }
}
