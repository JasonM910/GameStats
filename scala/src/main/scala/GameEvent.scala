import upickle.default._
import java.time.{Instant, OffsetDateTime}

// Representa un evento individual dentro del flujo de partidas
case class GameEvent(
                      eventId: String, // Identificador único del evento.
                      timestamp: String, // Fecha y hora del evento en formato ISO-8601.
                      gameId: String, // Juego al que pertenece el evento.
                      gameVersion: String, // Versión del juego.
                      matchId: String, // Identificador de la partida.
                      playerId: String, // Jugador asociado al evento.
                      `type`: String, // Comillas invertidas porque 'type' es palabra reservada en Scala.
                      data: ujson.Value // Campo 'data' dinámico: cada juego define sus propias claves.
                    ) {
  def instant: Instant = OffsetDateTime.parse(timestamp).toInstant
}

object GameEvent {
  implicit val rw: ReadWriter[GameEvent] = macroRW
}

case class MatchData( // Representa una partida con sus datos principales y su lista de eventos.
                      gameId: String, // Juego al que pertenece la partida.
                      matchId: String, // Identificador de la partida.
                      players: List[String], // Lista de jugadores involucrados.
                      events: List[GameEvent] // Eventos ordenados o no del flujo de la partida.
                    )

object MatchData {
  implicit val rw: ReadWriter[MatchData] = macroRW
}

case class PartidasFile(
                         schemaVersion: Int,
                         matches: List[MatchData]
                       ) {
  def allEvents: List[GameEvent] = matches.flatMap(_.events)
}

object PartidasFile {
  implicit val rw: ReadWriter[PartidasFile] = macroRW
}