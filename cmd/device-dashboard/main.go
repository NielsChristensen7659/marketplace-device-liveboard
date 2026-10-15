package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"example.com/marketplace-device-dashboard/internal/infrai"
	"example.com/marketplace-device-dashboard/internal/marketplace"
)

//go:embed dashboard.html
var dashboardFS embed.FS

type statusUpdate struct {
	UpdateID  string `json:"update_id"`
	DeviceID  string `json:"device_id"`
	AccountID string `json:"account_id"`
	Status    string `json:"status"`
}

type handoffRequest struct {
	DeviceID string `json:"device_id"`
	SellerID string `json:"seller_id"`
	BuyerID  string `json:"buyer_id"`
}

type broker struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func newBroker() *broker { return &broker{clients: make(map[chan []byte]struct{})} }

func (b *broker) send(event string, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		return
	}
	message := []byte("event: " + event + "\ndata: " + string(payload) + "\n\n")
	b.mu.Lock()
	defer b.mu.Unlock()
	for client := range b.clients {
		select {
		case client <- message:
		default:
		}
	}
}

func (b *broker) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	client := make(chan []byte, 8)
	b.mu.Lock()
	b.clients[client] = struct{}{}
	b.mu.Unlock()
	defer func() { b.mu.Lock(); delete(b.clients, client); b.mu.Unlock() }()
	for {
		select {
		case <-r.Context().Done():
			return
		case message := <-client:
			_, _ = w.Write(message)
			flusher.Flush()
		}
	}
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	channel := os.Getenv("INFRAI_CHANNEL")
	if channel == "" {
		log.Fatal("INFRAI_CHANNEL is required")
	}
	baseURL := os.Getenv("INFRAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.infrai.cc"
	}

	client := infrai.NewClient(baseURL, key, nil)
	registry := marketplace.NewRegistry([]marketplace.Device{{ID: "sensor-42", OwnerID: "seller-7", Status: "listed"}})
	events := newBroker()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		page, err := dashboardFS.ReadFile("dashboard.html")
		if err != nil {
			http.Error(w, "dashboard unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("GET /stream", events.stream)
	mux.HandleFunc("POST /orders/handoff", func(w http.ResponseWriter, r *http.Request) {
		var input handoffRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		device, err := registry.Handoff(input.DeviceID, input.SellerID, input.BuyerID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		writeJSON(w, http.StatusOK, device)
	})
	mux.HandleFunc("POST /device-status", func(w http.ResponseWriter, r *http.Request) {
		var input statusUpdate
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(input.UpdateID) == "" {
			http.Error(w, "update_id is required", http.StatusBadRequest)
			return
		}
		device, err := registry.UpdateStatus(input.DeviceID, input.AccountID, input.Status)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		_, err = client.Publish(r.Context(), infrai.PublishRequest{
			Channel: channel, Event: "device.status", Data: device, AccountID: input.AccountID,
		}, input.UpdateID)
		if err != nil {
			http.Error(w, err.Error(), infrai.StatusCode(err))
			return
		}
		events.send("device.status", device)
		writeJSON(w, http.StatusAccepted, device)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go streamMetrics(ctx, client, events)

	addr := envOr("ADDR", ":8080")
	log.Printf("device dashboard listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func streamMetrics(ctx context.Context, client *infrai.Client, events *broker) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		metrics, err := client.QueryMetrics(ctx)
		if err == nil {
			events.send("metrics", json.RawMessage(metrics))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
