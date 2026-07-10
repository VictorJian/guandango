package main

import (
	"log"
	"net/http"
	"os"

	"guandango/internal/server"
)

func main() {
	roomManager := server.NewRoomManager()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.WSHandler(roomManager))

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "web/dist"
	}
	mux.HandleFunc("/", server.StaticHandler(staticDir))

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("Server running on port %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
