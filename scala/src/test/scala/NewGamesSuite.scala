import munit.FunSuite
import java.time.OffsetDateTime

class NewGamesSuite extends FunSuite {
  private val base = OffsetDateTime.parse("2026-10-08T17:00:00-06:00")

  private def event(id: String, second: Int, game: String, matchId: String, player: String,
                    kind: String, data: ujson.Value = ujson.Obj()): GameEvent =
    GameEvent(id, base.plusSeconds(second.toLong).toString, game, "1.0", matchId, player, kind, data)

  private def finish(id: String, second: Int, game: String, matchId: String, player: String,
                     result: String, duration: Int): GameEvent =
    event(id, second, game, matchId, player, "MATCH_FINISHED",
      ujson.Obj("result" -> result, "durationSeconds" -> duration))

  test("ajedrez: calcula métricas y resultado individual de ambos jugadores") {
    val game = "CHESS"
    val matchId = "C1"
    val events = List(
      event("E5", 4, game, matchId, "A", "CHECKMATE", ujson.Obj("ply" -> 7)),
      finish("E7", 5, game, matchId, "B", "LOSS", 5),
      event("E3", 2, game, matchId, "A", "PIECE_CAPTURED", ujson.Obj("ply" -> 7, "capturedPiece" -> "QUEEN")),
      event("E1", 1, game, matchId, "A", "MOVE_PLAYED", ujson.Obj("ply" -> 7, "piece" -> "QUEEN", "from" -> "d1", "to" -> "d8", "thinkTimeSeconds" -> 8)),
      event("E4", 3, game, matchId, "A", "CHECK_GIVEN", ujson.Obj("ply" -> 7)),
      event("E2", 1, game, matchId, "B", "MOVE_PLAYED", ujson.Obj("ply" -> 6, "piece" -> "KNIGHT", "from" -> "g8", "to" -> "f6", "thinkTimeSeconds" -> 6)),
      finish("E6", 5, game, matchId, "A", "WIN", 5)
    )
    val result = Analyzer.analizar(PartidasFile(1, List(MatchData(game, matchId, List("A", "B"), events))))
    assertEquals(result.matches.head.result, "DECIDED")
    assertEquals(result.matches.head.results, List(PlayerResult("A", "WIN"), PlayerResult("B", "LOSS")))
    assertEquals(result.players.map(p => (p.playerId, p.wins, p.losses)), List(("A", 1, 0), ("B", 0, 1)))
    val white = result.metrics.filter(_.playerId == "A").map(m => m.metric -> m.value).toMap
    assertEquals(white("movesPlayed"), 1.0)
    assertEquals(white("avgThinkTimeSeconds"), 8.0)
    assertEquals(white("capturedMaterial"), 9.0)
    assertEquals(white("checksGiven"), 1.0)
    assertEquals(result.patterns.map(p => (p.pattern, p.startEventId, p.endEventId)),
      List(("CAPTURE_CHECKMATE", "E3", "E5")))
  }

  test("ajedrez: las tablas cuentan para ambos y aparecen en el JSON") {
    val matchId = "C2"
    val events = List(
      finish("E1", 10, "CHESS", matchId, "A", "DRAW", 10),
      finish("E2", 10, "CHESS", matchId, "B", "DRAW", 10)
    )
    val result = Analyzer.analizar(PartidasFile(1, List(MatchData("CHESS", matchId, List("A", "B"), events))))
    assertEquals(result.matches.head.result, "DRAW")
    assert(result.players.forall(p => p.matchesPlayed == 1 && p.wins == 0 && p.losses == 0 && p.draws == 1))
    val json = ujson.read(upickle.default.write(result))
    assertEquals(json("matches")(0)("results").arr.length, 2)
    assertEquals(json("players")(0)("draws").num, 1.0)
  }

