package io.flimmer.app

import org.junit.Assert.assertEquals
import org.junit.Test

/** Die vier Beispiele aus docs/umbau-jellyfin.md, „Bildanpassung im Player“. */
class BildTest {
    private val handy = 2400.0 to 1080.0 // 20:9
    private val tv = 1920.0 to 1080.0 // 16:9

    private fun auto(vw: Double, vh: Double, crop: Crop?, screen: Pair<Double, Double>) = bildFlaeche(BildModus.Auto, vw, vh, crop, screen.first, screen.second)

    @Test fun cinemascopeAufHandyFuellt() {
        // 16:9-Datei mit eingebrannten Balken, sichtbar 2,39:1 → füllt, die Mitte bleibt in der Mitte
        val ch = 1920 / 2.39 / 1080
        val f = auto(1920.0, 1080.0, Crop(0.0, (1 - ch) / 2, 1.0, ch), handy)
        assertEquals(1080.0, ch * f.hoehe, 0.5) // der sichtbare Teil deckt die ganze Höhe
        assertEquals(0.0, f.dy, 0.5)
    }

    @Test fun breitwandAuf16zu9Fuellt() {
        val f = auto(1998.0, 1080.0, null, tv) // 1,85:1
        assertEquals(1080.0, f.hoehe, 0.5)
        assertEquals(1998.0, f.breite, 0.5) // ragt links und rechts heraus
    }

    @Test fun cinemascopeAufTvPasstEin() {
        val f = auto(2390.0, 1000.0, null, tv)
        assertEquals(1920.0, f.breite, 0.5)
        assertEquals(1920 / 2.39, f.hoehe, 0.5) // Balken oben und unten
    }

    @Test fun sechzehnNeuntelAufHandyPasstEin() {
        val f = auto(1920.0, 1080.0, null, handy)
        assertEquals(1080.0, f.hoehe, 0.5)
        assertEquals(1920.0, f.breite, 0.5) // Balken links und rechts
    }

    @Test fun cropAussermittigWirdZentriert() {
        // Balken nur unten: R liegt oben, also rutscht das Video nach unten
        val f = bildFlaeche(BildModus.Fuellen, 1000.0, 1000.0, Crop(0.0, 0.0, 1.0, 0.5), 2000.0, 1000.0)
        assertEquals(2000.0, f.breite, 0.5)
        assertEquals(500.0, f.dy, 0.5)
    }
}
