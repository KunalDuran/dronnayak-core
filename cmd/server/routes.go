package main

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/KunalDuran/dronnayak-core/internal/data"
	"github.com/KunalDuran/dronnayak-core/internal/web"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	tmpl     map[string]*template.Template
	validUID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,50}$`)
)

func initTemplates() {
	tmpl = map[string]*template.Template{
		"index":         template.Must(template.ParseFiles("templates/base.html", "templates/index.html")),
		"login":         template.Must(template.ParseFiles("templates/login.html")),
		"signup":        template.Must(template.ParseFiles("templates/signup.html")),
		"fleets":        template.Must(template.ParseFiles("templates/base.html", "templates/fleets.html")),
		"drones":        template.Must(template.ParseFiles("templates/base.html", "templates/drones.html")),
		"drone-details": template.Must(template.ParseFiles("templates/base.html", "templates/drone-details.html")),
		"log-viewer":    template.Must(template.ParseFiles("templates/base.html", "templates/log-viewer.html")),
	}
}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	t, ok := tmpl[name]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	if err := t.Execute(w, data); err != nil {
		slog.Error("template execution error", "template", name, "error", err)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "index", nil)
}

func login(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()

		email := r.Form.Get("email")
		password := r.Form.Get("password")

		var user data.User
		err := data.FindOne("user", map[string]interface{}{"email": email}, &user)
		if err != nil || user.Email == "" {
			slog.Warn("login failed: user not found", "email", email)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
			slog.Warn("login failed: wrong password", "email", email)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		token, err := generateSessionToken()
		if err != nil {
			slog.Error("failed to generate session token", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		sessions.Store(token, user.Email)
		setSessionCookie(w, token)

		slog.Info("user logged in", "email", user.Email)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	renderTemplate(w, "login", nil)
}

func signup(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()

		hashed, err := bcrypt.GenerateFromPassword([]byte(r.Form.Get("password")), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "failed to process password", http.StatusInternalServerError)
			return
		}

		u := data.User{
			Name:     r.Form.Get("name"),
			Email:    r.Form.Get("email"),
			Password: string(hashed),
		}

		if err := data.InsertOne("user", u); err != nil {
			slog.Error("failed to create user", "email", u.Email, "error", err)
			http.Error(w, "failed to create user", http.StatusInternalServerError)
			return
		}
		slog.Info("user registered", "email", u.Email)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	renderTemplate(w, "signup", nil)
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

		if err := data.InsertOne("fleet", f); err != nil {
			slog.Error("failed to create fleet", "user_id", userID, "error", err)
			http.Error(w, "failed to create fleet", http.StatusInternalServerError)
			return
		}
		slog.Info("fleet created", "fleet_uid", f.UID, "user_id", userID)
	}

	var fleetList []data.Fleet
	if err := data.FindAll("fleet", map[string]interface{}{"user_id": userID}, &fleetList); err != nil {
		slog.Error("failed to fetch fleets", "user_id", userID, "error", err)
		http.Error(w, "failed to fetch fleets", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "fleets", fleetList)
}

func devices(w http.ResponseWriter, r *http.Request) {
	fleetID := chi.URLParam(r, "fleet_id")

	if fleetID == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	var drones []data.Drone
	err := data.FindAll("drone", map[string]interface{}{"fleet_id": fleetID}, &drones)
	if err != nil {
		http.Redirect(w, r, "/fleets", http.StatusFound)
		return
	}

	result := struct {
		Drones []data.Drone
		ID     string
	}{
		Drones: drones,
		ID:     fleetID,
	}

	renderTemplate(w, "drones", result)
}

func deviceDetails(w http.ResponseWriter, r *http.Request) {
	droneID := chi.URLParam(r, "drone_id")

	if droneID == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if r.Method == "DELETE" {
		if err := data.DeleteOne("drone", map[string]interface{}{"uid": droneID}); err != nil {
			slog.Error("failed to delete drone", "drone_id", droneID, "error", err)
			http.Error(w, "failed to delete drone", http.StatusInternalServerError)
			return
		}
		slog.Info("drone deleted", "drone_id", droneID)
		w.Write([]byte("deleted"))
		return
	}

	var drone data.Drone
	err := data.FindOne("drone", map[string]interface{}{"uid": droneID}, &drone)
	if err != nil {
		http.Redirect(w, r, "/fleets", http.StatusFound)
		return
	}

	drone.DeviceConfig.Server.URL = web.CleanServerURL(getServerPath(r))

	view := struct {
		data.Drone
		StatsIntervalSec int64
	}{
		Drone:            drone,
		StatsIntervalSec: int64(drone.DeviceConfig.Stats.Interval / time.Second),
	}

	renderTemplate(w, "drone-details", view)
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

	deviceConfig.ApplyDefaults()

	if err := deviceConfig.Validate(); err != nil {
		http.Error(w, "config validation failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	drone := data.Drone{
		UID:          uuid,
		Name:         r.Form.Get("name"),
		Description:  r.Form.Get("description"),
		FleetID:      fleetID,
		DeviceConfig: deviceConfig,
	}

	if err := data.InsertOne("drone", drone); err != nil {
		slog.Error("failed to create drone", "fleet_id", fleetID, "error", err)
		http.Error(w, "failed to create drone", http.StatusInternalServerError)
		return
	}
	slog.Info("drone created", "drone_id", drone.UID, "fleet_id", fleetID)

	http.Redirect(w, r, "/fleets/"+fleetID, http.StatusSeeOther)
}

func DeviceConfigHandler(w http.ResponseWriter, r *http.Request) {
	droneID := chi.URLParam(r, "drone_id")
	if droneID == "" {
		http.Error(w, "missing uuid", http.StatusBadRequest)
		return
	}

	rawConfig := r.URL.Query().Has("raw")

	var drone data.Drone
	err := data.FindOne("drone", map[string]interface{}{"uid": droneID}, &drone)
	if err != nil {
		cfg := data.NewDefaultDeviceConfig(droneID, getServerPath(r))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
		return
	}

	cfg := drone.DeviceConfig
	if cfg.UUID == "" {
		cfg.UUID = droneID
	}

	if !rawConfig {
		cfg.Server.URL = getServerPath(r)
		cfg.ApplyDefaults()
	}

	if err := cfg.Validate(); err != nil {
		slog.Warn("drone config validation error", "drone_id", droneID, "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

func updateDeviceConfig(w http.ResponseWriter, r *http.Request) {
	droneID := chi.URLParam(r, "drone_id")
	if droneID == "" {
		http.Error(w, "missing drone_id", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1*1024*1024)
	var cfg data.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cfg.UUID = droneID
	cfg.ApplyDefaults()

	if err := cfg.Validate(); err != nil {
		http.Error(w, "config validation failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := data.UpdateOne("drone", map[string]interface{}{"uid": droneID}, map[string]interface{}{"device_config": cfg}); err != nil {
		slog.Error("failed to update drone config", "drone_id", droneID, "error", err)
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	slog.Info("drone config updated", "drone_id", droneID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

func deviceStatus(w http.ResponseWriter, r *http.Request) {
	droneID := chi.URLParam(r, "drone_id")
	if droneID == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if r.Method == "GET" {
		var drone data.Drone
		if err := data.FindOne("drone", map[string]interface{}{"uid": droneID}, &drone); err != nil {
			slog.Error("failed to find drone", "drone_id", droneID, "error", err)
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(drone)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1*1024*1024) // 1MB limit
	var status data.ResourceStats
	if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	status.LastUpdated = time.Now().Unix()
	if err := data.UpdateOne("drone", map[string]interface{}{"uid": droneID}, map[string]interface{}{"status": status}); err != nil {
		slog.Error("failed to update drone status", "drone_id", droneID, "error", err)
		http.Error(w, "error", http.StatusInternalServerError)
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

	if !validUID.MatchString(droneID) {
		http.Error(w, "invalid drone_id", http.StatusBadRequest)
		return
	}

	var drone data.Drone
	if err := data.FindOne("drone", map[string]interface{}{"uid": droneID, "fleet_id": fleetID}, &drone); err != nil {
		http.Error(w, "drone not found", http.StatusNotFound)
		return
	}

	serverURL := getServerPath(r)
	command := "wget -O - " + serverURL + "/device/" + droneID + "/installer.sh > install.sh && sudo sh install.sh"

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"command": command})
}

// getInstallerScript returns the installer script for a specific drone
func getInstallerScript(w http.ResponseWriter, r *http.Request) {
	droneID := chi.URLParam(r, "drone_id")
	if droneID == "" {
		http.Error(w, "missing drone_id", http.StatusBadRequest)
		return
	}

	if !validUID.MatchString(droneID) {
		http.Error(w, "invalid drone_id", http.StatusBadRequest)
		return
	}

	serverURL := getServerPath(r)
	script := GenerateInstallerScript(serverURL, droneID)

	w.Header().Set("Content-Type", "text/x-shellscript")
	w.Write([]byte(script))
}

func getServerPath(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
		scheme = forwardedProto
	}
	return scheme + "://" + r.Host
}

func logViewer(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "log-viewer", nil)
}
