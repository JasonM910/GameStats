// Package events lee y valida los eventos que recibe el módulo Go.
package events

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Event conserva data como JSON para que cada juego pueda definir sus propios campos.
type Event struct {
	EventID     string          `json:"eventId"`
	Timestamp   string          `json:"timestamp"`
	GameID      string          `json:"gameId"`
	GameVersion string          `json:"gameVersion"`
	MatchID     string          `json:"matchId"`
	PlayerID    string          `json:"playerId"`
	Type        string          `json:"type"`
	Data        json.RawMessage `json:"data"`
}

// RejectedEvent identifica un objeto rechazado sin detener la lectura del archivo.
type RejectedEvent struct {
	Index   int    // Posición en el arreglo JSON, comenzando en 1.
	EventID string // Puede estar vacío si el objeto no se pudo convertir.
	Reason  string
}

// IndexedEvent conserva la posición original para informar errores posteriores.
type IndexedEvent struct {
	Index int
	Event Event
}

type ReadResult struct {
	Total    int
	Events   []IndexedEvent
	Rejected []RejectedEvent
}

// Validate comprueba el contrato común. Los campos específicos de cada juego
// permanecen en data para su interpretación posterior.
func (e Event) Validate() error {
	for _, field := range []struct{ name, value string }{
		{"eventId", e.EventID},
		{"timestamp", e.Timestamp},
		{"gameId", e.GameID},
		{"gameVersion", e.GameVersion},
		{"matchId", e.MatchID},
		{"playerId", e.PlayerID},
		{"type", e.Type},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("falta %s o está vacío", field.name)
		}
	}
	if _, err := time.Parse(time.RFC3339, e.Timestamp); err != nil {
		return fmt.Errorf("timestamp debe usar RFC3339: %w", err)
	}
	if len(e.Data) == 0 || !json.Valid(e.Data) || e.Data[0] != '{' {
		return fmt.Errorf("data debe ser un objeto JSON (puede estar vacío)")
	}
	return nil
}

// ReadAndValidate acepta un arreglo JSON. Un objeto inválido se registra en
// Rejected y los demás continúan hacia el procesamiento de partidas.
func ReadAndValidate(reader io.Reader) (ReadResult, error) {
	var rawEvents []json.RawMessage
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&rawEvents); err != nil {
		return ReadResult{}, fmt.Errorf("el archivo debe contener un arreglo JSON válido: %w", err)
	}
	if rawEvents == nil {
		return ReadResult{}, fmt.Errorf("el archivo debe contener un arreglo JSON, no null")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return ReadResult{}, fmt.Errorf("hay contenido adicional después del arreglo JSON")
		}
		return ReadResult{}, fmt.Errorf("contenido adicional inválido: %w", err)
	}

	result := ReadResult{Total: len(rawEvents)}
	seenIDs := make(map[string]bool)
	for i, raw := range rawEvents {
		var event Event
		if err := json.Unmarshal(raw, &event); err != nil {
			result.Rejected = append(result.Rejected, RejectedEvent{i + 1, "", err.Error()})
			continue
		}
		if err := event.Validate(); err != nil {
			result.Rejected = append(result.Rejected, RejectedEvent{i + 1, event.EventID, err.Error()})
			continue
		}
		if seenIDs[event.EventID] {
			result.Rejected = append(result.Rejected, RejectedEvent{i + 1, event.EventID, "eventId duplicado"})
			continue
		}
		seenIDs[event.EventID] = true
		result.Events = append(result.Events, IndexedEvent{Index: i + 1, Event: event})
	}
	return result, nil
}
