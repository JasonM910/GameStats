package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"gamestats/internal/events"
	"gamestats/internal/exchange"
)

func sample(id, kind, data string) events.Event {
	return events.Event{EventID: id, Timestamp: "2026-11-16T12:00:00Z", GameID: "PACMAN", GameVersion: "0.1.0", MatchID: "P1", PlayerID: "PLAYER_ONE", Type: kind, Data: json.RawMessage(data)}
}
func setup(t *testing.T) (*Server, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "partidas.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	return NewServer(store, "http://127.0.0.1:1/analyze", "", filepath.Join(filepath.Dir(path), "resultados.json")), path
}
func request(server http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	out := httptest.NewRecorder()
	server.ServeHTTP(out, req)
	return out
}
func post(server http.Handler, batch ...events.Event) *httptest.ResponseRecorder {
	data, _ := json.Marshal(batch)
	return request(server, "POST", "/api/events", string(data))
}

func TestPersistRetryAndRestart(t *testing.T) {
	server, path := setup(t)
	batch := []events.Event{sample("E1", "MATCH_STARTED", "{}"), sample("E2", "PELLET_EATEN", `{"points":10}`), sample("E3", "MATCH_FINISHED", `{"result":"WIN","durationSeconds":2}`)}
	for i := 0; i < 2; i++ {
		if response := post(server, batch...); response.Code != 200 {
			t.Fatal(response.Body.String())
		}
	}
	restored, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := restored.Snapshot("", "")
	if len(doc.Matches) != 1 || len(doc.Matches[0].Events) != 3 {
		t.Fatalf("duplicados o pérdida: %+v", doc)
	}
	changed := batch[1]
	changed.Data = json.RawMessage(`{"points":99}`)
	if out := post(server, changed); out.Code != 409 {
		t.Fatal(out.Code, out.Body.String())
	}
	if out := post(server, sample("E4", "PELLET_EATEN", `{"points":10}`)); out.Code != 409 {
		t.Fatal(out.Code)
	}
}

func TestValidationAndAtomicBatch(t *testing.T) {
	server, path := setup(t)
	invalid := sample("E2", "PELLET_EATEN", `{"points":-1}`)
	if out := post(server, sample("E1", "MATCH_STARTED", "{}"), invalid); out.Code != 400 {
		t.Fatal(out.Code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("se guardó un lote inválido")
	}
	if out := post(server, sample("E2", "PELLET_EATEN", `{"points":1}`)); out.Code != 409 {
		t.Fatal(out.Code)
	}
	for _, body := range []string{"null", "[]", "{}", "[] {}", `[{"eventId":"E","unknown":true}]`} {
		if out := request(server, "POST", "/api/events", body); out.Code != 400 {
			t.Fatal(body, out.Code)
		}
	}
	if out := request(server, "GET", "/api/events", ""); out.Code != 405 {
		t.Fatal(out.Code)
	}
	if out := request(server, "GET", "/missing", ""); out.Code != 404 {
		t.Fatal(out.Code)
	}
	if out := request(server, "GET", "/api/stats?gameId=UNKNOWN", ""); out.Code != 400 {
		t.Fatal(out.Code)
	}
	if out := request(server, "GET", "/api/stats", ""); out.Code != 503 {
		t.Fatal(out.Code)
	}
}

func TestConcurrentRetries(t *testing.T) {
	server, _ := setup(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if out := post(server, sample("E1", "MATCH_STARTED", "{}")); out.Code != 200 {
				t.Error(out.Code)
			}
		}()
	}
	wg.Wait()
	if got := len(server.Store.Snapshot("", "").Matches[0].Events); got != 1 {
		t.Fatal(got)
	}
}

func TestStatsUsesScalaAndPreservesChessOpponents(t *testing.T) {
	server, path := setup(t)
	white := sample("W1", "MATCH_STARTED", `{"side":"WHITE"}`)
	white.GameID = "CHESS"
	white.PlayerID = "WHITE"
	black := white
	black.EventID = "B1"
	black.PlayerID = "BLACK"
	black.Data = json.RawMessage(`{"side":"BLACK"}`)
	if out := post(server, white, black); out.Code != 200 {
		t.Fatal(out.Body.String())
	}
	analyzer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			fmt.Fprint(w, `{"status":"ok"}`)
			return
		}
		var doc exchange.Document
		if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
			t.Error(err)
		}
		if len(doc.Matches) != 1 || len(doc.Matches[0].Players) != 2 {
			t.Errorf("se truncaron participantes: %+v", doc)
		}
		fmt.Fprint(w, `{"schemaVersion":1,"players":[],"matches":[],"games":[],"metrics":[],"patterns":[]}`)
	}))
	defer analyzer.Close()
	server.AnalyzerURL = analyzer.URL + "/analyze"
	if out := request(server, "GET", "/api/stats?gameId=CHESS&playerId=WHITE", ""); out.Code != 200 {
		t.Fatal(out.Code, out.Body.String())
	}
	if out := request(server, "GET", "/api/stats", ""); out.Code != 200 {
		t.Fatal(out.Code)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "resultados.json")); err != nil {
		t.Fatal(err)
	}
	if out := request(server, "GET", "/health", ""); out.Code != 200 {
		t.Fatal(out.Code)
	}
}

func TestBrokenHistoryIsNeverOverwritten(t *testing.T) {
	_, path := setup(t)
	if err := os.WriteFile(path, []byte(`{"schemaVersion":99,"matches":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenStore(path); err == nil {
		t.Fatal("se aceptó versión desconocida")
	}
}

func TestLargeIntegerIsNotAnIdenticalRetry(t *testing.T) {
	server, _ := setup(t)
	if out := post(server, sample("E1", "MATCH_STARTED", "{}"), sample("E2", "PELLET_EATEN", `{"points":9007199254740992}`)); out.Code != 200 {
		t.Fatal(out.Body.String())
	}
	if out := post(server, sample("E2", "PELLET_EATEN", `{"points":9007199254740993}`)); out.Code != 409 {
		t.Fatal("se confundieron enteros distintos", out.Code)
	}
}
