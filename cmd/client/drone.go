package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/KunalDuran/gowsrelay/client"
	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
)

// Dronnayak represents the main drone application
type Dronnayak struct {
	mavNode *gomavlib.Node
	config  *Config
}

// NewDronnayak creates a new Dronnayak instance
func NewDronnayak(configPath string) (*Dronnayak, error) {
	config, err := LoadConfig(configPath)
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
	go d.processMAVLinkEvents(ctx)

	// Start WebSocket tunnels
	d.startTunnels()

	// Start stats reporting if enabled
	if d.config.Stats.Enabled {
		go d.startStatsReporter(ctx)
	}

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal: %v, shutting down gracefully...", sig)

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
		log.Printf("Stream frequency: %d Hz", d.config.MAVLink.StreamFrequency)
	}

	node, err := gomavlib.NewNode(nodeConf)
	if err != nil {
		return fmt.Errorf("failed to create MAVLink node: %w", err)
	}

	d.mavNode = node
	log.Printf("MAVLink initialized on %s (baud: %d)", serialPort, d.config.MAVLink.BaudRate)
	return nil
}

// detectSerialPort detects the appropriate serial port based on OS
func (d *Dronnayak) detectSerialPort() string {
	if d.config.MAVLink.SerialPort != "" {
		log.Printf("Using configured serial port: %s", d.config.MAVLink.SerialPort)
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

	log.Printf("Using auto-detected serial port: %s", defaultPort)
	return defaultPort
}

// processMAVLinkEvents handles all MAVLink events
func (d *Dronnayak) processMAVLinkEvents(ctx context.Context) {
	for {
		select {
		case evt := <-d.mavNode.Events():
			switch e := evt.(type) {
			case *gomavlib.EventChannelOpen:
				log.Printf("Channel opened: %s", e.Channel)

			case *gomavlib.EventStreamRequested:
				log.Printf("Stream requested: chan=%s sid=%d cid=%d",
					e.Channel, e.SystemID, e.ComponentID)

			case *gomavlib.EventParseError:
				log.Printf("Parse error: %s", e.Error)

			case *gomavlib.EventFrame:
				// Forward frame to other endpoints (Pixhawk <-> Mission Planner)
				d.mavNode.WriteFrameExcept(e.Channel, e.Frame)

			case *gomavlib.EventChannelClose:
				log.Printf("Channel closed: %v", e.Channel)
			}

		case <-ctx.Done():
			return
		}
	}
}

// startTunnels starts WebSocket tunnels for configured ports
func (d *Dronnayak) startTunnels() {
	if len(d.config.Tunnel.Ports) == 0 {
		log.Println("No tunnel ports configured")
		return
	}

	serverHost := d.cleanServerURL(d.config.Server.URL)

	for _, port := range d.config.Tunnel.Ports {
		tunnelID := fmt.Sprintf("%s_%s", d.config.UUID, port)

		go func(port, id string) {
			log.Printf("Starting tunnel for port %s (ID: %s)", port, id)
			client.CreateWebSocketTunnel(serverHost, port, d.config.Tunnel.WSPath, id)
		}(port, tunnelID)
	}
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
		log.Println("Closing MAVLink node...")
	}
}
