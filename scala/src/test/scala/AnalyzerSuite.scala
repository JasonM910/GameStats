import munit.FunSuite
import java.time.OffsetDateTime

class AnalyzerSuite extends FunSuite { // Suite de pruebas para verificar el analizador general.

  // ---------- Utilidades para construir datos de prueba ----------
  private val base = OffsetDateTime.parse("2026-10-08T17:00:00-06:00")
  private def t(seg: Int): String = base.plusSeconds(seg.toLong).toString

  private def ev(id: String, seg: Int, game: String, matchId: String, player: String,
                 tipo: String, data: ujson.Value = ujson.Obj()): GameEvent = // Crea un evento genérico para pruebas.
    GameEvent(id, t(seg), game, "1.0", matchId, player, tipo, data)

  private def partida(game: String, matchId: String, player: String, eventos: List[GameEvent]): MatchData = // Crea una partida con un jugador.
    MatchData(game, matchId, List(player), eventos)

  private def fin(id: String, seg: Int, game: String, matchId: String, player: String,
                  resultado: String, duracion: Int): GameEvent = // Crea un MATCH_FINISHED con resultado y duración.
    ev(id, seg, game, matchId, player, "MATCH_FINISHED",
      ujson.Obj("result" -> resultado, "durationSeconds" -> duracion))

  // ---------- Estadísticas por jugador ----------
  test("winrate: 2 victorias de 3 partidas = 66.67%") {
    val eventos = List(
      fin("E1", 10, "PACMAN", "P1", "A", "WIN", 10),
      fin("E2", 20, "PACMAN", "P2", "A", "LOSS", 10),
      fin("E3", 30, "PACMAN", "P3", "A", "WIN", 10)
    )
    val a = Analyzer.estadisticasPorJugador(eventos).find(_.playerId == "A").get
    assertEquals(a.matchesPlayed, 3)
    assertEquals(a.wins, 2)
    assertEquals(a.losses, 1)
    assertEqualsDouble(a.winrate, 66.67, 0.01)
  }

  test("jugador sin partidas finalizadas tiene winrate 0 (sin dividir entre cero)") {
    val eventos = List(ev("E1", 0, "PACMAN", "P1", "A", "MATCH_STARTED"))
    val a = Analyzer.estadisticasPorJugador(eventos).head
    assertEquals(a.matchesPlayed, 0)
    assertEqualsDouble(a.winrate, 0.0, 0.0001)
  }

  test("lista vacia de eventos produce resultados vacios") {
    assertEquals(Analyzer.estadisticasPorJugador(Nil), Nil)
    assertEquals(Analyzer.estadisticasPorPartida(Nil), Nil)
    assertEquals(Analyzer.resumenPorJuego(Nil), Nil)
    val vacio = Analyzer.analizar(PartidasFile(1, Nil))
    assertEquals(vacio.players, Nil)
    assertEquals(vacio.metrics, Nil)
    assertEquals(vacio.patterns, Nil)
  }

  // ---------- Entradas con errores ----------
  test("MATCH_FINISHED sin campo 'result' no lanza excepcion y no cuenta como partida") {
    val malo = ev("E1", 10, "PACMAN", "P1", "A", "MATCH_FINISHED", ujson.Obj("durationSeconds" -> 5))
    val a = Analyzer.estadisticasPorJugador(List(malo)).head
    assertEquals(a.matchesPlayed, 0)
  }

  test("'data' que no es un objeto JSON no rompe el analisis") {
    val malo = ev("E1", 10, "PACMAN", "P1", "A", "MATCH_FINISHED", ujson.Str("basura"))
    val p = partida("PACMAN", "P1", "A", List(malo))
    val m = Analyzer.estadisticasPorPartida(List(p)).head
    assertEquals(m.result, "UNFINISHED")
    assertEquals(m.durationSeconds, 0)
  }

  // ---------- Estadísticas por partida y por juego ----------
  test("partida sin MATCH_FINISHED se reporta como UNFINISHED con duracion 0") {
    val p = partida("PACMAN", "P1", "A", List(ev("E1", 0, "PACMAN", "P1", "A", "MATCH_STARTED")))
    val m = Analyzer.estadisticasPorPartida(List(p)).head
    assertEquals(m.result, "UNFINISHED")
    assertEquals(m.eventCount, 1)
  }

