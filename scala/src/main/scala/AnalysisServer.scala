import com.sun.net.httpserver.{HttpExchange, HttpServer}
import java.net.InetSocketAddress
import java.nio.charset.StandardCharsets.UTF_8
import java.util.concurrent.Executors
import scala.util.control.NonFatal
import upickle.default._

// Adaptador HTTP: utiliza exactamente el mismo lector y Analyzer que Main.
object AnalysisServer {
  def analyze(json: String): Either[String, AnalysisResult] =
    EventLoader.fromJson(json).map(Analyzer.analizar)

  def start(port: Int, host: String = "0.0.0.0"): HttpServer = {
    val server = HttpServer.create(new InetSocketAddress(host, port), 32)
    server.setExecutor(Executors.newFixedThreadPool(4, (runnable: Runnable) => {
      val thread = new Thread(runnable)
      thread.setDaemon(true)
      thread
    }))
    server.createContext("/", (exchange: HttpExchange) => {
      try {
        (exchange.getRequestMethod, exchange.getRequestURI.getPath) match {
          case ("GET", "/health") => respond(exchange, 200, """{"status":"ok"}""")
          case ("POST", "/analyze") =>
            val bytes = exchange.getRequestBody.readNBytes(32 * 1024 * 1024 + 1)
            if (bytes.length > 32 * 1024 * 1024) respond(exchange, 413, error("Documento demasiado grande"))
            else analyze(new String(bytes, UTF_8)) match {
              case Right(result) => respond(exchange, 200, write(result))
              case Left(message) => respond(exchange, 400, error(message))
            }
          case (_, "/analyze" | "/health") => respond(exchange, 405, error("Método incorrecto"))
          case _ => respond(exchange, 404, error("Endpoint inexistente"))
        }
      } catch {
        case NonFatal(_) => respond(exchange, 500, error("Error al analizar el documento"))
      } finally exchange.close()
    })
    server.start()
    server
  }

  private def error(message: String): String = ujson.Obj("error" -> message).render()
  private def respond(exchange: HttpExchange, status: Int, content: String): Unit = {
    val bytes = content.getBytes(UTF_8)
    exchange.getResponseHeaders.set("Content-Type", "application/json; charset=utf-8")
    exchange.sendResponseHeaders(status, bytes.length)
    exchange.getResponseBody.write(bytes)
  }

  def main(args: Array[String]): Unit = {
    val port = sys.env.getOrElse("GAMESTATS_ANALYZER_PORT", "8081").toInt
    start(port)
    println(s"GameStats Scala escuchando en $port")
  }
}
