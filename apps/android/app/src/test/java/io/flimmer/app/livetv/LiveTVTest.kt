package io.flimmer.app.livetv

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class LiveTVTest {
    @Test fun kanaeleMitLueckenUndUnbekanntemFeld() {
        val l = kanaele(
            """[{"id":"a","number":"1","name":"Das Erste","group":"ARD","extra":42,
                 "now":{"start":"2026-09-29T20:15:00+02:00","stop":"2026-09-29T21:45:00+02:00","title":"Tatort","desc":"Krimi"},"next":null},
                {"id":"b","name":"arte","now":null}]""",
        )
        assertEquals(2, l.size)
        assertEquals("1", l[0].number)
        assertEquals("Tatort", l[0].now?.title)
        assertNull(l[0].next)
        assertNull(l[1].number)
        assertNull(l[1].logo)
        assertEquals(emptyList<Kanal>(), kanaele("null"))
        assertEquals(emptyList<Kanal>(), kanaele(""))
    }

    @Test fun programmMitNullListen() {
        val g = fuehrer(
            """{"from":"2026-09-29T18:40:00Z","to":"2026-09-30T06:40:00Z",
                "channels":[{"id":"a","programs":[{"start":"2026-09-29T18:15:00Z","stop":"2026-09-29T19:00:00Z","title":"Nachrichten"}]},
                            {"id":"b","programs":null}]}""",
        )
        assertEquals(2, g.channels!!.size)
        assertEquals("Nachrichten", g.channels!![0].programs!![0].title)
        assertNull(g.channels!![1].programs)
        assertNull(fuehrer("""{"from":"","to":"","channels":null}""").channels)
    }

    @Test fun zeitenAusRfc3339() {
        val utc = 1790705700000L // 2026-09-29T18:15:00Z
        assertEquals(utc, zeit("2026-09-29T18:15:00Z"))
        assertEquals(utc, zeit("2026-09-29T20:15:00+02:00"))
        assertEquals(utc, zeit("2026-09-29T20:15:00.123456789+02:00"))
        assertEquals(utc, zeit("2026-09-29T13:15:00-05:00"))
        assertEquals(0L, zeit("gestern"))
    }

    @Test fun rasterMinutenZuPosition() {
        // Beginn 20:40 → Raster ab 20:30, 5 dp pro Minute
        val r = Raster("2026-09-29T20:40:00+02:00", "2026-09-29T23:00:00+02:00", 5)
        assertEquals(zeit("2026-09-29T20:30:00+02:00"), r.von)
        assertEquals(50, r.px(zeit("2026-09-29T20:40:00+02:00")))
        assertEquals(150 * 5, r.breite)
        assertEquals(listOf("20:30", "21:00", "21:30", "22:00", "22:30").size, r.marken.size)
        assertEquals(r.von + HALBE_STUNDE, r.marken[1])

        // Beginnt vor dem Raster: links abgeschnitten, Breite bis zum Ende minus 4 dp Luft
        assertEquals(0 to 45 * 5 - 4, r.block(Sendung("2026-09-29T20:15:00+02:00", "2026-09-29T21:15:00+02:00", "Film")))
        // Mitten drin
        assertEquals(30 * 5 to 30 * 5 - 4, r.block(Sendung("2026-09-29T21:00:00+02:00", "2026-09-29T21:30:00+02:00", "Serie")))
        // Kurz: mindestens fünf Minuten breit
        assertEquals(60 * 5 to 25, r.block(Sendung("2026-09-29T21:30:00+02:00", "2026-09-29T21:31:00+02:00", "Wetter")))
        // Läuft über das Ende hinaus: endet am Raster
        assertEquals(120 * 5 to 30 * 5 - 4, r.block(Sendung("2026-09-29T22:30:00+02:00", "2026-09-30T01:00:00+02:00", "Nacht")))
    }

    @Test fun anteilUndLaeuft() {
        val s = Sendung("2026-09-29T20:00:00Z", "2026-09-29T21:00:00Z", "x")
        val halb = zeit("2026-09-29T20:30:00Z")
        assertEquals(0.5f, s.anteil(halb), 1e-6f)
        assertTrue(s.laeuft(halb))
        assertEquals(1f, s.anteil(zeit("2026-09-29T22:00:00Z")), 0f)
        assertTrue(!s.laeuft(zeit("2026-09-29T21:00:00Z")))
    }
}
