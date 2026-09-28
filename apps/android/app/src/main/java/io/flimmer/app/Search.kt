package io.flimmer.app

import java.text.Normalizer

// ponytail: Suche clientseitig über die geladene Bibliothek, bis der Server (FTS5) eine Route hat.

internal fun norm(s: String): String =
    Normalizer.normalize(s.lowercase().replace("ß", "ss"), Normalizer.Form.NFD)
        .replace(Regex("\\p{M}"), "") // ä → a, é → e
        .replace(Regex("[^a-z0-9]+"), " ").trim()

/** Levenshtein-Distanz mit frühem Abbruch – für kurze Wörter reicht das. */
private fun dist(a: String, b: String, max: Int): Int {
    if (kotlin.math.abs(a.length - b.length) > max) return max + 1
    var prev = IntArray(b.length + 1) { it }
    for (i in 1..a.length) {
        val cur = IntArray(b.length + 1)
        cur[0] = i
        for (j in 1..b.length) cur[j] = minOf(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + if (a[i - 1] == b[j - 1]) 0 else 1)
        prev = cur
    }
    return prev[b.length]
}

/** Ein Suchwort passt, wenn ein Titelwort damit beginnt oder höchstens einen Tippfehler (ab 4 Buchstaben zwei) entfernt ist. */
private fun wordMatches(q: String, words: List<String>) = words.any { w ->
    w.startsWith(q) || dist(q, w.take(q.length + 1), if (q.length >= 7) 2 else 1) <= (if (q.length >= 7) 2 else if (q.length >= 3) 1 else 0)
}

/** 0 = kein Treffer, höher = besser. */
fun score(query: String, title: String): Int {
    val q = norm(query).split(' ').filter { it.isNotEmpty() }
    if (q.isEmpty()) return 0
    val t = norm(title)
    val words = t.split(' ')
    if (!q.all { wordMatches(it, words) }) return 0
    return when {
        t == q.joinToString(" ") -> 3
        t.startsWith(q.joinToString(" ")) -> 2
        else -> 1
    }
}

fun search(query: String, items: List<Item>): List<Item> =
    items.map { it to maxOf(score(query, it.displayTitle), score(query, it.series ?: ""), score(query, it.title)) }
        .filter { it.second > 0 }
        .sortedByDescending { it.second }
        .map { it.first }
        .distinctBy { it.series ?: it.id } // Serien nur einmal
