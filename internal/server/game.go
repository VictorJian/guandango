package server

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"guandango/internal/game"
)

// Pacing knobs — package vars so tests can shrink them.
var nextGameDelay = 3 * time.Second

type GamePhase string

const (
	PhaseWaiting        GamePhase = "Waiting"
	PhaseDealing        GamePhase = "Dealing"
	PhaseTribute        GamePhase = "Tribute"
	PhaseReturnTribute  GamePhase = "ReturnTribute"
	PhaseTributeConfirm GamePhase = "TributeConfirm" // 貢牌完成，等待上局第四名確認開局
	PhasePlaying        GamePhase = "Playing"
	PhaseScore          GamePhase = "Score"
)

type TributeEntry struct {
	From int        `json:"from"`
	To   int        `json:"to"`
	Card *game.Card `json:"card,omitempty"`
}

type TributeState struct {
	PendingTributes   []*TributeEntry `json:"pendingTributes"`
	PendingReturns    []*TributeEntry `json:"pendingReturns"`
	CompletedTributes []*TributeEntry `json:"completedTributes,omitempty"` // 已完成的進貢（供畫面顯示）
	NextStartPlayer   *int            `json:"nextStartPlayer,omitempty"`
}

type RoundAction struct {
	Type  string      `json:"type"` // "play" | "pass"
	Cards []game.Card `json:"cards,omitempty"`
	Hand  *game.Hand  `json:"hand,omitempty"`
}

type LastHand struct {
	PlayerIndex int        `json:"playerIndex"`
	Hand        *game.Hand `json:"hand"`
}

// Game is a single round of GuanDan (one deal, play until 3 players finish).
// All methods must be called with the owning Room's mutex held; timer
// callbacks re-acquire it before touching game state.
type Game struct {
	room    *Room
	players []*Player

	level        int
	currentPhase GamePhase

	onGameEnd func(winners []int)

	isActive bool
	timers   []*time.Timer

	hands       [4][]game.Card
	currentTurn int

	lastHand     *LastHand
	roundActions map[int]RoundAction

	winners      []int
	tributeState TributeState
	confirmSeat  int // 上一局第四名，負責在貢牌完成後按下確認

	lowCardAlerted [4]bool // 每位玩家「剩10張以下」的全體通知，每局只發一次

	teamLevels  map[int]int // Team 0 (seats 0,2), Team 1 (seats 1,3)
	activeTeam  int
	prevWinners []int

	history          []game.HistoryEntry
	historyIDCounter int
	currentRound     int
}

func NewGame(room *Room, players []*Player) *Game {
	g := &Game{
		room:         room,
		players:      players,
		level:        2,
		currentPhase: PhaseWaiting,
		isActive:     true,
		roundActions: map[int]RoundAction{},
		winners:      []int{},
		teamLevels:   map[int]int{0: 2, 1: 2},
		history:      []game.HistoryEntry{},
		confirmSeat:  -1,
	}
	for _, p := range players {
		if p.Client != nil {
			g.bindPlayerListeners(p)
		}
	}
	return g
}

func (g *Game) RebindPlayer(p *Player) {
	if p.Client != nil {
		g.bindPlayerListeners(p)
	}
}

func (g *Game) bindPlayerListeners(p *Player) {
	c := p.Client
	seat := p.SeatIndex

	c.On("playHand", func(data json.RawMessage) {
		// Support both formats: bare Card[] or {cards, handType}
		var cards []game.Card
		var provided *game.Hand
		var obj struct {
			Cards    []game.Card `json:"cards"`
			HandType *game.Hand  `json:"handType"`
		}
		if err := json.Unmarshal(data, &obj); err == nil && obj.Cards != nil {
			cards = obj.Cards
			provided = obj.HandType
		} else if err := json.Unmarshal(data, &cards); err != nil {
			return
		}
		g.room.withLock(func() { g.handlePlayHand(seat, cards, provided) })
	})
	c.On("pass", func(json.RawMessage) {
		g.room.withLock(func() { g.handlePass(seat) })
	})
	c.On("tribute", func(data json.RawMessage) {
		var cards []game.Card
		if json.Unmarshal(data, &cards) != nil {
			return
		}
		g.room.withLock(func() { g.handleTribute(seat, cards) })
	})
	c.On("returnTribute", func(data json.RawMessage) {
		var cards []game.Card
		if json.Unmarshal(data, &cards) != nil {
			return
		}
		g.room.withLock(func() { g.handleReturnTribute(seat, cards) })
	})
	c.On("confirmStart", func(json.RawMessage) {
		g.room.withLock(func() { g.handleConfirmStart(seat) })
	})
}

