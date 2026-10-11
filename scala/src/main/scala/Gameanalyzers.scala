trait GameAnalyzer { // Define la interfaz común que debe implementar cualquier analizador de juego.
  def gameId: String
  def metrics(partida: MatchData): List[MetricResult]
  def patterns(partida: MatchData): List[PatternDetection]
  protected def ordenados(partida: MatchData): List[GameEvent] = partida.events.sortBy(_.instant)
}

object GameAnalyzers { // Contenedor de todos los analizadores disponibles en el sistema.
  val all: List[GameAnalyzer] = List(PacmanAnalyzer, MinesweeperAnalyzer)
  val byGame: Map[String, GameAnalyzer] = all.map(a => a.gameId -> a).toMap
}

// ---------------------------------------------------------------------------------------------
object PacmanAnalyzer extends GameAnalyzer {
  val gameId = "PACMAN"
  private val tiposConPuntos = Set("PELLET_EATEN", "POWER_PELLET_EATEN", "GHOST_EATEN", "FRUIT_EATEN")

  // Combina 4 tipos de evento (puntos) + MATCH_FINISHED (duración)
  def metrics(partida: MatchData): List[MetricResult] =
    ordenados(partida).groupBy(_.playerId).toList.sortBy(_._1).flatMap { case (jugador, eventos) =>
      val puntos = eventos.filter(e => tiposConPuntos(e.`type`)).flatMap(EventData.num(_, "points")).sum // Suma puntos de los eventos relevantes.
      val duracion = eventos.find(_.`type` == "MATCH_FINISHED").flatMap(EventData.num(_, "durationSeconds")) // Obtiene duración total de la partida.

      val total = MetricResult(gameId, partida.matchId, jugador, "totalPoints", puntos) // Métrica total de puntos del jugador.
      val porMinuto = duracion.filter(_ > 0).map { d =>
        MetricResult(gameId, partida.matchId, jugador, "pointsPerMinute", puntos / d * 60.0) // Puntos por minuto usando la duración final.
      }
      total :: porMinuto.toList
    }

  def patterns(partida: MatchData): List[PatternDetection] = { // Busca patrones detectables en las partidas de Pac-Man.
    val porJugador = ordenados(partida).groupBy(_.playerId).toList.sortBy(_._1) // Separa eventos por jugador.
    porJugador.flatMap { case (jugador, eventos) =>
      powerPelletCombo(partida, jugador, eventos) ++ comeback(partida, jugador, eventos) // Aplica los dos patrones posibles.
    }
  }

  // Patrón 1: POWER_PELLET_EATEN seguido de 2 o más GHOST_EATEN dentro de su powerDurationSeconds
  private def powerPelletCombo(partida: MatchData, jugador: String, eventos: List[GameEvent]): List[PatternDetection] =
    eventos.filter(_.`type` == "POWER_PELLET_EATEN").flatMap { power =>
      val fin = power.instant.plusSeconds(EventData.num(power, "powerDurationSeconds").getOrElse(0.0).toLong)
      val fantasmas = eventos.filter { g =>
        g.`type` == "GHOST_EATEN" && !g.instant.isBefore(power.instant) && !g.instant.isAfter(fin)
      }
      if (fantasmas.length >= 2)
        List(PatternDetection(gameId, partida.matchId, jugador, "POWER_PELLET_COMBO",
          s"${fantasmas.length} fantasmas comidos durante un power pellet",
          power.eventId, fantasmas.last.eventId))
      else Nil
    }

  // Patrón 2 (remontada): perder al menos una vida y, después, ganar la partida
  private def comeback(partida: MatchData, jugador: String, eventos: List[GameEvent]): List[PatternDetection] = // Detecta remontadas tras perder vidas.
    eventos.find(_.`type` == "LIFE_LOST").toList.flatMap { primeraPerdida =>
      val despues = eventos.dropWhile(_.eventId != primeraPerdida.eventId).drop(1) // Obtiene los eventos después de la primera pérdida.
      despues
        .find(e => e.`type` == "MATCH_FINISHED" && EventData.str(e, "result").contains("WIN")) // Busca una victoria después de la caída.
        .map { victoria =>
          val vidas = eventos.count(_.`type` == "LIFE_LOST") // Cuenta cuántas vidas perdió antes de ganar.
          PatternDetection(gameId, partida.matchId, jugador, "COMEBACK",
            s"Gano la partida despues de perder $vidas vida(s)",
            primeraPerdida.eventId, victoria.eventId) // Registra la remontada detectada.
        }
        .toList
    }
}

// ---------------------------------------------------------------------------------------------
object MinesweeperAnalyzer extends GameAnalyzer {
  val gameId = "MINESWEEPER"

  // Combina FLAG_PLACED (campo 'correct') y CELL_REVEALED
  def metrics(partida: MatchData): List[MetricResult] =
    ordenados(partida).groupBy(_.playerId).toList.sortBy(_._1).flatMap { case (jugador, eventos) =>
      val banderas = eventos.filter(_.`type` == "FLAG_PLACED").flatMap(EventData.bool(_, "correct")) // Obtiene si cada bandera fue correcta o no.
      val precision = if (banderas.isEmpty) Nil
      else List(MetricResult(gameId, partida.matchId, jugador, "flagAccuracyPercent",
        banderas.count(identity) * 100.0 / banderas.length)) // Calcula precisión de banderas.
      val reveladas = MetricResult(gameId, partida.matchId, jugador, "cellsRevealed",
        eventos.count(_.`type` == "CELL_REVEALED").toDouble) // Cuenta celdas reveladas.
      precision :+ reveladas
    }

  // Patrón: bandera incorrecta colocada y luego retirada de la MISMA celda (error corregido)
  def patterns(partida: MatchData): List[PatternDetection] = { // Detecta correcciones de banderas mal ubicadas.
    def celda(e: GameEvent): (Option[Double], Option[Double]) = // Extrae fila y columna de un evento de celda.
      (EventData.num(e, "row"), EventData.num(e, "column"))

    ordenados(partida).groupBy(_.playerId).toList.sortBy(_._1).flatMap { case (jugador, eventos) =>
      eventos
        .filter(e => e.`type` == "FLAG_PLACED" && EventData.bool(e, "correct").contains(false)) // Solo considera banderas incorrectas.
        .flatMap { bandera =>
          val c = celda(bandera) // Guarda las coordenadas de la bandera mala.
          val despues = eventos.dropWhile(_.eventId != bandera.eventId).drop(1) // Eventos después de la bandera incorrecta.
          despues
            .find(r => r.`type` == "FLAG_REMOVED" && c._1.isDefined && c._2.isDefined && celda(r) == c) // Busca retiro de la misma celda.
            .map(r => PatternDetection(gameId, partida.matchId, jugador, "FLAG_CORRECTED",
              s"Bandera incorrecta en (${c._1.get.toInt},${c._2.get.toInt}) retirada despues",
              bandera.eventId, r.eventId)) // Registra el patrón de error corregido.
            .toList
        }
    }
  }
}