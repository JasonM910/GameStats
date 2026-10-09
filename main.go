package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gamestats/internal/events"
	"gamestats/internal/exchange"
	"gamestats/internal/matches"
)

func main() {
	input := flag.String("input", "eventos.json", "archivo JSON de eventos")
	output := flag.String("output", "partidas.json", "archivo JSON para Scala")
	flag.Parse()

	file, err := os.Open(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No se pudo abrir %s: %v\n", *input, err)
		os.Exit(1)
	}
	defer file.Close()
	inputPath, err := filepath.Abs(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ruta de entrada inválida: %v\n", err)
		os.Exit(1)
	}
	outputPath, err := filepath.Abs(*output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ruta de salida inválida: %v\n", err)
		os.Exit(1)
	}
	if inputPath == outputPath {
		fmt.Fprintln(os.Stderr, "La salida no puede ser el mismo archivo de entrada")
		os.Exit(1)
	}
	if outputInfo, err := os.Stat(outputPath); err == nil {
		inputInfo, err := file.Stat()
		if err != nil {
			fmt.Fprintf(os.Stderr, "No se pudo verificar la entrada: %v\n", err)
			os.Exit(1)
		}
		if os.SameFile(inputInfo, outputInfo) {
			fmt.Fprintln(os.Stderr, "La salida no puede ser el mismo archivo de entrada")
			os.Exit(1)
		}
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "No se pudo verificar la salida: %v\n", err)
		os.Exit(1)
	}

	result, err := events.ReadAndValidate(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No se pudo leer %s: %v\n", *input, err)
		os.Exit(1)
	}
	states, processingRejected := matches.Process(result.Events)
	rejected := append(result.Rejected, processingRejected...)
	sort.Slice(rejected, func(i, j int) bool { return rejected[i].Index < rejected[j].Index })
	if err := exchange.WriteFile(*output, states); err != nil {
		fmt.Fprintf(os.Stderr, "No se pudo escribir %s: %v\n", *output, err)
		os.Exit(1)
	}

	fmt.Printf("Eventos leídos: %d | válidos: %d | inválidos: %d\n",
		result.Total, result.Total-len(rejected), len(rejected))
	fmt.Printf("Archivo para Scala: %s\n", *output)

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
