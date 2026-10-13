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
		{"eventId":"E3","timestamp":"2026-10-08T17:00:02-06:00","gameId":"MINESWEEPER","gameVersion":"1.0","matchId":"P1","playerId":"PLAYER2","type":"MATCH_STARTED","data":{"rows":2,"columns":3,"mineCount":2,"difficulty":"EASY"}},
		{"eventId":"E4","timestamp":"2026-10-08T17:00:03-06:00","gameId":"MINESWEEPER","gameVersion":"1.0","matchId":"M1","playerId":"PLAYER2","type":"MATCH_STARTED","data":{"rows":2,"columns":3,"mineCount":2,"difficulty":"EASY"}},
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

func TestValidateGameSpecificData(t *testing.T) {
	base := Event{
		EventID: "E1", Timestamp: "2026-10-08T17:00:00-06:00",
		GameID: "PACMAN", GameVersion: "1.0", MatchID: "P1",
		PlayerID: "PLAYER1", Type: "PELLET_EATEN",
	}
	tests := []struct {
		name, game, kind, data, wantError string
	}{
		{"puntos válidos", "PACMAN", "PELLET_EATEN", `{"points":10}`, ""},
		{"puntos ausentes", "PACMAN", "PELLET_EATEN", `{}`, "data.points"},
		{"puntos texto", "PACMAN", "PELLET_EATEN", `{"points":"10"}`, "data.points"},
		{"puntos fraccionarios", "PACMAN", "PELLET_EATEN", `{"points":1.5}`, "entero"},
		{"puntos negativos", "PACMAN", "PELLET_EATEN", `{"points":-1}`, "cero"},
		{"campo adicional", "PACMAN", "PELLET_EATEN", `{"points":10,"ponts":10}`, "data.ponts"},
		{"juego desconocido", "OTHER", "MATCH_STARTED", `{}`, "gameId"},
		{"tipo desconocido", "PACMAN", "OTHER", `{}`, "type"},
		{"tipo de otro juego", "PACMAN", "FLAG_PLACED", `{"row":0,"column":1,"correct":true}`, "type"},
		{"bandera válida", "MINESWEEPER", "FLAG_PLACED", `{"row":0,"column":1,"correct":false}`, ""},
		{"bandera texto", "MINESWEEPER", "FLAG_PLACED", `{"row":0,"column":1,"correct":"sí"}`, "data.correct"},
		{"bandera null", "MINESWEEPER", "FLAG_PLACED", `{"row":0,"column":1,"correct":null}`, "data.correct"},
		{"coordenada negativa", "MINESWEEPER", "FLAG_PLACED", `{"row":-1,"column":1,"correct":true}`, "data.row"},
		{"ghost vacío", "PACMAN", "GHOST_EATEN", `{"ghost":" ","points":200}`, "data.ghost"},
		{"tablero incompleto", "MINESWEEPER", "MATCH_STARTED", `{"rows":2,"columns":3}`, "data.mineCount"},
		{"ajedrez inicio", "CHESS", "MATCH_STARTED", `{"side":"WHITE"}`, ""},
		{"ajedrez color inválido", "CHESS", "MATCH_STARTED", `{"side":"GREEN"}`, "data.side"},
		{"ajedrez jugada", "CHESS", "MOVE_PLAYED", `{"ply":1,"piece":"KNIGHT","from":"g1","to":"f3","thinkTimeSeconds":4}`, ""},
		{"ajedrez casilla inválida", "CHESS", "MOVE_PLAYED", `{"ply":1,"piece":"KNIGHT","from":"i1","to":"f3","thinkTimeSeconds":4}`, "data.from"},
		{"ajedrez misma casilla", "CHESS", "MOVE_PLAYED", `{"ply":1,"piece":"KNIGHT","from":"g1","to":"g1","thinkTimeSeconds":4}`, "casillas diferentes"},
		{"ajedrez pieza inválida", "CHESS", "MOVE_PLAYED", `{"ply":1,"piece":"HORSE","from":"g1","to":"f3","thinkTimeSeconds":4}`, "data.piece"},
		{"ajedrez tiempo negativo", "CHESS", "MOVE_PLAYED", `{"ply":1,"piece":"KNIGHT","from":"g1","to":"f3","thinkTimeSeconds":-1}`, "data.thinkTimeSeconds"},
		{"ajedrez captura", "CHESS", "PIECE_CAPTURED", `{"ply":7,"capturedPiece":"PAWN"}`, ""},
		{"ajedrez jaque", "CHESS", "CHECK_GIVEN", `{"ply":7}`, ""},
		{"ajedrez mate", "CHESS", "CHECKMATE", `{"ply":7}`, ""},
		{"ajedrez ply cero", "CHESS", "CHECK_GIVEN", `{"ply":0}`, "data.ply"},
		{"ajedrez tablas", "CHESS", "MATCH_FINISHED", `{"result":"DRAW","durationSeconds":50}`, ""},
		{"ajedrez resultado inválido", "CHESS", "MATCH_FINISHED", `{"result":"TIE","durationSeconds":50}`, "data.result"},
		{"memoria inicio", "MEMORY", "MATCH_STARTED", `{"rows":2,"columns":2,"pairCount":2,"timeLimitSeconds":20}`, ""},
		{"memoria pares ausentes", "MEMORY", "MATCH_STARTED", `{"rows":2,"columns":2,"timeLimitSeconds":20}`, "data.pairCount"},
		{"memoria tiempo cero", "MEMORY", "MATCH_STARTED", `{"rows":2,"columns":2,"pairCount":2,"timeLimitSeconds":0}`, "data.timeLimitSeconds"},
		{"memoria carta", "MEMORY", "CARD_FLIPPED", `{"cardId":"A1","symbol":"STAR","turn":1}`, ""},
		{"memoria símbolo vacío", "MEMORY", "CARD_FLIPPED", `{"cardId":"A1","symbol":" ","turn":1}`, "data.symbol"},
		{"memoria turno fraccionario", "MEMORY", "CARD_FLIPPED", `{"cardId":"A1","symbol":"STAR","turn":1.5}`, "data.turn"},
		{"memoria par", "MEMORY", "PAIR_MATCHED", `{"firstCardId":"A1","secondCardId":"A2","turn":2}`, ""},
		{"memoria intento fallido", "MEMORY", "PAIR_MISMATCHED", `{"firstCardId":"A1","secondCardId":"B1","turn":1}`, ""},
		{"memoria misma carta", "MEMORY", "PAIR_MATCHED", `{"firstCardId":"A1","secondCardId":"A1","turn":2}`, "cartas diferentes"},
		{"memoria tiempo agotado", "MEMORY", "TIME_EXPIRED", `{"pairsFound":1}`, ""},
		{"memoria pares negativos", "MEMORY", "TIME_EXPIRED", `{"pairsFound":-1}`, "data.pairsFound"},
		{"memoria derrota", "MEMORY", "MATCH_FINISHED", `{"result":"LOSS","durationSeconds":20}`, ""},
		{"memoria resultado tablas", "MEMORY", "MATCH_FINISHED", `{"result":"DRAW","durationSeconds":20}`, "data.result"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			event := base
			event.GameID, event.Type, event.Data = tc.game, tc.kind, []byte(tc.data)
			err := event.Validate()
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("evento válido rechazado: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("se esperaba error con %q, obtenido: %v", tc.wantError, err)
			}
		})
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
	if result.Total != 88 || len(result.Events) != 88 || len(result.Rejected) != 0 {
		t.Fatalf("eventos.json no produjo 88 eventos válidos: %+v", result)
	}
}
