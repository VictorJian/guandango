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

// driveTribute pays the largest card for the first pending tribute.
func driveTribute(g *Game) {
	for _, tr := range g.tributeState.PendingTributes {
		if tr.Card == nil {
			largest := game.GetLargestCard(g.hands[tr.From], g.level)
			g.handleTribute(tr.From, []game.Card{largest})
			return
		}
	}
}

// driveReturnTribute returns the smallest eligible card (rank <= 10 and not
// the level card), or the smallest card overall if none qualify.
func driveReturnTribute(g *Game) {
	for _, ret := range g.tributeState.PendingReturns {
		if ret.Card == nil {
			hand := g.hands[ret.From]
			pick := hand[len(hand)-1]
			for i := len(hand) - 1; i >= 0; i-- {
				if hand[i].Rank <= 10 && int(hand[i].Rank) != g.level {
					pick = hand[i]
					break
				}
			}
			g.handleReturnTribute(ret.From, []game.Card{pick})
			return
		}
	}
}

// drivePlaying plays the smallest single on a free turn, beats singles when
// possible, otherwise passes.
func drivePlaying(g *Game) {
	seat := g.currentTurn
	hand := g.hands[seat]
	if len(hand) == 0 {
		return
	}
	if g.lastHand == nil {
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

// step performs one action for whoever must act, using a naive but legal
// strategy.
func step(room *Room) {
	room.withLock(func() {
		if room.match == nil || room.match.CurrentGame == nil {
			return
		}
		g := room.match.CurrentGame

		switch g.currentPhase {
		case PhaseTribute:
			driveTribute(g)
		case PhaseReturnTribute:
			driveReturnTribute(g)
		case PhaseTributeConfirm:
			g.handleConfirmStart(g.confirmSeat) // 第四名確認開局
		case PhasePlaying:
			drivePlaying(g)
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

// TestReturnTributeRankLimit verifies the return-tribute rule: cards above
// rank 10 are rejected while a smaller card is available.
func TestReturnTributeRankLimit(t *testing.T) {
	nextGameDelay = time.Millisecond
	defer func() { nextGameDelay = 3 * time.Second }()

	room := newTestRoom(t)

	// Drive games until a ReturnTribute phase appears, without auto-returning.
	deadline := time.Now().Add(120 * time.Second)
	reached := false
	for time.Now().Before(deadline) && !reached {
		room.withLock(func() {
			g := room.match.CurrentGame
			if g == nil {
				return
			}
			switch g.currentPhase {
			case PhaseReturnTribute:
				reached = true
			case PhaseTribute:
				driveTribute(g)
			case PhasePlaying:
				drivePlaying(g)
			}
		})
	}
	if !reached {
		t.Fatal("did not reach ReturnTribute phase within timeout")
	}

	room.withLock(func() {
		g := room.match.CurrentGame
		var ret *TributeEntry
		for _, r := range g.tributeState.PendingReturns {
			if r.Card == nil {
				ret = r
				break
			}
		}
		if ret == nil {
			t.Fatal("no pending return found")
		}

		hand := g.hands[ret.From]
		eligible := func(c game.Card) bool { return c.Rank <= 10 && int(c.Rank) != g.level }
		var big, levelCard, small *game.Card
		for i := range hand {
			if hand[i].Rank > 10 && big == nil {
				big = &hand[i]
			}
			if int(hand[i].Rank) == g.level && levelCard == nil {
				levelCard = &hand[i]
			}
			if eligible(hand[i]) && small == nil {
				small = &hand[i]
			}
		}

		if big != nil && small != nil {
			g.handleReturnTribute(ret.From, []game.Card{*big})
			if ret.Card != nil {
				t.Error("return with rank > 10 should be rejected while eligible cards remain")
			}
		}
		if levelCard != nil && small != nil {
			g.handleReturnTribute(ret.From, []game.Card{*levelCard})
			if ret.Card != nil {
				t.Error("return with the level card should be rejected while eligible cards remain")
			}
		}
		if small != nil {
			g.handleReturnTribute(ret.From, []game.Card{*small})
			if ret.Card == nil {
				t.Error("valid return (<=10, not level) should be accepted")
			}
		}
	})

	// Complete remaining returns, then verify the 4th-place confirm gate
	room.withLock(func() {
		g := room.match.CurrentGame
		for g.currentPhase == PhaseReturnTribute {
			driveReturnTribute(g)
		}
		if g.currentPhase != PhaseTributeConfirm {
			t.Fatalf("expected TributeConfirm after returns, got %s", g.currentPhase)
		}

		// Playing is blocked until confirmation
		drivePlaying(g)
		if g.currentPhase != PhaseTributeConfirm {
			t.Fatal("play should be ignored before confirmation")
		}

		// Only the previous game's 4th place can confirm
		wrong := (g.confirmSeat + 1) % 4
		g.handleConfirmStart(wrong)
		if g.currentPhase != PhaseTributeConfirm {
			t.Fatal("non-4th-place confirm should be rejected")
		}

		g.handleConfirmStart(g.confirmSeat)
		if g.currentPhase != PhasePlaying {
			t.Fatalf("expected Playing after 4th-place confirm, got %s", g.currentPhase)
		}
		// The confirming 4th-place player leads the new game
		if g.currentTurn != g.confirmSeat {
			t.Errorf("expected 4th place (seat %d) to lead, got seat %d", g.confirmSeat, g.currentTurn)
		}
	})
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

// TestSpectatorMode verifies that a 5th client joining a full room becomes a
// spectator, sees the watched player's hand, and can switch targets.
func TestSpectatorMode(t *testing.T) {
	rm := NewRoomManager()
	srv := httptest.NewServer(http.HandlerFunc(WSHandler(rm)))
	defer srv.Close()

	players := make([]*wsClient, 4)
	for i := range players {
		players[i] = dialWS(t, srv.URL)
		defer players[i].conn.Close()
		players[i].waitFor("connected")
		players[i].send("joinRoom", map[string]any{"playerName": fmt.Sprintf("P%d", i), "roomId": "spec-room"})
		players[i].waitFor("roomState")
	}

	// 5th joins the full room -> spectator
	spec := dialWS(t, srv.URL)
	defer spec.conn.Close()
	spec.waitFor("connected")
	spec.send("joinRoom", map[string]any{"playerName": "Watcher", "roomId": "spec-room"})
	var sm struct {
		WatchSeat int `json:"watchSeat"`
	}
	if err := json.Unmarshal(spec.waitFor("spectatorMode"), &sm); err != nil {
		t.Fatalf("bad spectatorMode: %v", err)
	}
	if sm.WatchSeat != 0 {
		t.Errorf("default watch seat should be 0, got %d", sm.WatchSeat)
	}

	// Everyone readies up -> match starts, spectator receives a spectating gameState
	for _, p := range players {
		p.send("ready", nil)
	}

	type specState struct {
		Spectating bool  `json:"spectating"`
		WatchSeat  int   `json:"watchSeat"`
		Hands      []any `json:"hands"`
	}
	var gs specState
	if err := json.Unmarshal(spec.waitFor("gameState"), &gs); err != nil {
		t.Fatalf("bad gameState: %v", err)
	}
	if !gs.Spectating {
		t.Error("expected spectating=true in spectator gameState")
	}
	if h, ok := gs.Hands[gs.WatchSeat].([]any); !ok || len(h) != 27 {
		t.Fatalf("expected watched player's 27-card hand at seat %d, got %T", gs.WatchSeat, gs.Hands[gs.WatchSeat])
	}

	// Switch to watching seat 2
	spec.send("watchPlayer", 2)
	switched := false
	for i := 0; i < 5 && !switched; i++ {
		var gs2 specState
		if err := json.Unmarshal(spec.waitFor("gameState"), &gs2); err != nil {
			t.Fatalf("bad gameState after watchPlayer: %v", err)
		}
		if gs2.WatchSeat == 2 {
			switched = true
			if h, ok := gs2.Hands[2].([]any); !ok || len(h) != 27 {
				t.Fatalf("expected seat 2's hand visible after switch, got %T", gs2.Hands[2])
			}
		}
	}
	if !switched {
		t.Fatal("watchPlayer(2) did not take effect")
	}

	// Spectator must not be able to play: no crash expected, state unchanged
	spec.send("playHand", []any{})
}

// TestSameNameTakeover verifies that re-joining with the same name takes over
// the seat immediately (mobile sleep/zombie-connection recovery), and the new
// connection's actions work.
func TestSameNameTakeover(t *testing.T) {
	rm := NewRoomManager()
	srv := httptest.NewServer(http.HandlerFunc(WSHandler(rm)))
	defer srv.Close()

	// First connection joins as "Dup"
	a := dialWS(t, srv.URL)
	defer a.conn.Close()
	a.waitFor("connected")
	a.send("joinRoom", map[string]any{"playerName": "Dup", "roomId": "takeover-room"})
	a.waitFor("roomState")

	// Second connection with the same name takes over the seat (a is NOT closed,
	// simulating a zombie connection the server hasn't noticed yet)
	b := dialWS(t, srv.URL)
	defer b.conn.Close()
	var connectedB struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b.waitFor("connected"), &connectedB); err != nil {
		t.Fatal(err)
	}
	b.send("joinRoom", map[string]any{"playerName": "Dup", "roomId": "takeover-room"})

	var rs struct {
		Players []*struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"players"`
	}
	if err := json.Unmarshal(b.waitFor("roomState"), &rs); err != nil {
		t.Fatalf("bad roomState: %v", err)
	}
	if rs.Players[0] == nil || rs.Players[0].Name != "Dup" {
		t.Fatalf("seat 0 should still be Dup, got %+v", rs.Players[0])
	}
	if rs.Players[0].ID != connectedB.ID {
		t.Errorf("seat 0 should be owned by the NEW connection %s, got %s", connectedB.ID, rs.Players[0].ID)
	}

	// New connection's actions work: ready toggles state
	b.send("ready", nil)
	var rs2 struct {
		Players []*struct {
			IsReady bool `json:"isReady"`
		} `json:"players"`
	}
	if err := json.Unmarshal(b.waitFor("roomState"), &rs2); err != nil {
		t.Fatalf("bad roomState after ready: %v", err)
	}
	if rs2.Players[0] == nil || !rs2.Players[0].IsReady {
		t.Error("ready from the new connection should take effect")
	}
}
