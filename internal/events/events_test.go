package events

import (
	"os"
	"strings"
	"testing"
)

const validEvent = `{"eventId":"E1","timestamp":"2026-10-08T17:00:00-06:00","gameId":"PACMAN","gameVersion":"1.0","matchId":"P1","playerId":"PLAYER1","type":"MATCH_STARTED","data":{}}`

func TestReadAndValidateKeepsValidEventsAndReportsRejected(t *testing.T) {
	input := `[` + validEvent + `,
		{"eventId":"E2","timestamp":"bad","gameId":"PACMAN","gameVersion":"1.0","matchId":"P1","playerId":"PLAYER1","type":"PELLET_EATEN","data":{"points":10}},
		{"eventId":"E1","timestamp":"2026-10-08T17:00:01-06:00","gameId":"PACMAN","gameVersion":"1.0","matchId":"P1","playerId":"PLAYER1","type":"PELLET_EATEN","data":{"points":10}},
		{"eventId":"E3","timestamp":"2026-10-08T17:00:02-06:00","gameId":"MINESWEEPER","gameVersion":"1.0","matchId":"P1","playerId":"PLAYER2","type":"MATCH_STARTED","data":{}},
		{"eventId":"E4","timestamp":"2026-10-08T17:00:03-06:00","gameId":"MINESWEEPER","gameVersion":"1.0","matchId":"M1","playerId":"PLAYER2","type":"MATCH_STARTED","data":{}},
		{"eventId":5},
		{"eventId":"E5","timestamp":"2026-10-08T17:00:04-06:00","gameId":"PACMAN","gameVersion":"1.0","matchId":"P1","playerId":"PLAYER1","type":"PELLET_EATEN","data":[]}
	]`
	result, err := ReadAndValidate(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 7 || len(result.Events) != 3 || len(result.Rejected) != 4 {
		t.Fatalf("conteos inesperados: %+v", result)
	}
	if result.Events[0].Index != 1 || result.Events[1].Index != 4 || result.Events[2].Index != 5 {
		t.Fatalf("posiciones de eventos válidos inesperadas: %+v", result.Events)
	}
	for i, expected := range []string{"timestamp", "duplicado", "unmarshal", "data"} {
		if !strings.Contains(result.Rejected[i].Reason, expected) {
			t.Errorf("rechazo %d = %q; se esperaba %q", i, result.Rejected[i].Reason, expected)
		}
	}
}

func TestReadAndValidateRejectsInvalidFile(t *testing.T) {
	for _, input := range []string{`{}`, `null`, `[`, `[] {}`} {
		if _, err := ReadAndValidate(strings.NewReader(input)); err == nil {
			t.Errorf("se aceptó archivo inválido: %q", input)
		}
	}
}

func TestValidateRequiresCommonFields(t *testing.T) {
	event := Event{
		EventID: "E1", Timestamp: "2026-10-08T17:00:00-06:00",
		GameID: "PACMAN", GameVersion: "1.0", MatchID: "P1",
		PlayerID: "PLAYER1", Type: "MATCH_STARTED", Data: []byte(`{}`),
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("evento válido rechazado: %v", err)
	}
	event.PlayerID = "  "
	if err := event.Validate(); err == nil || !strings.Contains(err.Error(), "playerId") {
		t.Fatalf("playerId vacío debe rechazarse: %v", err)
	}
}

func TestSampleEvents(t *testing.T) {
	file, err := os.Open("../../eventos.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	result, err := ReadAndValidate(file)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 44 || len(result.Events) != 44 || len(result.Rejected) != 0 {
		t.Fatalf("eventos.json no produjo 44 eventos válidos: %+v", result)
	}
}