func (g *Game) registerTimer(d time.Duration, fn func()) {
	t := time.AfterFunc(d, func() {
		g.room.withLock(func() {
			if !g.isActive {
				return
			}
			fn()
		})
	})
	g.timers = append(g.timers, t)
}

func (g *Game) Destroy() {
	log.Printf("[Game] Destroying game instance for room %s", g.room.ID)
	g.isActive = false
	for _, t := range g.timers {
		t.Stop()
	}
	g.timers = nil

	for _, p := range g.players {
		if p.Client != nil {
			p.Client.Off("playHand", "pass", "tribute", "returnTribute", "confirmStart")
		}
	}
}

func (g *Game) addHistoryEntry(typ game.HistoryEventType, message string, playerIndex *int, details any) {
	entry := game.HistoryEntry{
		ID:          fmt.Sprintf("history-%d", g.historyIDCounter),
		Timestamp:   time.Now().UnixMilli(),
		Type:        typ,
		PlayerIndex: playerIndex,
		Message:     message,
		Details:     details,
	}
	g.historyIDCounter++
	if playerIndex != nil {
		entry.PlayerName = g.players[*playerIndex].Name
	}
	g.history = append(g.history, entry)
	g.room.Broadcast("historyUpdate", entry)
}

func cardDescription(cards []game.Card) string {
	if len(cards) == 0 {
		return ""
	}
	if len(cards) == 1 {
		c := cards[0]
		suitNames := []string{"♠", "♥", "♣", "♦", "Joker"}
		var rankName string
		switch c.Rank {
		case 15:
			rankName = "黑鬼"
		case 16:
			rankName = "紅鬼"
		case 11:
			rankName = "J"
		case 12:
			rankName = "Q"
		case 13:
			rankName = "K"
		case 14:
			rankName = "A"
		default:
			rankName = fmt.Sprintf("%d", c.Rank)
		}
		if c.Rank >= 15 {
			return rankName
		}
		return suitNames[c.Suit] + rankName
	}
	return fmt.Sprintf("%d張牌", len(cards))
}

func (g *Game) Start() {
	g.currentPhase = PhaseDealing

	if len(g.prevWinners) == 0 && len(g.winners) == 0 {
		// Fresh game
		g.activeTeam = 0
		g.teamLevels = map[int]int{0: 2, 1: 2}
		g.currentRound = 1
		g.history = []game.HistoryEntry{}
		g.historyIDCounter = 0
	} else {
		g.currentRound++
	}

	g.level = g.teamLevels[g.activeTeam]

	teamName := "隊伍0（座位0、2）"
	if g.activeTeam == 1 {
		teamName = "隊伍1（座位1、3）"
	}
	g.addHistoryEntry(game.HistGameStart,
		fmt.Sprintf("第%d局開始 - 目前等級: %d - 莊家: %s", g.currentRound, g.level, teamName),
		nil,
		map[string]any{"level": g.level, "activeTeam": g.activeTeam, "round": g.currentRound})

	deck := game.ShuffleDeck(game.CreateDeck())

	for i := range g.hands {
		g.hands[i] = nil
	}
	for i := 0; i < 108; i++ {
		g.hands[i%4] = append(g.hands[i%4], deck[i])
	}
	for i := range g.hands {
		g.hands[i] = game.SortCards(game.UpdateCardProperties(g.hands[i], g.level), g.level)
	}

	if len(g.prevWinners) > 0 {
		g.initTributePhase()
	} else {
		g.currentTurn = 0
		g.currentPhase = PhasePlaying
		g.lastHand = nil
	}

	g.broadcastGameState()
}

func sameTeam(a, b int) bool { return a%2 == b%2 }

