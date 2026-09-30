package io.flimmer.app.einstellungen

import org.junit.Assert.assertEquals
import org.junit.Test

class VorliebenTest {
    @Test fun startReihenSortiertUndBlendetAus() {
        val v = StartVorlieben(reihenfolge = listOf("nextup", "continue"), aus = listOf("recent-movies"))
        val rows = listOf("continue", "nextup", "recent-movies", "recent-series", "neu")
        assertEquals(listOf("nextup", "continue", "recent-series", "neu"), v.sortiere(rows) { it })
        assertEquals(listOf("nextup", "continue", "recent-movies", "recent-series"), v.liste)
    }
}
