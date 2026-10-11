package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"gamestats/internal/events"
	"gamestats/internal/exchange"
	"gamestats/internal/matches"
)

func main() {
	input := flag.String("input", "eventos.json", "archivo JSON de eventos")
	output := flag.String("output", "salida/partidas.json", "archivo JSON para Scala")
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

	printReport(*input, *output, result.Total, states, rejected)
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
