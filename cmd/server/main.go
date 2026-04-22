package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/KunalDuran/gowsrelay/server"

	"github.com/KunalDuran/dronnayak-core/internal/data"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func initLogger() {
	level := parseLogLevel(os.Getenv("LOG_LEVEL"))
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug, // include file/line only in debug mode
	}

	var handler slog.Handler
	if os.Getenv("LOG_FORMAT") == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// requestLogger is a structured slog-based replacement for middleware.Logger.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func main() {
	initLogger()

	mongoURI := os.Getenv("MONGO_URI")
	data.InitDB(mongoURI)
	initTemplates()

	server.Configure(server.Config{})

	r := chi.NewRouter()

	r.Use(requestLogger)
	r.Use(securityHeaders)

	fs := http.FileServer(http.Dir("./static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))
	r.Handle("/bin/*", http.StripPrefix("/bin/", http.FileServer(http.Dir("./bin"))))

	r.Get("/login", login)
	r.Post("/login", login)
	r.Get("/logout", logout)
	r.Get("/signup", signup)
	r.Post("/signup", signup)
	r.Get("/device/{drone_id}/config.json", DeviceConfigHandler)
	r.Post("/device-status/{drone_id}", deviceStatus)
	r.Get("/device-status/{drone_id}", deviceStatus)
	r.Put("/device/{drone_id}/config", updateDeviceConfig)

	r.HandleFunc("/ws", server.HandleWebSocket)
	r.HandleFunc("/tcp", server.HandleTCPProxy)
	r.HandleFunc("/status", server.HandleStatus)
	r.HandleFunc("/health", server.HandleHealth)
	r.HandleFunc("/persistence", server.HandleAdminPersist)

	r.Group(func(rauth chi.Router) {
		rauth.Use(SessionAuth)

		rauth.Get("/", index)
		rauth.Get("/fleets", fleets)
		rauth.Post("/fleets", fleets)
		rauth.Get("/fleets/{fleet_id}", devices)
		rauth.Post("/fleets/{fleet_id}/drones", createDrone)
		rauth.Get("/fleets/{fleet_id}/drones/{drone_id}/install-command", getInstallCommand)
		rauth.Get("/device/{drone_id}", deviceDetails)
		rauth.Delete("/device/{drone_id}", deviceDetails)
		rauth.Get("/device/{drone_id}/logs", logViewer)
	})

	r.Get("/device/{drone_id}/installer.sh", getInstallerScript)

	addr := "0.0.0.0:8090"
	slog.Info("server starting", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
