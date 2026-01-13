package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/KunalDuran/dronnayak-core/internal/data"
	"github.com/KunalDuran/dronnayak-core/internal/web"
	"github.com/go-chi/chi/v5"
)

func index(w http.ResponseWriter, r *http.Request) {
	template.Must(template.ParseFiles("templates/base.html", "templates/index.html")).Execute(w, nil)
}

func login(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()

		filter := map[string]interface{}{"email": r.Form.Get("email"), "password": r.Form.Get("password")}
		var user data.User
		err := data.FindOne("user", filter, &user)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusUnauthorized)
			return
		}

		if user.Email == "" {
			http.Redirect(w, r, "/login", http.StatusUnauthorized)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:  "session",
			Value: user.Email,
			Path:  "/",
		})

		http.SetCookie(w, &http.Cookie{
			Name:  "authenticated",
			Value: "true",
			Path:  "/",
		})

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	template.Must(template.ParseFiles("templates/login.html")).Execute(w, nil)
}

func signup(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var u data.User

		r.ParseForm()

		u.Name = r.Form.Get("name")
		u.Email = r.Form.Get("email")
		u.Password = r.Form.Get("password")

		err := data.InsertOne("user", u)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}

	template.Must(template.ParseFiles("templates/signup.html")).Execute(w, nil)
}

func fleets(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromSession(r)
	if r.Method == "POST" {
		var f data.Fleet
		r.ParseForm()

		f.Name = r.Form.Get("name")
		f.Description = r.Form.Get("description")
		f.UID = data.GenerateUID()
		f.UserID = userID

		err := data.InsertOne("fleet", f)
		if err != nil {
			log.Fatal(err)
		}
	}

	var fleets []data.Fleet
	err := data.FindAll("fleet", map[string]interface{}{"user_id": userID}, &fleets)
	if err != nil {
		log.Fatal(err)
	}

	template.Must(template.ParseFiles("templates/base.html", "templates/fleets.html")).Execute(w, fleets)
}

func devices(w http.ResponseWriter, r *http.Request) {
	fleetID := chi.URLParam(r, "fleet_id")

	if fleetID == "" {
		http.RedirectHandler("/", 302).ServeHTTP(w, r)
	}

	var drones []data.Drone
	err := data.FindAll("drone", map[string]interface{}{"fleet_id": fleetID}, &drones)
	if err != nil {
		http.RedirectHandler("/fleets", 302).ServeHTTP(w, r)
		return
	}

	result := struct {
		Drones []data.Drone
		ID     string
	}{
		Drones: drones,
		ID:     fleetID,
	}

	template.Must(template.ParseFiles("templates/base.html", "templates/drones.html")).Execute(w, result)
}

func deviceDetails(w http.ResponseWriter, r *http.Request) {
	droneID := chi.URLParam(r, "drone_id")

	if droneID == "" {
		http.RedirectHandler("/", 302).ServeHTTP(w, r)
	}

	if r.Method == "DELETE" {
		r.ParseForm()
		data.DeleteOne("drone", map[string]interface{}{"uid": droneID})
		w.Write([]byte("deleted"))
		return
	}

	var drone data.Drone
	err := data.FindOne("drone", map[string]interface{}{"uid": droneID}, &drone)
	if err != nil {
		http.RedirectHandler("/fleets", 302).ServeHTTP(w, r)
		return
	}

	drone.DeviceConfig.Server.URL = web.CleanServerURL(drone.DeviceConfig.Server.URL)

	template.Must(template.ParseFiles("templates/base.html", "templates/drone-details.html")).Execute(w, drone)
}

