package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/KunalDuran/devstat"
	"github.com/KunalDuran/dronnayak-core/internal/web"
)

func (d *Dronnayak) startStatsReporter(ctx context.Context) {
	endpoint := d.config.Server.URL + d.config.Stats.Endpoint

	log.Printf("Stats reporting enabled: interval=%s, endpoint=%s",
		d.config.Stats.Interval, endpoint)

	// Initial stats collection (warm-up)
	devstat.Stats()

	ticker := time.NewTicker(d.config.Stats.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := sendStats(endpoint); err != nil {
				log.Printf("Stats reporting error: %v", err)
			}

		case <-ctx.Done():
			log.Println("Stats reporter shutting down")
			return
		}
	}
}

func sendStats(endpoint string) error {
	statsData, err := devstat.Stats()
	if err != nil {
		return fmt.Errorf("failed to collect stats: %w", err)
	}

	jsonData, err := json.Marshal(statsData)
	if err != nil {
		return fmt.Errorf("failed to marshal stats: %w", err)
	}

	_, statusCode, err := web.WebRequest(http.MethodPost, endpoint, string(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send stats: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", statusCode)
	}

	return nil
}
