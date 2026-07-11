package server

import (
	"log"
	"time"
)

// Match represents a full game series (從2打到A).
// Contains multiple Games until one team reaches A and wins twice consecutively.
// All methods must be called with the owning Room's mutex held.
type Match struct {
	room    *Room
	players []*Player

	CurrentGame *Game
	startLevel  int // 起始階層（開發環境使用，預設 2）
	teamLevels  map[int]int
	activeTeam  int

	consecutiveWins map[int]int
	MatchWinner     *int

	lastWinners []int

	nextGameTimer *time.Timer
	active        bool
}

func NewMatch(room *Room, players []*Player, startLevel int) *Match {
	if startLevel < 2 || startLevel > 14 {
		startLevel = 2
	}
	return &Match{
		room:            room,
		players:         players,
		startLevel:      startLevel,
		teamLevels:      map[int]int{0: startLevel, 1: startLevel},
		consecutiveWins: map[int]int{0: 0, 1: 0},
		active:          true,
	}
}

func (m *Match) StartMatch() {
	log.Printf("[Match %s] Starting new match (start level %d)", m.room.ID, m.startLevel)
	m.teamLevels = map[int]int{0: m.startLevel, 1: m.startLevel}
	m.activeTeam = 0
	m.consecutiveWins = map[int]int{0: 0, 1: 0}
	m.MatchWinner = nil
	m.startNextGame()
}

func (m *Match) startNextGame() {
	if m.MatchWinner != nil {
		log.Printf("[Match %s] Match already won by Team %d", m.room.ID, *m.MatchWinner)
		return
	}

	log.Printf("[Match %s] Starting new game. Team levels: %v, Active team: %d", m.room.ID, m.teamLevels, m.activeTeam)

	if m.CurrentGame != nil {
		m.CurrentGame.Destroy()
	}

	m.CurrentGame = NewGame(m.room, m.players)
	m.CurrentGame.teamLevels = map[int]int{0: m.teamLevels[0], 1: m.teamLevels[1]}
	m.CurrentGame.activeTeam = m.activeTeam
	m.CurrentGame.prevWinners = m.lastWinners

	m.CurrentGame.onGameEnd = func(winners []int) { m.handleGameEnd(winners) }

	m.CurrentGame.Start()
}

func (m *Match) handleGameEnd(winners []int) {
	if len(winners) != 4 {
		log.Printf("[Match %s] Invalid winners array: %v", m.room.ID, winners)
		return
	}

	log.Printf("[Match %s] Game ended. Winners order: %v", m.room.ID, winners)

	winningTeam, levelIncrease := calculateLevelUp(winners)

	oldLevel := m.teamLevels[winningTeam]
	m.teamLevels[winningTeam] += levelIncrease
	if m.teamLevels[winningTeam] > 14 {
		m.teamLevels[winningTeam] = 14
	}
	log.Printf("[Match %s] Team %d level: %d -> %d (+%d)", m.room.ID, winningTeam, oldLevel, m.teamLevels[winningTeam], levelIncrease)

	if winningTeam != m.activeTeam {
		m.activeTeam = winningTeam
		log.Printf("[Match %s] Banker changed to Team %d", m.room.ID, m.activeTeam)
	}

	if m.teamLevels[winningTeam] == 14 {
		m.consecutiveWins[winningTeam]++
		m.consecutiveWins[1-winningTeam] = 0

		log.Printf("[Match %s] Team %d at level A. Consecutive wins: %d", m.room.ID, winningTeam, m.consecutiveWins[winningTeam])

		if m.consecutiveWins[winningTeam] >= 2 {
			team := winningTeam
			m.MatchWinner = &team
			log.Printf("[Match %s] MATCH WON by Team %d!", m.room.ID, winningTeam)
			m.broadcastMatchEnd(winningTeam)
			return
		}
	} else {
		m.consecutiveWins[0] = 0
		m.consecutiveWins[1] = 0
	}

	m.lastWinners = winners

	// Auto-start the next game after a short delay
	m.nextGameTimer = time.AfterFunc(nextGameDelay, func() {
		m.room.withLock(func() {
			if !m.active {
				return
			}
			m.startNextGame()
		})
	})
}

func calculateLevelUp(winners []int) (winningTeam, levelIncrease int) {
	p1, p2, p3 := winners[0], winners[1], winners[2]
	winningTeam = p1 % 2

	switch {
	case sameTeam(p1, p2):
		levelIncrease = 3 // Double win
	case sameTeam(p1, p3):
		levelIncrease = 2
	default:
		levelIncrease = 1
	}
	return
}

func (m *Match) broadcastMatchEnd(winningTeam int) {
	type winnerInfo struct {
		Name      string `json:"name"`
		SeatIndex int    `json:"seatIndex"`
	}
	var teamPlayers []winnerInfo
	for _, p := range m.players {
		if p.SeatIndex%2 == winningTeam {
			teamPlayers = append(teamPlayers, winnerInfo{Name: p.Name, SeatIndex: p.SeatIndex})
		}
	}
	m.room.Broadcast("matchOver", map[string]any{
		"winningTeam": winningTeam,
		"winners":     teamPlayers,
		"finalLevels": m.teamLevels,
	})
}

// ForceEndMatch stops the match, its pending timers, and the current game.
func (m *Match) ForceEndMatch() {
	log.Printf("[Match %s] Force ending match", m.room.ID)
	m.active = false
	if m.nextGameTimer != nil {
		m.nextGameTimer.Stop()
		m.nextGameTimer = nil
	}
	if m.CurrentGame != nil {
		m.CurrentGame.Destroy()
		m.CurrentGame = nil
	}
	m.MatchWinner = nil
	m.consecutiveWins = map[int]int{0: 0, 1: 0}
}
