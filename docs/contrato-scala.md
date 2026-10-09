# Contrato Go → Scala, versión 1

Go produce un archivo JSON UTF-8 llamado `partidas.json` por ejecución. Scala
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
- Los campos de `data` específicos de cada `type` quedan por acordar entre Jason
  y Paula. Go no calcula estadísticas ni transforma esos valores.
- `Started` y `Finished` son parte del estado interno de Go y no se envían;
  Scala puede inferirlos de `MATCH_STARTED` y `MATCH_FINISHED`.

Para usar otra ruta de salida, ejecutar `go run . -output ruta/al/archivo.json`.
En Docker, ambos servicios podrán montar un volumen compartido con ese archivo.