func (g *Game) initTributePhase() {
	if len(g.prevWinners) < 4 {
		g.currentPhase = PhasePlaying
		g.currentTurn = g.activeTeam
		return
	}

	p1, p2 := g.prevWinners[0], g.prevWinners[1]
	p3, p4 := g.prevWinners[2], g.prevWinners[3]

	g.tributeState = TributeState{PendingTributes: []*TributeEntry{}, PendingReturns: []*TributeEntry{}}
	g.confirmSeat = p4 // 上一局第四名負責確認開局

	var losingTeam []int
	isDouble := false

	if sameTeam(p1, p2) {
		isDouble = true
		losingTeam = []int{p3, p4}
	} else {
		losingTeam = []int{p4}
		if sameTeam(p1, p4) {
			// Tie (1,4 same team) -> No tribute
			g.currentPhase = PhasePlaying
			g.currentTurn = p1
			return
		}
	}

	// Anti-tribute: 2 big jokers in losing team's hands
	bigJokerCount := 0
	for _, seat := range losingTeam {
		for _, c := range g.hands[seat] {
			if c.Rank == game.BigJoker {
				bigJokerCount++
			}
		}
	}
	if bigJokerCount == 2 {
		g.currentPhase = PhasePlaying
		g.currentTurn = p1
		g.room.Broadcast("error", "抗貢成功！雙紅鬼在手，免除進貢！")
		return
	}

	if isDouble {
		// Double: 4->1, 3->2
		g.tributeState.PendingTributes = append(g.tributeState.PendingTributes,
			&TributeEntry{From: p4, To: p1},
			&TributeEntry{From: p3, To: p2})
	} else {
		// Single: 4->1
		g.tributeState.PendingTributes = append(g.tributeState.PendingTributes,
			&TributeEntry{From: p4, To: p1})
	}

	if len(g.tributeState.PendingTributes) > 0 {
		g.currentPhase = PhaseTribute
	} else {
		g.currentPhase = PhasePlaying
		g.currentTurn = p1
	}
}

func (g *Game) removeCardByID(seat int, id string) {
	out := g.hands[seat][:0]
	for _, c := range g.hands[seat] {
		if c.ID != id {
			out = append(out, c)
		}
	}
	g.hands[seat] = out
}

func (g *Game) giveCard(seat int, card game.Card) {
	g.hands[seat] = game.SortCards(append(g.hands[seat], card), g.level)
}

func (g *Game) handleTribute(seatIndex int, cards []game.Card) {
	if g.currentPhase != PhaseTribute || len(cards) != 1 {
		return
	}

	var tribute *TributeEntry
	for _, t := range g.tributeState.PendingTributes {
		if t.From == seatIndex && t.Card == nil {
			tribute = t
			break
		}
	}
	if tribute == nil {
		return
	}

	largest := game.GetLargestCard(g.hands[seatIndex], g.level)
	valPlay := game.GetLogicValue(cards[0].Rank, g.level)
	valMax := game.GetLogicValue(largest.Rank, g.level)
	if valPlay < valMax {
		g.emitError(seatIndex, "Must pay the largest card")
		return
	}

	cc := cards[0]
	tribute.Card = &cc
	g.removeCardByID(seatIndex, cards[0].ID)
	g.giveCard(tribute.To, cards[0])

	seat := seatIndex
	g.addHistoryEntry(game.HistTribute,
		fmt.Sprintf("%s 向 %s 進貢: %s", g.players[seatIndex].Name, g.players[tribute.To].Name, cardDescription(cards[:1])),
		&seat,
		map[string]any{"card": cards[0], "to": tribute.To})

	allDone := true
	for _, t := range g.tributeState.PendingTributes {
		if t.Card == nil {
			allDone = false
		}
	}
	if allDone {
		// The player who paid the largest tribute card leads the next game.
		// Tie goes to the earlier entry in the list (last place first).
		maxVal, maxPayer := -1, -1
		for _, t := range g.tributeState.PendingTributes {
			if t.Card != nil {
				val := game.GetLogicValue(t.Card.Rank, g.level)
				if val > maxVal {
					maxVal = val
					maxPayer = t.From
				}
			}
		}
		g.tributeState.NextStartPlayer = &maxPayer

		g.currentPhase = PhaseReturnTribute
		g.tributeState.PendingReturns = nil
		for _, t := range g.tributeState.PendingTributes {
			g.tributeState.PendingReturns = append(g.tributeState.PendingReturns,
				&TributeEntry{From: t.To, To: t.From})
		}
		// 保留已完成的進貢供畫面顯示
		g.tributeState.CompletedTributes = g.tributeState.PendingTributes
		g.tributeState.PendingTributes = []*TributeEntry{}
	}
	g.broadcastGameState()
}

