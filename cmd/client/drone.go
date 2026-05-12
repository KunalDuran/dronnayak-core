package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/KunalDuran/dronnayak-core/internal/data"
	"github.com/KunalDuran/gowsrelay/client"
	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
)

// Dronnayak represents the main drone application
type Dronnayak struct {
	mavNode        *gomavlib.Node
	config         *data.Config
	ctx            context.Context
	tunnelManagers map[string]*TunnelManager
	tunnelMu       sync.Mutex
	wg             sync.WaitGroup
}

// NewDronnayak creates a new Dronnayak instance
func NewDronnayak(configPath string) (*Dronnayak, error) {
	bootstrap, err := data.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	config, err := data.LoadConfigV2(bootstrap.Server.URL, bootstrap.UUID)
	if err != nil {
		slog.Warn("failed to fetch config from server, falling back to local config", "error", err)
		config = bootstrap
	}

	return &Dronnayak{
		config:         config,
		tunnelManagers: make(map[string]*TunnelManager),
	}, nil
}

// Run starts the application and blocks until shutdown
func (d *Dronnayak) Run(ctx context.Context) error {
	// Create a cancellable context and store it so goroutines started later inherit it
	ctx, cancel := context.WithCancel(ctx)
	d.ctx = ctx
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Initialize MAVLink node
	if err := d.initMAVLink(); err != nil {
		return fmt.Errorf("failed to initialize MAVLink: %w", err)
	}
	defer d.Close()

	// Start MAVLink event processing
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.processMAVLinkEvents(ctx)
	}()

	// Start WebSocket tunnels
	d.startTunnels(ctx, d.config.Tunnel.Endpoints)

	// Start stats reporting if enabled
	if d.config.Stats.Enabled {
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			d.startStatsReporter(ctx)
		}()
	}

	// Wait for shutdown signal
	sig := <-sigChan
	slog.Info("received shutdown signal, shutting down gracefully", "signal", sig)

	// Cancel context to signal all goroutines to stop
	cancel()

	// Wait for all goroutines to finish
	d.wg.Wait()
	slog.Info("all services stopped")

	return nil
}

// initMAVLink initializes the MAVLink node with current configuration
func (d *Dronnayak) initMAVLink() error {
	serialPort := d.detectSerialPort()

	nodeConf := gomavlib.NodeConf{
		Endpoints: []gomavlib.EndpointConf{
			gomavlib.EndpointSerial{
				Device: serialPort,
				Baud:   d.config.MAVLink.BaudRate,
			},
			&gomavlib.EndpointTCPServer{
				Address: d.config.MAVLink.TCPAddress,
			},
		},
		Dialect:     common.Dialect,
		OutVersion:  gomavlib.V2,
		OutSystemID: d.config.MAVLink.OutSystemID,
	}

	if d.config.MAVLink.StreamFrequency > 0 {
		nodeConf.StreamRequestFrequency = d.config.MAVLink.StreamFrequency
		slog.Info("stream frequency configured", "hz", d.config.MAVLink.StreamFrequency)
	}

	node, err := gomavlib.NewNode(nodeConf)
	if err != nil {
		return fmt.Errorf("failed to create MAVLink node: %w", err)
	}

	d.mavNode = node
	slog.Info("MAVLink initialized", "port", serialPort, "baud", d.config.MAVLink.BaudRate)
	return nil
}

// detectSerialPort detects the appropriate serial port based on OS
func (d *Dronnayak) detectSerialPort() string {
	if d.config.MAVLink.SerialPort != "" {
		slog.Info("using configured serial port", "port", d.config.MAVLink.SerialPort)
		return d.config.MAVLink.SerialPort
	}

	var defaultPort string
	switch runtime.GOOS {
	case "linux":
		defaultPort = "/dev/ttyACM0"
	case "windows":
		defaultPort = "COM4"
	case "darwin":
		defaultPort = "/dev/tty.usbmodem1"
	default:
		defaultPort = "/dev/ttyACM0"
	}

	slog.Info("using auto-detected serial port", "port", defaultPort, "os", runtime.GOOS)
	return defaultPort
}

