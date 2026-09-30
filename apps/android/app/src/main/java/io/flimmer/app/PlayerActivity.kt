package io.flimmer.app

import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.view.Gravity
import android.view.KeyEvent
import android.view.SurfaceView
import android.view.ViewGroup
import android.view.WindowManager
import android.widget.FrameLayout
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.addCallback
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.focusable
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.gestures.calculateZoom
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.ComposeView
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.lifecycleScope
import androidx.compose.material3.CircularProgressIndicator
import androidx.media3.common.C
import androidx.media3.common.Format
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.PlaybackParameters
import androidx.media3.common.Player
import androidx.media3.common.TrackSelectionOverride
import androidx.media3.common.Tracks
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.datasource.DefaultDataSource
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.ui.SubtitleView
import androidx.media3.common.VideoSize
import androidx.media3.common.text.CueGroup
import kotlin.math.roundToInt
import io.flimmer.app.ui.*
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File

/**
 * Nativer Player (Media3/ExoPlayer). ExoPlayer kann MKV/HEVC und mit dem ffmpeg-Decoder auch DTS/TrueHD,
 * deshalb plant der Server fast immer Direct Play; eingebettete Ton- und Untertitelspuren (SRT/ASS/PGS)
 * wählt man direkt im Player. HLS kommt nur als Fallback.
 * Bedienung in Compose wie im Entwurf (Handy-Player, TV-Player): oben Titel, Mitte Springen/Pause,
 * unten Zeit, Leiste und Ton/Untertitel/Qualität; Auswahl in einer Leiste rechts.
 * Mit Extra „party“ läuft die Wiedergabe synchron mit einem Raum („Gemeinsam schauen“),
 * mit Extra „file“ aus einem Download (ohne Server; Fortschritt wird nachgereicht),
 * mit „queue“ folgen danach weitere Titel (nächste Folgen, Wiedergabeliste).
 */
class PlayerActivity : ComponentActivity() {
    private var player: ExoPlayer? = null
    private lateinit var api: ApiClient
    private lateinit var id: String
    private lateinit var store: Store
    private lateinit var downloads: Downloads
    private lateinit var rahmen: FrameLayout
    private lateinit var flaeche: SurfaceView // Video-Fläche; Größe und Lage setzt anpassen()
    private var videoB = 0.0 // Video in Anzeige-Pixeln (Breite mit Pixel-Seitenverhältnis)
    private var videoH = 0.0
    private var crop: Crop? = null
    private var cropJob: kotlinx.coroutines.Job? = null
    private var file: String? = null
    private var live: String? = null // Live-TV: fertige HLS-URL, ohne Zeitleiste und Fortschritt
    private val kapitel = mutableStateOf<List<Chapter>>(emptyList())
    private var nachtFx: android.media.audiofx.DynamicsProcessing? = null
    private val nacht = mutableStateOf(false)
    private val tempo = mutableFloatStateOf(1f)
    private val weiterAbgebrochen = mutableStateOf(false) // Nächste-Folge-Karte weggeklickt: am Ende nicht weiter
    private var sprungZiel: Long? = null // „Vorspann überspringen“ gerade möglich (für OK auf dem TV)
    private val queue = ArrayDeque<Triple<String, String, String>>() // id, Titel, Zeile
    private var tonNr = -2 // Wunsch von der Detailseite: Position unter den Tonspuren, -2 = keiner
    private var utNr = -2 // dito Untertitel, -1 = aus

    // Zustand der Bedienung
    private val titel = mutableStateOf("")
    private val zeile = mutableStateOf("")
    private val sichtbar = mutableStateOf(true)
    private val panel = mutableStateOf<String?>(null) // ton | ut | qualitaet
    private val spielt = mutableStateOf(false)
    private val tracks = mutableStateOf(Tracks.EMPTY)
    private val ampel = mutableStateOf("green")
    private val aktion = mutableLongStateOf(0L) // letzte Bedienung, für das Ausblenden
    private val qualitaet = mutableIntStateOf(0)
    private val quelleHoehe = mutableIntStateOf(0) // Bildhöhe der Quelle, 0 = unbekannt
    private val laeuftHoehe = mutableIntStateOf(0) // Bildhöhe, die gerade wirklich dekodiert wird
    private val laedt = mutableStateOf(false) // puffert (z. B. Server wandelt nach einem Qualitätswechsel erst um)
    private val modus = mutableStateOf(BildModus.Auto)
    private val hinweis = mutableStateOf<String?>(null) // kurzer Hinweis in der Mitte (Bildmodus nach Pinch)

    // Gemeinsam schauen
    private val party = mutableStateOf<String?>(null)
    private var member = ""
    private var offset = 0L
    private var roomState: PartyState? = null
    private var applying = false // eigene Korrekturen nicht als Nutzeraktion zurückmelden
    private val messages = mutableStateListOf<String>()
    private val members = mutableStateOf<List<String>>(emptyList())

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        api = ApiClient(intent.getStringExtra("server") ?: return finish(), intent.getStringExtra("token") ?: "")
        live = intent.getStringExtra("live_url")
        id = intent.getStringExtra("id") ?: if (live != null) "" else return finish()
        party.value = intent.getStringExtra("party")
        titel.value = intent.getStringExtra("titel") ?: intent.getStringExtra("live_titel") ?: ""
        zeile.value = intent.getStringExtra("zeile") ?: if (live != null) "Live" else ""
        val ids = intent.getStringArrayExtra("queue").orEmpty()
        val qt = intent.getStringArrayExtra("queueTitel").orEmpty()
        val qz = intent.getStringArrayExtra("queueZeile").orEmpty()
        ids.forEachIndexed { i, q -> queue.add(Triple(q, qt.getOrElse(i) { "" }, qz.getOrElse(i) { "" })) }
        tonNr = intent.getIntExtra("tonNr", -2)
        utNr = intent.getIntExtra("utNr", -2)
        val start = intent.getDoubleExtra("start", -1.0)
        file = intent.getStringExtra("file")
        store = Store(this)
        downloads = Downloads(this)
        qualitaet.intValue = store.quality
        modus.value = store.bildModus
        nacht.value = store.night
        val tv = isTv(this)
        // Zurück: erst die Auswahl schließen, auf dem TV dann die Bedienung ausblenden, sonst den Player verlassen.
        onBackPressedDispatcher.addCallback(this) {
            when {
                panel.value != null -> panel.value = null
                tv && sichtbar.value -> sichtbar.value = false
                else -> finish()
            }
        }

