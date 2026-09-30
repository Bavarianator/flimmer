package io.flimmer.app

// Reine Regeln für die Detailseiten (docs/umbau-jellyfin.md, „Volle Detailseiten ohne TMDB-Schlüssel“), ohne Android.

private val allerwelt = listOf(
    Regex("""(?i)^\s*(chapter|kapitel|scene|szene|chap|ch)\.?\s*\d+\s*\.?\s*$"""), // „Chapter 1“, „Kapitel 01“, „Chapter 1.“
    Regex("""^\s*\d+\s*\.?\s*$"""), // „01“
    Regex("""^\s*\d{1,2}(:\d{2}){1,2}([.,]\d+)?\s*$"""), // reine Zeitstempel „00:12:30.000“
)

/** Kapitelname für die Szenen-Reihe: Allerweltsnamen und leere werden zu „Szene N“ ([nr] ab 1). */
fun szenenName(name: String, nr: Int): String =
    if (name.isBlank() || allerwelt.any { it.matches(name) }) "Szene $nr" else name.trim()

/**
 * „Mehr wie dieses“, nie leer, solange es Kandidaten gibt: erst gleiche Genres (meiste Überschneidung, dann Bewertung),
 * dann gleiche Sammlung, dann gleiches Jahrzehnt, zuletzt die neuesten. [kandidaten] enthält den Titel selbst nicht mehr
 * (bei Serien: je andere Serie eine Folge). [key]: Schlüssel des Titels wie in Sammlungen.
 */
fun mehrWieDieses(genres: List<String>, jahr: Int?, key: String, kandidaten: List<Item>, sammlungen: List<Liste>?, max: Int = 16): List<Item> {
    val out = LinkedHashMap<String, Item>()
    fun add(l: List<Item>) = l.forEach { if (out.size < max) out.putIfAbsent(it.key, it) }
    add(kandidaten.map { it to it.meta?.genres.orEmpty().count { g -> g in genres } }.filter { it.second > 0 }
        .sortedWith(compareByDescending<Pair<Item, Int>> { it.second }.thenByDescending { it.first.meta?.rating ?: 0.0 }).map { it.first })
    val geschwister = sammlungen.orEmpty().filter { key in it.items }.flatMap { it.items }.toSet()
    add(kandidaten.filter { it.key in geschwister })
    val jahrzehnt = jahr?.div(10)
    if (jahrzehnt != null) add(kandidaten.filter { (it.meta?.year ?: it.year)?.div(10) == jahrzehnt })
    add(kandidaten.sortedByDescending { it.added })
    return out.values.toList()
}
