package main

import (
	"encoding/json"
	"log"
	"net/http"
	"shared/types"

	"github.com/nats-io/nats.go"
	"github.com/yousuf64/shift"
)

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	router := shift.New()
	router.POST("/analyze", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		var req types.AnalyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return err
		}

		data, err := json.Marshal(req)
		if err != nil {
			return err
		}

		err = nc.Publish("url.analyze", data)
		if err != nil {
			return err
		}
		log.Printf("Message published: %v", req)

		w.WriteHeader(http.StatusAccepted)
		return nil
	})

	log.Printf("API server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", router.Serve()))
}
