// Package exchange define el archivo JSON que Go entrega al módulo Scala.
package exchange

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gamestats/internal/events"
	"gamestats/internal/matches"
)

const SchemaVersion = 1

type Document struct {
	SchemaVersion int     `json:"schemaVersion"`
	Matches       []Match `json:"matches"`
}

type Match struct {
	GameID  string         `json:"gameId"`
	MatchID string         `json:"matchId"`
	Players []string       `json:"players"`
	Events  []events.Event `json:"events"`
}

// FromStates crea un documento estable: las partidas se ordenan por matchId,
// mientras que jugadores y eventos mantienen su orden de llegada.
func FromStates(states map[string]matches.State) Document {
	ids := make([]string, 0, len(states))
	for id := range states {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	document := Document{SchemaVersion: SchemaVersion, Matches: make([]Match, 0, len(ids))}
	for _, id := range ids {
		state := states[id]
		document.Matches = append(document.Matches, Match{
			GameID: state.GameID, MatchID: state.MatchID,
			Players: append([]string{}, state.Players...),
			Events:  append([]events.Event{}, state.Events...),
		})
	}
	return document
}

// WriteFile escribe el JSON completo en un archivo temporal y lo renombra al
// destino cuando la escritura termina, para evitar que Scala lea un archivo parcial.
func WriteFile(path string, states map[string]matches.State) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("crear directorio de salida: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".gamestats-*.tmp")
	if err != nil {
		return fmt.Errorf("crear archivo temporal: %w", err)
	}
	defer os.Remove(temp.Name())

	if err := temp.Chmod(0644); err != nil {
		temp.Close()
		return fmt.Errorf("preparar permisos del archivo: %w", err)
	}
	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(FromStates(states)); err != nil {
		temp.Close()
		return fmt.Errorf("codificar partidas: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("cerrar archivo temporal: %w", err)
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return fmt.Errorf("publicar archivo de partidas: %w", err)
	}
	return nil
}
