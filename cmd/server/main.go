package main

import (
	"log"
	"net/http"
	"os"

	"github.com/KunalDuran/gowsrelay/server"

	"github.com/KunalDuran/dronnayak-core/internal/data"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func main() {

	mongoURI := os.Getenv("MONGO_URI")

	data.InitDB(mongoURI)
	initTemplates()

	r := chi.NewRouter()

	r.Use(middleware.Logger)
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

	// Public routes for device installation
	r.Get("/device/{drone_id}/installer.sh", getInstallerScript)

	log.Println("Server started on port 8090")
	log.Fatal(http.ListenAndServe("0.0.0.0:8090", r))
}
