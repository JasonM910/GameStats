package events

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type fieldKind uint8

const (
	textField fieldKind = iota
	boolField
	nonNegativeIntField
	positiveIntField
	chessSideField
	chessPieceField
	chessSquareField
	chessResultField
	memoryResultField
)

type fieldSpec struct {
	name string
	kind fieldKind
}

var matchFinishedFields = []fieldSpec{
	{"result", textField}, {"durationSeconds", nonNegativeIntField},
}

// El catálogo se basa en el formato definitivo de eventos.json.
// Cada tipo admite únicamente los campos indicados en su data.
var dataSchemas = map[string]map[string][]fieldSpec{
	"PACMAN": {
		"MATCH_STARTED":      {},
		"MATCH_FINISHED":     matchFinishedFields,
		"PELLET_EATEN":       {{"points", nonNegativeIntField}},
		"POWER_PELLET_EATEN": {{"points", nonNegativeIntField}, {"powerDurationSeconds", positiveIntField}},
		"GHOST_EATEN":        {{"ghost", textField}, {"points", nonNegativeIntField}},
		"FRUIT_EATEN":        {{"fruit", textField}, {"points", nonNegativeIntField}},
		"LIFE_LOST":          {{"remainingLives", nonNegativeIntField}, {"ghost", textField}},
		"LEVEL_COMPLETED":    {{"level", positiveIntField}, {"durationSeconds", nonNegativeIntField}},
	},
	"MINESWEEPER": {
		"MATCH_STARTED":   {{"rows", positiveIntField}, {"columns", positiveIntField}, {"mineCount", nonNegativeIntField}, {"difficulty", textField}},
		"MATCH_FINISHED":  matchFinishedFields,
		"CELL_REVEALED":   {{"row", nonNegativeIntField}, {"column", nonNegativeIntField}, {"adjacentMines", nonNegativeIntField}},
		"FLAG_PLACED":     {{"row", nonNegativeIntField}, {"column", nonNegativeIntField}, {"correct", boolField}},
		"FLAG_REMOVED":    {{"row", nonNegativeIntField}, {"column", nonNegativeIntField}},
		"MINE_TRIGGERED":  {{"row", nonNegativeIntField}, {"column", nonNegativeIntField}},
		"BOARD_COMPLETED": {{"difficulty", textField}, {"durationSeconds", nonNegativeIntField}},
	},
	"CHESS": {
		"MATCH_STARTED":  {{"side", chessSideField}},
		"MOVE_PLAYED":    {{"ply", positiveIntField}, {"piece", chessPieceField}, {"from", chessSquareField}, {"to", chessSquareField}, {"thinkTimeSeconds", nonNegativeIntField}},
		"PIECE_CAPTURED": {{"ply", positiveIntField}, {"capturedPiece", chessPieceField}},
		"CHECK_GIVEN":    {{"ply", positiveIntField}},
		"CHECKMATE":      {{"ply", positiveIntField}},
		"MATCH_FINISHED": {{"result", chessResultField}, {"durationSeconds", nonNegativeIntField}},
	},
	"MEMORY": {
		"MATCH_STARTED":   {{"rows", positiveIntField}, {"columns", positiveIntField}, {"pairCount", positiveIntField}, {"timeLimitSeconds", positiveIntField}},
		"CARD_FLIPPED":    {{"cardId", textField}, {"symbol", textField}, {"turn", positiveIntField}},
		"PAIR_MATCHED":    {{"firstCardId", textField}, {"secondCardId", textField}, {"turn", positiveIntField}},
		"PAIR_MISMATCHED": {{"firstCardId", textField}, {"secondCardId", textField}, {"turn", positiveIntField}},
		"TIME_EXPIRED":    {{"pairsFound", nonNegativeIntField}},
		"MATCH_FINISHED":  {{"result", memoryResultField}, {"durationSeconds", nonNegativeIntField}},
	},
}

func validateData(gameID, eventType string, data json.RawMessage) error {
	types, ok := dataSchemas[gameID]
	if !ok {
		return fmt.Errorf("gameId %q no está soportado", gameID)
	}
	fields, ok := types[eventType]
	if !ok {
		return fmt.Errorf("type %q no está definido para %s", eventType, gameID)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("data inválido: %w", err)
	}
	allowed := make(map[string]bool, len(fields))
	for _, field := range fields {
		allowed[field.name] = true
		raw, exists := values[field.name]
		if !exists {
			return fmt.Errorf("falta data.%s", field.name)
		}
		if err := validateField(field, raw); err != nil {
			return fmt.Errorf("data.%s: %w", field.name, err)
		}
	}
	for name := range values {
		if !allowed[name] {
			return fmt.Errorf("data.%s no está definido para %s", name, eventType)
		}
	}
	if gameID == "CHESS" && eventType == "MOVE_PLAYED" && stringValue(values["from"]) == stringValue(values["to"]) {
		return fmt.Errorf("data.from y data.to deben ser casillas diferentes")
	}
	if gameID == "MEMORY" && (eventType == "PAIR_MATCHED" || eventType == "PAIR_MISMATCHED") &&
		stringValue(values["firstCardId"]) == stringValue(values["secondCardId"]) {
		return fmt.Errorf("data.firstCardId y data.secondCardId deben ser cartas diferentes")
	}
	return nil
}

func stringValue(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value) // Los campos ya fueron validados como texto.
	return value
}

func validateField(field fieldSpec, raw json.RawMessage) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("no puede ser null")
	}
	switch field.kind {
	case textField, chessSideField, chessPieceField, chessSquareField, chessResultField, memoryResultField:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("debe ser texto")
		}
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("no puede estar vacío")
		}
		switch field.kind {
		case chessSideField:
			if value != "WHITE" && value != "BLACK" {
				return fmt.Errorf("debe ser WHITE o BLACK")
			}
		case chessPieceField:
			switch value {
			case "PAWN", "KNIGHT", "BISHOP", "ROOK", "QUEEN", "KING":
			default:
				return fmt.Errorf("debe ser una pieza de ajedrez conocida")
			}
		case chessSquareField:
			if len(value) != 2 || value[0] < 'a' || value[0] > 'h' || value[1] < '1' || value[1] > '8' {
				return fmt.Errorf("debe ser una casilla entre a1 y h8")
			}
		case chessResultField:
			if value != "WIN" && value != "LOSS" && value != "DRAW" {
				return fmt.Errorf("debe ser WIN, LOSS o DRAW")
			}
		case memoryResultField:
			if value != "WIN" && value != "LOSS" {
				return fmt.Errorf("debe ser WIN o LOSS")
			}
		}
	case boolField:
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("debe ser booleano (true o false)")
		}
	case nonNegativeIntField, positiveIntField:
		var value int64
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("debe ser un entero")
		}
		if field.kind == nonNegativeIntField && value < 0 {
			return fmt.Errorf("debe ser mayor o igual a cero")
		}
		if field.kind == positiveIntField && value <= 0 {
			return fmt.Errorf("debe ser mayor que cero")
		}
	}
	return nil
}
