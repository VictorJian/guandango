package server

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"guandango/internal/game"
)

type Player struct {
	ID             string // current socket/client id
	Name           string
	Client         *Client
	SeatIndex      int
	IsReady        bool
	IsDisconnected bool
}

// Spectator watches the game from one player's perspective (no actions).
type Spectator struct {
	ID        string
	Name      string
	Client    *Client
	WatchSeat int // 0-3, whose hand the spectator sees
}

// RoomManager owns all rooms and routes clients to them.
type RoomManager struct {
	mu    sync.Mutex
	rooms map[string]*Room
}

func NewRoomManager() *RoomManager {
	return &RoomManager{rooms: map[string]*Room{}}
}

func (rm *RoomManager) JoinRoom(c *Client, playerName, roomID string) {
	if roomID == "" {
		roomID = "default"
	}
	rm.mu.Lock()
	room, ok := rm.rooms[roomID]
	if !ok {
		room = NewRoom(roomID)
		rm.rooms[roomID] = room
	}
	rm.mu.Unlock()

	room.withLock(func() { room.addPlayer(c, playerName) })
}

func (rm *RoomManager) HandleDisconnect(c *Client) {
	rm.mu.Lock()
	rooms := make([]*Room, 0, len(rm.rooms))
	for _, r := range rm.rooms {
		rooms = append(rooms, r)
	}
	rm.mu.Unlock()

	for _, room := range rooms {
		room.withLock(func() { room.handleDisconnect(c) })
	}
}

type roomListEntry struct {
	ID          string        `json:"id"`
	PlayerCount int           `json:"playerCount"`
	MaxPlayers  int           `json:"maxPlayers"`
	InGame      bool          `json:"inGame"`
	GameMode    game.GameMode `json:"gameMode"`
	HostName    string        `json:"hostName"`
}

func (rm *RoomManager) HandleGetRoomList(c *Client) {
	rm.mu.Lock()
	rooms := make([]*Room, 0, len(rm.rooms))
	for _, r := range rm.rooms {
		rooms = append(rooms, r)
	}
	rm.mu.Unlock()

	list := make([]roomListEntry, 0, len(rooms))
	for _, room := range rooms {
		room.withLock(func() {
			count := 0
			hostName := "Unknown"
			for _, p := range room.players {
				if p != nil && !p.IsDisconnected {
					count++
				}
			}
			if room.players[0] != nil {
				hostName = room.players[0].Name
			}
			list = append(list, roomListEntry{
				ID:          room.ID,
				PlayerCount: count,
				MaxPlayers:  4,
				InGame:      room.match != nil && room.match.CurrentGame != nil,
				GameMode:    room.gameMode,
				HostName:    hostName,
			})
		})
	}
	c.Emit("roomList", list)
}

// Room holds up to 4 seats and at most one running Match.
// Room.mu serializes everything that touches the room, its match and game.
type Room struct {
	ID         string
	mu         sync.Mutex
	players    [4]*Player
	spectators []*Spectator
	match      *Match
	gameMode   game.GameMode
}

func NewRoom(id string) *Room {
	return &Room{ID: id, gameMode: game.ModeNormal}
}

func (r *Room) withLock(fn func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fn()
}

// Broadcast sends an event to every connected player and spectator in the room.
func (r *Room) Broadcast(event string, data any) {
	for _, p := range r.players {
		if p != nil && p.Client != nil {
			p.Client.Emit(event, data)
		}
	}
	for _, sp := range r.spectators {
		if sp.Client != nil {
			sp.Client.Emit(event, data)
		}
	}
}

func (r *Room) addPlayer(c *Client, name string) {
	// Reconnection: same name marked disconnected
	for _, p := range r.players {
		if p != nil && p.Name == name && p.IsDisconnected {
			p.IsDisconnected = false
			p.ID = c.ID
			p.Client = c

			r.bindSocketListeners(c)

			if r.match != nil && r.match.CurrentGame != nil {
				g := r.match.CurrentGame
				g.RebindPlayer(p)
				g.SendStateTo(p)
			}

			r.Broadcast("error", fmt.Sprintf("Player %s reconnected!", name))
			r.broadcastState()
			return
		}
	}

	// Normal join: find an empty seat
	seatIndex := -1
	for i, p := range r.players {
		if p == nil {
			seatIndex = i
			break
		}
	}
	if seatIndex == -1 {
		// 房間已滿：改以觀戰模式加入
		r.addSpectator(c, name)
		return
	}

	r.players[seatIndex] = &Player{
		ID:        c.ID,
		Name:      name,
		Client:    c,
		SeatIndex: seatIndex,
	}

	r.bindSocketListeners(c)
	r.broadcastState()
}

