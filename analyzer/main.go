package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"shared/types"
	"syscall"

	"github.com/nats-io/nats.go"
)

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	sub, err := nc.Subscribe("url.analyze", func(msg *nats.Msg) {
		var req types.AnalyzeRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			log.Printf("Failed to unmarshal: %v", err)
			return
		}

		log.Printf("Message received %+v", req)
	})
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	log.Println("Analyzer service is running...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down analyzer...")
}
