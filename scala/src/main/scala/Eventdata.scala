object EventData {
  private def field(e: GameEvent, key: String): Option[ujson.Value] = e.data.objOpt.flatMap(_.get(key)) // Si data no es objeto o la clave no existe, devuelve None.

  def str(e: GameEvent, key: String): Option[String] = field(e, key).flatMap(_.strOpt)
  def num(e: GameEvent, key: String): Option[Double] = field(e, key).flatMap(_.numOpt)
  def bool(e: GameEvent, key: String): Option[Boolean] = field(e, key).flatMap(_.boolOpt)
}