func (g *Game) handleReturnTribute(seatIndex int, cards []game.Card) {
	if g.currentPhase != PhaseReturnTribute || len(cards) != 1 {
		return
	}

	var ret *TributeEntry
	for _, r := range g.tributeState.PendingReturns {
		if r.From == seatIndex && r.Card == nil {
			ret = r
			break
		}
	}
	if ret == nil {
		return
	}

	// 還貢的牌必須是自己手上的牌
	owned := false
	for _, c := range g.hands[seatIndex] {
		if c.ID == cards[0].ID {
			owned = true
			break
		}
	}
	if !owned {
		g.emitError(seatIndex, "你沒有這張牌")
		return
	}

	// 還貢限制：不可大於10、不可是當前等級的牌（除非手上完全沒有合規的牌）
	isEligible := func(c game.Card) bool {
		return c.Rank <= 10 && int(c.Rank) != g.level
	}
	if !isEligible(cards[0]) {
		hasEligible := false
		for _, c := range g.hands[seatIndex] {
			if isEligible(c) {
				hasEligible = true
				break
			}
		}
		if hasEligible {
			if int(cards[0].Rank) == g.level {
				g.emitError(seatIndex, "還貢的牌不能是當前等級的牌")
			} else {
				g.emitError(seatIndex, "還貢的牌不能大於10")
			}
			return
		}
	}

	cc := cards[0]
	ret.Card = &cc
	g.removeCardByID(seatIndex, cards[0].ID)
	g.giveCard(ret.To, cards[0])

	seat := seatIndex
	g.addHistoryEntry(game.HistReturnTribute,
		fmt.Sprintf("%s 向 %s 還貢: %s", g.players[seatIndex].Name, g.players[ret.To].Name, cardDescription(cards[:1])),
		&seat,
		map[string]any{"card": cards[0], "to": ret.To})

	g.checkReturnDone()
	g.broadcastGameState()
}

func (g *Game) checkReturnDone() {
	for _, r := range g.tributeState.PendingReturns {
		if r.Card == nil {
			return
		}
	}
	// 貢牌交換完成：等待上一局第四名確認後才開始出牌
	g.currentPhase = PhaseTributeConfirm
	g.broadcastGameState()
}

// handleConfirmStart is triggered by the previous game's 4th-place player to
// actually start the new game after the tribute exchange.
func (g *Game) handleConfirmStart(seatIndex int) {
	if g.currentPhase != PhaseTributeConfirm {
		return
	}
	if seatIndex != g.confirmSeat {
		g.emitError(seatIndex, "只有上一局第四名可以確認開始")
		return
	}

	g.currentPhase = PhasePlaying
	// 確認開局的第四名同時是該局的先手
	g.currentTurn = g.confirmSeat
	g.tributeState = TributeState{PendingTributes: []*TributeEntry{}, PendingReturns: []*TributeEntry{}}

	seat := seatIndex
	g.addHistoryEntry(game.HistPhaseChange,
		fmt.Sprintf("%s 確認開始，新一局開打，由其先出牌！", g.players[seatIndex].Name),
		&seat, nil)

	g.broadcastGameState()
}