// addSpectator joins a client as a spectator (room already full).
func (r *Room) addSpectator(c *Client, name string) {
	sp := &Spectator{ID: c.ID, Name: name, Client: c, WatchSeat: 0}
	r.spectators = append(r.spectators, sp)
	r.bindSpectatorListeners(c, sp)

	c.Emit("spectatorMode", map[string]any{"watchSeat": sp.WatchSeat})
	r.Broadcast("error", fmt.Sprintf("%s 以觀戰模式加入", name))
	r.broadcastState()

	if r.match != nil && r.match.CurrentGame != nil {
		r.match.CurrentGame.SendStateToSpectator(sp)
	}
}

func (r *Room) bindSpectatorListeners(c *Client, sp *Spectator) {
	c.On("watchPlayer", func(data json.RawMessage) {
		var seat int
		if json.Unmarshal(data, &seat) != nil || seat < 0 || seat > 3 {
			return
		}
		r.withLock(func() {
			sp.WatchSeat = seat
			c.Emit("spectatorMode", map[string]any{"watchSeat": seat})
			if r.match != nil && r.match.CurrentGame != nil {
				r.match.CurrentGame.SendStateToSpectator(sp)
			}
		})
	})
	c.On("chatMessage", func(data json.RawMessage) {
		var msg string
		if json.Unmarshal(data, &msg) != nil {
			return
		}
		r.withLock(func() {
			r.Broadcast("chatMessage", map[string]any{
				"sender":    sp.Name + "（觀戰）",
				"text":      msg,
				"time":      time.Now().Format("15:04:05"),
				"seatIndex": -1,
			})
		})
	})
}

func (r *Room) bindSocketListeners(c *Client) {
	c.On("ready", func(json.RawMessage) {
		r.withLock(func() {
			if idx := r.getSeat(c); idx != -1 {
				r.setReady(idx, true)
			}
		})
	})
	c.On("start", func(json.RawMessage) {
		r.withLock(func() {
			if idx := r.getSeat(c); idx != -1 {
				r.forceStart(idx)
			}
		})
	})
	c.On("chatMessage", func(data json.RawMessage) {
		var msg string
		if json.Unmarshal(data, &msg) != nil {
			return
		}
		r.withLock(func() { r.handleChat(c, msg) })
	})
	c.On("switchSeat", func(data json.RawMessage) {
		var target int
		if json.Unmarshal(data, &target) != nil {
			return
		}
		r.withLock(func() { r.switchSeat(c, target) })
	})
	c.On("setGameMode", func(json.RawMessage) {
		// Skill mode is not available in this port — Normal mode only.
		c.Emit("error", "目前僅支援普通模式")
	})
	c.On("forceEndGame", func(json.RawMessage) {
		r.withLock(func() { r.handleForceEnd(c) })
	})
}

func (r *Room) handleForceEnd(c *Client) {
	if r.getSeat(c) != 0 {
		c.Emit("error", "只有房主可以強制結束遊戲")
		return
	}
	if r.match == nil {
		c.Emit("error", "目前沒有正在進行的對局")
		return
	}

	log.Printf("[Room %s] Host forced end match.", r.ID)

	r.match.ForceEndMatch()
	r.match = nil

	for _, p := range r.players {
		if p != nil {
			p.IsReady = false
		}
	}

	r.Broadcast("error", "房主強制結束了對局")
	r.Broadcast("gameTerminated", nil)
	r.broadcastState()
}

