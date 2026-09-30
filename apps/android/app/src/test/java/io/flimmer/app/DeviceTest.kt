package io.flimmer.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class DeviceTest {
    @Test fun normalize() {
        assertEquals("http://192.168.1.5:8096", normalizeServer(" 192.168.1.5 "))
        assertEquals("http://nas:9000", normalizeServer("nas:9000/"))
        assertEquals("https://flimmer.example.de:8096", normalizeServer("https://flimmer.example.de"))
    }

    @Test fun links() {
        assertEquals(FlimmerLink("https://ab12.flimmer.direct", invite = "tOk-_9"), parseLink("https://ab12.flimmer.direct/einladung#tOk-_9"))
        assertEquals(FlimmerLink("http://192.168.55.190:8096", pairCode = "123456"), parseLink("http://192.168.55.190:8096/#/koppeln/123456"))
        assertEquals(FlimmerLink("http://192.168.1.5:8096"), parseLink("192.168.1.5")) // Adresse wie bisher
        assertNull(parseLink("https://example.org/koppeln/12").pairCode)
    }

    @Test fun ssdpLocation() {
        val r = "HTTP/1.1 200 OK\r\nST: urn:flimmer-media:service:flimmer:1\r\nLOCATION: http://192.168.1.5:8096/\r\n\r\n"
        assertEquals("http://192.168.1.5:8096", parseLocation(r))
        assertNull(parseLocation("HTTP/1.1 200 OK\r\n\r\n"))
    }
}