  test("duracion promedio por juego ignora partidas sin finalizar") {
    val p1 = partida("PACMAN", "P1", "A", List(fin("E1", 10, "PACMAN", "P1", "A", "WIN", 100)))
    val p2 = partida("PACMAN", "P2", "A", List(fin("E2", 20, "PACMAN", "P2", "A", "LOSS", 50)))
    val p3 = partida("PACMAN", "P3", "A", List(ev("E3", 30, "PACMAN", "P3", "A", "MATCH_STARTED")))
    val g = Analyzer.resumenPorJuego(List(p1, p2, p3)).head
    assertEquals(g.matches, 3)
    assertEqualsDouble(g.avgDurationSeconds, 75.0, 0.0001)
  }

  // ---------- Pac-Man: estadística combinada ----------
  test("Pac-Man: totalPoints suma 4 tipos de evento y pointsPerMinute usa la duracion") {
    val eventos = List(
      ev("E1", 1, "PACMAN", "P1", "A", "PELLET_EATEN", ujson.Obj("points" -> 10)),
      ev("E2", 2, "PACMAN", "P1", "A", "POWER_PELLET_EATEN", ujson.Obj("points" -> 50, "powerDurationSeconds" -> 10)),
      ev("E3", 3, "PACMAN", "P1", "A", "GHOST_EATEN", ujson.Obj("points" -> 200)),
      ev("E4", 4, "PACMAN", "P1", "A", "FRUIT_EATEN", ujson.Obj("points" -> 100)),
      fin("E5", 5, "PACMAN", "P1", "A", "WIN", 60)
    )
    val ms = PacmanAnalyzer.metrics(partida("PACMAN", "P1", "A", eventos))
    assertEqualsDouble(ms.find(_.metric == "totalPoints").get.value, 360.0, 0.0001)
    assertEqualsDouble(ms.find(_.metric == "pointsPerMinute").get.value, 360.0, 0.0001)
  }

  test("Pac-Man: sin MATCH_FINISHED no hay pointsPerMinute") {
    val eventos = List(ev("E1", 1, "PACMAN", "P1", "A", "PELLET_EATEN", ujson.Obj("points" -> 10)))
    val ms = PacmanAnalyzer.metrics(partida("PACMAN", "P1", "A", eventos))
    assertEquals(ms.map(_.metric), List("totalPoints"))
  }

  // ---------- Pac-Man: patrones ----------
  private def power(id: String, seg: Int) =
    ev(id, seg, "PACMAN", "P1", "A", "POWER_PELLET_EATEN", ujson.Obj("points" -> 50, "powerDurationSeconds" -> 10))
  private def ghost(id: String, seg: Int) =
    ev(id, seg, "PACMAN", "P1", "A", "GHOST_EATEN", ujson.Obj("points" -> 200))

  test("POWER_PELLET_COMBO: no se detecta con un solo fantasma o fuera de la ventana") {
    val unoSolo = partida("PACMAN", "P1", "A", List(power("E1", 0), ghost("E2", 3)))
    val fuera = partida("PACMAN", "P1", "A", List(power("E1", 0), ghost("E2", 3), ghost("E3", 15)))
    assertEquals(PacmanAnalyzer.patterns(unoSolo), Nil)
    assertEquals(PacmanAnalyzer.patterns(fuera), Nil)
  }

  test("POWER_PELLET_COMBO: 2 fantasmas dentro de la ventana del power pellet") {
    val p = partida("PACMAN", "P1", "A", List(power("E1", 0), ghost("E2", 3), ghost("E3", 6)))
    val ps = PacmanAnalyzer.patterns(p)
    assertEquals(ps.map(_.pattern), List("POWER_PELLET_COMBO"))
    assertEquals(ps.head.startEventId, "E1")
    assertEquals(ps.head.endEventId, "E3")
  }

  test("POWER_PELLET_COMBO: funciona aunque los eventos lleguen desordenados") {
    val p = partida("PACMAN", "P1", "A", List(ghost("E3", 6), power("E1", 0), ghost("E2", 3)))
    assertEquals(PacmanAnalyzer.patterns(p).map(_.pattern), List("POWER_PELLET_COMBO"))
  }

  private def vidaPerdida(id: String, seg: Int) =
    ev(id, seg, "PACMAN", "P1", "A", "LIFE_LOST", ujson.Obj("remainingLives" -> 2))

  test("COMEBACK: perder una vida y luego ganar") {
    val p = partida("PACMAN", "P1", "A", List(vidaPerdida("E1", 5), fin("E2", 10, "PACMAN", "P1", "A", "WIN", 10)))
    val ps = PacmanAnalyzer.patterns(p)
    assertEquals(ps.map(_.pattern), List("COMEBACK"))
    assertEquals((ps.head.startEventId, ps.head.endEventId), ("E1", "E2"))
  }