// processMAVLinkEvents handles all MAVLink events
func (d *Dronnayak) processMAVLinkEvents(ctx context.Context) {
	slog.Info("MAVLink event processor started")
	defer slog.Info("MAVLink event processor stopped")

	for {
		select {
		case evt := <-d.mavNode.Events():
			switch e := evt.(type) {
			case *gomavlib.EventChannelOpen:
				slog.Info("channel opened", "channel", e.Channel)

			case *gomavlib.EventStreamRequested:
				slog.Info("stream requested",
					"channel", e.Channel,
					"system_id", e.SystemID,
					"component_id", e.ComponentID)

			case *gomavlib.EventParseError:
				if strings.Contains(e.Error.Error(), "invalid magic byte") {
					continue // ignore noise
				}
				slog.Warn("parse error", "error", e.Error)

			case *gomavlib.EventFrame:
				// Forward frame to other endpoints (Pixhawk <-> Mission Planner)
				d.mavNode.WriteFrameExcept(e.Channel, e.Frame)

			case *gomavlib.EventChannelClose:
				slog.Info("channel closed", "channel", e.Channel)
			}

		case <-ctx.Done():
			return
		}
	}
}

// endpointFactory returns an EndpointFactory for the given TunnelEntry.
// Add new endpoint types here as the client package grows.
func endpointFactory(entry data.TunnelEntry) (EndpointFactory, error) {
	switch entry.Type {
	case data.EndpointTypeTCP:
		if entry.Port == "" {
			return nil, fmt.Errorf("tcp endpoint requires a port")
		}
		port := entry.Port
		return func() (client.LocalEndpoint, error) {
			return client.TCPEndpoint("localhost", port)
		}, nil
	case data.EndpointTypeCmd:
		return func() (client.LocalEndpoint, error) {
			return client.NewCmdEndpoint(), nil
		}, nil
	default:
		return nil, fmt.Errorf("unknown endpoint type %q", entry.Type)
	}
}

// makeTunnelID builds the unique tunnel ID for a given endpoint entry.
func (d *Dronnayak) makeTunnelID(entry data.TunnelEntry) string {
	label := entry.Label
	if label == "" {
		label = string(entry.Type)
	}
	return fmt.Sprintf("%s_%s", d.config.UUID, label)
}

// stopTunnel gracefully stops the tunnel with the given ID by cancelling its context.
func (d *Dronnayak) stopTunnel(tunnelID string) {
	d.tunnelMu.Lock()
	tm, ok := d.tunnelManagers[tunnelID]
	d.tunnelMu.Unlock()

	if !ok {
		slog.Warn("stop requested for unknown tunnel", "id", tunnelID)
		return
	}
	tm.Stop()
}

// startTunnels starts WebSocket tunnels for the given endpoints with automatic reconnection.
// It is safe to call multiple times; already-running tunnels are skipped.
func (d *Dronnayak) startTunnels(ctx context.Context, endpoints []data.TunnelEntry) {
	if len(endpoints) == 0 {
		slog.Warn("no tunnel endpoints configured")
		return
	}

	serverHost := d.cleanServerURL(d.config.Server.URL)
	started := 0

	for _, entry := range endpoints {
		tunnelID := d.makeTunnelID(entry)

		d.tunnelMu.Lock()
		_, exists := d.tunnelManagers[tunnelID]
		d.tunnelMu.Unlock()

		if exists {
			slog.Warn("tunnel already running, skipping", "id", tunnelID)
			continue
		}

		factory, err := endpointFactory(entry)
		if err != nil {
			slog.Warn("skipping tunnel endpoint", "label", entry.Label, "error", err)
			continue
		}

		tunnelCtx, tunnelCancel := context.WithCancel(ctx)
		tm := NewTunnelManager(serverHost, d.config.Tunnel.WSPath, tunnelID, entry.Label, factory, tunnelCancel)
		if strings.Contains(d.config.Server.URL, "https") {
			tm.wsScheme = "wss"
		}

		d.tunnelMu.Lock()
		d.tunnelManagers[tunnelID] = tm
		d.tunnelMu.Unlock()

		started++
		d.wg.Add(1)
		go func(manager *TunnelManager) {
			defer d.wg.Done()
			defer func() {
				d.tunnelMu.Lock()
				delete(d.tunnelManagers, manager.tunnelID)
				d.tunnelMu.Unlock()
			}()
			manager.Start(tunnelCtx)
		}(tm)
	}

	slog.Info("tunnels started", "count", started, "auto_reconnect", true)
}

// cleanServerURL removes http/https schema from server URL
func (d *Dronnayak) cleanServerURL(url string) string {
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	return url
}

// Close gracefully shuts down the MAVLink node
func (d *Dronnayak) Close() {
	if d.mavNode != nil {
		slog.Info("closing MAVLink node")
		d.mavNode.Close()
	}
}
