object Analyzer {

  def estadisticasPorJugador(eventos: List[GameEvent]): List[PlayerStats] = {
    val finalesPorJugador = eventos.filter(_.`type` == "MATCH_FINISHED").groupBy(_.playerId)

    eventos.map(_.playerId).distinct.sorted.map { jugador =>
      val resultados = finalesPorJugador.getOrElse(jugador, Nil).flatMap(EventData.str(_, "result"))
      val total = resultados.length
      val victorias = resultados.count(_ == "WIN")
      val derrotas = resultados.count(_ == "LOSS")
      val empates = resultados.count(_ == "DRAW")
      val winrate = if (total == 0) 0.0 else victorias * 100.0 / total
      PlayerStats(jugador, total, victorias, derrotas, empates, winrate)
    }
  }

  def estadisticasPorPartida(partidas: List[MatchData]): List[MatchStats] =
    partidas.map { p =>
      val finales = p.events.filter(_.`type` == "MATCH_FINISHED")
      val porJugador = finales
        .flatMap(e => EventData.str(e, "result").map(PlayerResult(e.playerId, _)))
        .sortBy(_.playerId)
      // Un solo resultado compartido por todos, o "DECIDED" cuando hay ganador y perdedor
      val resumen = porJugador.map(_.result).distinct match {
        case Nil         => "UNFINISHED"
        case List(unico) => unico
        case _           => "DECIDED"
      }
      MatchStats(
        matchId = p.matchId,
        gameId = p.gameId,
        players = p.players,
        eventCount = p.events.length,
        result = resumen,
        durationSeconds = finales.flatMap(EventData.num(_, "durationSeconds")).headOption.map(_.toInt).getOrElse(0),
        results = porJugador
      )
    }

  def resumenPorJuego(partidas: List[MatchData]): List[GameSummary] =
    partidas.groupBy(_.gameId).toList.sortBy(_._1).map { case (juego, ps) =>
      val finalizadas = estadisticasPorPartida(ps).filter(_.result != "UNFINISHED")
      val promedio =
        if (finalizadas.isEmpty) 0.0
        else finalizadas.map(_.durationSeconds.toLong).reduce(_ + _).toDouble / finalizadas.length
      val totalEventos = ps.foldLeft(0)((total, partida) => total + partida.events.length)
      GameSummary(juego, ps.length, totalEventos, promedio)
    }

  // Un juego sin analizador registrado no produce métricas ni patrones específicos
  def metricasCombinadas(partidas: List[MatchData]): List[MetricResult] =
    partidas.flatMap(p => GameAnalyzers.byGame.get(p.gameId).toList.flatMap(_.metrics(p)))

  def detectarPatrones(partidas: List[MatchData]): List[PatternDetection] =
    partidas.flatMap(p => GameAnalyzers.byGame.get(p.gameId).toList.flatMap(_.patterns(p)))

  def analizar(archivo: PartidasFile): AnalysisResult =
    AnalysisResult(
      schemaVersion = archivo.schemaVersion,
      players = estadisticasPorJugador(archivo.allEvents),
      matches = estadisticasPorPartida(archivo.matches),
      games = resumenPorJuego(archivo.matches),
      metrics = metricasCombinadas(archivo.matches),
      patterns = detectarPatrones(archivo.matches)
    )
}