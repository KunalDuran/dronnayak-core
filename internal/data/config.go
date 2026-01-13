package data

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config holds all application configuration
type Config struct {
	// Device identification
	UUID string `json:"uuid"`

	// MAVLink configuration
	MAVLink MAVLinkConfig `json:"mavlink"`

	// Server configuration
	Server ServerConfig `json:"server"`

	// Tunnel configuration
	Tunnel TunnelConfig `json:"tunnel"`

	// Stats configuration
	Stats StatsConfig `json:"stats"`
}

type MAVLinkConfig struct {
	SerialPort      string `json:"serial_port"`      // Override auto-detection
	BaudRate        int    `json:"baud_rate"`        // Default: 57600
	TCPAddress      string `json:"tcp_address"`      // Default: 0.0.0.0:5760
	StreamFrequency int    `json:"stream_frequency"` // Hz, 0 = disabled
	OutSystemID     byte   `json:"out_system_id"`    // Default: 255
}

type ServerConfig struct {
	URL string `json:"url"` // Base server URL
}

type TunnelConfig struct {
	Ports  []string `json:"ports"`   // Ports to tunnel
	WSPath string   `json:"ws_path"` // WebSocket path, default: /ws
}

type StatsConfig struct {
	Enabled  bool          `json:"enabled"`  // Default: true
	Interval time.Duration `json:"interval"` // Default: 5s
	Endpoint string        `json:"endpoint"` // Default: /device-status/{uuid}
}

func LoadConfig(configPath string) (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	config.applyDefaults()

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

func (c *Config) applyDefaults() {
	if c.MAVLink.BaudRate == 0 {
		c.MAVLink.BaudRate = 57600
	}
	if c.MAVLink.TCPAddress == "" {
		c.MAVLink.TCPAddress = "0.0.0.0:5760"
	}
	if c.MAVLink.OutSystemID == 0 {
		c.MAVLink.OutSystemID = 255
	}

	if c.Tunnel.WSPath == "" {
		c.Tunnel.WSPath = "/ws"
	}

	if c.Stats.Interval == 0 {
		c.Stats.Interval = 5 * time.Second
	}
	if c.Stats.Endpoint == "" {
		c.Stats.Endpoint = fmt.Sprintf("/device-status/%s", c.UUID)
	}
}

func (c *Config) Validate() error {
	if c.UUID == "" {
		return fmt.Errorf("uuid is required")
	}

	if c.MAVLink.BaudRate < 0 {
		return fmt.Errorf("invalid baud rate: %d", c.MAVLink.BaudRate)
	}

	if c.Server.URL == "" {
		return fmt.Errorf("server URL is required")
	}

	if c.Stats.Interval < time.Second {
		return fmt.Errorf("stats interval must be at least 1 second")
	}

	return nil
}

func NewDefaultDeviceConfig(uuid, serverURL string) Config {
	return Config{
		UUID: uuid,
		MAVLink: MAVLinkConfig{
			BaudRate:        57600,
			TCPAddress:      "0.0.0.0:5760",
			StreamFrequency: 10,
			OutSystemID:     255,
		},
		Server: ServerConfig{
			URL: serverURL,
		},
		Tunnel: TunnelConfig{
			Ports:  []string{"5760"},
			WSPath: "/ws",
		},
		Stats: StatsConfig{
			Enabled:  true,
			Interval: 5 * time.Second,
			Endpoint: fmt.Sprintf("/device-status/%s", uuid),
		},
	}
}
