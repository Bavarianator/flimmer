package io.flimmer.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class DownloadsTest {
    @Test fun pendingRoundTrip() {
        val p = Downloads.Pending(1234, 5400, "ger", "off")
        assertEquals(p, Downloads.Pending.decode(p.encode()))
        assertEquals(Downloads.Pending(1, 2, "", ""), Downloads.Pending.decode("1|2||"))
    }

    @Test fun pendingKaputtIstNull() {
        assertNull(Downloads.Pending.decode("x|2|a|b"))
        assertNull(Downloads.Pending.decode("1|2"))
    }
}
