package io.flimmer.app

import org.junit.Assert.assertEquals
import org.junit.Test

class AuswahlTest {
    @Test fun allerweltsnamenWerdenSzenen() {
        listOf("Chapter 1", "Kapitel 01", "Chapter 1.", "chapter 12", "Ch. 3", "", "  ", "00:12:30.000", "1:02:03", "07")
            .forEachIndexed { i, n -> assertEquals(n, "Szene ${i + 1}", szenenName(n, i + 1)) }
        assertEquals("Banküberfall", szenenName(" Banküberfall ", 2))
        assertEquals("Kapitel 1: Der Anfang", szenenName("Kapitel 1: Der Anfang", 1))
    }

    private fun film(id: String, jahr: Int, added: String, vararg genres: String) =
        Item(id, id, year = jahr, added = added, meta = Meta(genres = genres.toList()))

    @Test fun mehrWieDiesesMitRueckfall() {
        val a = film("a", 1995, "2026-01-01", "Krimi")
        val b = film("b", 1990, "2026-01-02", "Krimi", "Drama")
        val c = film("c", 1971, "2026-01-03")
        val d = film("d", 1998, "2026-01-04")
        val e = film("e", 2020, "2026-09-01")
        val alle = listOf(a, b, c, d, e)
        val sammlung = listOf(Liste("s1", "Reihe", items = listOf("x", "c")))
        // Genres zuerst (b vor a: zwei Treffer), dann Sammlung (c), dann Jahrzehnt (d), dann die neuesten (e)
        assertEquals(listOf("b", "a", "c", "d", "e"), mehrWieDieses(listOf("Krimi", "Drama"), 1994, "x", alle, sammlung).map { it.id })
        // ohne Genres und Sammlungen: Jahrzehnt, dann zuletzt hinzugefügt
        assertEquals(listOf("a", "b", "d", "e", "c"), mehrWieDieses(emptyList(), 1999, "x", alle, null).map { it.id })
        assertEquals(2, mehrWieDieses(emptyList(), null, "x", alle, null, max = 2).size)
    }
}
