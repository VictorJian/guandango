package game

// Suit mirrors the TS enum: Spades=0, Hearts=1, Clubs=2, Diamonds=3, Joker=4.
type Suit int

const (
	Spades Suit = iota
	Hearts
	Clubs
	Diamonds
	JokerSuit
)

// Rank mirrors the TS enum: 2..14 (A), 15 small joker, 16 big joker.
type Rank int

const (
	RankTwo    Rank = 2
	RankAce    Rank = 14
	SmallJoker Rank = 15
	BigJoker   Rank = 16
)

type Card struct {
	Suit        Suit   `json:"suit"`
	Rank        Rank   `json:"rank"`
	ID          string `json:"id"`
	IsLevelCard bool   `json:"isLevelCard,omitempty"`
	IsWild      bool   `json:"isWild,omitempty"`
}

type HandType string

// 牌型值同時是前後端協定值與顯示名稱（台灣用語），
// 前端 web/src/shared/types.ts 的 HandType enum 必須保持一致。
const (
	Single        HandType = "單張"
	Pair          HandType = "一對"
	Trips         HandType = "三張"
	TripsWithPair HandType = "葫蘆"
	Straight      HandType = "順子"
	Tube          HandType = "鐵板"
	Plate         HandType = "連對"
	Bomb          HandType = "炸彈"
	StraightFlush HandType = "同花順"
	FourKings     HandType = "天王炸"
)

type Hand struct {
	Type      HandType `json:"type"`
	Cards     []Card   `json:"cards"`
	Value     int      `json:"value"`
	BombCount int      `json:"bombCount,omitempty"`
}

type GameMode string

const (
	ModeNormal GameMode = "Normal"
	ModeSkill  GameMode = "Skill"
)

type HistoryEventType string

const (
	HistGameStart     HistoryEventType = "GameStart"
	HistPhaseChange   HistoryEventType = "PhaseChange"
	HistPlay          HistoryEventType = "Play"
	HistPass          HistoryEventType = "Pass"
	HistTribute       HistoryEventType = "Tribute"
	HistReturnTribute HistoryEventType = "ReturnTribute"
	HistSkillUse      HistoryEventType = "SkillUse"
	HistRoundEnd      HistoryEventType = "RoundEnd"
	HistPlayerFinish  HistoryEventType = "PlayerFinish"
	HistGameEnd       HistoryEventType = "GameEnd"
	HistLevelUp       HistoryEventType = "LevelUp"
)

type HistoryEntry struct {
	ID          string           `json:"id"`
	Timestamp   int64            `json:"timestamp"`
	Type        HistoryEventType `json:"type"`
	PlayerIndex *int             `json:"playerIndex,omitempty"`
	PlayerName  string           `json:"playerName,omitempty"`
	Message     string           `json:"message"`
	Details     any              `json:"details,omitempty"`
}
