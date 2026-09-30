package io.flimmer.app.admin

import io.flimmer.app.ApiClient
import io.flimmer.app.ApiException
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

// Beispiel-JSON aus docs/api-neu.md.
class AdminApiTest {
    private val server = MockWebServer()
    private lateinit var api: ApiClient

    @Before fun start() {
        server.start()
        api = ApiClient(server.url("/").toString(), "t0k")
    }

    @After fun stop() = server.close()

    private fun antwort(body: String, code: Int = 200) =
        server.enqueue(MockResponse.Builder().code(code).body(body).addHeader("Content-Type", "application/json").build())

    @Test fun overview() = runTest {
        antwort("""{"server":{"name":"NAS","version":"v0.4.0","os":"linux","arch":"amd64","uptime":86400,"started":"2026-09-28T18:00:00+02:00",
           "update":{"version":"v0.5.0","url":"https://github.com/Bavarianator/flimmer/releases/tag/v0.5.0"}},
 "library":{"movies":412,"series":37,"episodes":1840,"sizeBytes":8123456789012,"lastScan":"2026-09-29T17:45:00+02:00"},
 "disks":[{"path":"/srv/media","free":1200000000000,"total":4000000000000}],
 "sessions":[{"id":"anna|4b9b5d8402a0","user":"anna","userColor":120,"device":"Wohnzimmer-TV","client":"LG webOS",
  "itemId":"4b9b5d8402a0","title":"Heat","position":1204.5,"duration":10200,"paused":false,
  "method":"transcode","light":"yellow","reason":"Codec hevc10 nicht unterstützt","since":"2026-09-29T20:01:00+02:00"}],
 "activity":[{"time":"2026-09-29T20:01:00+02:00","kind":"play","user":"anna","text":"Spielt „Heat“"},
 {"time":"2026-09-29T19:00:00+02:00","kind":"scan","text":"Bibliothek gescannt: 2289 Titel"}]}""")
        val o = api.uebersicht()
        assertEquals("/api/admin/overview", server.takeRequest().url.encodedPath)
        assertEquals("NAS", o.server.name)
        assertEquals("v0.5.0", o.server.update?.version)
        assertEquals("1 Tag", seit(o.server.uptime))
        assertEquals(412, o.library.movies)
        assertEquals(8123456789012, o.library.sizeBytes)
        assertEquals("2,8 TB", groesse(o.disks[0].total - o.disks[0].free))
        val s = o.sessions.single()
        assertEquals(120, s.userColor)
        assertEquals("Wird umgewandelt", methode(s.method))
        assertEquals("Codec hevc10 nicht unterstützt", s.reason)
        assertEquals(listOf("play", "scan"), o.activity.map { it.kind })
        assertNull(o.activity[1].user)
    }

    @Test fun overviewMitNullListen() = runTest {
        antwort("""{"server":{"name":"NAS"},"library":{"movies":0},"disks":null,"sessions":null,"activity":null}""")
        val o = api.uebersicht()
        assertTrue(o.disks.isEmpty() && o.sessions.isEmpty() && o.activity.isEmpty())
        assertNull(o.server.update)
    }

    @Test fun tasks() = runTest {
        antwort("""[{"id":"meta","name":"Metadaten aktualisieren","group":"Bibliothek","description":"…","lastRun":"2026-09-29T03:00:00+02:00",
  "lastResult":"ok","duration":412.3,"running":true,"progress":0.42},
  {"id":"backup","name":"Sicherung erstellen","group":"Wartung","description":"","lastRun":"2026-09-29T03:00:00+02:00","lastResult":"error",
  "lastError":"Platte voll","running":false,"next":"2026-09-30T03:00:00+02:00"},
  {"id":"images","name":"Bild-Cache aufräumen","group":"Wartung","description":"","running":false}]""")
        val l = api.aufgaben()
        assertEquals("/api/tasks", server.takeRequest().url.encodedPath)
        assertEquals(3, l.size)
        assertEquals(0.42, l[0].progress!!, 1e-9)
        assertEquals("Läuft · 42 %", aufgabeStand(l[0]))
        val jetzt = zeit("2026-09-29T06:00:00+02:00")!!
        assertEquals("Fehlgeschlagen vor 3 Std. · Platte voll · Nächster Lauf 30.09.2026, 3:00", aufgabeStand(l[1], jetzt).replace(Regex("\\d+:00$"), "3:00"))
        assertEquals("Noch nie gelaufen", aufgabeStand(l[2]))
    }

