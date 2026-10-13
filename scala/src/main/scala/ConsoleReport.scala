object ConsoleReport {
  private def section(title: String): Unit = {
    println()
    println(title)
    println("-" * title.length)
  }

  private def counted(amount: Int, singular: String, plural: String): String =
    s"$amount ${if (amount == 1) singular else plural}"

  private def resultLabel(result: String): String = result match {
    case "WIN"        => "victoria"
    case "LOSS"       => "derrota"
    case "DRAW"       => "empate"
    case "DECIDED"    => "con ganador"
    case "UNFINISHED" => "sin terminar"
    case other        => other
  }

  private def metricLabel(metric: String): String = metric match {
    case "totalPoints"         => "Puntos totales"
    case "pointsPerMinute"     => "Puntos por minuto"
    case "flagAccuracyPercent" => "Precisión de banderas"
    case "cellsRevealed"       => "Casillas reveladas"
    case "movesPlayed"         => "Jugadas realizadas"
    case "avgThinkTimeSeconds" => "Tiempo medio de reflexión (s)"
    case "capturedMaterial"    => "Material capturado"
    case "checksGiven"         => "Jaques dados"
    case "pairsFound"          => "Parejas encontradas"
    case "matchAccuracyPercent" => "Precisión de parejas"
    case "timeUsedPercent"     => "Tiempo usado del límite"
    case other                 => other
  }

  private def metricValue(metric: MetricResult): String = metric.metric match {
    case "flagAccuracyPercent" | "matchAccuracyPercent" | "timeUsedPercent" => f"${metric.value}%.1f %%"
    case "totalPoints" | "cellsRevealed" | "movesPlayed" | "capturedMaterial" | "checksGiven" | "pairsFound" =>
      f"${metric.value}%.0f"
    case _ => f"${metric.value}%.1f"
  }

  def print(input: os.Path, output: os.Path, result: AnalysisResult): Unit = {
    println("=== GameStats | Scala: análisis de partidas ===")
    println(s"Entrada: $input")
    println(s"Resultados: $output")
    println(s"Resumen: ${result.matches.length} partidas | ${result.players.length} jugadores | " +
      s"${result.metrics.length} métricas | ${result.patterns.length} patrones")

    section("Partidas")
    if (result.matches.isEmpty) println("  (ninguna)")
    result.matches.foreach { m =>
      val duration = if (m.result == "UNFINISHED") "--" else s"${m.durationSeconds} s"
      val detalle =
        if (m.result == "DECIDED") m.results.map(r => s"${r.playerId} ${resultLabel(r.result)}").mkString(" (", ", ", ")")
        else ""
      println(s"  ${m.matchId} (${m.gameId})  ${resultLabel(m.result)}$detalle  |  $duration  |  " +
        counted(m.eventCount, "evento", "eventos"))
    }

    section("Jugadores")
    if (result.players.isEmpty) println("  (ninguno)")
    result.players.foreach { p =>
      println(s"  ${p.playerId}: ${counted(p.matchesPlayed, "partida", "partidas")} | " +
        s"${counted(p.wins, "victoria", "victorias")} | " +
        s"${counted(p.losses, "derrota", "derrotas")} | " +
        s"${counted(p.draws, "empate", "empates")} | " +
        f"${p.winrate}%.1f%% de victorias")
    }

    section("Resumen por juego")
    if (result.games.isEmpty) println("  (ninguno)")
    result.games.foreach { g =>
      println(s"  ${g.gameId}: ${counted(g.matches, "partida", "partidas")} | " +
        s"${counted(g.eventCount, "evento", "eventos")} | " +
        f"duración media ${g.avgDurationSeconds}%.1f s")
    }

    section("Métricas por partida y jugador")
    if (result.metrics.isEmpty) println("  (ninguna)")
    result.metrics.groupBy(m => (m.gameId, m.matchId, m.playerId)).toList
      .sortBy(_._1).foreach { case ((game, matchId, playerId), metrics) =>
        println(s"  $matchId ($game) - $playerId")
        metrics.foreach { m => println(s"    ${metricLabel(m.metric)}: ${metricValue(m)}") }
      }

    section("Patrones detectados")
    if (result.patterns.isEmpty) println("  (ninguno)")
    result.patterns.groupBy(p => (p.gameId, p.matchId, p.playerId)).toList
      .sortBy(_._1).foreach { case ((game, matchId, playerId), patterns) =>
        println(s"  $matchId ($game) - $playerId")
        patterns.foreach { p =>
          println(s"    ${p.pattern}: ${p.description}")
          println(s"      Eventos: ${p.startEventId} -> ${p.endEventId}")
        }
      }
  }
}