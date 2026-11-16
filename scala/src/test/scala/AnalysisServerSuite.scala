import munit.FunSuite
import java.net.URI
import java.net.http.{HttpClient, HttpRequest, HttpResponse}
import upickle.default._

class AnalysisServerSuite extends FunSuite {
  test("API y archivos reutilizan el mismo análisis, incluyendo métricas y patrones") {
    val input = """{"schemaVersion":1,"matches":[{"gameId":"PACMAN","matchId":"P1","players":["A"],"events":[
      {"eventId":"E1","timestamp":"2026-11-16T12:00:00Z","gameId":"PACMAN","gameVersion":"1","matchId":"P1","playerId":"A","type":"MATCH_STARTED","data":{}},
      {"eventId":"E2","timestamp":"2026-11-16T12:00:01Z","gameId":"PACMAN","gameVersion":"1","matchId":"P1","playerId":"A","type":"PELLET_EATEN","data":{"points":10}},
      {"eventId":"E3","timestamp":"2026-11-16T12:00:02Z","gameId":"PACMAN","gameVersion":"1","matchId":"P1","playerId":"A","type":"MATCH_FINISHED","data":{"result":"WIN","durationSeconds":2}}
    ]}]}"""
    val fromFile = EventLoader.load(os.temp(input, suffix = ".json")).map(Analyzer.analizar)
    assertEquals(AnalysisServer.analyze(input), fromFile)
    val server = AnalysisServer.start(0, "127.0.0.1")
    try {
      val request = HttpRequest.newBuilder(URI.create(s"http://127.0.0.1:${server.getAddress.getPort}/analyze"))
        .POST(HttpRequest.BodyPublishers.ofString(input)).build()
      val response = HttpClient.newHttpClient().send(request, HttpResponse.BodyHandlers.ofString())
      assertEquals(response.statusCode(), 200)
      assertEquals(read[AnalysisResult](response.body()), fromFile.toOption.get)
    } finally server.stop(0)
  }
  test("API rechaza JSON inválido y versiones desconocidas") {
    assert(AnalysisServer.analyze("no json").isLeft)
    assert(AnalysisServer.analyze("""{"schemaVersion":99,"matches":[]}""").isLeft)
  }
}
