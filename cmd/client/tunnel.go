package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/KunalDuran/gowsrelay/client"
)

// EndpointFactory creates a fresh LocalEndpoint for each tunnel connection attempt.
type EndpointFactory func() (client.LocalEndpoint, error)

// TunnelManager handles WebSocket tunnel lifecycle with reconnection
type TunnelManager struct {
	serverHost      string
	wsPath          string
	tunnelID        string
	wsScheme        string
	label           string
	endpointFactory EndpointFactory
	cancel          context.CancelFunc

	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

// NewTunnelManager creates a new tunnel manager instance
func NewTunnelManager(serverHost, wsPath, tunnelID, label string, factory EndpointFactory, cancel context.CancelFunc) *TunnelManager {
	return &TunnelManager{
		serverHost:      serverHost,
		wsPath:          wsPath,
		wsScheme:        "ws",
		tunnelID:        tunnelID,
		label:           label,
		endpointFactory: factory,
		cancel:          cancel,
		maxRetries:      -1, // infinite retries
		baseDelay:       2 * time.Second,
		maxDelay:        2 * time.Minute,
	}
}

// Stop cancels the tunnel's context, causing it to shut down gracefully.
func (tm *TunnelManager) Stop() {
	slog.Info("stopping tunnel", "label", tm.label, "id", tm.tunnelID)
	tm.cancel()
}

// Start begins the tunnel connection with automatic reconnection
func (tm *TunnelManager) Start(ctx context.Context) {
	slog.Info("starting tunnel", "label", tm.label, "id", tm.tunnelID)

	retryCount := 0

	for {
		select {
		case <-ctx.Done():
			slog.Info("tunnel shutting down", "label", tm.label, "id", tm.tunnelID)
			return
		default:
			tunConfig := client.TunnelConfig{
				Topic:  tm.tunnelID,
				Host:   tm.serverHost,
				Path:   tm.wsPath,
				Scheme: tm.wsScheme,
			}
			ep, err := tm.endpointFactory()
			if err != nil {
				slog.Error("endpoint creation failed, shutting down tunnel", "label", tm.label, "id", tm.tunnelID, "error", err)
				return
			}

			if err := client.CreateWebSocketTunnel(ctx, tunConfig, ep); err != nil {
				retryCount++
				delay := tm.calculateBackoff(retryCount)

				slog.Warn("tunnel error, reconnecting",
					"label", tm.label,
					"id", tm.tunnelID,
					"error", err,
					"delay", delay,
					"attempt", retryCount)

				// Wait before reconnecting, but respect context cancellation
				select {
				case <-time.After(delay):
					continue
				case <-ctx.Done():
					slog.Info("tunnel shutting down during reconnect wait", "label", tm.label, "id", tm.tunnelID)
					return
				}
			}

			// If connection closed gracefully (no error), reset retry count
			retryCount = 0
			slog.Info("tunnel closed gracefully, reconnecting", "label", tm.label, "id", tm.tunnelID)

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
