package io.flimmer.app

import kotlin.math.max
import kotlin.math.min

// Bildanpassung im Player (docs/umbau-jellyfin.md, „Bildanpassung im Player“). Für alle Clients gleich gerechnet.

enum class BildModus(val label: String) { Auto("Automatisch"), Einpassen("Einpassen"), Fuellen("Füllen"), Strecken("Strecken") }

/**
 * Größe der ganzen Video-Fläche und Verschiebung ihrer Mitte gegenüber der Bildschirmmitte, in Bildschirm-Pixeln.
 * Was über den Bildschirm hinausragt, schneidet der Rahmen ab.
 */
data class Flaeche(val breite: Double, val hoehe: Double, val dx: Double, val dy: Double)

/** Automatisch füllt aus, wenn dabei höchstens so viel vom Bild verloren geht. */
const val AUTO_VERLUST = 0.12

/**
 * [vw]×[vh]: Video in Anzeige-Pixeln (Breite schon mit dem Pixel-Seitenverhältnis multipliziert).
 * [crop]: eingebrannte Balken weg, als Anteile 0..1; Einpassen ignoriert ihn. [w]×[h]: Bildschirm.
 */
fun bildFlaeche(modus: BildModus, vw: Double, vh: Double, crop: Crop?, w: Double, h: Double): Flaeche {
    val r = crop?.takeIf { modus != BildModus.Einpassen && it.w > 0 && it.h > 0 } ?: Crop(0.0, 0.0, 1.0, 1.0)
    val rw = r.w * vw
    val rh = r.h * vh
    val fuellen = max(w / rw, h / rh)
    val einpassen = min(w / rw, h / rh)
    val (sx, sy) = when (modus) {
        BildModus.Strecken -> w / rw to h / rh
        BildModus.Fuellen -> fuellen to fuellen
        BildModus.Einpassen -> einpassen to einpassen
        // sichtbarer Anteil von R beim Füllen = (w·h) / (s²·rw·rh)
        BildModus.Auto -> (if (1 - (w * h) / (fuellen * fuellen * rw * rh) <= AUTO_VERLUST) fuellen else einpassen).let { it to it }
    }
    // Mitte von R in die Bildschirmmitte: Abstand der R-Mitte zur Video-Mitte, skaliert und umgekehrt
    val cx = (r.x + r.w / 2 - 0.5) * vw
    val cy = (r.y + r.h / 2 - 0.5) * vh
    return Flaeche(vw * sx, vh * sy, -cx * sx, -cy * sy)
}
