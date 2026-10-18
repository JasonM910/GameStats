package simulator

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"gamestats/internal/events"
	"gamestats/internal/matches"
)

func TestStreamLimitsActiveMatchesAndPreservesContract(t *testing.T) {
	stream, errors, err := Stream(context.Background(), Options{Matches: 8, Seed: 42, Interval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	active := map[string]bool{}
	finished := map[string]int{}
	started := map[string]bool{}
	lastTime := map[string]time.Time{}
	gameCounts := map[string]int{}
	seen := map[string]bool{}
	var input []events.IndexedEvent
	maximum := 0
	for event := range stream {
		if err := event.Validate(); err != nil {
			t.Fatalf("%s: %v", event.EventID, err)
		}
		if seen[event.EventID] {
			t.Fatalf("eventId repetido: %s", event.EventID)
		}
		seen[event.EventID] = true
		when, err := time.Parse(time.RFC3339, event.Timestamp)
		if err != nil || when.Before(lastTime[event.MatchID]) {
			t.Fatalf("tiempo fuera de orden: %+v", event)
		}
		lastTime[event.MatchID] = when
		if event.Type == "MATCH_STARTED" && !started[event.MatchID] {
			started[event.MatchID] = true
			active[event.MatchID] = true
			gameCounts[event.GameID]++
			if len(active) > maximum {
				maximum = len(active)
			}
		}
		if event.Type == "MATCH_FINISHED" {
			finished[event.MatchID]++
			if event.GameID != "CHESS" || finished[event.MatchID] == 2 {
				delete(active, event.MatchID)
			}
		}
		if len(active) > 2 {
			t.Fatalf("hay %d partidas activas", len(active))
		}
		input = append(input, events.IndexedEvent{Index: len(input) + 1, Event: event})
	}
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if maximum != 2 || len(started) != 8 || len(active) != 0 {
		t.Fatalf("concurrencia incorrecta: máximo=%d iniciadas=%d activas=%d", maximum, len(started), len(active))
	}
	for _, game := range games {
		if gameCounts[game] != 2 {
			t.Fatalf("%s tiene %d partidas", game, gameCounts[game])
		}
	}
	states, rejected := matches.Process(input)
	if len(rejected) != 0 || len(states) != 8 {
		t.Fatalf("procesamiento: %d estados, %d rechazos", len(states), len(rejected))
	}
	for _, state := range states {
		if !state.Started || !state.Finished {
			t.Fatalf("partida incompleta: %+v", state)
		}
		wantPlayers, wantFinishes := 1, 1
		if state.GameID == "CHESS" {
			wantPlayers, wantFinishes = 2, 2
		}
		if len(state.Players) != wantPlayers || finished[state.MatchID] != wantFinishes {
			t.Fatalf("participantes o finales incorrectos: %+v", state)
		}
	}
}

func TestGeneratedMovesRespectGameState(t *testing.T) {
	stream, errors, err := Stream(context.Background(), Options{Matches: 40, Seed: 91})
	if err != nil {
		t.Fatal(err)
	}
	type board struct {
		mines    map[int]bool
		flagged  map[int]bool
		revealed map[int]int
	}
	boards := map[string]*board{}
	cards := map[string]map[string]string{}
	flips := map[string]map[int][]string{}
	chessPly := map[string]int{}
	chessFinished := map[string]map[string]string{}
	for e := range stream {
		var data map[string]any
		if err := json.Unmarshal(e.Data, &data); err != nil {
			t.Fatal(err)
		}
		switch e.GameID {
		case "MINESWEEPER":
			b := boards[e.MatchID]
			if b == nil {
				b = &board{map[int]bool{}, map[int]bool{}, map[int]int{}}
				boards[e.MatchID] = b
			}
			if e.Type == "MATCH_STARTED" {
				continue
			}
			cell := func() int { return int(data["row"].(float64))*4 + int(data["column"].(float64)) }
			switch e.Type {
			case "FLAG_PLACED":
				if data["correct"].(bool) {
					b.mines[cell()] = true
				}
				b.flagged[cell()] = true
			case "FLAG_REMOVED":
				if !b.flagged[cell()] {
					t.Fatal("bandera retirada sin colocar")
				}
				delete(b.flagged, cell())
			case "CELL_REVEALED":
				if b.flagged[cell()] {
					t.Fatal("casilla revelada dos veces o con bandera")
				}
				if _, exists := b.revealed[cell()]; exists {
					t.Fatal("casilla revelada dos veces")
				}
				b.revealed[cell()] = int(data["adjacentMines"].(float64))
			case "MINE_TRIGGERED":
				if !b.mines[cell()] || b.flagged[cell()] {
					t.Fatal("mina imposible")
				}
			case "BOARD_COMPLETED":
				if len(b.revealed) != 13 {
					t.Fatalf("tablero incompleto: %d", len(b.revealed))
				}
			}
		case "MEMORY":
			if cards[e.MatchID] == nil {
				cards[e.MatchID] = map[string]string{}
				flips[e.MatchID] = map[int][]string{}
			}
			if e.Type == "CARD_FLIPPED" {
				id, symbol, turn := data["cardId"].(string), data["symbol"].(string), int(data["turn"].(float64))
				if old := cards[e.MatchID][id]; old != "" && old != symbol {
					t.Fatal("símbolo cambió")
				}
				cards[e.MatchID][id] = symbol
				flips[e.MatchID][turn] = append(flips[e.MatchID][turn], id)
			}
			if e.Type == "PAIR_MATCHED" || e.Type == "PAIR_MISMATCHED" {
				turn := int(data["turn"].(float64))
				ids := flips[e.MatchID][turn]
				if len(ids) != 2 || ids[0] != data["firstCardId"] || ids[1] != data["secondCardId"] {
					t.Fatal("intento no corresponde a cartas volteadas")
				}
				matched := cards[e.MatchID][ids[0]] == cards[e.MatchID][ids[1]]
				if matched != (e.Type == "PAIR_MATCHED") {
					t.Fatal("resultado de par incorrecto")
				}
			}
		case "CHESS":
			if e.Type == "MOVE_PLAYED" {
				chessPly[e.MatchID]++
				if int(data["ply"].(float64)) != chessPly[e.MatchID] {
					t.Fatal("ply fuera de secuencia")
				}
			}
			if e.Type == "MATCH_FINISHED" {
				if chessFinished[e.MatchID] == nil {
					chessFinished[e.MatchID] = map[string]string{}
				}
				chessFinished[e.MatchID][e.PlayerID] = data["result"].(string)
			}
		}
	}
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	for id, b := range boards {
		if len(b.mines) != 3 {
			t.Fatalf("%s tiene %d minas", id, len(b.mines))
		}
		for cell, reported := range b.revealed {
			if b.mines[cell] {
				t.Fatalf("%s reveló mina como casilla segura", id)
			}
			actual := 0
			for dr := -1; dr <= 1; dr++ {
				for dc := -1; dc <= 1; dc++ {
					r, c := cell/4+dr, cell%4+dc
					if r >= 0 && r < 4 && c >= 0 && c < 4 && b.mines[r*4+c] {
						actual++
					}
				}
			}
			if actual != reported {
				t.Fatalf("%s casilla %d: pista %d, real %d", id, cell, reported, actual)
			}
		}
	}
	for id, results := range chessFinished {
		if len(results) != 2 || chessPly[id] < 4 {
			t.Fatalf("ajedrez inválido %s: %+v", id, results)
		}
	}
}

func TestStreamRejectsInvalidOptions(t *testing.T) {
	for _, options := range []Options{{Matches: 0}, {Matches: 1001}, {Matches: 1, Interval: -time.Second}} {
		if _, _, err := Stream(context.Background(), options); err == nil {
			t.Fatalf("opciones aceptadas: %+v", options)
		}
	}
}
