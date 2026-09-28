package io.flimmer.app

import android.net.Uri
import android.os.Bundle
import android.view.WindowManager
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.lifecycle.lifecycleScope
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.ui.PlayerView
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/**
 * Nativer Player (Media3/ExoPlayer). ExoPlayer kann MKV/HEVC und mit dem ffmpeg-Decoder auch DTS/TrueHD,
 * deshalb plant der Server fast immer Direct Play; eingebettete Ton- und Untertitelspuren (SRT/ASS/PGS)
 * wählt man direkt im Player. HLS kommt nur als Fallback.
 */
class PlayerActivity : ComponentActivity() {
    private var player: ExoPlayer? = null
    private lateinit var api: ApiClient
    private lateinit var id: String

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        api = ApiClient(intent.getStringExtra("server") ?: return finish(), intent.getStringExtra("token") ?: "")
        id = intent.getStringExtra("id") ?: return finish()
        val start = intent.getDoubleExtra("start", -1.0)

        // Plattform-Decoder zuerst, ffmpeg nur für das, was das Gerät nicht kann (DTS, TrueHD …).
        val renderers = DefaultRenderersFactory(this).setExtensionRendererMode(DefaultRenderersFactory.EXTENSION_RENDERER_MODE_ON)
        val p = ExoPlayer.Builder(this, renderers).build()
        player = p
        setContentView(PlayerView(this).apply {
            this.player = p
            setShowSubtitleButton(true)
            keepScreenOn = true
            requestFocus() // D-Pad sofort im Player
        })
        p.addListener(object : Player.Listener {
            override fun onPlaybackStateChanged(state: Int) {
                if (state == Player.STATE_ENDED) finish()
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
            p.play()
            while (true) { // Fortschritt alle 10 s
                delay(10_000)
                if (p.isPlaying) report()
            }
        }
    }

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
        player?.pause()
        lifecycleScope.launch { withContext(NonCancellable) { report() } }
    }

    override fun onDestroy() {
        player?.release()
        player = null
        super.onDestroy()
    }
}
