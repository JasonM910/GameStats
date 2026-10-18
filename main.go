package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"gamestats/internal/events"
	"gamestats/internal/exchange"
	"gamestats/internal/matches"
	"gamestats/internal/simulator"
)

func main() {
	input := flag.String("input", "", "archivo JSON existente (opcional; desactiva el simulador)")
	output := flag.String("output", "salida/partidas.json", "archivo JSON para Scala")
	count := flag.Int("matches", 8, "cantidad de partidas simuladas")
	seed := flag.Int64("seed", time.Now().UnixNano(), "semilla aleatoria del simulador")
	interval := flag.Duration("interval", 100*time.Millisecond, "pausa entre eventos de una partida simulada")
	flag.Parse()

	if *input != "" {
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
		if inputInfo, err := os.Stat(inputPath); err == nil {
			if outputInfo, err := os.Stat(outputPath); err == nil && os.SameFile(inputInfo, outputInfo) {
				fmt.Fprintln(os.Stderr, "La salida no puede ser el mismo archivo de entrada")
				os.Exit(1)
			}
		}
	}

	var result events.ReadResult
	var states map[string]matches.State
	var processingRejected []events.RejectedEvent
	var err error
	source := *input
	if source == "" {
		source = fmt.Sprintf("simulador (%d partidas, semilla %d, intervalo %s)", *count, *seed, *interval)
		result, states, processingRejected, err = simulate(simulator.Options{Matches: *count, Seed: *seed, Interval: *interval})
	} else {
		file, openErr := os.Open(*input)
		if openErr != nil {
			fmt.Fprintf(os.Stderr, "No se pudo abrir %s: %v\n", *input, openErr)
			os.Exit(1)
		}
		defer file.Close()
		result, err = events.ReadAndValidate(file)
		if err == nil {
			states, processingRejected = matches.Process(result.Events)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "No se pudieron preparar los eventos: %v\n", err)
		os.Exit(1)
	}
	rejected := append(result.Rejected, processingRejected...)
	sort.Slice(rejected, func(i, j int) bool { return rejected[i].Index < rejected[j].Index })
	if err := exchange.WriteFile(*output, states); err != nil {
		fmt.Fprintf(os.Stderr, "No se pudo escribir %s: %v\n", *output, err)
		os.Exit(1)
	}

	printReport(source, *output, result.Total, states, rejected)
}

func simulate(options simulator.Options) (events.ReadResult, map[string]matches.State, []events.RejectedEvent, error) {
	stream, errors, err := simulator.Stream(context.Background(), options)
	if err != nil {
		return events.ReadResult{}, nil, nil, err
	}
	result := events.ReadResult{}
	seen := make(map[string]bool)
	collector := matches.NewAccumulator()
	var processingRejected []events.RejectedEvent
	fmt.Println("Generando eventos de hasta dos partidas activas...")
	for event := range stream {
		result.Total++
		if err := event.Validate(); err != nil {
			result.Rejected = append(result.Rejected, events.RejectedEvent{Index: result.Total, EventID: event.EventID, Reason: err.Error()})
			continue
		}
		if seen[event.EventID] {
			result.Rejected = append(result.Rejected, events.RejectedEvent{Index: result.Total, EventID: event.EventID, Reason: "eventId duplicado"})
			continue
		}
		seen[event.EventID] = true
		if err := collector.Add(events.IndexedEvent{Index: result.Total, Event: event}); err != nil {
			processingRejected = append(processingRejected, events.RejectedEvent{Index: result.Total, EventID: event.EventID, Reason: err.Error()})
			continue
		}
		fmt.Printf("  %-7s %-10s %-18s %s\n", event.MatchID, event.GameID, event.Type, event.PlayerID)
	}
	for err := range errors {
		if err != nil {
			return result, nil, processingRejected, err
		}
	}
	return result, collector.States(), processingRejected, nil
}

func printReport(input, output string, total int, states map[string]matches.State, rejected []events.RejectedEvent) {
	fmt.Println("=== GameStats | Go: preparación de eventos ===")
	fmt.Printf("Entrada: %s\n", input)
	fmt.Printf("Resumen: %s | %s | %s | %s\n",
		countLabel(total, "leído", "leídos"),
		countLabel(total-len(rejected), "válido", "válidos"),
		countLabel(len(rejected), "rechazado", "rechazados"),
		countLabel(len(states), "partida", "partidas"))
	fmt.Printf("Archivo para Scala: %s\n", output)
	fmt.Println("\nPartidas preparadas")
	if len(states) == 0 {
		fmt.Println("  (ninguna)")
	}

	matchIDs := make([]string, 0, len(states))
	for matchID := range states {
		matchIDs = append(matchIDs, matchID)
	}
	sort.Strings(matchIDs)
	table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if len(matchIDs) > 0 {
		fmt.Fprintln(table, "  ID\tJUEGO\tJUGADORES\tEVENTOS\tESTADO")
	}
	for _, matchID := range matchIDs {
		state := states[matchID]
		fmt.Fprintf(table, "  %s\t%s\t%s\t%d\t%s\n",
			matchID, state.GameID, strings.Join(state.Players, ", "), len(state.Events), matchStatus(state))
	}
	table.Flush()

	if len(rejected) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "\nEventos rechazados (%d)\n", len(rejected))
	for _, item := range rejected {
		id := item.EventID
		if id == "" {
			id = "sin eventId"
		}
		fmt.Fprintf(os.Stderr, "  #%d [%s] %s\n", item.Index, id, item.Reason)
	}
}

func matchStatus(state matches.State) string {
	switch {
	case state.Started && state.Finished:
		return "finalizada"
	case state.Finished:
		return "finalizada sin inicio"
	case state.Started:
		return "en curso"
	default:
		return "sin inicio"
	}
}

func countLabel(amount int, singular, plural string) string {
	if amount == 1 {
		return fmt.Sprintf("%d %s", amount, singular)
	}
	return fmt.Sprintf("%d %s", amount, plural)
}
