package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// Same-origin serving; allow all origins like the original socket.io config
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSHandler upgrades connections and wires clients to the room manager.
func WSHandler(roomManager *RoomManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
			return
		}

		client := NewClient(conn, func(c *Client) {
			log.Printf("User disconnected: %s", c.ID)
			roomManager.HandleDisconnect(c)
		})

		client.On("joinRoom", func(data json.RawMessage) {
			var req struct {
				PlayerName string `json:"playerName"`
				RoomID     string `json:"roomId"`
			}
			if err := json.Unmarshal(data, &req); err != nil || req.PlayerName == "" {
				return
			}
			roomManager.JoinRoom(client, req.PlayerName, req.RoomID)
		})
		client.On("getRoomList", func(json.RawMessage) {
			roomManager.HandleGetRoomList(client)
		})

		log.Printf("User connected: %s", client.ID)
		client.Emit("connected", map[string]string{"id": client.ID})

		client.Run()
	}
}

// StaticHandler serves the built SPA with an index.html fallback.
// index.html 不快取：資產檔名有 hash 可長存，但 HTML 一定要拿最新版，
// 否則手機瀏覽器會一直用舊版程式。
func StaticHandler(staticDir string) http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(staticDir))
	return func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(staticDir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			if r.URL.Path != "/" {
				w.Header().Set("Cache-Control", "no-cache")
				http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
				return
			}
		}
		if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	}
}
