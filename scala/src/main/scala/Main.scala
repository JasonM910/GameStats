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
        val salida = resolverSalida(entrada)
        os.write.over(salida, write(resultado, indent = 2), createFolders = true)
        ConsoleReport.print(entrada, salida, resultado)
    }
  }
}
