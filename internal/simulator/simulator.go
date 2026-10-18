// Package simulator genera partidas gradualmente para los cuatro juegos.
package simulator

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"gamestats/internal/events"
)

var games = [...]string{"PACMAN", "MINESWEEPER", "CHESS", "MEMORY"}

type Options struct {
	Matches  int
	Seed     int64
	Interval time.Duration
}

type action struct {
	player string
	kind   string
	data   any
	at     int // Segundos lógicos desde el inicio; cero usa el siguiente segundo.
}

type job struct {
	number int
	game   string
}

// Stream ejecuta como máximo dos partidas simultáneas. Cada goroutine emite
// eventos uno por uno hacia el mismo channel; al finalizar libera su puesto.
func Stream(ctx context.Context, options Options) (<-chan events.Event, <-chan error, error) {
	if options.Matches < 1 || options.Matches > 1000 {
		return nil, nil, fmt.Errorf("matches debe estar entre 1 y 1000")
	}
	if options.Interval < 0 {
		return nil, nil, fmt.Errorf("interval no puede ser negativo")
	}
	out := make(chan events.Event)
	errors := make(chan error, 1)
	jobs := make(chan job)
	var workers sync.WaitGroup
	for worker := 0; worker < 2; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for next := range jobs {
				if err := generate(ctx, next.number, next.game, options, out); err != nil {
					select {
					case errors <- err:
					default:
					}
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		order := make([]string, options.Matches)
		for i := range order {
			order[i] = games[i%len(games)]
		}
		shuffle := rand.New(rand.NewSource(options.Seed))
		shuffle.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		for number := 1; number <= options.Matches; number++ {
			select {
			case jobs <- job{number: number, game: order[number-1]}:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(out)
		close(errors)
	}()
	return out, errors, nil
}

func generate(ctx context.Context, number int, game string, options Options, out chan<- events.Event) error {
	matchID := fmt.Sprintf("SIM%04d", number)
	rng := rand.New(rand.NewSource(options.Seed + int64(number)*1000003))
	start := time.Now().UTC().Truncate(time.Second)
	var planned []action
	switch game {
	case "PACMAN":
		planned = pacman(number, rng)
	case "MINESWEEPER":
		planned = minesweeper(number, rng)
	case "CHESS":
		planned = chess(number, rng)
	case "MEMORY":
		planned = memory(number, rng)
	}
	elapsed := 0
	for step, item := range planned {
		if step > 0 && options.Interval > 0 {
			timer := time.NewTimer(options.Interval)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		}
		if item.at > 0 {
			if item.at > elapsed {
				elapsed = item.at
			}
		} else if step > 0 {
			elapsed++
		}
		data, err := json.Marshal(item.data)
		if err != nil {
			return err
		}
		event := events.Event{
			EventID:   fmt.Sprintf("SIM%04d-E%03d", number, step+1),
			Timestamp: start.Add(time.Duration(elapsed) * time.Second).Format(time.RFC3339),
			GameID:    game, GameVersion: "1.0", MatchID: matchID,
			PlayerID: item.player, Type: item.kind, Data: data,
		}
		select {
		case out <- event:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func player(number int) string { return fmt.Sprintf("PLAYER%04d", number) }

func pacman(number int, rng *rand.Rand) []action {
	p := player(number)
	plan := []action{{p, "MATCH_STARTED", map[string]any{}, 0}}
	for i := 0; i < 2+rng.Intn(4); i++ {
		plan = append(plan, action{p, "PELLET_EATEN", map[string]any{"points": 10}, 0})
	}
	if rng.Intn(2) == 0 {
		plan = append(plan, action{p, "POWER_PELLET_EATEN", map[string]any{"points": 50, "powerDurationSeconds": 10}, 0})
		for _, ghost := range []string{"BLINKY", "PINKY"} {
			plan = append(plan, action{p, "GHOST_EATEN", map[string]any{"ghost": ghost, "points": 200}, 0})
		}
	}
	if rng.Intn(2) == 0 {
		plan = append(plan, action{p, "FRUIT_EATEN", map[string]any{"fruit": "CHERRY", "points": 100}, 0})
	}
	lives := 3
	if rng.Intn(2) == 0 {
		lives--
		plan = append(plan, action{p, "LIFE_LOST", map[string]any{"remainingLives": lives, "ghost": "INKY"}, 0})
	}
	result := "WIN"
	if rng.Intn(4) == 0 {
		for lives > 0 {
			lives--
			plan = append(plan, action{p, "LIFE_LOST", map[string]any{"remainingLives": lives, "ghost": "CLYDE"}, 0})
		}
		result = "LOSS"
	} else {
		plan = append(plan, action{p, "LEVEL_COMPLETED", map[string]any{"level": 1, "durationSeconds": len(plan)}, 0})
	}
	plan = append(plan, action{p, "MATCH_FINISHED", map[string]any{"result": result, "durationSeconds": len(plan)}, 0})
	return plan
}

func minesweeper(number int, rng *rand.Rand) []action {
	p := player(number)
	const rows, cols, mineCount = 4, 4, 3
	permutation := rng.Perm(rows * cols)
	mines := map[int]bool{}
	for _, cell := range permutation[:mineCount] {
		mines[cell] = true
	}
	plan := []action{{p, "MATCH_STARTED", map[string]any{"rows": rows, "columns": cols, "mineCount": mineCount, "difficulty": "EASY"}, 0}}
	safe := make([]int, 0, rows*cols-mineCount)
	for _, cell := range permutation[mineCount:] {
		safe = append(safe, cell)
	}
	if rng.Intn(2) == 0 {
		cell := safe[0]
		plan = append(plan, action{p, "FLAG_PLACED", map[string]any{"row": cell / cols, "column": cell % cols, "correct": false}, 0})
		plan = append(plan, action{p, "FLAG_REMOVED", map[string]any{"row": cell / cols, "column": cell % cols}, 0})
	}
	for _, cell := range permutation[:mineCount] {
		plan = append(plan, action{p, "FLAG_PLACED", map[string]any{"row": cell / cols, "column": cell % cols, "correct": true}, 0})
	}
	win := rng.Intn(4) != 0
	limit := len(safe)
	if !win {
		limit = 2 + rng.Intn(4)
	}
	for _, cell := range safe[:limit] {
		adjacent := 0
		for dr := -1; dr <= 1; dr++ {
			for dc := -1; dc <= 1; dc++ {
				r, c := cell/cols+dr, cell%cols+dc
				if r >= 0 && r < rows && c >= 0 && c < cols && mines[r*cols+c] {
					adjacent++
				}
			}
		}
		plan = append(plan, action{p, "CELL_REVEALED", map[string]any{"row": cell / cols, "column": cell % cols, "adjacentMines": adjacent}, 0})
	}
	result := "WIN"
	if win {
		plan = append(plan, action{p, "BOARD_COMPLETED", map[string]any{"difficulty": "EASY", "durationSeconds": len(plan)}, 0})
	} else {
		// Una bandera se retira antes de pisar la mina, para conservar la lógica del tablero.
		cell := permutation[0]
		plan = append(plan, action{p, "FLAG_REMOVED", map[string]any{"row": cell / cols, "column": cell % cols}, 0})
		plan = append(plan, action{p, "MINE_TRIGGERED", map[string]any{"row": cell / cols, "column": cell % cols}, 0})
		result = "LOSS"
	}
	plan = append(plan, action{p, "MATCH_FINISHED", map[string]any{"result": result, "durationSeconds": len(plan)}, 0})
	return plan
}

type move struct {
	side, piece, from, to, capture string
	check, mate                    bool
}

func chess(number int, rng *rand.Rand) []action {
	white, black := player(number), player(1000+number)
	plan := []action{
		{white, "MATCH_STARTED", map[string]any{"side": "WHITE"}, 0},
		{black, "MATCH_STARTED", map[string]any{"side": "BLACK"}, 0},
	}
	var moves []move
	whiteResult, blackResult := "WIN", "LOSS"
	switch rng.Intn(3) {
	case 0: // Mate del pastor: todas las jugadas pertenecen a una línea legal.
		moves = []move{{"WHITE", "PAWN", "e2", "e4", "", false, false}, {"BLACK", "PAWN", "e7", "e5", "", false, false},
			{"WHITE", "QUEEN", "d1", "h5", "", false, false}, {"BLACK", "KNIGHT", "b8", "c6", "", false, false},
			{"WHITE", "BISHOP", "f1", "c4", "", false, false}, {"BLACK", "KNIGHT", "g8", "f6", "", false, false},
			{"WHITE", "QUEEN", "h5", "f7", "PAWN", true, true}}
	case 1: // Mate del loco.
		moves = []move{{"WHITE", "PAWN", "f2", "f3", "", false, false}, {"BLACK", "PAWN", "e7", "e5", "", false, false},
			{"WHITE", "PAWN", "g2", "g4", "", false, false}, {"BLACK", "QUEEN", "d8", "h4", "", true, true}}
		whiteResult, blackResult = "LOSS", "WIN"
	case 2: // Repetición legal de caballos; tablas por acuerdo.
		moves = []move{{"WHITE", "KNIGHT", "g1", "f3", "", false, false}, {"BLACK", "KNIGHT", "g8", "f6", "", false, false},
			{"WHITE", "KNIGHT", "f3", "g1", "", false, false}, {"BLACK", "KNIGHT", "f6", "g8", "", false, false}}
		whiteResult, blackResult = "DRAW", "DRAW"
	}
	for i, m := range moves {
		p := white
		if m.side == "BLACK" {
			p = black
		}
		ply := i + 1
		plan = append(plan, action{p, "MOVE_PLAYED", map[string]any{"ply": ply, "piece": m.piece, "from": m.from, "to": m.to, "thinkTimeSeconds": 1 + rng.Intn(10)}, 0})
		if m.capture != "" {
			plan = append(plan, action{p, "PIECE_CAPTURED", map[string]any{"ply": ply, "capturedPiece": m.capture}, 0})
		}
		if m.check {
			plan = append(plan, action{p, "CHECK_GIVEN", map[string]any{"ply": ply}, 0})
		}
		if m.mate {
			plan = append(plan, action{p, "CHECKMATE", map[string]any{"ply": ply}, 0})
		}
	}
	duration := len(plan)
	plan = append(plan, action{white, "MATCH_FINISHED", map[string]any{"result": whiteResult, "durationSeconds": duration}, duration})
	plan = append(plan, action{black, "MATCH_FINISHED", map[string]any{"result": blackResult, "durationSeconds": duration}, duration})
	return plan
}

func memory(number int, rng *rand.Rand) []action {
	p := player(number)
	const pairs, cols, timeLimit = 3, 3, 40
	symbols := []string{"STAR", "MOON", "SUN"}
	cards := append(append([]string{}, symbols...), symbols...)
	rng.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })
	plan := []action{{p, "MATCH_STARTED", map[string]any{"rows": 2, "columns": cols, "pairCount": pairs, "timeLimitSeconds": timeLimit}, 0}}
	flip := func(card, turn int) action {
		return action{p, "CARD_FLIPPED", map[string]any{"cardId": fmt.Sprintf("C%d", card), "symbol": cards[card], "turn": turn}, 0}
	}
	paired := map[int]bool{}
	turn := 1
	// Primer intento fallido con dos símbolos diferentes; las cartas quedan recordadas.
	first, second := 0, 1
	if cards[first] == cards[second] {
		for second = 2; cards[first] == cards[second]; second++ {
		}
	}
	plan = append(plan, flip(first, turn), flip(second, turn), action{p, "PAIR_MISMATCHED", map[string]any{"firstCardId": "C0", "secondCardId": fmt.Sprintf("C%d", second), "turn": turn}, 0})
	turn++
	order := rng.Perm(len(cards))
	lose := rng.Intn(4) == 0
	found := 0
	for _, i := range order {
		if paired[i] {
			continue
		}
		if lose && found == 2 {
			break
		}
		j := -1
		for k := range cards {
			if k != i && !paired[k] && cards[k] == cards[i] {
				j = k
				break
			}
		}
		paired[i], paired[j] = true, true
		plan = append(plan, flip(i, turn), flip(j, turn), action{p, "PAIR_MATCHED", map[string]any{"firstCardId": fmt.Sprintf("C%d", i), "secondCardId": fmt.Sprintf("C%d", j), "turn": turn}, 0})
		turn++
		found++
	}
	result := "WIN"
	duration := len(plan)
	if lose {
		plan = append(plan, action{p, "TIME_EXPIRED", map[string]any{"pairsFound": found}, timeLimit})
		result, duration = "LOSS", timeLimit
	}
	finishAt := 0
	if lose {
		finishAt = timeLimit
	}
	plan = append(plan, action{p, "MATCH_FINISHED", map[string]any{"result": result, "durationSeconds": duration}, finishAt})
	return plan
}