        // Plattform-Decoder zuerst, ffmpeg nur für das, was das Gerät nicht kann (DTS, TrueHD …).
        val renderers = DefaultRenderersFactory(this).setExtensionRendererMode(DefaultRenderersFactory.EXTENSION_RENDERER_MODE_ON)
        // Anmeldung auch als Header (Live-TV-Streams); die Pfade von /play tragen ihr Token ohnehin
        // Lesezeit 30 s statt 8: ein langsamer Server (NAS) braucht fürs erste umgewandelte Segment länger, sonst bricht ExoPlayer ab und fragt neu an
        val http = DefaultHttpDataSource.Factory().setReadTimeoutMs(30_000).setDefaultRequestProperties(mapOf("Authorization" to "Bearer ${api.token}"))
        val p = ExoPlayer.Builder(this, renderers).setSeekBackIncrementMs(10_000).setSeekForwardIncrementMs(30_000)
            .setMediaSourceFactory(DefaultMediaSourceFactory(DefaultDataSource.Factory(this, http))).build()
        player = p
        // SurfaceView statt PlayerView: Die Bildanpassung (Crop, Füllen, Strecken) setzt Größe und Lage der Fläche selbst,
        // Untertitel liegen darüber am Bildschirm ausgerichtet. SurfaceView behält HDR und schont schwache TV-Sticks.
        flaeche = SurfaceView(this)
        p.setVideoSurfaceView(flaeche)
        val untertitel = SubtitleView(this).apply { setUserDefaultStyle(); setUserDefaultTextSize() }
        rahmen = FrameLayout(this).apply {
            setBackgroundColor(android.graphics.Color.BLACK)
            addView(flaeche, FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT))
            addView(untertitel, ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT)
            addView(ComposeView(this@PlayerActivity).apply {
                setContent { FlimmerTheme(tv) { Bedienung() } }
            }, ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT)
            // Drehung und Größenänderung (Fenster, Split-Screen): neu rechnen
            addOnLayoutChangeListener { _, l, t, r, b, ol, ot, or, ob -> if (r - l != or - ol || b - t != ob - ot) anpassen() }
        }
        setContentView(rahmen)
        p.addListener(object : Player.Listener {
            override fun onPlaybackStateChanged(state: Int) {
                laedt.value = state == Player.STATE_BUFFERING
                if (state == Player.STATE_ENDED && party.value == null) naechster()
                if (party.value != null) when (state) {
                    Player.STATE_BUFFERING -> send("buffering", buffering = true)
                    Player.STATE_READY -> send("buffering", buffering = false)
                }
            }

            override fun onIsPlayingChanged(isPlaying: Boolean) { spielt.value = isPlaying }

            override fun onVideoSizeChanged(v: VideoSize) {
                if (v.width == 0 || v.height == 0) return
                videoB = v.width * v.pixelWidthHeightRatio.toDouble()
                videoH = v.height.toDouble()
                laeuftHoehe.intValue = v.height
                anpassen()
            }

            override fun onCues(cueGroup: CueGroup) { untertitel.setCues(cueGroup.cues) }

            override fun onTracksChanged(t: Tracks) {
                tracks.value = t
                wunschSpur(t)
            }

            override fun onPlayWhenReadyChanged(playWhenReady: Boolean, reason: Int) {
                if (reason == Player.PLAY_WHEN_READY_CHANGE_REASON_USER_REQUEST && !applying) send(if (playWhenReady) "play" else "pause")
            }

            override fun onPositionDiscontinuity(old: Player.PositionInfo, new: Player.PositionInfo, reason: Int) {
                if (reason == Player.DISCONTINUITY_REASON_SEEK && !applying) send("seek")
            }

            override fun onPlayerError(e: PlaybackException) {
                Toast.makeText(this@PlayerActivity, "Wiedergabefehler: ${e.errorCodeName}", Toast.LENGTH_LONG).show()
            }
        })

        nachtAnwenden()
        if (live == null) detailsLaden()
        lifecycleScope.launch {
            val f = file
            val l = live
            if (l != null) {
                val url = api.abs(l)
                p.setMediaItem(MediaItem.Builder().setUri(url).apply { if (url.contains(".m3u8")) setMimeType(MimeTypes.APPLICATION_M3U8) }.build())
                p.prepare()
                p.play()
                return@launch // live: kein Fortschritt
            } else if (f != null) {
                p.setMediaItem(MediaItem.fromUri(Uri.fromFile(File(f))), (maxOf(start, 0.0) * 1000).toLong())
                p.prepare()
                p.play()
            } else {
                if (!load(start.takeIf { it >= 0 })) return@launch finish()
                val room = party.value
                if (room == null) p.play() else joinParty(room)
            }
            while (true) { // Fortschritt alle 10 s, auch pausiert (sonst verschwindet die Sitzung nach 60 s), aber nicht im Hintergrund
                delay(10_000)
                if (lifecycle.currentState.isAtLeast(androidx.lifecycle.Lifecycle.State.STARTED) && p.playbackState != Player.STATE_IDLE) report()
            }
        }
    }

    /** Plan holen und Quelle setzen – beim Start, nach einer anderen Qualität (dann an derselben Stelle) und für den nächsten Titel. */
    private suspend fun load(startSec: Double?): Boolean {
        val p = player ?: return false
        // Nachtmodus macht ab Android 9 der Player selbst (DynamicsProcessing), davor wandelt der Server den Ton um.
        val profil = deviceProfile(this).copy(maxHeight = qualitaet.intValue, audioLangs = store.audioLangs, subtitleMode = store.subtitleMode,
            night = nacht.value && Build.VERSION.SDK_INT < Build.VERSION_CODES.P)
        val plan = runCatching { api.play(id, profil) }.getOrElse {
            Toast.makeText(this, "Server nicht erreichbar", Toast.LENGTH_LONG).show()
            return false
        }
        ampel.value = plan.light
        if (titel.value.isEmpty()) titel.value = plan.title
        val url = api.abs(plan.url)
        val item = MediaItem.Builder().setUri(url)
        if (url.contains(".m3u8")) item.setMimeType(MimeTypes.APPLICATION_M3U8)
        // Bei Direct Play stecken alle Untertitel in der Datei; nachgeladen werden sie nur für HLS.
        if (plan.method != "direct-play") {
            item.setSubtitleConfigurations(plan.subtitles.orEmpty().map { s ->
                MediaItem.SubtitleConfiguration.Builder(Uri.parse(api.abs(s.url)))
                    .setMimeType(if (s.format == "pgs") MimeTypes.APPLICATION_PGS else MimeTypes.TEXT_VTT)
                    .setLanguage(s.language).setLabel(s.title).build()
            })
        }
        // Gemerkte Sprachen der Serie vorwählen.
        p.trackSelectionParameters = p.trackSelectionParameters.buildUpon().clearOverrides().apply {
            if (plan.prefs.audio.isNotEmpty()) setPreferredAudioLanguage(plan.prefs.audio)
            if (plan.prefs.subtitle.isNotEmpty() && plan.prefs.subtitle != "off") setPreferredTextLanguage(plan.prefs.subtitle)
        }.build()
        p.setMediaItem(item.build(), ((startSec ?: plan.resume) * 1000).toLong())
        p.prepare()
        return true
    }

    /** Ende erreicht: nächster Titel aus der Warteschlange (wenn gewünscht), sonst zurück. */
    private fun naechster() {
        val n = queue.removeFirstOrNull()
        if (n == null || !store.autoNext || file != null || weiterAbgebrochen.value) return finish()
        lifecycleScope.launch {
            report()
            id = n.first
            titel.value = n.second
            zeile.value = n.third
            tonNr = -2; utNr = -2
            weiterAbgebrochen.value = false
            detailsLaden()
            if (!load(0.0)) finish() else player?.play()
        }
    }

    // ---------- Bildanpassung ----------

    /** Größe und Lage der Video-Fläche nach bildFlaeche(); was übersteht, schneidet der Bildschirm ab. */
    private fun anpassen() {
        val w = rahmen.width.toDouble()
        val h = rahmen.height.toDouble()
        if (w <= 0 || h <= 0 || videoB <= 0 || videoH <= 0) return
        val f = bildFlaeche(modus.value, videoB, videoH, crop, w, h)
        val lp = FrameLayout.LayoutParams(f.breite.roundToInt(), f.hoehe.roundToInt(), Gravity.TOP or Gravity.START).apply {
            leftMargin = ((w - f.breite) / 2 + f.dx).roundToInt()
            topMargin = ((h - f.hoehe) / 2 + f.dy).roundToInt()
        }
        val alt = flaeche.layoutParams as? FrameLayout.LayoutParams
        if (alt == null || alt.width != lp.width || alt.height != lp.height || alt.leftMargin != lp.leftMargin || alt.topMargin != lp.topMargin) flaeche.layoutParams = lp
    }

    /**
     * Details vom Server: Kapitel für Zeitleiste und „Vorspann überspringen“, crop für die Bildanpassung.
     * Die crop-Erkennung läuft beim ersten Abruf im Hintergrund, also später noch zweimal nachfragen.
     */
    private fun detailsLaden() {
        cropJob?.cancel()
        crop = null
        kapitel.value = emptyList()
        quelleHoehe.intValue = 0
        anpassen()
        val itemId = id
        cropJob = lifecycleScope.launch {
            for (warten in listOf(0L, 15_000L, 60_000L)) {
                delay(warten)
                val det = runCatching { optional { api.details(itemId, store.deviceId) } }.getOrNull() ?: continue
                if (kapitel.value.isEmpty()) kapitel.value = det.chapters.sortedBy { it.start }
                if (quelleHoehe.intValue == 0) quelleHoehe.intValue = det.video?.height ?: 0
                det.video?.crop?.let { crop = it; anpassen(); return@launch }
            }
        }
    }

    // ---------- Nachtmodus ----------

    /** Kompressor mit Anhebung und Begrenzer auf der Audio-Session (ab Android 9); bei Durchreichen an den AV-Receiver wirkungslos. */
    private fun nachtAnwenden() {
        nachtFx?.release()
        nachtFx = null
        val p = player ?: return
        if (!nacht.value || Build.VERSION.SDK_INT < Build.VERSION_CODES.P) return
        runCatching {
            val kanaele = 8
            val cfg = android.media.audiofx.DynamicsProcessing.Config.Builder(
                android.media.audiofx.DynamicsProcessing.VARIANT_FAVOR_FREQUENCY_RESOLUTION, kanaele, false, 0, true, 1, false, 0, true).build()
            val fx = android.media.audiofx.DynamicsProcessing(0, p.audioSessionId, cfg)
            for (k in 0 until kanaele) {
                val band = fx.getMbcBandByChannelIndex(k, 0)
                band.threshold = -30f; band.ratio = 4f; band.attackTime = 5f; band.releaseTime = 250f; band.postGain = 10f
                fx.setMbcBandByChannelIndex(k, 0, band)
                fx.setLimiterByChannelIndex(k, android.media.audiofx.DynamicsProcessing.Limiter(true, true, 0, 1f, 60f, 10f, -2f, 0f))
            }
            fx.enabled = true
            nachtFx = fx
        }
    }

    private fun nachtUmschalten() {
        nacht.value = !nacht.value
        store.night = nacht.value
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) nachtAnwenden()
        else lifecycleScope.launch { load((player?.currentPosition ?: 0) / 1000.0) } // Server wandelt den Ton um
    }

    private fun bildModus(m: BildModus, zeigen: Boolean) {
        modus.value = m
        store.bildModus = m
        anpassen()
        if (zeigen) hinweis.value = m.label
    }

    // ---------- Spuren ----------

    private fun gruppen(t: Tracks, typ: Int) = t.groups.filter { it.type == typ && it.isSupported }

    /** Wunsch von der Detailseite einmal anwenden, sobald die Spuren bekannt sind. */
    private fun wunschSpur(t: Tracks) {
        val p = player ?: return
        if (tonNr >= 0) gruppen(t, C.TRACK_TYPE_AUDIO).getOrNull(tonNr)?.let { g ->
            p.trackSelectionParameters = p.trackSelectionParameters.buildUpon().setOverrideForType(TrackSelectionOverride(g.mediaTrackGroup, 0)).build()
            tonNr = -2
        }
        if (utNr == -1) { p.trackSelectionParameters = p.trackSelectionParameters.buildUpon().setTrackTypeDisabled(C.TRACK_TYPE_TEXT, true).build(); utNr = -2 }
        if (utNr >= 0) gruppen(t, C.TRACK_TYPE_TEXT).getOrNull(utNr)?.let { g ->
            p.trackSelectionParameters = p.trackSelectionParameters.buildUpon().setTrackTypeDisabled(C.TRACK_TYPE_TEXT, false)
                .setOverrideForType(TrackSelectionOverride(g.mediaTrackGroup, 0)).build()
            utNr = -2
        }
    }

    private fun spurName(f: Format): String {
        val codec = when (f.sampleMimeType) {
            MimeTypes.AUDIO_AAC -> "AAC"; MimeTypes.AUDIO_AC3 -> "AC-3"; MimeTypes.AUDIO_E_AC3, MimeTypes.AUDIO_E_AC3_JOC -> "E-AC-3"
            MimeTypes.AUDIO_DTS, MimeTypes.AUDIO_DTS_HD -> "DTS"; MimeTypes.AUDIO_TRUEHD -> "TrueHD"; MimeTypes.AUDIO_OPUS -> "Opus"
            MimeTypes.AUDIO_FLAC -> "FLAC"; MimeTypes.AUDIO_MPEG -> "MP3"; MimeTypes.APPLICATION_SUBRIP -> "SRT"; MimeTypes.TEXT_SSA -> "ASS"
            MimeTypes.APPLICATION_PGS -> "PGS"; MimeTypes.TEXT_VTT -> "VTT"; else -> null
        }
        val kanaele = when {
            f.channelCount >= 6 -> "${f.channelCount - 1}.1"
            f.channelCount == 2 -> "Stereo"
            f.channelCount == 1 -> "Mono"
            else -> null
        }
        val name = f.label ?: f.language?.let(::sprache) ?: "Spur"
        return name + listOfNotNull(kanaele, codec).joinToString(" · ").let { if (it.isEmpty()) "" else " – $it" }
    }

    private fun waehle(typ: Int, g: Tracks.Group?) {
        val p = player ?: return
        p.trackSelectionParameters = p.trackSelectionParameters.buildUpon().apply {
            if (g == null) setTrackTypeDisabled(typ, true)
            else setTrackTypeDisabled(typ, false).setOverrideForType(TrackSelectionOverride(g.mediaTrackGroup, 0))
        }.build()
    }

    private fun qualitaetWaehlen(q: Int) {
        if (q == qualitaet.intValue) return
        qualitaet.intValue = q
        store.quality = q
        val at = (player?.currentPosition ?: 0) / 1000.0
        lifecycleScope.launch { load(at) } // playWhenReady bleibt, es läuft an derselben Stelle weiter
    }

    // ---------- Fernbedienung ----------

    private val steuerTasten = setOf(
        KeyEvent.KEYCODE_DPAD_UP, KeyEvent.KEYCODE_DPAD_DOWN, KeyEvent.KEYCODE_DPAD_LEFT, KeyEvent.KEYCODE_DPAD_RIGHT, KeyEvent.KEYCODE_DPAD_CENTER,
        KeyEvent.KEYCODE_ENTER, KeyEvent.KEYCODE_MEDIA_REWIND, KeyEvent.KEYCODE_MEDIA_FAST_FORWARD,
    )

    /** TV: bei ausgeblendeter Bedienung springen ←/→ um 10 s zurück bzw. 30 s vor, die anderen Steuertasten blenden ein. */
    override fun dispatchKeyEvent(event: KeyEvent): Boolean {
        val p = player ?: return super.dispatchKeyEvent(event)
        if (event.keyCode == KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE && event.action == KeyEvent.ACTION_DOWN) { umschalten(); return true }
        if (event.keyCode !in steuerTasten) return super.dispatchKeyEvent(event) // Zurück, Lautstärke … wie gewohnt
        aktion.longValue = System.currentTimeMillis()
        if (!sichtbar.value) {
            val ziel = sprungZiel
            if (ziel != null && event.action == KeyEvent.ACTION_DOWN && (event.keyCode == KeyEvent.KEYCODE_DPAD_CENTER || event.keyCode == KeyEvent.KEYCODE_ENTER)) {
                p.seekTo(ziel); return true
            }
            if (live != null && event.keyCode in setOf(KeyEvent.KEYCODE_DPAD_LEFT, KeyEvent.KEYCODE_DPAD_RIGHT)) { sichtbar.value = true; return true }
            if (event.action == KeyEvent.ACTION_DOWN) when (event.keyCode) {
                KeyEvent.KEYCODE_DPAD_LEFT, KeyEvent.KEYCODE_MEDIA_REWIND -> p.seekBack()
                KeyEvent.KEYCODE_DPAD_RIGHT, KeyEvent.KEYCODE_MEDIA_FAST_FORWARD -> p.seekForward()
                else -> sichtbar.value = true
            }
            return true
        }
        return super.dispatchKeyEvent(event)
    }

    private fun umschalten() {
        val p = player ?: return
        if (p.isPlaying) p.pause() else p.play()
    }

    // ---------- Bedienung (Compose) ----------

    @Composable
    private fun Bedienung() {
        val tv = LocalTv.current
        val p = player ?: return
        var pos by remember { mutableLongStateOf(0L) }
        var dauer by remember { mutableLongStateOf(0L) }
        var puffer by remember { mutableLongStateOf(0L) }
        LaunchedEffect(Unit) {
            while (true) {
                pos = p.currentPosition; puffer = p.bufferedPosition
                dauer = if (p.duration == C.TIME_UNSET) 0 else p.duration
                delay(250)
            }
        }
        // Ausblenden nach 4 s ohne Bedienung, solange es läuft und kein Menü offen ist.
        LaunchedEffect(sichtbar.value, aktion.longValue, spielt.value, panel.value) {
            if (sichtbar.value && spielt.value && panel.value == null) { delay(4000); sichtbar.value = false }
        }
        val pause = remember { FocusRequester() }
        LaunchedEffect(sichtbar.value) { if (sichtbar.value && tv) { delay(50); runCatching { pause.requestFocus() } } }
        val tippen: () -> Unit = { aktion.longValue = System.currentTimeMillis() }

        // Tippen aufs Bild blendet ein/aus (ohne clickable, damit die Fläche auf dem TV keinen Fokus nimmt)
        Box(Modifier.fillMaxSize().pointerInput(Unit) { detectTapGestures { sichtbar.value = !sichtbar.value; panel.value = null } }
            // Zwei-Finger-Zoom (Handy): auseinander = Füllen, zusammen = Automatisch
            .pointerInput(Unit) {
                awaitEachGesture {
                    var zoom = 1f
                    awaitFirstDown(requireUnconsumed = false)
                    do {
                        val e = awaitPointerEvent()
                        if (e.changes.size >= 2) { zoom *= e.calculateZoom(); e.changes.forEach { it.consume() } }
                    } while (e.changes.any { it.pressed })
                    if (zoom > 1.15f && modus.value != BildModus.Fuellen) bildModus(BildModus.Fuellen, true)
                    else if (zoom < 0.87f && modus.value != BildModus.Auto) bildModus(BildModus.Auto, true)
                }
            }) {
            if (laedt.value) CircularProgressIndicator(Modifier.align(Alignment.Center).size(if (tv) 72.dp else 48.dp), color = K.Text)
            hinweis.value?.let { text ->
                LaunchedEffect(text) { delay(1500); hinweis.value = null }
                T(text, if (tv) 28.sp else 15.sp, K.Text, FontWeight.Medium, modifier = Modifier.align(Alignment.Center).offset(y = if (tv) (-160).dp else (-88).dp)
                    .background(K.BadgeGrund, RoundedCornerShape(4.dp)).padding(horizontal = 14.dp, vertical = 8.dp))
            }
            PartyAnzeige(Modifier.align(Alignment.TopEnd).padding(top = if (tv) 160.dp else 72.dp, end = if (tv) 96.dp else 24.dp))
            if (sichtbar.value) {
                Box(Modifier.fillMaxWidth().height(if (tv) 320.dp else 120.dp).background(Brush.verticalGradient(listOf(Color.Black.copy(alpha = 0.7f), Color.Transparent))))
                Box(Modifier.align(Alignment.BottomStart).fillMaxWidth().height(if (tv) 480.dp else 180.dp)
                    .background(Brush.verticalGradient(listOf(Color.Transparent, Color.Black.copy(alpha = 0.8f)))))
                // Oben: Zurück, Titel, Gemeinsam schauen
                Row(Modifier.fillMaxWidth().padding(horizontal = if (tv) 96.dp else 8.dp, vertical = if (tv) 54.dp else 8.dp), verticalAlignment = Alignment.CenterVertically) {
                    if (!tv) IconKnopf(Ic.ZURUECK, { finish() })
                    Column(Modifier.weight(1f).padding(start = 4.dp)) {
                        T(titel.value, if (tv) 48.sp else 18.sp, K.Text, FontWeight.SemiBold, maxLines = 1)
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            if (zeile.value.isNotEmpty()) T(zeile.value, if (tv) 28.sp else 13.sp, K.Text2, maxLines = 1, modifier = Modifier.padding(end = 12.dp))
                            if (live != null) T("LIVE", if (tv) 22.sp else 11.sp, K.AufMarke, family = Mono, spacing = 2.sp,
                                modifier = Modifier.background(K.Marke, RoundedCornerShape(2.dp)).padding(horizontal = 6.dp, vertical = 1.dp))
                            else Ampel(ampel.value, withText = tv)
                        }
                    }
                    if (file == null && live == null && party.value == null) IconKnopf(Ic.GEMEINSAM, { tippen(); gemeinsam() })
                }
                // Mitte: −10 s, Pause, +30 s
                Row(Modifier.align(Alignment.Center), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(if (tv) 48.dp else 32.dp)) {
                    if (live == null) Springen(Ic.NEUSTART, "10") { tippen(); p.seekBack() }
                    Box(Modifier.size(if (tv) 96.dp else 64.dp).focusRequester(pause).klick({ tippen(); umschalten() }).background(K.Marke), contentAlignment = Alignment.Center) {
                        Ico(if (spielt.value) Ic.PAUSE else Ic.ABSPIELEN, if (tv) 40.dp else 28.dp, K.AufMarke, voll = !spielt.value)
                    }
                    if (live == null) Springen(Ic.VOR, "30") { tippen(); p.seekForward() }
                }
                // Unten: Zeit, Leiste, Knöpfe
                Column(Modifier.align(Alignment.BottomStart).fillMaxWidth().padding(horizontal = if (tv) 96.dp else 24.dp).padding(bottom = if (tv) 54.dp else 6.dp)) {
                    if (live == null) {
                        Row {
                            T(fmtTime(pos / 1000.0), if (tv) 28.sp else 13.sp, K.Text, family = Mono, modifier = Modifier.weight(1f))
                            kapitelBei(pos)?.let { k -> T(k, if (tv) 24.sp else 12.sp, K.Text2, maxLines = 1, modifier = Modifier.padding(horizontal = 12.dp)) }
                            T("−" + fmtTime((dauer - pos).coerceAtLeast(0) / 1000.0), if (tv) 28.sp else 13.sp, K.Text2, family = Mono)
                        }
                        Leiste(pos, dauer, puffer) { tippen(); p.seekTo(it) }
                    }
                    Row(Modifier.height(if (tv) 88.dp else 48.dp), verticalAlignment = Alignment.CenterVertically) {
                        val g = if (tv) 72.dp else 44.dp
                        IconKnopf(Ic.TON, { tippen(); panel.value = if (panel.value == "ton") null else "ton" }, groesse = g, an = panel.value == "ton")
                        IconKnopf(Ic.UNTERTITEL, { tippen(); panel.value = if (panel.value == "ut") null else "ut" }, groesse = g, an = panel.value == "ut")
                        IconKnopf(Ic.QUALITAET, { tippen(); panel.value = if (panel.value == "q") null else "q" }, groesse = g, an = panel.value == "q")
                        T(if (dauer > 0 && live == null) "ENDET UM ${endetUm((dauer - pos) / 1000.0)}" else "", if (tv) 24.sp else 12.sp, K.Text2, family = Mono, spacing = 1.sp,
                            modifier = Modifier.weight(1f).wrapContentWidth(Alignment.CenterHorizontally))
                        // Bildmodus der Reihe nach (auch per D-Pad), mit Hinweis
                        IconKnopf(Ic.SEITENVERHAELTNIS, { tippen(); bildModus(BildModus.entries[(modus.value.ordinal + 1) % BildModus.entries.size], true) }, groesse = g)
                    }
                }
            }
            // „Vorspann überspringen“ und Nächste-Folge-Karte, auch bei ausgeblendeter Bedienung
            Column(Modifier.align(Alignment.BottomEnd).padding(end = if (tv) 96.dp else 24.dp, bottom = if (sichtbar.value) (if (tv) 240.dp else 120.dp) else (if (tv) 54.dp else 24.dp)),
                horizontalAlignment = Alignment.End) {
                val ziel = vorspannEnde(pos)
                SideEffect { sprungZiel = ziel }
                if (ziel != null) Knopf("Vorspann überspringen", { tippen(); p.seekTo(ziel) }, icon = Ic.NAECHSTE)
                val n = queue.firstOrNull()
                if (n != null && live == null && party.value == null && store.autoNext && !weiterAbgebrochen.value && dauer > 0 && dauer - pos in 1L..30_000L) {
                    WeiterKarte(n.second, n.third, ((dauer - pos) / 1000).toInt(), { tippen(); naechster() }, { weiterAbgebrochen.value = true })
                }
            }
            panel.value?.let { Auswahl(it, Modifier.align(Alignment.TopEnd)) }
        }
    }

    /** Name des Kapitels an Position [pos] (ms), wenn es Kapitel gibt. */
    private fun kapitelBei(pos: Long): String? {
        val k = kapitel.value
        if (k.size < 2) return null
        val i = k.indexOfLast { it.start * 1000 <= pos }.coerceAtLeast(0)
        return szenenName(k[i].name, i + 1)
    }

    /** Ende des Vorspann-Kapitels (Name Vorspann, Intro oder Opening), solange man darin steht, sonst null. */
    private fun vorspannEnde(pos: Long): Long? {
        val k = kapitel.value
        val i = k.indexOfLast { it.start * 1000 <= pos }
        if (i < 0 || i + 1 >= k.size || !Regex("(?i)vorspann|intro|opening").containsMatchIn(k[i].name)) return null
        return (k[i + 1].start * 1000).toLong()
    }

    /** Karte „Als Nächstes“ in den letzten 30 Sekunden: sofort weiter oder abbrechen. */
    @Composable
    private fun WeiterKarte(titel: String, zeile: String, sek: Int, onJetzt: () -> Unit, onAbbrechen: () -> Unit) {
        val tv = LocalTv.current
        Column(Modifier.padding(top = 12.dp).width(if (tv) 560.dp else 300.dp).background(K.PlayerLeiste, RoundedCornerShape(4.dp))
            .border(1.dp, K.Linie, RoundedCornerShape(4.dp)).padding(16.dp)) {
            Label("Als Nächstes · in $sek s")
            T(titel, if (tv) 30.sp else 16.sp, K.Text, FontWeight.SemiBold, maxLines = 1, modifier = Modifier.padding(top = 4.dp))
            if (zeile.isNotEmpty()) T(zeile, if (tv) 24.sp else 13.sp, K.Text2, maxLines = 1)
            Row(Modifier.padding(top = 12.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Knopf("Jetzt abspielen", onJetzt, primary = true, icon = Ic.ABSPIELEN)
                Knopf("Abbrechen", onAbbrechen)
            }
        }
    }

    @Composable
    private fun Springen(icon: String, sek: String, onClick: () -> Unit) {
        val tv = LocalTv.current
        Box(Modifier.size(if (tv) 80.dp else 56.dp).klick(onClick), contentAlignment = Alignment.Center) {
            Ico(icon, if (tv) 48.dp else 32.dp, K.Text)
            T(sek, if (tv) 18.sp else 11.sp, K.Text, family = Mono)
        }
    }

    /** Positionsleiste: tippen oder ziehen (Handy), ←/→ mit Fokus (TV). */
    @Composable
    private fun Leiste(pos: Long, dauer: Long, puffer: Long, onSeek: (Long) -> Unit) {
        val tv = LocalTv.current
        var fokus by remember { mutableStateOf(false) }
        val frac = if (dauer > 0) pos.toFloat() / dauer else 0f
        Box(Modifier.fillMaxWidth().height(if (tv) 40.dp else 24.dp)
            .onFocusChanged { fokus = it.isFocused }.focusable()
            .onKeyEvent { e ->
                if (e.type != KeyEventType.KeyDown || dauer <= 0) return@onKeyEvent false
                when (e.key) {
                    Key.DirectionLeft -> { onSeek((pos - 10_000).coerceAtLeast(0)); true }
                    Key.DirectionRight -> { onSeek((pos + 30_000).coerceAtMost(dauer)); true }
                    else -> false
                }
            }
            .pointerInput(dauer) {
                detectTapGestures { o -> if (dauer > 0) onSeek((o.x / size.width * dauer).toLong()) }
            }
            .pointerInput(dauer) { detectHorizontalDragGestures { ch, _ -> if (dauer > 0) onSeek((ch.position.x.coerceIn(0f, size.width.toFloat()) / size.width * dauer).toLong()) } },
            contentAlignment = Alignment.CenterStart) {
            // Kapitel als Abschnitte mit 2 px Abstand (wie im Entwurf), sonst eine durchgehende Leiste
            val grenzen = kapitel.value.map { (it.start * 1000).toLong() }.filter { it in 1 until dauer }.let { listOf(0L) + it + dauer }
            Row(Modifier.fillMaxWidth().height(if (tv) 6.dp else 4.dp), horizontalArrangement = Arrangement.spacedBy(2.dp)) {
                grenzen.zipWithNext().forEach { (a, b) ->
                    val laenge = (b - a).coerceAtLeast(1)
                    Box(Modifier.weight(laenge.toFloat()).fillMaxHeight().background(K.Text.copy(alpha = 0.22f))) {
                        Box(Modifier.fillMaxWidth(((puffer - a).toFloat() / laenge).coerceIn(0f, 1f)).fillMaxHeight().background(K.Text.copy(alpha = 0.4f)))
                        Box(Modifier.fillMaxWidth(((pos - a).toFloat() / laenge).coerceIn(0f, 1f)).fillMaxHeight().background(K.Text))
                    }
                }
            }
            BoxWithConstraints(Modifier.fillMaxWidth()) {
                val k = if (tv) 20.dp else 14.dp
                Box(Modifier.offset(x = (maxWidth - k) * frac.coerceIn(0f, 1f)).size(k).background(K.Text, RoundedCornerShape(2.dp))
                    .then(if (fokus) Modifier.border(3.dp, K.Saal, RoundedCornerShape(2.dp)) else Modifier))
            }
        }
    }

    /** Auswahl rechts (Ton, Untertitel, Qualität) wie im Entwurf. */
    @Composable
    private fun Auswahl(art: String, modifier: Modifier) {
        val tv = LocalTv.current
        val t = tracks.value
        Column(modifier.fillMaxHeight().width(if (tv) 560.dp else 280.dp).background(K.PlayerLeiste).border(1.dp, K.Linie)
            .pointerInput(Unit) { detectTapGestures { } }.padding(top = if (tv) 54.dp else 0.dp)) {
            Row(Modifier.height(if (tv) 72.dp else 52.dp).padding(start = 16.dp, end = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                T(when (art) { "ton" -> "Ton"; "ut" -> "Untertitel"; else -> "Wiedergabe & Bild" }, if (tv) 30.sp else 18.sp, K.Text, FontWeight.SemiBold, modifier = Modifier.weight(1f))
                IconKnopf(Ic.SCHLIESSEN, { panel.value = null })
            }
            LazyColumn {
                when (art) {
                    "ton" -> {
                        item { Label("Audiospur", Modifier.padding(start = 16.dp, bottom = 4.dp)) }
                        items(gruppen(t, C.TRACK_TYPE_AUDIO)) { g -> SpurEintrag(spurName(g.getTrackFormat(0)), g.isSelected) { waehle(C.TRACK_TYPE_AUDIO, g) } }
                        item {
                            Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
                            Row(Modifier.fillMaxWidth().heightIn(min = if (tv) 88.dp else 64.dp).klick({ aktion.longValue = System.currentTimeMillis(); nachtUmschalten() }, skala = false)
                                .padding(horizontal = 16.dp, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                                Column(Modifier.weight(1f)) {
                                    T("Nachtmodus", if (tv) 28.sp else 15.sp, K.Text, FontWeight.Medium)
                                    T("Laute Stellen leiser, leise Dialoge lauter", if (tv) 22.sp else 13.sp, K.Text2)
                                }
                                Schalter(nacht.value)
                            }
                        }
                    }
                    "ut" -> {
                        val text = gruppen(t, C.TRACK_TYPE_TEXT)
                        item { SpurEintrag("Aus", text.none { it.isSelected }) { waehle(C.TRACK_TYPE_TEXT, null) } }
                        items(text) { g -> SpurEintrag(spurName(g.getTrackFormat(0)) + if ((g.getTrackFormat(0).selectionFlags and C.SELECTION_FLAG_FORCED) != 0) " (erzwungen)" else "",
                            g.isSelected) { waehle(C.TRACK_TYPE_TEXT, g) } }
                    }
                    else -> {
                        if (live == null && party.value == null) {
                            item { Label("Geschwindigkeit", Modifier.padding(start = 16.dp, bottom = 4.dp)) }
                            items(listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 2f)) { v ->
                                SpurEintrag(if (v == 1f) "Normal" else "${v.toString().replace('.', ',').removeSuffix(",0")}×", v == tempo.floatValue) {
                                    tempo.floatValue = v; player?.setPlaybackSpeed(v)
                                }
                            }
                        }
                        if (file == null && live == null) {
                            item { Label("Qualität" + (laeuftHoehe.intValue.takeIf { it > 0 }?.let { " · läuft in ${it}p" } ?: ""), Modifier.padding(start = 16.dp, top = 16.dp, bottom = 4.dp)) }
                            // nur Stufen bis zur Quelle; eine Grenze darüber ist das Original
                            val h = quelleHoehe.intValue
                            val stufen = listOf(0 to "Automatisch (Original)", 1080 to "1080p", 720 to "720p", 480 to "480p – spart Daten").filter { (q, _) -> q == 0 || h == 0 || q <= h }
                            items(stufen) { (q, name) ->
                                SpurEintrag(name, q == qualitaet.intValue || q == 0 && h > 0 && qualitaet.intValue > h) { qualitaetWaehlen(q); panel.value = null }
                            }
                        }
                        item { Label("Bild", Modifier.padding(start = 16.dp, top = 16.dp, bottom = 4.dp)) }
                        items(BildModus.entries) { m -> SpurEintrag(m.label, m == modus.value) { bildModus(m, false) } }
                    }
                }
            }
        }
    }

    @Composable
    private fun SpurEintrag(name: String, an: Boolean, onClick: () -> Unit) {
        val tv = LocalTv.current
        Box(Modifier.fillMaxWidth().height(1.dp).background(K.Linie))
        Row(Modifier.fillMaxWidth().heightIn(min = if (tv) 72.dp else 52.dp).klick({ aktion.longValue = System.currentTimeMillis(); onClick() }, skala = false)
            .background(if (an) K.Flaeche3 else Color.Transparent).padding(horizontal = 16.dp, vertical = 6.dp), verticalAlignment = Alignment.CenterVertically) {
            T(name, if (tv) 28.sp else 15.sp, K.Text, if (an) FontWeight.SemiBold else FontWeight.Medium, maxLines = 2, modifier = Modifier.weight(1f))
            if (an) Ico(Ic.HAKEN, if (tv) 32.dp else 24.dp, K.Text)
        }
    }

    // ---------- Gemeinsam schauen ----------

    /** Aus dem Player heraus einen Raum öffnen: alle anderen steigen an dieser Stelle ein. */
    private fun gemeinsam() {
        lifecycleScope.launch {
            runCatching { PartyClient(api).create(id) }
                .onSuccess { room -> party.value = room.id; joinParty(room.id) }
                .onFailure { Toast.makeText(this@PlayerActivity, "Gemeinsam schauen geht gerade nicht", Toast.LENGTH_LONG).show() }
        }
    }

    private fun joinParty(room: String) {
        val client = PartyClient(api)
        lifecycleScope.launch {
            offset = runCatching { client.offset() }.getOrDefault(0L)
            val info = runCatching { client.get(room) }.getOrElse {
                Toast.makeText(this@PlayerActivity, "Raum gibt es nicht (mehr)", Toast.LENGTH_LONG).show()
                return@launch finish()
            }
            messages += "Raum $room – Code zum Mitmachen weitergeben"
            launch { // Drift-Korrektur jede Sekunde
                while (true) {
                    delay(1000)
                    syncTo(roomState ?: continue)
                }
            }
            client.events(info.eventsUrl).collect { ev ->
                when (ev.name) {
                    "hello" -> { member = client.field(ev.data, "member"); roomState = client.state(ev.data); roomState?.let(::syncTo) }
                    "state" -> { roomState = client.state(ev.data); roomState?.let(::syncTo) }
                    "members" -> members.value = Regex("\"([^\"]+)\"").findAll(ev.data.substringAfter('[')).map { it.groupValues[1] }.toList()
                    "chat" -> push("${client.field(ev.data, "from")}: ${client.field(ev.data, "text")}")
                    "reaction" -> push("${client.field(ev.data, "from")} ${client.field(ev.data, "text")}")
                }
            }
        }
    }

    private fun push(msg: String) {
        messages += msg
        lifecycleScope.launch { delay(8000); messages.remove(msg) }
    }

    private fun syncTo(s: PartyState) {
        val p = player ?: return
        applying = true
        try {
            val target = PartySync.target(s, System.currentTimeMillis() + offset)
            val current = p.currentPosition / 1000.0
            if (s.paused) {
                if (p.playWhenReady) p.pause()
                if (kotlin.math.abs(current - target) > 1.0) p.seekTo((target * 1000).toLong())
                return
            }
            if (!p.playWhenReady) p.play()
            when (val fix = PartySync.correct(current, target, s.rate)) {
                is PartySync.Fix.Rate -> if (p.playbackParameters.speed != fix.speed) p.playbackParameters = PlaybackParameters(fix.speed)
                is PartySync.Fix.Seek -> p.seekTo((fix.to * 1000).toLong())
                PartySync.Fix.None -> {}
            }
        } finally {
            applying = false
        }
    }

    private fun send(type: String, buffering: Boolean = false, text: String = "") {
        val room = party.value ?: return
        if (member.isEmpty()) return
        val pos = (player?.currentPosition ?: 0) / 1000.0
        roomState = roomState?.let { st ->
            val now = System.currentTimeMillis() + offset
            when (type) {
                "play" -> st.copy(paused = false, pos = pos, serverTs = now)
                "pause" -> st.copy(paused = true, pos = pos, serverTs = now)
                "seek" -> st.copy(pos = pos, serverTs = now)
                else -> st
            }
        }
        lifecycleScope.launch { PartyClient(api).act(room, PartyAction(member, type, pos = pos, buffering = buffering, text = text)) }
    }

    /** Raum, Mitglieder, Nachrichten; auf dem Handy dazu Reaktionen. */
    @Composable
    private fun PartyAnzeige(modifier: Modifier) {
        val room = party.value ?: return
        Column(modifier, horizontalAlignment = Alignment.End, verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Label("Gemeinsam · $room · ${members.value.size} dabei", color = K.Text)
            messages.takeLast(4).forEach {
                T(it, 16.sp, K.Text, modifier = Modifier.background(K.BadgeGrund, RoundedCornerShape(4.dp)).padding(horizontal = 10.dp, vertical = 6.dp))
            }
            if (!LocalTv.current && sichtbar.value) Row(Modifier.padding(top = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                listOf("👍", "😂", "😮", "❤️", "🍿").forEach { e ->
                    T(e, 26.sp, modifier = Modifier.background(K.BadgeGrund, RoundedCornerShape(999.dp)).clickable { send("reaction", text = e) }.padding(8.dp))
                }
            }
        }
    }

    // ---------- Fortschritt ----------

    /** Aktuelle Sprachen der gewählten Spuren – der Server merkt sie sich pro Serie. */
    private fun language(type: Int): String {
        val g = player?.currentTracks?.groups?.firstOrNull { it.type == type && it.isSelected } ?: return if (type == C.TRACK_TYPE_TEXT) "off" else ""
        for (i in 0 until g.length) if (g.isTrackSelected(i)) return g.getTrackFormat(i).language ?: "und"
        return ""
    }

    /** Meldet den Fortschritt; kommt er nicht an (offline), merkt ihn Downloads und sendet ihn beim nächsten Kontakt. */
    private suspend fun report() {
        if (live != null) return
        val p = player ?: return
        val dur = if (p.duration == C.TIME_UNSET) 0 else p.duration / 1000
        val pos = p.currentPosition / 1000
        val audio = language(C.TRACK_TYPE_AUDIO)
        val text = language(C.TRACK_TYPE_TEXT)
        val itemId = id
        runCatching { api.progress(itemId, pos, dur, audio, text, paused = !p.isPlaying) }
            .onSuccess { downloads.forget(itemId) }
            .onFailure { downloads.remember(itemId, pos, dur, audio, text) }
    }

    override fun onStop() {
        super.onStop()
        applying = true // Verlassen pausiert nur bei mir, nicht den ganzen Raum
        player?.pause()
        applying = false
        lifecycleScope.launch { withContext(NonCancellable) { report() } }
    }

    override fun onDestroy() {
        nachtFx?.release()
        player?.release()
        player = null
        super.onDestroy()
    }
}