func (g *Game) handlePlayHand(seatIndex int, cards []game.Card, providedHandType *game.Hand) {
	if g.currentPhase != PhasePlaying || g.currentTurn != seatIndex {
		return
	}

	var hand *game.Hand
	if providedHandType != nil {
		// Validate the cards form some valid hand, then use the provided interpretation
		if game.GetHandType(cards, g.level) == nil {
			g.emitError(seatIndex, "Invalid hand")
			return
		}
		hand = providedHandType
	} else {
		hand = game.GetHandType(cards, g.level)
		if hand == nil {
			g.emitError(seatIndex, "Invalid hand")
			return
		}
	}

	log.Printf("Player %d plays. Level: %d. Hand: %s (Val: %d)", seatIndex, g.level, hand.Type, hand.Value)

	if g.lastHand != nil && g.lastHand.PlayerIndex != seatIndex {
		if game.CompareHands(hand, g.lastHand.Hand) <= 0 {
			g.emitError(seatIndex, "Hand not big enough")
			return
		}
	}

	// Check the player actually owns these cards (by ID)
	owned := map[string]bool{}
	for _, c := range g.hands[seatIndex] {
		owned[c.ID] = true
	}
	for _, c := range cards {
		if !owned[c.ID] {
			g.emitError(seatIndex, "You do not have these cards")
			return
		}
	}

	// Remove played cards
	played := map[string]bool{}
	for _, c := range cards {
		played[c.ID] = true
	}
	newHand := g.hands[seatIndex][:0]
	for _, c := range g.hands[seatIndex] {
		if !played[c.ID] {
			newHand = append(newHand, c)
		}
	}
	g.hands[seatIndex] = newHand

	g.lastHand = &LastHand{PlayerIndex: seatIndex, Hand: hand}

	// New round display starts when someone plays
	g.roundActions = map[int]RoundAction{
		seatIndex: {Type: "play", Cards: cards, Hand: hand},
	}

	// HandType 的值本身就是中文名稱，直接用於歷史紀錄
	handTypeName := string(hand.Type)
	seat := seatIndex
	g.addHistoryEntry(game.HistPlay,
		fmt.Sprintf("%s 出牌: %s (%s)", g.players[seatIndex].Name, handTypeName, cardDescription(cards)),
		&seat,
		map[string]any{"cards": cards, "handType": hand.Type, "cardsCount": len(cards)})

	// 手牌降到10張（含）以下時，向全體發送一次性通知（畫面中央倒數顯示）
	if remaining := len(g.hands[seatIndex]); remaining > 0 && remaining <= 10 && !g.lowCardAlerted[seatIndex] {
		g.lowCardAlerted[seatIndex] = true
		g.room.Broadcast("announce", fmt.Sprintf("⚠️ %s 只剩 %d 張牌！", g.players[seatIndex].Name, remaining))
	}

	if len(g.hands[seatIndex]) == 0 {
		g.winners = append(g.winners, seatIndex)

		positions := []string{"第一名", "第二名", "第三名", "第四名"}
		g.addHistoryEntry(game.HistPlayerFinish,
			fmt.Sprintf("%s 出完所有牌，獲得%s！", g.players[seatIndex].Name, positions[len(g.winners)-1]),
			&seat,
			map[string]any{"position": len(g.winners)})

		// 名次要實際打出來：即使前兩名同隊（雙下）也繼續打，
		// 直到第三名出完牌，剩下的一位即為第四名
		if len(g.winners) == 3 {
			for i := 0; i < 4; i++ {
				if !contains(g.winners, i) {
					g.winners = append(g.winners, i)
					break
				}
			}
			g.endGame()
			return
		}
	}

	g.advanceTurn()
	g.broadcastGameState()
}

func contains(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// endRoundAndFindNext ends the trick; winner (or partner via 接風) leads next.
func (g *Game) endRoundAndFindNext(winner int) {
	log.Printf("[endRound] Round ended. Winner: %d", winner)

	// 接風: if winner already finished, partner leads
	if len(g.hands[winner]) == 0 {
		winner = (winner + 2) % 4
	}

	g.lastHand = nil
	g.roundActions = map[int]RoundAction{}

	found := false
	for i := 0; i < 4; i++ {
		seat := (winner + i) % 4
		if len(g.hands[seat]) > 0 {
			g.currentTurn = seat
			found = true
			break
		}
	}

	if !found {
		g.endGame()
		return
	}

	g.broadcastGameState()
}

func (g *Game) handlePass(seatIndex int) {
	if g.currentPhase != PhasePlaying || g.currentTurn != seatIndex {
		return
	}
	if g.lastHand == nil || g.lastHand.PlayerIndex == seatIndex {
		g.emitError(seatIndex, "Cannot pass on free turn")
		return
	}

	g.roundActions[seatIndex] = RoundAction{Type: "pass"}

	seat := seatIndex
	g.addHistoryEntry(game.HistPass,
		fmt.Sprintf("%s 選擇過牌", g.players[seatIndex].Name),
		&seat, nil)

	g.advanceTurn()
	g.broadcastGameState()
}

func (g *Game) advanceTurn() {
	next := (g.currentTurn + 1) % 4

	for i := 0; i < 4; i++ {
		// Cycled back to the trick winner -> round over
		if g.lastHand != nil && next == g.lastHand.PlayerIndex {
			g.endRoundAndFindNext(next)
			return
		}

		if len(g.hands[next]) == 0 {
			next = (next + 1) % 4
			continue
		}

		g.currentTurn = next
		return
	}

	g.endGame()
}

func (g *Game) endGame() {
	log.Printf("[endGame] Game ended. Winners: %v", g.winners)
	g.currentPhase = PhaseScore

	names := make([]string, len(g.winners))
	for i, w := range g.winners {
		names[i] = g.players[w].Name
	}

	var resultType string
	switch {
	case g.winners[0]%2 == 0 && g.winners[1]%2 == 0:
		resultType = "隊伍0 雙扣！"
	case g.winners[0]%2 == 1 && g.winners[1]%2 == 1:
		resultType = "隊伍1 雙扣！"
	case g.winners[0]%2 == g.winners[2]%2:
		resultType = fmt.Sprintf("隊伍%d 單扣", g.winners[0]%2)
	default:
		resultType = fmt.Sprintf("隊伍%d 保級", g.winners[0]%2)
	}

	g.addHistoryEntry(game.HistGameEnd,
		fmt.Sprintf("遊戲結束！%s - 排名: %s", resultType, joinStrings(names, ", ")),
		nil,
		map[string]any{"winners": g.winners, "resultType": resultType})

	// Broadcast final state first so clients see the last hand
	g.broadcastGameState()

	g.room.Broadcast("gameOver", map[string]any{"winners": g.winners})

	if g.onGameEnd != nil {
		g.onGameEnd(g.winners)
	}
}

func joinStrings(s []string, sep string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += sep
		}
		out += v
	}
	return out
}