  test("una partida multijugador no termina hasta tener resultado de todos") {
    val matchId = "C3"
    val events = List(
      event("E1", 0, "CHESS", matchId, "A", "MATCH_STARTED", ujson.Obj("side" -> "WHITE")),
      event("E2", 0, "CHESS", matchId, "B", "MATCH_STARTED", ujson.Obj("side" -> "BLACK")),
      finish("E3", 10, "CHESS", matchId, "A", "WIN", 10)
    )
    val result = Analyzer.analizar(PartidasFile(1, List(MatchData("CHESS", matchId, List("A", "B"), events))))
    assertEquals(result.matches.head.result, "UNFINISHED")
    assertEquals(result.matches.head.durationSeconds, 0)
    assertEquals(result.matches.head.results, List(PlayerResult("A", "WIN")))
    assertEquals(result.games.head.avgDurationSeconds, 0.0)
    assert(result.players.forall(p => p.matchesPlayed == 0 && p.wins == 0 && p.losses == 0))
  }

  test("ajedrez: captura, jaque y mate deben ocurrir en ese orden") {
    val matchId = "C4"
    val checkBeforeCapture = MatchData("CHESS", matchId, List("A"), List(
      event("E1", 1, "CHESS", matchId, "A", "CHECK_GIVEN", ujson.Obj("ply" -> 7)),
      event("E2", 2, "CHESS", matchId, "A", "PIECE_CAPTURED", ujson.Obj("ply" -> 7, "capturedPiece" -> "PAWN")),
      event("E3", 3, "CHESS", matchId, "A", "CHECKMATE", ujson.Obj("ply" -> 7))
    ))
    val mateBeforeCheck = checkBeforeCapture.copy(events = List(
      event("E1", 1, "CHESS", matchId, "A", "PIECE_CAPTURED", ujson.Obj("ply" -> 7, "capturedPiece" -> "PAWN")),
      event("E2", 2, "CHESS", matchId, "A", "CHECKMATE", ujson.Obj("ply" -> 7)),
      event("E3", 3, "CHESS", matchId, "A", "CHECK_GIVEN", ujson.Obj("ply" -> 7))
    ))
    assertEquals(ChessAnalyzer.patterns(checkBeforeCapture), Nil)
    assertEquals(ChessAnalyzer.patterns(mateBeforeCheck), Nil)
  }

  test("memoria: calcula precisión y tiempo, detecta racha y carta recordada") {
    val matchId = "MEM1"
    val game = "MEMORY"
    val events = List(
      event("E4", 4, game, matchId, "A", "PAIR_MATCHED", ujson.Obj("firstCardId" -> "A1", "secondCardId" -> "A2", "turn" -> 2)),
      event("E1", 0, game, matchId, "A", "MATCH_STARTED", ujson.Obj("rows" -> 2, "columns" -> 2, "pairCount" -> 2, "timeLimitSeconds" -> 30)),
      event("E3", 2, game, matchId, "A", "PAIR_MISMATCHED", ujson.Obj("firstCardId" -> "A1", "secondCardId" -> "B1", "turn" -> 1)),
      event("E5", 5, game, matchId, "A", "PAIR_MATCHED", ujson.Obj("firstCardId" -> "B1", "secondCardId" -> "B2", "turn" -> 3)),
      finish("E6", 19, game, matchId, "A", "WIN", 19)
    )
    val result = Analyzer.analizar(PartidasFile(1, List(MatchData(game, matchId, List("A"), events))))
    val metrics = result.metrics.map(m => m.metric -> m.value).toMap
    assertEquals(metrics("pairsFound"), 2.0)
    assertEqualsDouble(metrics("matchAccuracyPercent"), 200.0 / 3, 0.0001)
    assertEqualsDouble(metrics("timeUsedPercent"), 190.0 / 3, 0.0001)
    assertEquals(result.patterns.map(_.pattern), List("MATCH_STREAK", "REMEMBERED_CARD", "REMEMBERED_CARD"))
  }

  test("memoria: sin intentos ni cierre no divide entre cero") {
    val matchId = "MEM2"
    val start = event("E1", 0, "MEMORY", matchId, "A", "MATCH_STARTED", ujson.Obj("rows" -> 2, "columns" -> 2, "pairCount" -> 2, "timeLimitSeconds" -> 30))
    val matchData = MatchData("MEMORY", matchId, List("A"), List(start))
    assertEquals(MemoryAnalyzer.metrics(matchData).map(_.metric), List("pairsFound"))
    assertEquals(MemoryAnalyzer.patterns(matchData), Nil)
  }
}
