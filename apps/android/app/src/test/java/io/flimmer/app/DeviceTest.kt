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

    @Test fun ssdpLocation() {
        val r = "HTTP/1.1 200 OK\r\nST: urn:flimmer-media:service:flimmer:1\r\nLOCATION: http://192.168.1.5:8096/\r\n\r\n"
        assertEquals("http://192.168.1.5:8096", parseLocation(r))
        assertNull(parseLocation("HTTP/1.1 200 OK\r\n\r\n"))
    }
}
