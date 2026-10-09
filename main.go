package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"gamestats/internal/events"
	"gamestats/internal/matches"
)

func main() {
	input := flag.String("input", "eventos.json", "archivo JSON de eventos")
	flag.Parse()

	file, err := os.Open(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No se pudo abrir %s: %v\n", *input, err)
		os.Exit(1)
	}
	defer file.Close()

	result, err := events.ReadAndValidate(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No se pudo leer %s: %v\n", *input, err)
		os.Exit(1)
	}
	states, processingRejected := matches.Process(result.Events)
	rejected := append(result.Rejected, processingRejected...)
	sort.Slice(rejected, func(i, j int) bool { return rejected[i].Index < rejected[j].Index })

	fmt.Printf("Eventos leídos: %d | válidos: %d | inválidos: %d\n",
		result.Total, result.Total-len(rejected), len(rejected))

	matchIDs := make([]string, 0, len(states))
	for matchID := range states {
		matchIDs = append(matchIDs, matchID)
	}
	sort.Strings(matchIDs)
	for _, matchID := range matchIDs {
		state := states[matchID]
		ids := make([]string, 0, len(state.Events))
		for _, event := range state.Events {
			ids = append(ids, event.EventID)
		}
		fmt.Printf("Partida %s (%s): %d eventos válidos | jugadores: %s | inicio: %t | fin: %t [%s]\n",
			matchID, state.GameID, len(state.Events), strings.Join(state.Players, ", "),
			state.Started, state.Finished, strings.Join(ids, ", "))
	}
	for _, item := range rejected {
		fmt.Fprintf(os.Stderr, "Evento %d (%s) inválido: %s\n",
			item.Index, item.EventID, item.Reason)
	}
}
