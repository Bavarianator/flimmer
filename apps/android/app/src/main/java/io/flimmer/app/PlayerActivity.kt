package io.flimmer.app

import android.net.Uri
import android.os.Bundle
import android.view.ViewGroup
import android.view.WindowManager
import android.widget.FrameLayout
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.ComposeView
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.lifecycleScope
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.PlaybackParameters
import androidx.media3.common.Player
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.ui.PlayerView
import io.flimmer.app.ui.FlimmerTheme
import io.flimmer.app.ui.K
import io.flimmer.app.ui.Label
import io.flimmer.app.ui.T
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/**
 * Nativer Player (Media3/ExoPlayer). ExoPlayer kann MKV/HEVC und mit dem ffmpeg-Decoder auch DTS/TrueHD,
 * deshalb plant der Server fast immer Direct Play; eingebettete Ton- und Untertitelspuren (SRT/ASS/PGS)
 * wählt man direkt im Player. HLS kommt nur als Fallback.
 * Mit Extra „party“ läuft die Wiedergabe synchron mit einem Raum („Gemeinsam schauen“).
 */
class PlayerActivity : ComponentActivity() {
    private var player: ExoPlayer? = null
    private lateinit var api: ApiClient
    private lateinit var id: String

    // Gemeinsam schauen
    private var partyId: String? = null
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
        id = intent.getStringExtra("id") ?: return finish()
        partyId = intent.getStringExtra("party")
        val start = intent.getDoubleExtra("start", -1.0)

        // Plattform-Decoder zuerst, ffmpeg nur für das, was das Gerät nicht kann (DTS, TrueHD …).
        val renderers = DefaultRenderersFactory(this).setExtensionRendererMode(DefaultRenderersFactory.EXTENSION_RENDERER_MODE_ON)
        val p = ExoPlayer.Builder(this, renderers).build()
        player = p
        val view = PlayerView(this).apply {
            this.player = p
            setShowSubtitleButton(true)
            keepScreenOn = true
            requestFocus() // D-Pad sofort im Player
        }
        setContentView(FrameLayout(this).apply {
            addView(view, ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT)
            if (partyId != null) addView(ComposeView(this@PlayerActivity).apply {
                isFocusable = false
                setContent { FlimmerTheme(isTv(this@PlayerActivity)) { PartyOverlay() } }
            }, ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT)
        })
        p.addListener(object : Player.Listener {
            override fun onPlaybackStateChanged(state: Int) {
                if (state == Player.STATE_ENDED && partyId == null) finish()
                if (partyId != null) when (state) {
                    Player.STATE_BUFFERING -> send("buffering", buffering = true)
                    Player.STATE_READY -> send("buffering", buffering = false)
                }
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

        lifecycleScope.launch {
            val plan = runCatching { api.play(id, deviceProfile(this@PlayerActivity)) }.getOrElse {
                Toast.makeText(this@PlayerActivity, "Server nicht erreichbar", Toast.LENGTH_LONG).show()
                return@launch finish()
            }
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
            p.trackSelectionParameters = p.trackSelectionParameters.buildUpon().apply {
                if (plan.prefs.audio.isNotEmpty()) setPreferredAudioLanguage(plan.prefs.audio)
                if (plan.prefs.subtitle.isNotEmpty() && plan.prefs.subtitle != "off") setPreferredTextLanguage(plan.prefs.subtitle)
            }.build()
            val pos = if (start >= 0) start else plan.resume
            p.setMediaItem(item.build(), (pos * 1000).toLong())
            p.prepare()
            if (partyId == null) p.play() else joinParty(partyId!!)
            while (true) { // Fortschritt alle 10 s
                delay(10_000)
                if (p.isPlaying) report()
            }
        }
    }

    // ---------- Gemeinsam schauen ----------

    private fun joinParty(room: String) {
        val party = PartyClient(api)
        lifecycleScope.launch {
            offset = runCatching { party.offset() }.getOrDefault(0L)
            val info = runCatching { party.get(room) }.getOrElse {
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
            party.events(info.eventsUrl).collect { ev ->
                when (ev.name) {
                    "hello" -> { member = party.field(ev.data, "member"); roomState = party.state(ev.data); roomState?.let(::syncTo) }
                    "state" -> { roomState = party.state(ev.data); roomState?.let(::syncTo) }
                    "members" -> members.value = Regex("\"([^\"]+)\"").findAll(ev.data.substringAfter('[')).map { it.groupValues[1] }.toList()
                    "chat" -> push("${party.field(ev.data, "from")}: ${party.field(ev.data, "text")}")
                    "reaction" -> push("${party.field(ev.data, "from")} ${party.field(ev.data, "text")}")
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
        val room = partyId ?: return
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

    @androidx.compose.runtime.Composable
    private fun PartyOverlay() {
        Box(Modifier.fillMaxSize().padding(24.dp)) {
            Column(Modifier.align(Alignment.TopEnd), horizontalAlignment = Alignment.End, verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Label("Gemeinsam · ${partyId ?: ""} · ${members.value.size} dabei")
                messages.takeLast(4).forEach {
                    T(it, 16.sp, K.Text, modifier = Modifier.background(K.BadgeGrund, RoundedCornerShape(4.dp)).padding(horizontal = 10.dp, vertical = 6.dp))
                }
            }
            if (!isTv(this@PlayerActivity)) Row(Modifier.align(Alignment.BottomEnd).padding(bottom = 72.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
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

    private suspend fun report() {
        val p = player ?: return
        val dur = if (p.duration == C.TIME_UNSET) 0 else p.duration / 1000
        runCatching { api.progress(id, p.currentPosition / 1000, dur, language(C.TRACK_TYPE_AUDIO), language(C.TRACK_TYPE_TEXT)) }
    }

    override fun onStop() {
        super.onStop()
        applying = true // Verlassen pausiert nur bei mir, nicht den ganzen Raum
        player?.pause()
        applying = false
        lifecycleScope.launch { withContext(NonCancellable) { report() } }
    }

    override fun onDestroy() {
        player?.release()
        player = null
        super.onDestroy()
    }
}
