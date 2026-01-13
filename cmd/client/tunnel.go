package main

import (
	"context"
	"log"
	"time"

	"github.com/KunalDuran/gowsrelay/client"
)

// TunnelManager handles WebSocket tunnel lifecycle with reconnection
type TunnelManager struct {
	serverHost string
	port       string
	wsPath     string
	tunnelID   string

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
		tunnelID:   tunnelID,
		maxRetries: -1, // infinite retries
		baseDelay:  2 * time.Second,
		maxDelay:   2 * time.Minute,
	}
}

// Start begins the tunnel connection with automatic reconnection
func (tm *TunnelManager) Start(ctx context.Context) {
	log.Printf("Starting tunnel for port %s (ID: %s)", tm.port, tm.tunnelID)

	retryCount := 0

	for {
		select {
		case <-ctx.Done():
			log.Printf("Tunnel for port %s (ID: %s) shutting down", tm.port, tm.tunnelID)
			return
		default:
			err := client.CreateWebSocketTunnel(tm.serverHost, tm.port, tm.wsPath, tm.tunnelID)

			if err != nil {
				retryCount++
				delay := tm.calculateBackoff(retryCount)

				log.Printf("Tunnel error for port %s (ID: %s): %v - reconnecting in %v (attempt %d)",
					tm.port, tm.tunnelID, err, delay, retryCount)

				// Wait before reconnecting, but respect context cancellation
				select {
				case <-time.After(delay):
					continue
				case <-ctx.Done():
					log.Printf("Tunnel for port %s (ID: %s) shutting down during reconnect wait",
						tm.port, tm.tunnelID)
					return
				}
			}

			// If connection closed gracefully (no error), reset retry count
			retryCount = 0
			log.Printf("Tunnel for port %s (ID: %s) closed gracefully, reconnecting...",
				tm.port, tm.tunnelID)

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
