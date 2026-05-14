package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// WorkerFunc processes a single binary message received from a relay topic.
type WorkerFunc func(ctx context.Context, data []byte) error

type workerEntry struct {
	cancel context.CancelFunc
	done   <-chan struct{}
}

var (
	workersMu sync.RWMutex
	workers   = map[string]*workerEntry{}
)

// StartTopicWorker subscribes to the relay topic and calls fn for each binary
// message. teardown is deferred when the goroutine exits — use it to close
// resources opened during worker setup (e.g. a file handle).
func StartTopicWorker(parentCtx context.Context, wsBase, topic string, fn WorkerFunc, teardown func()) error {
	workersMu.Lock()
	defer workersMu.Unlock()

	if _, exists := workers[topic]; exists {
		return fmt.Errorf("worker already running for topic %q", topic)
	}

	wsURL, err := relaySubscriberURL(wsBase, topic)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(parentCtx)
	done := make(chan struct{})

	workers[topic] = &workerEntry{cancel: cancel, done: done}

	go func() {
		defer close(done)
		defer func() {
			workersMu.Lock()
			delete(workers, topic)
			workersMu.Unlock()
		}()
		if teardown != nil {
			defer teardown()
		}
		runTopicWorker(ctx, wsURL, topic, fn)
	}()

	slog.Info("worker started", "topic", topic)
	return nil
}

// StopTopicWorker cancels the running worker for topic and waits for it to exit.
func StopTopicWorker(topic string) {
	workersMu.RLock()
	entry, exists := workers[topic]
	workersMu.RUnlock()
	if !exists {
		return
	}
	entry.cancel()
	<-entry.done
	slog.Info("worker stopped", "topic", topic)
}

func runTopicWorker(ctx context.Context, wsURL, topic string, fn WorkerFunc) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		slog.Error("worker: dial failed", "topic", topic, "error", err)
		return
	}
	defer conn.Close()

	// Force-close the WebSocket when context is cancelled so ReadMessage unblocks.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	slog.Info("worker: subscribed", "topic", topic)

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Error("worker: read error", "topic", topic, "error", err)
			}
			return
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		if err := fn(ctx, data); err != nil {
			slog.Error("worker: fn error", "topic", topic, "error", err)
		}
	}
}

func relaySubscriberURL(serverBase, topic string) (string, error) {
	u, err := url.Parse(serverBase)
	if err != nil {
		return "", fmt.Errorf("parse server base: %w", err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	u.Path = "/ws"
	q := u.Query()
	q.Set("role", "subscriber")
	q.Set("topic", topic)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// manageWorker handles POST (start) and DELETE (stop) for topic workers.
//
// POST   /device/{drone_id}/worker
//
//	{"type": "file-writer", "topic": "<droneUID>_<label>", "options": {"path": "/tmp/out.bin"}}
//
// DELETE /device/{drone_id}/worker?topic=<droneUID>_<label>
func manageWorker(w http.ResponseWriter, r *http.Request) {
	droneID := chi.URLParam(r, "drone_id")
	if droneID == "" {
		http.Error(w, "missing drone_id", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodDelete {
		topic := r.URL.Query().Get("topic")
		if topic == "" {
			http.Error(w, "missing topic query param", http.StatusBadRequest)
			return
		}
		StopTopicWorker(topic)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// POST: parse worker request
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req struct {
		Type    string          `json:"type"`
		Topic   string          `json:"topic"`
		Options json.RawMessage `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Type == "" || req.Topic == "" {
		http.Error(w, "type and topic are required", http.StatusBadRequest)
		return
	}

	wsBase := getServerPath(r)

	var (
		fn       WorkerFunc
		teardown func()
		err      error
		filePath string
	)

	switch req.Type {
	case "file-writer":
		filePath = fmt.Sprintf("/tmp/%s_%s.bin", req.Topic, time.Now().Format(time.RFC3339Nano))
		fn, teardown, err = FileWriterWorker(filePath)
		if err != nil {
			slog.Error("failed to init file-writer worker", "error", err)
			http.Error(w, "failed to create file writer: "+err.Error(), http.StatusInternalServerError)
			return
		}

	default:
		http.Error(w, "unknown worker type: "+req.Type, http.StatusBadRequest)
		return
	}

	if err := StartTopicWorker(context.Background(), wsBase, req.Topic, fn, teardown); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "started", "topic": req.Topic, "path": filePath})
}
