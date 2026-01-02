package main

import (
	"log"
	"net/http"
	"os"

	"github.com/KunalDuran/dronnayak/internal/data"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {

	mongoURI := os.Getenv("MONGO_URI")

	data.InitDB(mongoURI)

	r := chi.NewRouter()

	r.Use(middleware.Logger)

	fs := http.FileServer(http.Dir("./static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	r.Handle("/bin/*", http.StripPrefix("/bin/", http.FileServer(http.Dir("./bin"))))

	r.Get("/login", login)
	r.Post("/login", login)
	r.Get("/logout", logout)
	r.Get("/signup", signup)
	r.Post("/signup", signup)
	r.Get("/register/{fleet_id}", registerDevice)
	r.Post("/device-status/{drone_id}", deviceStatus)
	r.Get("/device-status/{drone_id}", deviceStatus)

	r.HandleFunc("/ws", handleWebSocket)
	r.HandleFunc("/tcp", handleTCPProxy)
	r.HandleFunc("/status", handleStatus)
	r.HandleFunc("/health", handleHealth)

	r.Group(func(rauth chi.Router) {
		rauth.Use(SessionAuth)

		rauth.Get("/", index)
		rauth.Get("/fleets", fleets)
		rauth.Post("/fleets", fleets)
		rauth.Get("/fleets/{fleet_id}", devices)
		rauth.Get("/device/{drone_id}", deviceDetails)
		rauth.Delete("/device/{drone_id}", deviceDetails)
	})

	log.Println("Server started on port 8090")
	log.Fatal(http.ListenAndServe("0.0.0.0:8090", r))
}