// createDrone handles POST request to create a new drone with configuration
func createDrone(w http.ResponseWriter, r *http.Request) {
	fleetID := chi.URLParam(r, "fleet_id")
	if fleetID == "" {
		http.Error(w, "missing fleet_id", http.StatusBadRequest)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	fmt.Println(r.Form)

	uuid := data.GenerateUID()
	serverURL := getServerPath(r)

	// Build MAVLink config
	mavlinkConfig := data.MAVLinkConfig{
		SerialPort:      r.Form.Get("serial_port"),
		TCPAddress:      r.Form.Get("tcp_address"),
		StreamFrequency: 10,
		OutSystemID:     255,
	}

	if streamFreqStr := r.Form.Get("stream_frequency"); streamFreqStr != "" {
		if streamFreq, err := strconv.Atoi(streamFreqStr); err == nil {
			mavlinkConfig.StreamFrequency = streamFreq
		}
	}

	// Build Tunnel config
	tunnelPorts := []string{}
	portValues := r.Form["tunnel_ports[]"]
	for _, port := range portValues {
		port = strings.TrimSpace(port)
		if port != "" {
			tunnelPorts = append(tunnelPorts, port)
		}
	}
	if len(tunnelPorts) == 0 {
		tunnelPorts = []string{"5760"}
	}

	// Build Stats config
	statsEnabled := r.Form.Get("stats_enabled") == "on"
	statsInterval := 5 * time.Second
	if intervalStr := r.Form.Get("stats_interval"); intervalStr != "" {
		if intervalSec, err := strconv.Atoi(intervalStr); err == nil && intervalSec >= 1 {
			statsInterval = time.Duration(intervalSec) * time.Second
		}
	}

	// Build complete config
	deviceConfig := data.Config{
		UUID:    uuid,
		MAVLink: mavlinkConfig,
		Server: data.ServerConfig{
			URL: serverURL,
		},
		Tunnel: data.TunnelConfig{
			Ports: tunnelPorts,
		},
		Stats: data.StatsConfig{
			Enabled:  statsEnabled,
			Interval: statsInterval,
		},
	}

	// Apply defaults
	deviceConfig.ApplyDefaults()

	// Validate config
	if err := deviceConfig.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("config validation failed: %v", err), http.StatusBadRequest)
		return
	}

	// Create drone
	drone := data.Drone{
		UID:          uuid,
		Name:         r.Form.Get("name"),
		Description:  r.Form.Get("description"),
		FleetID:      fleetID,
		DeviceConfig: deviceConfig,
	}

	if err := data.InsertOne("drone", drone); err != nil {
		log.Printf("Error creating drone: %v", err)
		http.Error(w, "failed to create drone", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/fleets/"+fleetID, http.StatusSeeOther)
}

func DeviceConfigHandler(w http.ResponseWriter, r *http.Request) {
	droneID := chi.URLParam(r, "drone_id")
	if droneID == "" {
		http.Error(w, "missing uuid", http.StatusBadRequest)
		return
	}

	// Fetch drone from database
	var drone data.Drone
	err := data.FindOne("drone", map[string]interface{}{"uid": droneID}, &drone)
	if err != nil {
		// If drone not found, return default config
		serverURL := getServerPath(r)
		cfg := data.NewDefaultDeviceConfig(droneID, serverURL)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
		return
	}

	// Use device config from database
	cfg := drone.DeviceConfig

	// Ensure UUID is set
	if cfg.UUID == "" {
		cfg.UUID = droneID
	}

	// Update server URL to current request URL
	serverURL := getServerPath(r)
	cfg.Server.URL = serverURL

	// Apply defaults for any missing fields
	cfg.ApplyDefaults()

	// Validate config
	if err := cfg.Validate(); err != nil {
		log.Printf("Config validation error for drone %s: %v", droneID, err)
		// Still return the config, but log the error
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

func deviceStatus(w http.ResponseWriter, r *http.Request) {
	droneID := chi.URLParam(r, "drone_id")
	if droneID == "" {
		http.RedirectHandler("/", 302).ServeHTTP(w, r)
	}

	if r.Method == "GET" {
		var drone data.Drone
		err := data.FindOne("drone", map[string]interface{}{"uid": droneID}, &drone)
		if err != nil {
			log.Println("Error finding drone: ", err)
			w.Write([]byte("error"))
			return
		}
		json.NewEncoder(w).Encode(drone)
		return
	}

	var deviceStatus data.ResourceStats
	json.NewDecoder(r.Body).Decode(&deviceStatus)

	if err := data.UpdateOne("drone", map[string]interface{}{"uid": droneID}, map[string]interface{}{"status": deviceStatus}); err != nil {
		log.Println("Error updating drone status: ", err)
		w.Write([]byte("error"))
		return
	}

	w.Write([]byte("status updated"))
}

// getInstallCommand returns the install command for a specific drone
func getInstallCommand(w http.ResponseWriter, r *http.Request) {
	fleetID := chi.URLParam(r, "fleet_id")
	droneID := chi.URLParam(r, "drone_id")

	if fleetID == "" || droneID == "" {
		http.Error(w, "missing fleet_id or drone_id", http.StatusBadRequest)
		return
	}

	// Verify drone exists and belongs to fleet
	var drone data.Drone
	err := data.FindOne("drone", map[string]interface{}{"uid": droneID, "fleet_id": fleetID}, &drone)
	if err != nil {
		http.Error(w, "drone not found", http.StatusNotFound)
		return
	}

	serverURL := getServerPath(r)
	command := fmt.Sprintf("wget -O - %s/device/%s/installer.sh > install.sh && sudo sh install.sh", serverURL, droneID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"command": command,
	})
}

// getInstallerScript returns the installer script for a specific drone
func getInstallerScript(w http.ResponseWriter, r *http.Request) {
	droneID := chi.URLParam(r, "drone_id")
	if droneID == "" {
		http.Error(w, "missing drone_id", http.StatusBadRequest)
		return
	}

	serverURL := getServerPath(r)
	script := GenerateInstallerScript(serverURL, droneID)

	w.Header().Set("Content-Type", "text/x-shellscript")
	w.Write([]byte(script))
}

func getServerPath(r *http.Request) string {
	// Determine the scheme (http or https)
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
		// Useful if behind a reverse proxy/load balancer
		scheme = forwardedProto
	}

	// Get the host (domain or IP:port)
	host := r.Host

	// Build the base URL
	baseURL := fmt.Sprintf("%s://%s", scheme, host)

	return baseURL
}
