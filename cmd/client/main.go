package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/KunalDuran/devstat"

	"github.com/KunalDuran/dronnayak-core/internal/web"
)

func main() {
	app := NewDronenayak()

	go sendStats(app.Config.ServerPath + "/device-status/" + app.Config.UUID)

	app.Boot()
	defer app.MavNode.Close()
}

func sendStats(url string) {
	devstat.Stats()
	for {
		<-time.After(5 * time.Second)
		statsData, err := devstat.Stats()
		if err != nil {
			log.Println("Error fetching stats: ", err)
			continue
		}
		jsonData, err := json.Marshal(statsData)
		if err != nil {
			log.Println("Error converting stats to json: ", err)
			continue
		}
		_, _, err = web.WebRequest(http.MethodPost, url, string(jsonData))
		if err != nil {
			log.Println("Error sending stats to endpoint: ", err)
			continue
		}
	}
}
