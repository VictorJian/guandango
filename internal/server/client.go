package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 45 * time.Second
)

// message is the wire format: {"event": "...", "data": ...}
type message struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// Client wraps a websocket connection with socket.io-like event semantics:
// one handler per event name, Emit sends {event, data} JSON frames.
type Client struct {
	ID string

	conn *websocket.Conn
	send chan []byte

	mu       sync.RWMutex
	handlers map[string]func(json.RawMessage)
	closed   bool

	onDisconnect func(*Client)
}

func newClientID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func NewClient(conn *websocket.Conn, onDisconnect func(*Client)) *Client {
	return &Client{
		ID:           newClientID(),
		conn:         conn,
		send:         make(chan []byte, 256),
		handlers:     map[string]func(json.RawMessage){},
		onDisconnect: onDisconnect,
	}
}

// On registers the handler for an event, replacing any previous one.
func (c *Client) On(event string, handler func(json.RawMessage)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[event] = handler
}

// Off removes the handler for an event.
func (c *Client) Off(events ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range events {
		delete(c.handlers, e)
	}
}

// Emit sends an event to this client. Safe to call from any goroutine;
// drops the message if the client's buffer is full or connection closed.
func (c *Client) Emit(event string, data any) {
	payload, err := json.Marshal(struct {
		Event string `json:"event"`
		Data  any    `json:"data,omitempty"`
	}{event, data})
	if err != nil {
		log.Printf("[Client %s] marshal error for %s: %v", c.ID, event, err)
		return
	}

	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()
	if closed {
		return
	}

	select {
	case c.send <- payload:
	default:
		log.Printf("[Client %s] send buffer full, dropping %s", c.ID, event)
	}
}

// Run starts the read/write pumps and blocks until the connection closes.
func (c *Client) Run() {
	go c.writePump()
	c.readPump()
}

func (c *Client) readPump() {
	defer c.close()
	c.conn.SetReadLimit(64 * 1024)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg message
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("[Client %s] bad message: %v", c.ID, err)
			continue
		}
		c.mu.RLock()
		handler := c.handlers[msg.Event]
		c.mu.RUnlock()
		if handler != nil {
			handler(msg.Data)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case payload, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.send)
	c.mu.Unlock()

	_ = c.conn.Close()
	if c.onDisconnect != nil {
		c.onDisconnect(c)
	}
}
