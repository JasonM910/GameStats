# GameStats — módulo Go

El módulo lee un archivo JSON, convierte cada objeto en `Event`, valida el
contrato común y envía los eventos válidos a una goroutine por partida. Cada
partida tiene un channel propio y un estado temporal con juego, eventos,
jugadores únicos y marcas de inicio y fin. Los inválidos se informan con su
posición en el arreglo y la causa; no impiden procesar el resto del archivo.

## Ejecutar

```powershell
go run .
go run . -input ruta\a\otro-archivo.json
go run . -output otra-carpeta\archivo.json
go test ./...
go test -race ./...
```

En Windows, `go test -race` requiere CGO activado y un compilador C disponible;
si el entorno no los tiene, `go test ./...` sí ejecuta las pruebas normales.

## Docker (módulo Go)

Con Docker Desktop iniciado, desde la raíz del proyecto en PowerShell:

```powershell
docker build -t gamestats-go .
$outputDir = Join-Path (Get-Location) 'salida'
New-Item -ItemType Directory -Force $outputDir | Out-Null
docker run --rm --mount "type=bind,source=$outputDir,target=/output" gamestats-go -output /output/partidas.json
Get-Content -Raw (Join-Path $outputDir 'partidas.json')
```

La imagen incluye `eventos.json`. Go lee ese archivo y escribe el resultado en
`salida/partidas.json` del equipo mediante el montaje de la carpeta. Para usar
otra entrada, monta el archivo o una carpeta adicional y pasa `-input` con su
ruta dentro del contenedor. Scala podrá leer el mismo archivo cuando ambos
servicios compartan esa carpeta o un volumen en la integración final.

La entrada debe ser un arreglo JSON. Cada evento requiere `eventId`, `timestamp`
en formato RFC3339 (con zona horaria), `gameId`, `gameVersion`, `matchId`,
`playerId`, `type` y `data` como objeto JSON, aunque esté vacío. Se rechazan
`eventId` duplicados y eventos que asignan un mismo `matchId` a distintos juegos.
Para PACMAN y MINESWEEPER también se comprueba que `type` esté definido y que
`data` contenga exactamente los campos esperados con los tipos correctos. El
catálogo está en [docs/contrato-scala.md](docs/contrato-scala.md). Go conserva
`data` para el análisis posterior de Scala.

Cada partida mantiene sus eventos y jugadores en orden de llegada. Las partidas
se procesan simultáneamente. Al terminar, Go genera `salida/partidas.json` con solo
eventos válidos para que Scala lo lea. El formato está documentado en
[docs/contrato-scala.md](docs/contrato-scala.md). Go todavía no ordena los eventos
por fecha ni calcula estadísticas. `Started` y `Finished` indican si se
recibieron `MATCH_STARTED` y `MATCH_FINISHED`; no validan el orden de esos eventos.
