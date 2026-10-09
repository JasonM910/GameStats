package matches

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"gamestats/internal/events"
)

func TestProcessInterleavedMatches(t *testing.T) {
	input := []events.IndexedEvent{
		indexed(1, "E1", "PACMAN", "P1", "PLAYER1", "MATCH_STARTED"),
		indexed(2, "E2", "MINESWEEPER", "M1", "PLAYER2", "MATCH_STARTED"),
		indexed(3, "E3", "PACMAN", "P1", "PLAYER1", "PELLET_EATEN"),
		indexed(4, "E4", "PACMAN", "P1", "PLAYER3", "GHOST_EATEN"),
		indexed(5, "E5", "MINESWEEPER", "M1", "PLAYER2", "MATCH_FINISHED"),
		indexed(6, "E6", "PACMAN", "P1", "PLAYER1", "MATCH_FINISHED"),
	}
	states, rejected := Process(input)
	if len(rejected) != 0 || len(states) != 2 {
		t.Fatalf("estado inesperado: %+v, rechazos: %+v", states, rejected)
	}
	p := states["P1"]
	if p.GameID != "PACMAN" || !p.Started || !p.Finished || !reflect.DeepEqual(p.Players, []string{"PLAYER1", "PLAYER3"}) {
		t.Fatalf("estado de Pac-Man inesperado: %+v", p)
	}
	var ids []string
	for _, event := range p.Events {
		ids = append(ids, event.EventID)
	}
	if !reflect.DeepEqual(ids, []string{"E1", "E3", "E4", "E6"}) {
		t.Fatalf("se perdió el orden de entrada: %v", ids)
	}
	m := states["M1"]
	if m.GameID != "MINESWEEPER" || !m.Started || !m.Finished || !reflect.DeepEqual(m.Players, []string{"PLAYER2"}) || len(m.Events) != 2 {
		t.Fatalf("estado de Buscaminas inesperado: %+v", m)
	}
}

func TestProcessRejectsGameConflict(t *testing.T) {
	input := []events.IndexedEvent{
		indexed(1, "E1", "PACMAN", "P1", "PLAYER1", "MATCH_STARTED"),
		indexed(2, "E2", "MINESWEEPER", "P1", "PLAYER2", "MATCH_STARTED"),
		indexed(3, "E3", "PACMAN", "P1", "PLAYER1", "MATCH_FINISHED"),
	}
	states, rejected := Process(input)
	if len(rejected) != 1 || rejected[0].Index != 2 || rejected[0].EventID != "E2" {
		t.Fatalf("conflicto no identificado: %+v", rejected)
	}
	if len(states["P1"].Events) != 2 || len(states["P1"].Players) != 1 {
		t.Fatalf("el evento rechazado alteró la partida: %+v", states["P1"])
	}
}

func TestProcessManyMatchesConcurrently(t *testing.T) {
	const count = 100
	var input []events.IndexedEvent
	for round := 0; round < 3; round++ {
		for match := 0; match < count; match++ {
			input = append(input, indexed(len(input)+1, fmt.Sprintf("E%d-%d", match, round),
				"PACMAN", fmt.Sprintf("P%d", match), fmt.Sprintf("PLAYER%d", match), "PELLET_EATEN"))
		}
	}
	states, rejected := Process(input)
	if len(rejected) != 0 || len(states) != count {
		t.Fatalf("procesamiento concurrente incompleto: %d partidas, %d rechazos", len(states), len(rejected))
	}
	for match := 0; match < count; match++ {
		state := states[fmt.Sprintf("P%d", match)]
		if len(state.Events) != 3 || len(state.Players) != 1 {
			t.Fatalf("partida P%d incompleta: %+v", match, state)
		}
		for round, event := range state.Events {
			if event.EventID != fmt.Sprintf("E%d-%d", match, round) {
				t.Fatalf("partida P%d fuera de orden: %+v", match, state.Events)
			}
		}
	}
}

func TestSampleMatches(t *testing.T) {
	file, err := os.Open("../../eventos.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	read, err := events.ReadAndValidate(file)
	if err != nil {
		t.Fatal(err)
	}
	states, rejected := Process(read.Events)
	if len(rejected) != 0 || len(states) != 5 || len(states["P001"].Events) != 12 || len(states["M003"].Events) != 11 {
		t.Fatalf("partidas de ejemplo inesperadas: %+v; rechazados: %+v", states, rejected)
	}
}

func indexed(index int, id, game, match, player, kind string) events.IndexedEvent {
	return events.IndexedEvent{Index: index, Event: events.Event{
		EventID: id, GameID: game, MatchID: match, PlayerID: player, Type: kind,
	}}
}