func (r *Room) handleChat(c *Client, msg string) {
	for _, p := range r.players {
		if p != nil && p.ID == c.ID {
			r.Broadcast("chatMessage", map[string]any{
				"sender":    p.Name,
				"text":      msg,
				"time":      time.Now().Format("15:04:05"),
				"seatIndex": p.SeatIndex,
			})
			return
		}
	}
}

func (r *Room) switchSeat(c *Client, targetSeat int) {
	if r.match != nil && r.match.MatchWinner == nil {
		return // Cannot switch during match
	}
	if targetSeat < 0 || targetSeat > 3 {
		return
	}

	currentIdx := -1
	for i, p := range r.players {
		if p != nil && p.ID == c.ID {
			currentIdx = i
			break
		}
	}
	if currentIdx == -1 {
		return
	}

	if r.players[targetSeat] == nil {
		p := r.players[currentIdx]
		p.SeatIndex = targetSeat
		r.players[targetSeat] = p
		r.players[currentIdx] = nil
		r.broadcastState()
	}
}

func (r *Room) getSeat(c *Client) int {
	for _, p := range r.players {
		if p != nil && p.ID == c.ID {
			return p.SeatIndex
		}
	}
	return -1
}

func (r *Room) handleDisconnect(c *Client) {
	// Spectator disconnect: just remove from the list
	for i, sp := range r.spectators {
		if sp.ID == c.ID {
			r.spectators = append(r.spectators[:i], r.spectators[i+1:]...)
			r.Broadcast("error", fmt.Sprintf("%s 離開觀戰", sp.Name))
			r.broadcastState()
			return
		}
	}

	for i, p := range r.players {
		if p == nil || p.ID != c.ID {
			continue
		}

		p.IsDisconnected = true
		p.IsReady = false

		if r.match != nil && r.match.CurrentGame != nil {
			// Match running: keep the seat and allow reconnect
			r.Broadcast("error", fmt.Sprintf("Player %s disconnected (Waiting for reconnect...)", p.Name))
		} else {
			r.players[i] = nil
			r.Broadcast("error", fmt.Sprintf("Player %s left the room", p.Name))
		}
		r.broadcastState()
		return
	}
}

func (r *Room) setReady(seatIndex int, ready bool) {
	if r.players[seatIndex] != nil {
		r.players[seatIndex].IsReady = ready
		r.broadcastState()
		r.tryAutoStart()
	}
}

func (r *Room) tryAutoStart() {
	readyCount := 0
	for _, p := range r.players {
		if p != nil && p.IsReady {
			readyCount++
		}
	}
	if readyCount == 4 && r.match == nil {
		r.startGame()
	}
}

func (r *Room) forceStart(seatIndex int) {
	if seatIndex != 0 {
		return // Only host can force start
	}
	if r.match != nil && r.match.MatchWinner == nil {
		return // Match is still ongoing
	}
	if !r.startGame() {
		if host := r.players[0]; host != nil && host.Client != nil {
			host.Client.Emit("error", "需要4位玩家才能開始遊戲")
		}
	}
}

func (r *Room) startGame() bool {
	// All 4 seats must be filled by connected players (no bots)
	for _, p := range r.players {
		if p == nil || p.IsDisconnected {
			return false
		}
	}
	r.broadcastState()

	gamePlayers := make([]*Player, 4)
	copy(gamePlayers, r.players[:])

	r.match = NewMatch(r, gamePlayers)
	r.match.StartMatch()

	r.Broadcast("matchStarted", nil)
	return true
}

func (r *Room) broadcastState() {
	playerList := make([]any, 4)
	for i, p := range r.players {
		if p == nil {
			playerList[i] = nil
			continue
		}
		playerList[i] = map[string]any{
			"id":        p.ID,
			"name":      p.Name,
			"seatIndex": p.SeatIndex,
			"isReady":   p.IsReady,
			"isBot":     false,
		}
	}
	spectatorNames := make([]string, 0, len(r.spectators))
	for _, sp := range r.spectators {
		spectatorNames = append(spectatorNames, sp.Name)
	}
	r.Broadcast("roomState", map[string]any{
		"roomId":     r.ID,
		"players":    playerList,
		"gameMode":   r.gameMode,
		"spectators": spectatorNames,
	})
}
