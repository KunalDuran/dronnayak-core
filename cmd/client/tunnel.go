package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/KunalDuran/gowsrelay/client"
)

// TunnelManager handles WebSocket tunnel lifecycle with reconnection
type TunnelManager struct {
	serverHost string
	port       string
	wsPath     string
	tunnelID   string
	wsScheme   string

	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

// NewTunnelManager creates a new tunnel manager instance
func NewTunnelManager(serverHost, port, wsPath, tunnelID string) *TunnelManager {
	return &TunnelManager{
		serverHost: serverHost,
		port:       port,
		wsPath:     wsPath,
		wsScheme:   "ws",
		tunnelID:   tunnelID,
		maxRetries: -1, // infinite retries
		baseDelay:  2 * time.Second,
		maxDelay:   2 * time.Minute,
	}
}

// Start begins the tunnel connection with automatic reconnection
func (tm *TunnelManager) Start(ctx context.Context) {
	slog.Info("starting tunnel", "port", tm.port, "id", tm.tunnelID)

	retryCount := 0

	for {
		select {
		case <-ctx.Done():
			slog.Info("tunnel shutting down", "port", tm.port, "id", tm.tunnelID)
			return
		default:
			tunConfig := client.TunnelConfig{
				Topic:  tm.tunnelID,
				Host:   tm.serverHost,
				Port:   tm.port,
				Path:   tm.wsPath,
				Scheme: tm.wsScheme,
			}
			err := client.CreateWebSocketTunnel(tunConfig)

			if err != nil {
				retryCount++
				delay := tm.calculateBackoff(retryCount)

				slog.Warn("tunnel error, reconnecting",
					"port", tm.port,
					"id", tm.tunnelID,
					"error", err,
					"delay", delay,
					"attempt", retryCount)

				// Wait before reconnecting, but respect context cancellation
				select {
				case <-time.After(delay):
					continue
				case <-ctx.Done():
					slog.Info("tunnel shutting down during reconnect wait", "port", tm.port, "id", tm.tunnelID)
					return
				}
			}

			// If connection closed gracefully (no error), reset retry count
			retryCount = 0
			slog.Info("tunnel closed gracefully, reconnecting", "port", tm.port, "id", tm.tunnelID)

			// Brief pause before reconnecting on graceful close
			select {
			case <-time.After(time.Second):
				continue
			case <-ctx.Done():
				return
			}
		}
	}
}

// calculateBackoff implements exponential backoff with jitter
func (tm *TunnelManager) calculateBackoff(retryCount int) time.Duration {
	delay := tm.baseDelay * time.Duration(1<<uint(retryCount-1))

	if delay > tm.maxDelay {
		delay = tm.maxDelay
	}

	return delay
}
