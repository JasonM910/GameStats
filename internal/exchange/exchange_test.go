package exchange

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gamestats/internal/events"
	"gamestats/internal/matches"
)

func TestWriteFileContract(t *testing.T) {
	input := `[
		{"eventId":"E1","timestamp":"2026-10-08T17:00:00-06:00","gameId":"PACMAN","gameVersion":"1.0","matchId":"P1","playerId":"PLAYER1","type":"MATCH_STARTED","data":{}},
		{"eventId":"E2","timestamp":"2026-10-08T17:00:01-06:00","gameId":"MINESWEEPER","gameVersion":"1.0","matchId":"M1","playerId":"PLAYER2","type":"MATCH_STARTED","data":{"rows":2,"columns":3,"mineCount":2,"difficulty":"EASY"}},
		{"eventId":"BAD","timestamp":"ayer","gameId":"PACMAN","gameVersion":"1.0","matchId":"P1","playerId":"PLAYER1","type":"PELLET_EATEN","data":{"points":10}},
		{"eventId":"E3","timestamp":"2026-10-08T17:00:02-06:00","gameId":"PACMAN","gameVersion":"1.0","matchId":"P1","playerId":"PLAYER1","type":"PELLET_EATEN","data":{"points":10}},
		{"eventId":"E4","timestamp":"2026-10-08T17:00:03-06:00","gameId":"MINESWEEPER","gameVersion":"1.0","matchId":"P1","playerId":"PLAYER2","type":"CELL_REVEALED","data":{"row":0,"column":0,"adjacentMines":1}},
		{"eventId":"E5","timestamp":"2026-10-08T17:00:04-06:00","gameId":"PACMAN","gameVersion":"1.0","matchId":"P1","playerId":"PLAYER1","type":"PELLET_EATEN","data":{}}
	]`
	read, err := events.ReadAndValidate(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	states, rejected := matches.Process(read.Events)
	if len(read.Rejected) != 2 || read.Rejected[1].EventID != "E5" || len(rejected) != 1 || rejected[0].EventID != "E4" {
		t.Fatalf("rechazos inesperados: lectura=%+v procesamiento=%+v", read.Rejected, rejected)
	}

	path := filepath.Join(t.TempDir(), "salida", "partidas.json")
	for i := 0; i < 2; i++ { // Repetir comprueba que una ejecución posterior sustituye el archivo.
		if err := WriteFile(path, states); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document Document
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != 1 || len(document.Matches) != 2 {
		t.Fatalf("documento inesperado: %+v", document)
	}
	if document.Matches[0].MatchID != "M1" || document.Matches[1].MatchID != "P1" {
		t.Fatalf("partidas no ordenadas por matchId: %+v", document.Matches)
	}
	p := document.Matches[1]
	if p.GameID != "PACMAN" || !reflect.DeepEqual(p.Players, []string{"PLAYER1"}) || len(p.Events) != 2 {
		t.Fatalf("partida P1 inesperada: %+v", p)
	}
	if p.Events[0].EventID != "E1" || p.Events[1].EventID != "E3" || p.Events[1].Timestamp != "2026-10-08T17:00:02-06:00" {
		t.Fatalf("eventos o fechas alterados: %+v", p.Events)
	}
	var payload struct {
		Points int `json:"points"`
	}
	if err := json.Unmarshal(p.Events[1].Data, &payload); err != nil || payload.Points != 10 {
		t.Fatalf("data no conservado: %+v (%v)", payload, err)
	}
	if strings.Contains(string(data), `"BAD"`) || strings.Contains(string(data), `"E4"`) || strings.Contains(string(data), `"E5"`) || strings.Contains(string(data), `"Started"`) {
		t.Fatalf("archivo contiene evento inválido o campos internos: %s", data)
	}
}

func TestFromStatesEmpty(t *testing.T) {
	document := FromStates(map[string]matches.State{})
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"schemaVersion":1,"matches":[]}` {
		t.Fatalf("salida vacía inesperada: %s", data)
	}
}
