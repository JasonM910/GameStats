// Package matches mantiene el estado temporal de partidas en goroutines.
package matches

import (
	"fmt"

	"gamestats/internal/events"
)

// State reúne eventos y jugadores de una partida. Events conserva el orden de
// entrada, aunque las partidas se procesen simultáneamente.
type State struct {
	MatchID  string
	GameID   string
	Players  []string
	Events   []events.Event
	Started  bool
	Finished bool
}

type worker struct {
	gameID string
	input  chan events.Event
}

// Process asigna una goroutine y un channel a cada matchId. Cada goroutine es
// la única que modifica el estado de su partida, evitando acceso compartido.
func Process(input []events.IndexedEvent) (map[string]State, []events.RejectedEvent) {
	workers := make(map[string]worker)
	results := make(chan State, len(input))
	var rejected []events.RejectedEvent

	for _, item := range input {
		event := item.Event
		w, exists := workers[event.MatchID]
		if exists && w.gameID != event.GameID {
			rejected = append(rejected, events.RejectedEvent{
				Index: item.Index, EventID: event.EventID,
				Reason: fmt.Sprintf("matchId %s ya pertenece al juego %s", event.MatchID, w.gameID),
			})
			continue
		}
		if !exists {
			w = worker{gameID: event.GameID, input: make(chan events.Event, 16)}
			workers[event.MatchID] = w
			go collect(event.MatchID, event.GameID, w.input, results)
		}
		w.input <- event
	}

	for _, w := range workers {
		close(w.input)
	}
	states := make(map[string]State, len(workers))
	for range workers {
		state := <-results
		states[state.MatchID] = state
	}
	return states, rejected
}

func collect(matchID, gameID string, input <-chan events.Event, results chan<- State) {
	state := State{MatchID: matchID, GameID: gameID}
	seenPlayers := make(map[string]bool)
	for event := range input {
		state.Events = append(state.Events, event)
		if !seenPlayers[event.PlayerID] {
			seenPlayers[event.PlayerID] = true
			state.Players = append(state.Players, event.PlayerID)
		}
		switch event.Type {
		case "MATCH_STARTED":
			state.Started = true
		case "MATCH_FINISHED":
			state.Finished = true
		}
	}
	results <- state
}