  test("COMEBACK: no se detecta si se pierde la partida o si nunca se perdio una vida") { // Verifica casos no válidos.
    val derrota = partida("PACMAN", "P1", "A", List(vidaPerdida("E1", 5), fin("E2", 10, "PACMAN", "P1", "A", "LOSS", 10)))
    val limpia = partida("PACMAN", "P1", "A", List(fin("E1", 10, "PACMAN", "P1", "A", "WIN", 10)))
    assertEquals(PacmanAnalyzer.patterns(derrota), Nil)
    assertEquals(PacmanAnalyzer.patterns(limpia), Nil)
  }

  test("COMEBACK: una victoria ANTERIOR a la perdida de vida no cuenta") {
    val p = partida("PACMAN", "P1", "A", List(fin("E1", 5, "PACMAN", "P1", "A", "WIN", 5), vidaPerdida("E2", 10)))
    assertEquals(PacmanAnalyzer.patterns(p), Nil)
  }

  // ---------- Buscaminas ----------
  private def bandera(id: String, seg: Int, row: Int, col: Int, correcta: Boolean) =
    ev(id, seg, "MINESWEEPER", "M1", "B", "FLAG_PLACED", ujson.Obj("row" -> row, "column" -> col, "correct" -> correcta))
  private def quitar(id: String, seg: Int, row: Int, col: Int) = // Helper para quitar banderas.
    ev(id, seg, "MINESWEEPER", "M1", "B", "FLAG_REMOVED", ujson.Obj("row" -> row, "column" -> col))

  test("Buscaminas: flagAccuracyPercent y cellsRevealed") {
    val eventos = List(
      bandera("E1", 1, 0, 0, correcta = false),
      bandera("E2", 2, 0, 1, correcta = true),
      bandera("E3", 3, 1, 1, correcta = true),
      ev("E4", 4, "MINESWEEPER", "M1", "B", "CELL_REVEALED", ujson.Obj("row" -> 1, "column" -> 0))
    )
    val ms = MinesweeperAnalyzer.metrics(partida("MINESWEEPER", "M1", "B", eventos))
    assertEqualsDouble(ms.find(_.metric == "flagAccuracyPercent").get.value, 66.67, 0.01)
    assertEqualsDouble(ms.find(_.metric == "cellsRevealed").get.value, 1.0, 0.0001)
  }

  test("Buscaminas: sin banderas no hay flagAccuracyPercent (evita dividir entre cero)") {
    val p = partida("MINESWEEPER", "M1", "B", List(ev("E1", 1, "MINESWEEPER", "M1", "B", "CELL_REVEALED")))
    assertEquals(MinesweeperAnalyzer.metrics(p).map(_.metric), List("cellsRevealed"))
  }

  test("FLAG_CORRECTED: bandera incorrecta retirada de la misma celda") {
    val p = partida("MINESWEEPER", "M1", "B", List(bandera("E1", 1, 0, 0, correcta = false), quitar("E2", 5, 0, 0)))
    val ps = MinesweeperAnalyzer.patterns(p)
    assertEquals(ps.map(_.pattern), List("FLAG_CORRECTED"))
    assertEquals((ps.head.startEventId, ps.head.endEventId), ("E1", "E2"))
  }

  test("FLAG_CORRECTED: no se detecta con otra celda, con bandera correcta o sin retirar") {
    val otraCelda = partida("MINESWEEPER", "M1", "B", List(bandera("E1", 1, 0, 0, correcta = false), quitar("E2", 5, 1, 1)))
    val correcta = partida("MINESWEEPER", "M1", "B", List(bandera("E1", 1, 0, 0, correcta = true), quitar("E2", 5, 0, 0)))
    val sinQuitar = partida("MINESWEEPER", "M1", "B", List(bandera("E1", 1, 0, 0, correcta = false)))
    assertEquals(MinesweeperAnalyzer.patterns(otraCelda), Nil)
    assertEquals(MinesweeperAnalyzer.patterns(correcta), Nil)
    assertEquals(MinesweeperAnalyzer.patterns(sinQuitar), Nil)
  }
  // ---------- Genericidad ----------
  test("un juego sin analizador registrado no produce metricas ni patrones, pero si estadisticas generales") {
    val p = partida("CHESS", "C1", "A", List(fin("E1", 10, "CHESS", "C1", "A", "WIN", 300)))
    val r = Analyzer.analizar(PartidasFile(1, List(p)))
    assertEquals(r.metrics, Nil)
    assertEquals(r.patterns, Nil)
    assertEquals(r.players.head.wins, 1)
    assertEquals(r.games.head.gameId, "CHESS")
  }
}