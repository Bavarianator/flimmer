package io.flimmer.app

import android.content.Context
import android.net.Uri
import androidx.mediarouter.media.MediaRouteSelector
import androidx.mediarouter.media.MediaRouter
import com.google.android.gms.cast.CastMediaControlIntent
import com.google.android.gms.cast.MediaInfo
import com.google.android.gms.cast.MediaLoadRequestData
import com.google.android.gms.cast.MediaMetadata
import com.google.android.gms.cast.MediaTrack
import com.google.android.gms.cast.framework.CastContext
import com.google.android.gms.cast.framework.CastOptions
import com.google.android.gms.cast.framework.OptionsProvider
import com.google.android.gms.cast.framework.SessionProvider
import com.google.android.gms.common.images.WebImage
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow

/** Chromecast über den Default Media Receiver – kein eigener Receiver nötig. */
class CastOptionsProvider : OptionsProvider {
    override fun getCastOptions(ctx: Context): CastOptions =
        CastOptions.Builder().setReceiverApplicationId(CastMediaControlIntent.DEFAULT_MEDIA_RECEIVER_APPLICATION_ID).build()

    override fun getAdditionalSessionProviders(ctx: Context): List<SessionProvider>? = null
}

object Cast {
    /**
     * Was der Default Media Receiver sicher kann (ältere Chromecasts eingeschlossen). Der Server entscheidet
     * damit wie für jedes andere Gerät: Direct Play, Remux oder Umwandeln.
     */
    val profile = Profile(
        name = "Chromecast",
        containers = listOf("mp4", "webm"),
        video = listOf("h264", "vp9"),
        audio = listOf("aac", "mp3", "opus", "flac"),
        nativeHls = true,
        hdr = emptyList(),
    )

    private val selector = MediaRouteSelector.Builder()
        .addControlCategory(CastMediaControlIntent.categoryForCast(CastMediaControlIntent.DEFAULT_MEDIA_RECEIVER_APPLICATION_ID))
        .build()

    /** Cast-Geräte im Heimnetz, laufend aktualisiert. */
    fun devices(ctx: Context): Flow<List<MediaRouter.RouteInfo>> = callbackFlow {
        CastContext.getSharedInstance(ctx) // initialisiert das Framework
        val router = MediaRouter.getInstance(ctx)
        fun emitRoutes() = trySend(router.routes.filter { !it.isDefault && it.matchesSelector(selector) && it.isEnabled })
        val cb = object : MediaRouter.Callback() {
            override fun onRouteAdded(r: MediaRouter, route: MediaRouter.RouteInfo) { emitRoutes() }
            override fun onRouteRemoved(r: MediaRouter, route: MediaRouter.RouteInfo) { emitRoutes() }
            override fun onRouteChanged(r: MediaRouter, route: MediaRouter.RouteInfo) { emitRoutes() }
        }
        router.addCallback(selector, cb, MediaRouter.CALLBACK_FLAG_REQUEST_DISCOVERY)
        emitRoutes()
        awaitClose { router.removeCallback(cb) }
    }

    /** Verbindet mit [route] und spielt [item] dort ab. Wirft, wenn keine Sitzung zustande kommt. */
    suspend fun play(ctx: Context, route: MediaRouter.RouteInfo, api: ApiClient, item: Item, start: Double?) {
        val plan = api.play(item.id, profile)
        MediaRouter.getInstance(ctx).selectRoute(route)
        val sessions = CastContext.getSharedInstance(ctx).sessionManager
        var client = sessions.currentCastSession?.remoteMediaClient
        repeat(50) { // Sitzungsaufbau dauert ein paar Sekunden
            if (client != null) return@repeat
            delay(200)
            client = sessions.currentCastSession?.remoteMediaClient
        }
        val remote = client ?: throw IllegalStateException("Chromecast antwortet nicht")

        val meta = MediaMetadata(if (item.series != null) MediaMetadata.MEDIA_TYPE_TV_SHOW else MediaMetadata.MEDIA_TYPE_MOVIE).apply {
            putString(MediaMetadata.KEY_TITLE, item.series?.let { "$it · S${item.season} E${item.episode}" } ?: item.displayTitle)
            item.meta?.overview?.let { putString(MediaMetadata.KEY_SUBTITLE, it.take(120)) }
            if (item.poster.isNotEmpty()) addImage(WebImage(Uri.parse(api.abs(item.poster))))
        }
        // Der Receiver kann nur WebVTT; Bild-Untertitel (PGS) lassen wir weg statt einzubrennen.
        val tracks = plan.subtitles.orEmpty().filter { it.format == "vtt" }.mapIndexed { i, s ->
            MediaTrack.Builder(i + 1L, MediaTrack.TYPE_TEXT)
                .setContentId(api.abs(s.url)).setContentType("text/vtt")
                .setSubtype(MediaTrack.SUBTYPE_SUBTITLES).setLanguage(s.language ?: "und").setName(s.title ?: s.language)
                .build()
        }
        val url = api.abs(plan.url) // Token steckt im Pfad – der Chromecast braucht keine Anmeldung
        val info = MediaInfo.Builder(url)
            .setStreamType(MediaInfo.STREAM_TYPE_BUFFERED)
            .setContentType(if (url.contains(".m3u8")) "application/x-mpegurl" else "video/mp4")
            .setMetadata(meta)
            .setMediaTracks(tracks)
            .build()
        val pos = (if (start != null && start >= 0) start else plan.resume)
        remote.load(MediaLoadRequestData.Builder().setMediaInfo(info).setAutoplay(true).setCurrentTime((pos * 1000).toLong()).build())
    }
}
