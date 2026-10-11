# GameStats

GameStats procesa eventos simulados de Pacman y Buscaminas en dos etapas. Go lee
`eventos.json`, valida cada evento y agrupa los válidos por partida. Scala lee
`salida/partidas.json`, calcula estadísticas y patrones, y escribe
`salida/resultados.json`. Ambos programas terminan después de procesar el archivo;
no son servidores HTTP.

## Requisitos y ejecución local

Para ejecutar sin Docker se necesitan Go, Java 21 y sbt. Desde la raíz del
proyecto, en PowerShell:

```powershell
go run .
cd scala
sbt test
sbt "run ../salida/partidas.json"
cd ..
Get-Content -Raw salida/resultados.json
```

Go acepta `-input ruta/al/eventos.json` y `-output ruta/al/partidas.json`. Scala
acepta como primer argumento la ruta del archivo preparado por Go. También se
pueden definir `GAMESTATS_INPUT` y `GAMESTATS_OUTPUT`; sin una ruta de salida,
Scala escribe `resultados.json` junto al archivo de entrada.

Los eventos inválidos se informan en la salida de error de Go y se excluyen de
`partidas.json`. Go procesa las partidas mediante goroutines y channels. El
formato común, las reglas de validación y el catálogo de eventos están en
[docs/contrato-scala.md](docs/contrato-scala.md).

## Docker

Con Docker Desktop iniciado, ejecutar desde la raíz:

```powershell
docker compose up --build
Get-Content -Raw salida/resultados.json
```

[compose.yaml](compose.yaml) construye las imágenes de Go y Scala, ejecuta Go
primero y espera a que termine correctamente antes de iniciar Scala. Ambos
contenedores comparten la carpeta `salida/` del proyecto. La construcción de la
imagen Scala ejecuta sus pruebas. Para retirar los contenedores terminados:

```powershell
docker compose down
```

Los Dockerfiles son [Dockerfile](Dockerfile) para Go y
[scala/Dockerfile](scala/Dockerfile) para Scala. Cada imagen también se puede
construir por separado:

```powershell
docker build -t gamestats-go .
docker build -t gamestats-scala ./scala
```

## Resultados de Scala

`resultados.json` contiene `schemaVersion`, `players`, `matches`, `games`,
`metrics` y `patterns`. Scala convierte el JSON de Go en `GameEvent`, `MatchData`
y `PartidasFile`; rechaza versiones de esquema desconocidas y timestamps
inválidos. El análisis agrupa por jugador y juego con `groupBy`, transforma
colecciones con `filter`, `map` y `flatMap`, y agrega valores con `foldLeft` y
`reduce`.

| Sección | Cálculo |
| --- | --- |
| `players` | Partidas finalizadas, victorias, derrotas y `winrate = victorias / partidas finalizadas × 100`. Si no hay partidas finalizadas, el porcentaje es 0. |
| `matches` | Juego, jugadores, cantidad de eventos, resultado y duración de cada partida. Sin `MATCH_FINISHED`, el resultado es `UNFINISHED` y la duración es 0. |
| `games` | Partidas y eventos por juego; duración media de las partidas finalizadas. Si ninguna terminó, el promedio es 0. |

Las métricas específicas se calculan por jugador y partida:

| Juego | Métrica | Cálculo |
| --- | --- | --- |
| Pacman | `totalPoints` | Suma de `points` en `PELLET_EATEN`, `POWER_PELLET_EATEN`, `GHOST_EATEN` y `FRUIT_EATEN`. |
| Pacman | `pointsPerMinute` | `totalPoints / durationSeconds × 60`; se emite solo si `MATCH_FINISHED` tiene duración mayor que 0. |
| Buscaminas | `flagAccuracyPercent` | Banderas con `correct: true` divididas entre todas las `FLAG_PLACED`, por 100; se emite solo si hubo banderas. |
| Buscaminas | `cellsRevealed` | Cantidad de eventos `CELL_REVEALED`. |

Los patrones se buscan por jugador dentro de cada partida, ordenando los eventos
por `timestamp`:

| Patrón | Regla |
| --- | --- |
| `POWER_PELLET_COMBO` | En Pacman, después de `POWER_PELLET_EATEN` se comen al menos dos fantasmas dentro de `powerDurationSeconds`. |
| `COMEBACK` | En Pacman, ocurre `LIFE_LOST` y posteriormente `MATCH_FINISHED` con resultado `WIN`. |
| `FLAG_CORRECTED` | En Buscaminas, se coloca una bandera incorrecta y luego se retira de la misma casilla. |

Cada patrón incluye los identificadores del evento inicial y final. Un juego
sin analizador específico conserva las estadísticas generales, pero no produce
métricas ni patrones propios.

## Pruebas e integración continua

```powershell
go test ./...
cd scala
sbt test
```

En Windows, `go test -race ./...` requiere CGO y un compilador C. El flujo de
[GitHub Actions](.github/workflows/ci.yml) ejecuta pruebas Go con detector de
carreras, `go vet`, compila Go, construye las dos imágenes Docker y ejecuta las
pruebas Scala durante su construcción. Luego procesa el archivo de ejemplo de
Go a Scala y verifica la estructura de `resultados.json`. Publica como artefacto
la imagen de Go y los dos JSON de ejemplo; no despliega servicios.
