package io.flimmer.app

import io.flimmer.app.PartySync.Fix
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class PartyTest {
    @Test fun offsetIstMedianUndIgnoriertAusreisser() {
        // Server 1000 ms voraus; eine Messung mit Netz-Ausreißer
        val s = listOf(Triple(0L, 1010L, 20L), Triple(100L, 1110L, 120L), Triple(200L, 5000L, 220L), Triple(300L, 1310L, 320L), Triple(400L, 1410L, 420L))
        assertEquals(1000L, PartySync.offset(s))
    }

    @Test fun sollPosition() {
        val st = PartyState(pos = 10.0, serverTs = 1_000, rate = 1.0, paused = false)
        assertEquals(12.5, PartySync.target(st, 3_500), 1e-9)
        assertEquals(10.0, PartySync.target(st.copy(paused = true), 3_500), 1e-9)
        assertEquals(15.0, PartySync.target(st.copy(rate = 2.0), 3_500), 1e-9)
    }

    @Test fun driftKorrektur() {
        assertEquals(Fix.Rate(1f), PartySync.correct(10.0, 10.2, 1.0))
        assertEquals(Fix.Rate(1.05f), PartySync.correct(10.0, 10.6, 1.0))
        assertEquals(Fix.Rate(0.95f), PartySync.correct(10.6, 10.0, 1.0))
        assertEquals(Fix.Seek(12.0), PartySync.correct(10.0, 12.0, 1.0))
    }

    @Test fun sseParser() {
        val p = SseParser()
        val lines = listOf("retry: 2000", "event: hello", "data: {\"member\":\"u1.a\"}", "", ": ping", "", "event: chat", "data: a", "data: b", "")
        val evs = lines.mapNotNull { p.line(it) }
        assertEquals(listOf(SseEvent("hello", "{\"member\":\"u1.a\"}"), SseEvent("chat", "a\nb")), evs)
        assertNull(p.line(""))
    }
}
