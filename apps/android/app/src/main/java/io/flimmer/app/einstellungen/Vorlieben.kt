package io.flimmer.app.einstellungen

import android.content.Context
import io.flimmer.app.HomeRow

// Vorlieben auf diesem Gerät, in denselben SharedPreferences „flimmer“ wie Store (Device.kt).
// Tonsprachen und Untertitelmodus liest Store (audioLangs, subtitleMode), der Player schickt sie im play-Body mit.
// Die Startseiten-Wahl liest StartSeite über startReihen(). Der Server kennt für beides (noch) keine Route, wie im Web.

private fun prefs(ctx: Context) = ctx.getSharedPreferences("flimmer", Context.MODE_PRIVATE)
private fun String.teile() = split(',').map { it.trim() }.filter { it.isNotEmpty() }

fun setTonsprachen(ctx: Context, l: List<String>) = prefs(ctx).edit().putString("audioLangs", l.joinToString(",")).apply()

/** "" = intelligent, "always", "off" (nur erzwungene). */
fun setUntertitelModus(ctx: Context, m: String) = prefs(ctx).edit().putString("subtitleMode", m).apply()

/** Reihen, die der Server heute liefert (POST /api/home). */
val START_REIHEN = listOf("continue", "nextup", "recent-movies", "recent-series")
val REIHEN_NAMEN = mapOf(
    "continue" to "Weiterschauen",
    "nextup" to "Als Nächstes",
    "recent-movies" to "Kürzlich hinzugefügt in Filme",
    "recent-series" to "Kürzlich hinzugefügt in Serien",
)

data class StartVorlieben(val reihenfolge: List<String> = START_REIHEN, val aus: List<String> = emptyList()) {
    /** Alle bekannten Reihen in der gewählten Reihenfolge, neue hinten. */
    val liste: List<String> get() = reihenfolge + START_REIHEN.filter { it !in reihenfolge }

    /** Wie startReihen() im Web: ausgeblendete weg, unbekannte ans Ende. */
    fun <T> sortiere(rows: List<T>, id: (T) -> String): List<T> =
        rows.filter { id(it) !in aus }.sortedBy { reihenfolge.indexOf(id(it)).let { i -> if (i < 0) 999 else i } }
}

fun startVorlieben(ctx: Context): StartVorlieben = prefs(ctx).let { p ->
    StartVorlieben(p.getString("startReihen", null)?.teile() ?: START_REIHEN, p.getString("startAus", "").orEmpty().teile())
}

fun setStartVorlieben(ctx: Context, v: StartVorlieben) =
    prefs(ctx).edit().putString("startReihen", v.reihenfolge.joinToString(",")).putString("startAus", v.aus.joinToString(",")).apply()

/** Für StartSeite: Reihen der Startseite nach der Wahl unter Einstellungen › Startseite. */
fun startReihen(ctx: Context, rows: List<HomeRow>): List<HomeRow> = startVorlieben(ctx).sortiere(rows) { it.id }
