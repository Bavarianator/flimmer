package io.flimmer.app.gemeinsam

import org.junit.Assert.assertEquals
import org.junit.Test

class LobbyTest {
    @Test fun codeAusCodeOderLink() {
        assertEquals("3f9a0c21b7e4", codeAus("3F9A 0C21 B7E4"))
        assertEquals("3f9a0c21b7e4", codeAus("3f9a-0c21-b7e4"))
        assertEquals("3f9a0c21b7e4", codeAus("http://192.168.1.5:8096/party/3f9a0c21b7e4"))
        assertEquals("", codeAus("3F9A 0C21"))
        assertEquals("", codeAus("Hallo Welt"))
    }
}
