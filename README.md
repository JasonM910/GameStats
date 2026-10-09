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
go run . -input eventos.json -output partidas.json
go test ./...
go test -race ./...
```

En Windows, `go test -race` requiere CGO activado y un compilador C disponible;
si el entorno no los tiene, `go test ./...` sí ejecuta las pruebas normales.

La entrada debe ser un arreglo JSON. Cada evento requiere `eventId`, `timestamp`
en formato RFC3339 (con zona horaria), `gameId`, `gameVersion`, `matchId`,
`playerId`, `type` y `data` como objeto JSON, aunque esté vacío. Se rechazan
`eventId` duplicados y eventos que asignan un mismo `matchId` a distintos juegos.
El campo `data` se conserva sin interpretar para admitir datos particulares de
Pac-Man, Buscaminas u otros juegos. La validación de campos específicos según
el tipo de evento queda para cuando el equipo cierre ese contrato.

Cada partida mantiene sus eventos y jugadores en orden de llegada. Las partidas
se procesan simultáneamente. Al terminar, Go genera `partidas.json` con solo
eventos válidos para que Scala lo lea. El formato está documentado en
[docs/contrato-scala.md](docs/contrato-scala.md). Go todavía no ordena los eventos
por fecha ni calcula estadísticas. `Started` y `Finished` indican si se
recibieron `MATCH_STARTED` y `MATCH_FINISHED`; no validan el orden de esos eventos.
