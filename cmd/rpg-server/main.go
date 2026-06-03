package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/sizolity/worldline/internal/world/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	workspace := flag.String("workspace", "", "workspace directory (required)")
	flag.Parse()

	if *workspace == "" {
		fmt.Fprintln(os.Stderr, "rpg-server: --workspace is required")
		os.Exit(2)
	}

	fs := store.NewFileStore(*workspace)
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /worlds/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		worldID := r.PathValue("id")
		if worldID == "" {
			http.Error(w, `{"error":"world id required"}`, http.StatusBadRequest)
			return
		}
		world, err := fs.LoadSnapshot(r.Context(), worldID)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"world_id": world.ID,
			"name":     world.Name,
			"clock":    world.Clock,
			"entities": len(world.Entities),
			"threads":  len(world.Threads),
			"events":   len(world.EventLog),
			"memories": len(world.Memories),
		})
	})

	mux.HandleFunc("POST /worlds/{id}/beat", func(w http.ResponseWriter, r *http.Request) {
		worldID := r.PathValue("id")
		if worldID == "" {
			http.Error(w, `{"error":"world id required"}`, http.StatusBadRequest)
			return
		}
		var body struct {
			Input string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Input) == "" {
			http.Error(w, `{"error":"input is required"}`, http.StatusBadRequest)
			return
		}
		// Beat execution requires LLM setup — return placeholder for now
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"world_id": worldID,
			"input":    body.Input,
			"status":   "not_implemented",
			"message":  "Beat execution requires LLM configuration. Full implementation pending.",
		})
	})

	srv := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("rpg-server listening on %s (workspace: %s)", *addr, *workspace)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
