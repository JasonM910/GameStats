// Package api añade transporte HTTP sin modificar el flujo por lotes.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"

	"gamestats/internal/events"
	"gamestats/internal/exchange"
	"gamestats/internal/matches"
)

type Store struct {
	mu      sync.Mutex
	path    string
	history []events.IndexedEvent
}

func OpenStore(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var doc exchange.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("leer histórico: %w", err)
	}
	if doc.SchemaVersion != exchange.SchemaVersion || doc.Matches == nil {
		return nil, fmt.Errorf("histórico debe usar el documento partidas.json schemaVersion 1")
	}
	seen := map[string]bool{}
	for _, match := range doc.Matches {
		for _, event := range match.Events {
			if err := event.Validate(); err != nil {
				return nil, fmt.Errorf("histórico %s: %w", event.EventID, err)
			}
			if event.MatchID != match.MatchID || event.GameID != match.GameID || seen[event.EventID] {
				return nil, fmt.Errorf("histórico inconsistente en evento %s", event.EventID)
			}
			seen[event.EventID] = true
			s.history = append(s.history, events.IndexedEvent{Index: len(s.history) + 1, Event: event})
		}
	}
	_, rejected := matches.Process(s.history)
	if len(rejected) > 0 {
		return nil, fmt.Errorf("histórico contiene partidas incompatibles")
	}
	return s, nil
}

type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

func equalEvent(a, b events.Event) bool {
	var ad, bd any
	for _, item := range []struct {
		data   json.RawMessage
		target *any
	}{{a.Data, &ad}, {b.Data, &bd}} {
		decoder := json.NewDecoder(bytes.NewReader(item.data))
		decoder.UseNumber()
		_ = decoder.Decode(item.target)
	}
	a.Data, b.Data = nil, nil
	return reflect.DeepEqual(a, b) && reflect.DeepEqual(ad, bd)
}

// Add confirma el lote completo en disco antes de responder. Repetir un eventId
// idéntico es seguro, pero reutilizarlo con otro contenido produce conflicto.
func (s *Store) Add(batch []events.Event) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	candidate := append([]events.IndexedEvent{}, s.history...)
	seen := map[string]events.Event{}
	for _, item := range candidate {
		seen[item.Event.EventID] = item.Event
	}
	accepted := make([]string, 0, len(batch))
	for _, event := range batch {
		if err := event.Validate(); err != nil {
			return nil, err
		}
		if previous, exists := seen[event.EventID]; exists {
			if !equalEvent(previous, event) {
				return nil, &ConflictError{"eventId ya tiene otro contenido: " + event.EventID}
			}
			accepted = append(accepted, event.EventID)
			continue
		}
		if err := validateSequence(candidate, event); err != nil {
			return nil, &ConflictError{err.Error()}
		}
		seen[event.EventID] = event
		candidate = append(candidate, events.IndexedEvent{Index: len(candidate) + 1, Event: event})
		accepted = append(accepted, event.EventID)
	}
	if len(candidate) == len(s.history) {
		return accepted, nil
	}
	states, rejected := matches.Process(candidate)
	if len(rejected) > 0 {
		return nil, &ConflictError{rejected[0].Reason}
	}
	if err := exchange.WriteFile(s.path, states); err != nil {
		return nil, fmt.Errorf("persistir eventos: %w", err)
	}
	s.history = candidate
	return accepted, nil
}

func validateSequence(history []events.IndexedEvent, event events.Event) error {
	started, finished := false, false
	participants := map[string]bool{}
	side := ""
	if event.GameID == "CHESS" && event.Type == "MATCH_STARTED" {
		var data struct {
			Side string `json:"side"`
		}
		_ = json.Unmarshal(event.Data, &data)
		side = data.Side
	}
	for _, item := range history {
		old := item.Event
		if old.MatchID != event.MatchID {
			continue
		}
		if old.GameID != event.GameID {
			return fmt.Errorf("matchId ya pertenece a %s", old.GameID)
		}
		participants[old.PlayerID] = true
		if side != "" && old.Type == "MATCH_STARTED" {
			var data struct {
				Side string `json:"side"`
			}
			_ = json.Unmarshal(old.Data, &data)
			if side == data.Side {
				return fmt.Errorf("el lado de Ajedrez ya tiene jugador")
			}
		}
		if old.PlayerID == event.PlayerID {
			started = started || old.Type == "MATCH_STARTED"
			finished = finished || old.Type == "MATCH_FINISHED"
		}
	}
	if finished {
		return fmt.Errorf("el jugador ya finalizó esta partida")
	}
	if event.Type == "MATCH_STARTED" {
		if started {
			return fmt.Errorf("el jugador ya inició esta partida")
		}
		limit := 1
		if event.GameID == "CHESS" {
			limit = 2
		}
		if len(participants) >= limit {
			return fmt.Errorf("la partida ya tiene todos sus jugadores")
		}
	} else if !started {
		return fmt.Errorf("falta MATCH_STARTED del jugador")
	}
	return nil
}

func (s *Store) Snapshot(gameID, playerID string) exchange.Document {
	s.mu.Lock()
	defer s.mu.Unlock()
	states, _ := matches.Process(s.history)
	document := exchange.FromStates(states)
	filtered := make([]exchange.Match, 0)
	for _, match := range document.Matches {
		if gameID != "" && match.GameID != gameID {
			continue
		}
		if playerID != "" {
			found := false
			for _, id := range match.Players {
				found = found || id == playerID
			}
			if !found {
				continue
			}
		}
		// Conservar todos los jugadores de una partida de Ajedrez al filtrar.
		filtered = append(filtered, match)
	}
	document.Matches = filtered
	return document
}

// Detecta JSON adicional y claves desconocidas en la API, sin endurecer el lector histórico.
func decodeBatch(data []byte) ([]events.Event, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var batch []events.Event
	if err := decoder.Decode(&batch); err != nil {
		return nil, err
	}
	if batch == nil || len(batch) == 0 || len(batch) > 500 {
		return nil, fmt.Errorf("se requiere un arreglo de 1 a 500 eventos")
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return nil, fmt.Errorf("contenido adicional después del arreglo")
	} else if err != io.EOF {
		return nil, err
	}
	for _, event := range batch {
		if err := event.Validate(); err != nil {
			return nil, err
		}
	}
	return batch, nil
}
