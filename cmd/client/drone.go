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
	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
)

// Dronnayak represents the main drone application
type Dronnayak struct {
	mavNode        *gomavlib.Node
	config         *data.Config
	tunnelManagers []*TunnelManager
	wg             sync.WaitGroup
}

// NewDronnayak creates a new Dronnayak instance
func NewDronnayak(configPath string) (*Dronnayak, error) {
	config, err := data.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &Dronnayak{
		config: config,
	}, nil
}

// Run starts the application and blocks until shutdown
func (d *Dronnayak) Run(ctx context.Context) error {
	// Create a cancellable context
	ctx, cancel := context.WithCancel(ctx)
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
	d.startTunnels(ctx)

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

// startTunnels starts WebSocket tunnels for configured ports with reconnection
func (d *Dronnayak) startTunnels(ctx context.Context) {
	if len(d.config.Tunnel.Ports) == 0 {
		slog.Warn("no tunnel ports configured")
		return
	}

	serverHost := d.cleanServerURL(d.config.Server.URL)
	d.tunnelManagers = make([]*TunnelManager, 0, len(d.config.Tunnel.Ports))

	for _, port := range d.config.Tunnel.Ports {
		tunnelID := fmt.Sprintf("%s_%s", d.config.UUID, port)

		tm := NewTunnelManager(serverHost, port, d.config.Tunnel.WSPath, tunnelID)
		if strings.Contains(d.config.Server.URL, "https") {
			tm.wsScheme = "wss"
		}
		d.tunnelManagers = append(d.tunnelManagers, tm)

		d.wg.Add(1)
		go func(manager *TunnelManager) {
			defer d.wg.Done()
			manager.Start(ctx)
		}(tm)
	}

	slog.Info("tunnels started", "count", len(d.config.Tunnel.Ports), "auto_reconnect", true)
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