    @Test fun tasksNullIstLeer() = runTest {
        antwort("null")
        assertTrue(api.aufgaben().isEmpty())
    }

    @Test fun devices() = runTest {
        antwort("""[{"id":"9f86d081884c7d65","userId":"anna","user":"anna","name":"Pixel 8","client":"Android-App",
                              "ip":"192.168.1.40","lastSeen":"2026-09-29T19:00:00+02:00","current":true},
                   {"id":"a1","userId":"ben","user":"ben","name":"Wohnzimmer-TV","lastSeen":"2026-09-29T18:00:00+02:00"}]""")
        val l = api.geraete()
        assertEquals("/api/devices", server.takeRequest().url.encodedPath)
        assertEquals("Pixel 8", l[0].name)
        assertTrue(l[0].current)
        assertEquals("192.168.1.40", l[0].ip)
        assertNull(l[1].client)
        assertFalse(l[1].current)
    }

    @Test fun logs() = runTest {
        antwort("""[{"time":"2026-09-29T20:01:00.123+02:00","level":"error","msg":"Aufgabe fehlgeschlagen","attrs":{"aufgabe":"Sicherung erstellen","fehler":"…"}},
 {"time":"2026-09-29T20:00:59+02:00","level":"info","msg":"play \"Heat\" für anna: transcode ([…])"}]""")
        val l = api.protokoll("info")
        assertEquals("level=info&limit=300", server.takeRequest().url.encodedQuery)
        assertEquals(2, l.size)
        assertTrue(logText(l[0]).matches(Regex("""\d\d:01 \[ERR] Aufgabe fehlgeschlagen \{"aufgabe":"Sicherung erstellen","fehler":"…"}""")))
        assertTrue(logText(l[1]).endsWith("[INF] play \"Heat\" für anna: transcode ([…])"))
        assertNull(l[1].attrs)
    }

    @Test fun liveTVVorlagen() {
        val l = json.decodeFromString<List<LiveTVVorlage>>("""[{"id":"frei","source":"flimmer:freie-sender","epg":"https://epgshare01.online/epgshare01/epg_ripper_DE1.xml.gz","channels":18,"active":false},
   {"id":"fritz","source":"http://fritz.box/dvb/m3u/tvhd.m3u","epg":"…","channels":0,"active":false}]""")
        assertEquals(18, l[0].channels)
        assertEquals("flimmer:freie-sender", l[0].source)
        assertEquals(0, l[1].channels)
    }

    @Test fun fehlendeRouteIst404() = runTest {
        antwort("404 page not found", 404)
        val e = runCatching { api.protokoll("") }.exceptionOrNull()
        assertTrue(e is ApiException && fehlt(e))
    }

    @Test fun zeiten() {
        val t = zeit("2026-09-29T20:01:00.123456789+02:00")!!
        assertEquals(zeit("2026-09-29T18:01:00Z"), t)
        assertNull(zeit("0001-01-01T00:00:00Z"))
        assertEquals("noch nie", vor(null))
        assertEquals("vor 26 Min.", vor("2026-09-29T20:01:00+02:00", t + 26 * 60_000))
        assertEquals("1 Std. 12 Min.", dauer(4320.0))
        assertEquals("34 Min.", dauer(2040.0))
    }

    @Test fun metaAenderungSchicktNullFuerAlter() = runTest {
        server.enqueue(MockResponse.Builder().code(200).body("{}").addHeader("Content-Type", "application/json").build())
        val d = MDetails(id = "x", title = "Heat", meta = MMeta(title = "Heat", rating = 8.3, age = 16), locked = listOf("title"))
        val f = formAus(d)
        assertEquals("8,3", f.rating)
        api.metaSpeichern("x", f.copy(age = -1, rating = "7,5").aenderung())
        val body = server.takeRequest().body!!.utf8()
        assertTrue(body, body.contains("\"age\":null"))
        assertTrue(body, body.contains("\"rating\":7.5"))
        assertTrue(body, body.contains("\"locked\":[\"title\"]"))
    }
}
