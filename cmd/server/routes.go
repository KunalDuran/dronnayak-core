package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/KunalDuran/dronnayak/internal/data"
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

	template.Must(template.ParseFiles("templates/base.html", "templates/drone-details.html")).Execute(w, drone)
}

func registerDevice(w http.ResponseWriter, r *http.Request) {
	fleetID := chi.URLParam(r, "fleet_id")
	if fleetID == "" {
		http.RedirectHandler("/", 302).ServeHTTP(w, r)
	}

	uuid := data.GenerateUID()
	err := data.InsertOne("drone", data.Drone{UID: uuid, FleetID: fleetID})
	if err != nil {
		log.Println(err)
		w.Write([]byte("error"))
		return
	}

	// serverPath := getServerPath(r)
	file := GetInstallerScript("https://dashboard.dronnayak.com", uuid)

	w.Write([]byte(file))

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
