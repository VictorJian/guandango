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

const (
	Single        HandType = "Single"
	Pair          HandType = "Pair"
	Trips         HandType = "Trips"
	TripsWithPair HandType = "TripsWithPair"
	Straight      HandType = "Straight"
	Tube          HandType = "Tube"
	Plate         HandType = "Plate"
	Bomb          HandType = "Bomb"
	StraightFlush HandType = "StraightFlush"
	FourKings     HandType = "FourKings"
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
