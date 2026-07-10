package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"guandango/internal/game"
)

// newTestRoom seats 4 players (no websocket clients) so the engine can be
// driven directly, simulating 4 human players.
func newTestRoom(t *testing.T) *Room {
	t.Helper()
	room := NewRoom("engine-test")
	room.withLock(func() {
		for i := 0; i < 4; i++ {
			room.players[i] = &Player{
				ID:        fmt.Sprintf("p%d", i),
				Name:      fmt.Sprintf("Player %d", i),
				SeatIndex: i,
				IsReady:   true,
			}
		}
		if !room.startGame() {
			t.Fatal("startGame failed with 4 seated players")
		}
	})
	return room
}

// step performs one action for whoever must act, using a naive but legal
// strategy: pay largest tribute, return smallest, play smallest single on a
// free turn, beat singles when possible, otherwise pass.
func step(room *Room) {
	room.withLock(func() {
		if room.match == nil || room.match.CurrentGame == nil {
			return
		}
		g := room.match.CurrentGame

		switch g.currentPhase {
		case PhaseTribute:
			for _, tr := range g.tributeState.PendingTributes {
				if tr.Card == nil {
					largest := game.GetLargestCard(g.hands[tr.From], g.level)
					g.handleTribute(tr.From, []game.Card{largest})
					return
				}
			}

		case PhaseReturnTribute:
			for _, ret := range g.tributeState.PendingReturns {
				if ret.Card == nil {
					hand := g.hands[ret.From]
					smallest := hand[len(hand)-1]
					g.handleReturnTribute(ret.From, []game.Card{smallest})
					return
				}
			}

		case PhasePlaying:
			seat := g.currentTurn
			hand := g.hands[seat]
			if len(hand) == 0 {
				return
			}
			if g.lastHand == nil {
				// Free turn: play smallest card
				g.handlePlayHand(seat, []game.Card{hand[len(hand)-1]}, nil)
				return
			}
			if g.lastHand.Hand.Type == game.Single {
				for i := len(hand) - 1; i >= 0; i-- {
					if game.GetLogicValue(hand[i].Rank, g.level) > g.lastHand.Hand.Value {
						g.handlePlayHand(seat, []game.Card{hand[i]}, nil)
						return
					}
				}
			}
			g.handlePass(seat)
		}
	})
}

// TestFullMatch drives an entire match (2 打到 A) with 4 simulated human
// players and asserts it terminates with a winning team at level A.
func TestFullMatch(t *testing.T) {
	nextGameDelay = time.Millisecond
	defer func() { nextGameDelay = 3 * time.Second }()

	room := newTestRoom(t)

	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		var winner *int
		room.withLock(func() {
			if room.match != nil {
				winner = room.match.MatchWinner
			}
		})
		if winner != nil {
			t.Logf("Match won by Team %d", *winner)
			room.withLock(func() {
				if lvl := room.match.teamLevels[*winner]; lvl != 14 {
					t.Errorf("winning team should be at level 14 (A), got %d", lvl)
				}
			})
			return
		}
		step(room)
	}
	t.Fatal("match did not finish within timeout")
}

// TestTributeFlow plays one game to completion and verifies the second game
// enters the tribute (or playing, on anti-tribute/tie) phase correctly.
func TestTributeFlow(t *testing.T) {
	nextGameDelay = time.Millisecond
	defer func() { nextGameDelay = 3 * time.Second }()

	room := newTestRoom(t)

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		secondGame := false
		var phase GamePhase
		room.withLock(func() {
			if g := room.match.CurrentGame; g != nil {
				secondGame = len(g.prevWinners) == 4 // set only from the 2nd game on
				phase = g.currentPhase
			}
		})
		if secondGame && (phase == PhaseTribute || phase == PhaseReturnTribute || phase == PhasePlaying) {
			return // second game started and reached a playable phase
		}
		step(room)
	}
	t.Fatal("second game did not start within timeout")
}

type wsClient struct {
	t    *testing.T
	conn *websocket.Conn
	got  map[string]json.RawMessage
}

func dialWS(t *testing.T, srvURL string) *wsClient {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srvURL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return &wsClient{t: t, conn: conn, got: map[string]json.RawMessage{}}
}

func (c *wsClient) send(event string, data any) {
	payload, _ := json.Marshal(map[string]any{"event": event, "data": data})
	if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		c.t.Fatalf("write %s: %v", event, err)
	}
}

func (c *wsClient) waitFor(event string) json.RawMessage {
	if v, ok := c.got[event]; ok {
		delete(c.got, event)
		return v
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			c.t.Fatalf("waiting for %s: %v", event, err)
		}
		var msg struct {
			Event string          `json:"event"`
			Data  json.RawMessage `json:"data"`
		}
		if json.Unmarshal(raw, &msg) != nil {
			continue
		}
		if msg.Event == event {
			return msg.Data
		}
		c.got[msg.Event] = msg.Data
	}
}

// TestWebSocketFourPlayers verifies the real websocket flow: 4 players join,
// starting with fewer players fails, and once everyone is ready the match
// starts and each player receives their own 27-card hand.
func TestWebSocketFourPlayers(t *testing.T) {
	rm := NewRoomManager()
	srv := httptest.NewServer(http.HandlerFunc(WSHandler(rm)))
	defer srv.Close()

	host := dialWS(t, srv.URL)
	defer host.conn.Close()
	host.waitFor("connected")
	host.send("joinRoom", map[string]any{"playerName": "Host", "roomId": "ws-room"})
	host.waitFor("roomState")

	// Starting with only 1 player must fail (no bots to fill seats)
	host.send("start", nil)
	var errMsg string
	if json.Unmarshal(host.waitFor("error"), &errMsg) != nil || !strings.Contains(errMsg, "4位玩家") {
		t.Fatalf("expected 'need 4 players' error, got %q", errMsg)
	}

	// Other 3 players join and everyone readies up -> auto start
	others := make([]*wsClient, 3)
	for i := range others {
		others[i] = dialWS(t, srv.URL)
		defer others[i].conn.Close()
		others[i].waitFor("connected")
		others[i].send("joinRoom", map[string]any{"playerName": fmt.Sprintf("P%d", i+1), "roomId": "ws-room"})
		others[i].waitFor("roomState")
	}

	host.send("ready", nil)
	for _, c := range others {
		c.send("ready", nil)
	}

	all := append([]*wsClient{host}, others...)
	for seat, c := range all {
		c.waitFor("matchStarted")
		var gs struct {
			Phase    string `json:"phase"`
			GameMode string `json:"gameMode"`
			Hands    []any  `json:"hands"`
		}
		if err := json.Unmarshal(c.waitFor("gameState"), &gs); err != nil {
			t.Fatalf("seat %d: bad gameState: %v", seat, err)
		}
		if gs.Phase != "Playing" {
			t.Errorf("seat %d: expected Playing phase, got %s", seat, gs.Phase)
		}
		if gs.GameMode != "Normal" {
			t.Errorf("seat %d: expected Normal mode, got %s", seat, gs.GameMode)
		}
		myHand, ok := gs.Hands[seat].([]any)
		if !ok || len(myHand) != 27 {
			t.Fatalf("seat %d: expected own 27-card hand, got %T", seat, gs.Hands[seat])
		}
		// All other hands must be hidden counts
		for other := 0; other < 4; other++ {
			if other == seat {
				continue
			}
			if _, isNum := gs.Hands[other].(float64); !isNum {
				t.Errorf("seat %d: expected hidden count for seat %d", seat, other)
			}
		}
	}
}
