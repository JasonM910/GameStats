import upickle.default._
import scala.util.{Failure, Success, Try}

object EventLoader {

  val SupportedSchemaVersions: Set[Int] = Set(1)

  def load(path: os.Path): Either[String, PartidasFile] =
    for {
      content <- readFile(path)
      parsed  <- parse(content)
      versioned <- validateVersion(parsed)
      valid   <- validateTimestamps(versioned)
    } yield valid

  private def readFile(path: os.Path): Either[String, String] =
    if (!os.exists(path)) Left(s"No existe el archivo: $path")
    else Try(os.read(path)) match {
      case Success(text) => Right(text)
      case Failure(e)    => Left(s"No se pudo leer $path: ${e.getMessage}")
    }

  private def parse(content: String): Either[String, PartidasFile] =
    Try(read[PartidasFile](content)) match {
      case Success(file) => Right(file)
      case Failure(e)    => Left(s"JSON con formato inválido: ${e.getMessage}")
    }

  private def validateVersion(file: PartidasFile): Either[String, PartidasFile] =
    if (SupportedSchemaVersions.contains(file.schemaVersion)) Right(file)
    else Left(
      s"schemaVersion ${file.schemaVersion} no soportada. " +
        s"Versiones soportadas: ${SupportedSchemaVersions.toList.sorted.mkString(", ")}"
    )

  private def validateTimestamps(file: PartidasFile): Either[String, PartidasFile] =
    file.allEvents.find(e => Try(e.instant).isFailure) match {
      case None      => Right(file)
      case Some(bad) => Left(s"Timestamp inválido en el evento ${bad.eventId}: '${bad.timestamp}'")
    }
}