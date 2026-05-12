package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/KunalDuran/devstat"
	"github.com/KunalDuran/dronnayak-core/internal/data"
	"github.com/KunalDuran/dronnayak-core/internal/web"
)

const (
	EventStartTunnel = "start_tunnel"
	EventStopTunnel  = "stop_tunnel"
)

func (d *Dronnayak) startStatsReporter(ctx context.Context) {
	endpoint := d.config.Server.URL + d.config.Stats.Endpoint

	slog.Info("stats reporting enabled", "interval", d.config.Stats.Interval, "endpoint", endpoint)

	// Initial stats collection (warm-up)
	devstat.Stats()

	ticker := time.NewTicker(d.config.Stats.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			resp, err := sendStats(endpoint)
			if err != nil {
				slog.Error("stats reporting error", "error", err)
			}

			d.processEvent(resp)

		case <-ctx.Done():
			slog.Info("stats reporter shutting down")
			return
		}
	}
}

func sendStats(endpoint string) ([]byte, error) {
	statsData, err := devstat.Stats()
	if err != nil {
		return nil, fmt.Errorf("failed to collect stats: %w", err)
	}

	jsonData, err := json.Marshal(statsData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stats: %w", err)
	}

	resp, statusCode, err := web.WebRequest(http.MethodPost, endpoint, string(jsonData), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to send stats: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code: %d", statusCode)
	}

	return resp, nil
}

func (d *Dronnayak) processEvent(resp []byte) {
	var command []data.DroneCommands
	if err := json.Unmarshal(resp, &command); err != nil {
		slog.Error("failed to unmarshal stats response", "error", err)
		return
	}

	for _, cmd := range command {
		switch cmd.Type {
		case EventStartTunnel:
			var evt data.TunnelEntry
			if err := json.Unmarshal(cmd.Payload, &evt); err != nil {
				slog.Error("failed to unmarshal tunnel response", "error", err)
			}
			d.startTunnels(context.Background(), []data.TunnelEntry{evt})
		}
	}
}
