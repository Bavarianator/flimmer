package io.flimmer.app

import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class ApiClientTest {
    private val server = MockWebServer()
    private lateinit var api: ApiClient
    private val profile = Profile("Test", listOf("mkv"), listOf("h264"), listOf("aac"))

    @Before fun start() {
        server.start()
        api = ApiClient(server.url("/").toString())
    }

    @After fun stop() = server.close()

    private fun json(body: String, code: Int = 200) = MockResponse.Builder().code(code).body(body).addHeader("Content-Type", "application/json").build()

    @Test fun loginSetsBearerForLaterCalls() = runTest {
        server.enqueue(json("""{"id":"u1","name":"Anna","admin":false,"token":"t0k"}"""))
        server.enqueue(json("""{"id":"u1","name":"Anna","color":120}"""))
        api.login("u1", null, "Android TV")
        val login = server.takeRequest()
        assertEquals("/api/login", login.url.encodedPath)
        assertTrue(login.body!!.utf8().contains("\"device\":\"Android TV\""))
        assertEquals("Anna", api.me().name)
        assertEquals("Bearer t0k", server.takeRequest().headers["Authorization"])
    }

    @Test fun libraryIgnoresUnknownFieldsAndNull() = runTest {
        server.enqueue(json("""[{"id":"a","title":"Heat","year":1995,"light":"yellow","neuesFeld":1,"meta":{"title":"Heat","overview":"LA"}}]"""))
        server.enqueue(json("null"))
        val items = api.library(profile)
        assertEquals("Heat", items[0].displayTitle)
        assertEquals("yellow", items[0].light)
        assertEquals(0, api.library(profile).size)
    }

    @Test fun playSendsProfileWithTrackWish() = runTest {
        server.enqueue(json("""{"method":"direct-play","url":"/api/m/x/items/a/file","resume":42.5,"prefs":{"audio":"eng","subtitle":"off"}}"""))
        val plan = api.play("a", profile.copy(audioLang = "eng"))
        val req = server.takeRequest()
        assertEquals("/api/items/a/play", req.url.encodedPath)
        assertTrue(req.body!!.utf8().contains("\"audioLang\":\"eng\""))
        assertEquals(42.5, plan.resume, 0.0)
        assertEquals(server.url("/api/m/x/items/a/file").toString(), api.abs(plan.url))
    }

    @Test fun pairPollWaitsThenReturnsToken() = runTest {
        server.enqueue(MockResponse.Builder().code(202).build())
        server.enqueue(json("""{"token":"tv-token"}"""))
        assertNull(api.pairPoll("123456", "s3cret"))
        assertEquals("tv-token", api.pairPoll("123456", "s3cret"))
        assertEquals("tv-token", api.token)
        assertEquals("s3cret", server.takeRequest().url.queryParameter("secret"))
    }

    @Test(expected = ApiException::class) fun unauthorizedThrows() = runTest {
        server.enqueue(json("""{"error":"login"}""", 401))
        api.me()
    }
}