func (g *Game) emitError(seatIndex int, msg string) {
	p := g.players[seatIndex]
	if p.Client != nil {
		p.Client.Emit("error", msg)
	}
}

type gameStatePayload struct {
	Phase        GamePhase           `json:"phase"`
	Level        int                 `json:"level"`
	CurrentTurn  int                 `json:"currentTurn"`
	Hands        []any               `json:"hands"`
	LastHand     *LastHand           `json:"lastHand"`
	RoundActions map[int]RoundAction `json:"roundActions"`
	Winners      []int               `json:"winners"`
	TributeState *TributeState       `json:"tributeState,omitempty"`
	TeamLevels   map[int]int         `json:"teamLevels"`
	ActiveTeam   int                 `json:"activeTeam"`
	GameMode     game.GameMode       `json:"gameMode"`
	History      []game.HistoryEntry `json:"history"`
	CurrentRound int                 `json:"currentRound"`
	// 觀戰模式：以 watchSeat 玩家的視角觀看，不可操作
	Spectating bool `json:"spectating,omitempty"`
	WatchSeat  *int `json:"watchSeat,omitempty"`
	// 貢牌完成後，負責確認開局的座位（上一局第四名）
	ConfirmSeat *int `json:"confirmSeat,omitempty"`
}

func (g *Game) stateFor(idx int) gameStatePayload {
	hands := make([]any, 4)
	for i := range g.hands {
		if i == idx {
			cards := g.hands[i]
			if cards == nil {
				cards = []game.Card{}
			}
			hands[i] = cards
		} else {
			hands[i] = len(g.hands[i])
		}
	}

	var tribute *TributeState
	var confirmSeat *int
	if g.currentPhase == PhaseTribute || g.currentPhase == PhaseReturnTribute || g.currentPhase == PhaseTributeConfirm {
		tribute = &g.tributeState
		if g.confirmSeat >= 0 {
			cs := g.confirmSeat
			confirmSeat = &cs
		}
	}

	winners := g.winners
	if winners == nil {
		winners = []int{}
	}
	history := g.history
	if history == nil {
		history = []game.HistoryEntry{}
	}

	return gameStatePayload{
		Phase:        g.currentPhase,
		Level:        g.level,
		CurrentTurn:  g.currentTurn,
		Hands:        hands,
		LastHand:     g.lastHand,
		RoundActions: g.roundActions,
		Winners:      winners,
		TributeState: tribute,
		ConfirmSeat:  confirmSeat,
		TeamLevels:   g.teamLevels,
		ActiveTeam:   g.activeTeam,
		GameMode:     game.ModeNormal,
		History:      history,
		CurrentRound: g.currentRound,
	}
}

// SendStateTo sends the current full game state to one (reconnected) player.
func (g *Game) SendStateTo(p *Player) {
	if p.Client == nil {
		return
	}
	p.Client.Emit("gameState", g.stateFor(p.SeatIndex))
}

// SendStateToSpectator sends the game state from the watched player's perspective.
func (g *Game) SendStateToSpectator(sp *Spectator) {
	if sp.Client == nil {
		return
	}
	st := g.stateFor(sp.WatchSeat)
	st.Spectating = true
	seat := sp.WatchSeat
	st.WatchSeat = &seat
	sp.Client.Emit("gameState", st)
}

func (g *Game) broadcastGameState() {
	for idx, p := range g.players {
		if p.Client == nil {
			continue
		}
		p.Client.Emit("gameState", g.stateFor(idx))
	}
	for _, sp := range g.room.spectators {
		g.SendStateToSpectator(sp)
	}
}
