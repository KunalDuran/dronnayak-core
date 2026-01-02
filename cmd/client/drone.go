package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/KunalDuran/gowsrelay/client"
	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
)

type Dronnayak struct {
	MavNode *gomavlib.Node
	Config  Config
}

func NewDronenayak() *Dronnayak {
	config := loadConfig("config.json")
	return &Dronnayak{
		Config: config,
	}
}

func (d *Dronnayak) Boot() {
	var serialPort string
	// if linux, use serial port
	if runtime.GOOS == "linux" {
		serialPort = "/dev/ttyACM0"
	} else {
		serialPort = "COM4"
	}

	if d.Config.SerialPort != "" {
		serialPort = d.Config.SerialPort
	}

	nodeConf := gomavlib.NodeConf{
		Endpoints: []gomavlib.EndpointConf{
			gomavlib.EndpointSerial{
				Device: serialPort, // Typical USB connection for Pixhawk
				Baud:   57600,      // Standard MAVLink baud rate
			},
			// TCP server for Mission Planner
			&gomavlib.EndpointTCPServer{
				Address: "0.0.0.0:5760",
			},
		},
		Dialect:     common.Dialect,
		OutVersion:  gomavlib.V2, // Use MAVLink v2
		OutSystemID: 255,         // Ground Control Station ID
	}

	if d.Config.StreamFrequency > 0 {
		nodeConf.StreamRequestFrequency = d.Config.StreamFrequency
	}
	log.Printf("Stream frequency: %d", nodeConf.StreamRequestFrequency)
	// Initialize MAVLink node connected to Pixhawk via USB
	node, err := gomavlib.NewNode(nodeConf)

	if err != nil {
		log.Fatal("Failed to create MAVLink node:", err)
	}
	defer node.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Goroutine to read from MAVLink and send to WebSocket
	go func() {

		for {
			select {
			case e := <-node.Events():
				switch evt := e.(type) {
				case *gomavlib.EventChannelOpen:
					log.Printf("channel opened: %s", evt.Channel)

				case *gomavlib.EventStreamRequested:
					log.Printf("stream requested to chan=%s sid=%d cid=%d", evt.Channel,
						evt.SystemID, evt.ComponentID)

				case *gomavlib.EventParseError:
					log.Printf("parse error: %s", evt.Error)

				case *gomavlib.EventFrame:
					// log.Printf("%#v, %#v\n", evt.Frame, evt.Message())
					// This line automatically sends any MAVLink frame
					// to the other side (Pixhawk <-> Mission Planner)
					node.WriteFrameExcept(evt.Channel, evt.Frame)

				case *gomavlib.EventChannelClose:
					log.Printf("Connection closed: %v", evt.Channel)

				}

			case <-ctx.Done():
				return
			}
		}
	}()

	for _, port := range d.Config.TunnelPorts {
		// remove schema from server path
		serverHost := strings.Replace(d.Config.ServerPath, "http://", "", 1)
		serverHost = strings.Replace(serverHost, "https://", "", 1)
		go client.CreateWebSocketTunnel(serverHost, port, "/ws", fmt.Sprintf("%s_%s", d.Config.UUID, port))
	}

	// Keep the main goroutine alive
	<-ctx.Done()
	log.Println("Shutting down MAVLink WebSocket bridge")
}

func (d *Dronnayak) CommandMessages(id ...int) {
	fmt.Println("command messages")
}

func (d *Dronnayak) FetchMessages() {
	node := d.MavNode
	for evt := range node.Events() {
		if frm, ok := evt.(*gomavlib.EventFrame); ok {
			fmt.Println(frm.Message())
		}
	}
}

type Config struct {
	UUID            string   `json:"uuid"`
	SerialPort      string   `json:"serial_port"`
	ServerPath      string   `json:"server_path"`
	TunnelPorts     []string `json:"tunnel_ports"`
	StreamFrequency int      `json:"stream_frequency"`
}

func loadConfig(configFile string) Config {

	file, err := os.Open(configFile)
	if err != nil {
		log.Fatal("Error opening config file:", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	var config Config
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatal("Error decoding config file:", err)
	}

	return config
}
