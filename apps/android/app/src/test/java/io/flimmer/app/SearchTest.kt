package io.flimmer.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class SearchTest {
    private fun hit(q: String, t: String) = score(q, t) > 0

    @Test fun tippfehlerTolerant() {
        assertTrue(hit("her der ringe", "Der Herr der Ringe"))
        assertTrue(hit("herr ringe", "Der Herr der Ringe: Die Gefährten"))
        assertTrue(hit("gefahrten", "Der Herr der Ringe: Die Gefährten"))
        assertTrue(hit("nosferatu", "Nosferatu – Eine Symphonie des Grauens"))
        assertTrue(hit("nosfaratu", "Nosferatu"))
        assertTrue(hit("strasse", "Die Straße"))
    }

    @Test fun keineFalschenTreffer() {
        assertEquals(0, score("dark", "Das Boot"))
        assertEquals(0, score("ab", "Heat"))
        assertEquals(0, score("", "Heat"))
    }

    @Test fun serienNurEinmalUndBesteZuerst() {
        val items = listOf(
            Item("1", "Episode 1", series = "Dark", season = 1, episode = 1),
            Item("2", "Episode 2", series = "Dark", season = 1, episode = 2),
            Item("3", "Dark Waters"),
        )
        val r = search("dark", items)
        assertEquals(listOf("1", "3"), r.map { it.id })
    }
}
