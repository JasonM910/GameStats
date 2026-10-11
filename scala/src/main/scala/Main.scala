import upickle.default._

object Main {
  private val rutasPorDefecto = List("../salida/partidas.json", "salida/partidas.json")

  private def resolverEntrada(args: Array[String]): os.Path = {
    val explicita = args.headOption.orElse(sys.env.get("GAMESTATS_INPUT"))
    val rutas = (explicita.toList ++ rutasPorDefecto).map(t => os.Path(t, os.pwd))
    rutas.find(os.exists).getOrElse(rutas.head)
  }

  private def resolverSalida(entrada: os.Path): os.Path =
    sys.env.get("GAMESTATS_OUTPUT")
      .map(t => os.Path(t, os.pwd))
      .getOrElse(entrada / os.up / "resultados.json")

  def main(args: Array[String]): Unit = {
    val entrada = resolverEntrada(args)

    EventLoader.load(entrada) match {
      case Left(error) =>
        Console.err.println(s"Error: $error")
        sys.exit(1)

      case Right(archivo) =>
        val resultado = Analyzer.analizar(archivo)

        println("--- Estadisticas por Jugador ---")
        resultado.players.foreach { p =>
          println(f"  * ${p.playerId}: ${p.matchesPlayed} partidas, ${p.wins} victorias, ${p.losses} derrotas (${p.winrate}%.1f%%)") // Imprime cada jugador con sus estadísticas.
        }

        println("\n--- Estadisticas por Partida ---")
        resultado.matches.foreach { m =>
          println(s"  * ${m.matchId} (${m.gameId}): ${m.result}, ${m.durationSeconds}s, ${m.eventCount} eventos") // Muestra resultado y duración de cada partida.
        }

        println("\n--- Resumen por Juego ---")
        resultado.games.foreach { g =>
          println(f"  * ${g.gameId}: ${g.matches} partidas, ${g.eventCount} eventos, duracion promedio ${g.avgDurationSeconds}%.1f s") // Imprime datos de cada juego.
        }

        println("\n--- Metricas combinadas ---")
        resultado.metrics.foreach { m =>
          println(f"  * ${m.matchId} ${m.playerId} ${m.metric}: ${m.value}%.1f") // Muestra cada métrica calculada.
        }

        println("\n--- Patrones detectados ---")
        if (resultado.patterns.isEmpty) println("  (ninguno)") // Indica si no se detectaron patrones.
        resultado.patterns.foreach { d =>
          println(s"  * ${d.matchId} ${d.playerId} ${d.pattern}: ${d.description} [${d.startEventId} -> ${d.endEventId}]") // Muestra cada patrón y sus eventos de inicio/fin.
        }

        val salida = resolverSalida(entrada)
        os.write.over(salida, write(resultado, indent = 2), createFolders = true)
    }
  }
}