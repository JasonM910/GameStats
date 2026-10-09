# Contrato Go → Scala, versión 1

Go produce un archivo JSON UTF-8 llamado `salida/partidas.json` por ejecución local. Scala
lo lee después de que Go termine correctamente. Los eventos inválidos se
informan en stderr de Go y nunca aparecen en este archivo.

```json
{
  "schemaVersion": 1,
  "matches": [
    {
      "gameId": "PACMAN",
      "matchId": "P001",
      "players": ["PLAYER01"],
      "events": [
        {
          "eventId": "E001",
          "timestamp": "2026-10-08T17:00:00-06:00",
          "gameId": "PACMAN",
          "gameVersion": "1.0",
          "matchId": "P001",
          "playerId": "PLAYER01",
          "type": "MATCH_STARTED",
          "data": {}
        },
        {
          "eventId": "E003",
          "timestamp": "2026-10-08T17:00:05-06:00",
          "gameId": "PACMAN",
          "gameVersion": "1.0",
          "matchId": "P001",
          "playerId": "PLAYER01",
          "type": "PELLET_EATEN",
          "data": { "points": 10 }
        }
      ]
    }
  ]
}
```

## Reglas del archivo

- `schemaVersion` es el número `1`. Scala debe rechazar versiones que no conozca.
- `matches` es un arreglo ordenado por `matchId`; puede estar vacío.
- Cada partida tiene `gameId`, `matchId`, `players` y `events`.
- `players` enumera identificadores únicos en orden de primera aparición.
- `events` conserva el orden de llegada del archivo de entrada. Para reglas que
  dependan del tiempo, Scala puede ordenar por `timestamp` cuando sea necesario.
- Cada evento conserva el contrato de `eventos.json`. `timestamp` usa RFC3339
  con zona horaria y `data` siempre es un objeto JSON, incluso si está vacío.
- Go valida el catálogo de `data` indicado abajo antes de exportar. Los campos
  son obligatorios y no se permiten claves adicionales. Go no calcula
  estadísticas ni transforma esos valores.
- `Started` y `Finished` son parte del estado interno de Go y no se envían;
  Scala puede inferirlos de `MATCH_STARTED` y `MATCH_FINISHED`.

Para usar otra ruta de salida, ejecutar `go run . -output ruta/al/archivo.json`.
En Docker, ambos servicios podrán montar un volumen compartido con ese archivo.

## Catálogo definitivo de eventos

Los nombres de juego y tipo se escriben exactamente como aparecen aquí. `texto`
significa cadena no vacía; `entero ≥ 0` y `entero > 0` son números JSON enteros,
no cadenas. Los booleanos son `true` o `false`, nunca `null`.

| Juego | `type` | Campos obligatorios de `data` |
| --- | --- | --- |
| PACMAN | MATCH_STARTED | Ninguno (`{}`) |
| PACMAN | PELLET_EATEN | `points`: entero ≥ 0 |
| PACMAN | POWER_PELLET_EATEN | `points`: entero ≥ 0; `powerDurationSeconds`: entero > 0 |
| PACMAN | GHOST_EATEN | `ghost`: texto; `points`: entero ≥ 0 |
| PACMAN | FRUIT_EATEN | `fruit`: texto; `points`: entero ≥ 0 |
| PACMAN | LIFE_LOST | `remainingLives`: entero ≥ 0; `ghost`: texto |
| PACMAN | LEVEL_COMPLETED | `level`: entero > 0; `durationSeconds`: entero ≥ 0 |
| PACMAN | MATCH_FINISHED | `result`: texto; `durationSeconds`: entero ≥ 0 |
| MINESWEEPER | MATCH_STARTED | `rows`, `columns`: enteros > 0; `mineCount`: entero ≥ 0; `difficulty`: texto |
| MINESWEEPER | CELL_REVEALED | `row`, `column`, `adjacentMines`: enteros ≥ 0 |
| MINESWEEPER | FLAG_PLACED | `row`, `column`: enteros ≥ 0; `correct`: booleano |
| MINESWEEPER | FLAG_REMOVED | `row`, `column`: enteros ≥ 0 |
| MINESWEEPER | MINE_TRIGGERED | `row`, `column`: enteros ≥ 0 |
| MINESWEEPER | BOARD_COMPLETED | `difficulty`: texto; `durationSeconds`: entero ≥ 0 |
| MINESWEEPER | MATCH_FINISHED | `result`: texto; `durationSeconds`: entero ≥ 0 |

Esta validación comprueba la forma de cada evento. Las reglas entre eventos,
como verificar si una casilla existe dentro del tablero de la partida, no se
aplican en esta etapa.
