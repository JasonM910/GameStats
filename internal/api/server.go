package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Server struct {
	Store         *Store
	AnalyzerURL   string
	AllowedOrigin string
	ResultsPath   string
	Client        *http.Client
	statsMu       sync.Mutex
}

func NewServer(store *Store, analyzerURL, origin, resultsPath string) *Server {
	return &Server{Store: store, AnalyzerURL: analyzerURL, AllowedOrigin: origin,
		ResultsPath: resultsPath, Client: &http.Client{Timeout: 15 * time.Second}}
}

func respond(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
func fail(w http.ResponseWriter, status int, code, message string) {
	respond(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" && s.AllowedOrigin != "" {
		if origin != s.AllowedOrigin {
			fail(w, 403, "origin_denied", "Origen no permitido")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch r.URL.Path {
	case "/health":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			fail(w, 405, "method_not_allowed", "Usa GET")
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, strings.TrimSuffix(s.AnalyzerURL, "/analyze")+"/health", nil)
		if err != nil {
			fail(w, 503, "analyzer_unavailable", "URL de Scala inválida")
			return
		}
		response, err := s.Client.Do(req)
		if err != nil {
			fail(w, 503, "analyzer_unavailable", "Scala no está disponible")
			return
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			fail(w, 503, "analyzer_unavailable", "Scala no está listo")
			return
		}
		respond(w, 200, map[string]string{"status": "ok"})
	case "/api/events":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			fail(w, 405, "method_not_allowed", "Usa POST")
			return
		}
		mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if mediaType != "application/json" {
			fail(w, 415, "invalid_content_type", "Usa application/json")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024*1024))
		if err != nil {
			fail(w, 413, "body_too_large", "Límite de solicitud: 1 MiB")
			return
		}
		batch, err := decodeBatch(body)
		if err != nil {
			fail(w, 400, "invalid_events", err.Error())
			return
		}
		ids, err := s.Store.Add(batch)
		if err != nil {
			var conflict *ConflictError
			if errors.As(err, &conflict) {
				fail(w, 409, "event_conflict", err.Error())
			} else {
				fail(w, 500, "storage_error", "No se pudieron guardar los eventos")
			}
			return
		}
		respond(w, 200, map[string]any{"status": "accepted", "eventIds": ids})
	case "/api/stats":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			fail(w, 405, "method_not_allowed", "Usa GET")
			return
		}
		s.stats(w, r)
	default:
		fail(w, 404, "not_found", "Endpoint inexistente")
	}
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	for key := range r.URL.Query() {
		if key != "gameId" && key != "playerId" {
			fail(w, 400, "invalid_filter", "Filtros admitidos: gameId, playerId")
			return
		}
	}
	gameID, playerID := r.URL.Query().Get("gameId"), r.URL.Query().Get("playerId")
	if gameID != "" && gameID != "CHESS" && gameID != "PACMAN" && gameID != "MEMORY" && gameID != "MINESWEEPER" {
		fail(w, 400, "invalid_filter", "Juego desconocido")
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	document := s.Store.Snapshot(gameID, playerID)
	body, _ := json.Marshal(document)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, s.AnalyzerURL, bytes.NewReader(body))
	if err != nil {
		fail(w, 503, "analyzer_unavailable", "URL de Scala inválida")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := s.Client.Do(req)
	if err != nil {
		fail(w, 503, "analyzer_unavailable", "No se pudo consultar Scala; los eventos siguen guardados")
		return
	}
	defer response.Body.Close()
	result, err := io.ReadAll(io.LimitReader(response.Body, 32*1024*1024+1))
	if err != nil || len(result) > 32*1024*1024 || response.StatusCode != 200 || !json.Valid(result) {
		fail(w, 502, "analyzer_error", "Scala no pudo procesar el histórico")
		return
	}
	if gameID == "" && playerID == "" && s.ResultsPath != "" {
		if err := writeResult(s.ResultsPath, result); err != nil {
			fail(w, 500, "storage_error", "No se pudo guardar resultados.json")
			return
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(result)
}

func writeResult(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".results-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("publicar resultados: %w", err)
	}
	return nil
}
