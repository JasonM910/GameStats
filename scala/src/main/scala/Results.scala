import upickle.default._

// Estadística agregada por jugador
case class PlayerStats(
                        playerId: String, // Identificador del jugador.
                        matchesPlayed: Int, // Partidas finalizadas jugadas por este jugador.
                        wins: Int, // Número de victorias.
                        losses: Int, // Número de derrotas.
                        draws: Int, // Número de empates.
                        winrate: Double // porcentaje de victorias sobre partidas finalizadas, 0-100
                      )
object PlayerStats { implicit val rw: ReadWriter[PlayerStats] = macroRW }

// Resultado de un jugador en una partida
case class PlayerResult(playerId: String, result: String)
object PlayerResult { implicit val rw: ReadWriter[PlayerResult] = macroRW }

// Estadística agregada por partida.
// result: el resultado común si todos los jugadores coinciden ("WIN", "LOSS", "DRAW"), "DECIDED" si hubo
// ganador y perdedor, o "UNFINISHED" si no hay MATCH_FINISHED. durationSeconds = 0 si no terminó.
case class MatchStats(
                       matchId: String, // Identificador de la partida.
                       gameId: String, // Juego al que pertenece.
                       players: List[String], // Jugadores participantes.
                       eventCount: Int, // Número de eventos en la partida.
                       result: String, // Resultado de la partida.
                       durationSeconds: Int, // Duración de la partida en segundos.
                       results: List[PlayerResult] // Resultado de cada jugador.
                     )
object MatchStats { implicit val rw: ReadWriter[MatchStats] = macroRW }

// Resumen por juego
case class GameSummary(
                        gameId: String, // Juego analizado.
                        matches: Int, // Cantidad de partidas del juego.
                        eventCount: Int, // Total de eventos del juego.
                        avgDurationSeconds: Double // promedio sobre partidas finalizadas; 0.0 si no hay
                      )
object GameSummary { implicit val rw: ReadWriter[GameSummary] = macroRW }

// Documento completo que se escribe como JSON
case class AnalysisResult(
                           schemaVersion: Int, // Versión del esquema de salida.
                           players: List[PlayerStats], // Estadísticas por jugador.
                           matches: List[MatchStats], // Estadísticas por partida.
                           games: List[GameSummary], // Resúmenes por juego.
                           metrics: List[MetricResult], // Métricas combinadas.
                           patterns: List[PatternDetection] // Patrones detectados.
                         )
object AnalysisResult { implicit val rw: ReadWriter[AnalysisResult] = macroRW }

// Métrica combinada de un jugador en una partida (genérica)
case class MetricResult( // Representa una métrica generada para un jugador y una partida.
                         gameId: String, // Juego.
                         matchId: String, // Partida.
                         playerId: String, // Jugador.
                         metric: String, // Nombre de la métrica.
                         value: Double // Valor numérico calculado.
                       )
object MetricResult { implicit val rw: ReadWriter[MetricResult] = macroRW }

// Patrón detectado a partir de una secuencia de eventos
case class PatternDetection(
                             gameId: String, // Juego.
                             matchId: String, // Partida.
                             playerId: String, // Jugador.
                             pattern: String, // Nombre del patrón.
                             description: String, // Explicación legible del patrón.
                             startEventId: String, // ID del evento inicial del patrón.
                             endEventId: String // ID del evento final del patrón.
                           )
object PatternDetection { implicit val rw: ReadWriter[PatternDetection] = macroRW }