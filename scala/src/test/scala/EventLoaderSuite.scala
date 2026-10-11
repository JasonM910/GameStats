import munit.FunSuite

class EventLoaderSuite extends FunSuite {

  private def json(version: Int = 1, timestamp: String = "2026-10-08T17:00:00-06:00"): String =
    s"""{
       |  "schemaVersion": $version,
       |  "matches": [{
       |    "gameId": "PACMAN", "matchId": "P1", "players": ["A"],
       |    "events": [{
       |      "eventId": "E1", "timestamp": "$timestamp", "gameId": "PACMAN",
       |      "gameVersion": "1.0", "matchId": "P1", "playerId": "A",
       |      "type": "MATCH_STARTED", "data": {}
       |    }]
       |  }]
       |}""".stripMargin

  private def cargar(contenido: String): Either[String, PartidasFile] =
    EventLoader.load(os.temp(contenido, suffix = ".json"))

  test("archivo valido se carga con su schemaVersion y sus eventos") {
    val r = cargar(json())
    assert(r.isRight)
    val archivo = r.toOption.get
    assertEquals(archivo.schemaVersion, 1)
    assertEquals(archivo.matches.length, 1)
    assertEquals(archivo.allEvents.map(_.eventId), List("E1"))
  }

  test("schemaVersion no soportada devuelve Left") {
    val r = cargar(json(version = 99))
    assert(r.isLeft)
    assert(r.left.toOption.get.contains("99"))
  }

  test("JSON con formato invalido devuelve Left") {
    assert(cargar("esto no es json").isLeft)
    assert(cargar("""{"schemaVersion": 1}""").isLeft)
  }

  test("timestamp invalido devuelve Left indicando el evento") {
    val r = cargar(json(timestamp = "ayer por la tarde"))
    assert(r.isLeft)
    assert(r.left.toOption.get.contains("E1"))
  }

  test("archivo inexistente devuelve Left") {
    val r = EventLoader.load(os.pwd / "no-existe-este-archivo.json")
    assert(r.isLeft)
  }
}