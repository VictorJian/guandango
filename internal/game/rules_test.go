package game

import (
	"fmt"
	"testing"
)

var cardSeq int

func card(suit Suit, rank Rank) Card {
	cardSeq++
	return Card{Suit: suit, Rank: rank, ID: fmt.Sprintf("t-%d", cardSeq)}
}

func wild(level int) Card {
	// Heart level card = wild
	return Card{Suit: Hearts, Rank: Rank(level), ID: "w", IsLevelCard: true, IsWild: true}
}

func TestGetLogicValue(t *testing.T) {
	if GetLogicValue(RankAce, 2) != 14 {
		t.Error("Ace should be 14")
	}
	if GetLogicValue(RankTwo, 2) != 19 {
		t.Error("level card should be 19")
	}
	if GetLogicValue(SmallJoker, 2) != 20 || GetLogicValue(BigJoker, 2) != 21 {
		t.Error("jokers should be 20/21")
	}
}

func TestSingleAndPair(t *testing.T) {
	h := GetHandType([]Card{card(Spades, 5)}, 2)
	if h == nil || h.Type != Single || h.Value != 5 {
		t.Fatalf("expected single 5, got %+v", h)
	}

	h = GetHandType([]Card{card(Spades, 9), card(Hearts, 9)}, 2)
	if h == nil || h.Type != Pair || h.Value != 9 {
		t.Fatalf("expected pair 9, got %+v", h)
	}

	// Wild + normal = pair of the normal card
	h = GetHandType([]Card{card(Spades, 9), wild(2)}, 2)
	if h == nil || h.Type != Pair || h.Value != 9 {
		t.Fatalf("expected wild pair 9, got %+v", h)
	}

	// Wild cannot pair with joker
	h = GetHandType([]Card{card(JokerSuit, SmallJoker), wild(2)}, 2)
	if h != nil {
		t.Fatalf("wild+joker should be invalid, got %+v", h)
	}
}

func TestTrips(t *testing.T) {
	h := GetHandType([]Card{card(Spades, 7), card(Hearts, 7), card(Clubs, 7)}, 2)
	if h == nil || h.Type != Trips || h.Value != 7 {
		t.Fatalf("expected trips 7, got %+v", h)
	}
	h = GetHandType([]Card{card(Spades, 7), card(Hearts, 7), wild(2)}, 2)
	if h == nil || h.Type != Trips || h.Value != 7 {
		t.Fatalf("expected wild trips 7, got %+v", h)
	}
}

func TestFullHouse(t *testing.T) {
	h := GetHandType([]Card{
		card(Spades, 8), card(Hearts, 8), card(Clubs, 8),
		card(Spades, 4), card(Hearts, 4),
	}, 2)
	if h == nil || h.Type != TripsWithPair || h.Value != 8 {
		t.Fatalf("expected full house 8, got %+v", h)
	}
}

func TestStraightAndStraightFlush(t *testing.T) {
	h := GetHandType([]Card{
		card(Spades, 3), card(Hearts, 4), card(Clubs, 5), card(Spades, 6), card(Hearts, 7),
	}, 2)
	if h == nil || h.Type != Straight || h.Value != 7 {
		t.Fatalf("expected straight 7, got %+v", h)
	}

	h = GetHandType([]Card{
		card(Spades, 3), card(Spades, 4), card(Spades, 5), card(Spades, 6), card(Spades, 7),
	}, 2)
	if h == nil || h.Type != StraightFlush {
		t.Fatalf("expected straight flush, got %+v", h)
	}

	// A-2-3-4-5 low straight (level 10 so 2 is a normal card)
	h = GetHandType([]Card{
		card(Spades, RankAce), card(Hearts, 2), card(Clubs, 3), card(Spades, 4), card(Hearts, 5),
	}, 10)
	if h == nil || h.Type != Straight || h.Value != 5 {
		t.Fatalf("expected low straight value 5, got %+v", h)
	}
}

func TestBombs(t *testing.T) {
	bomb4 := GetHandType([]Card{
		card(Spades, 6), card(Hearts, 6), card(Clubs, 6), card(Diamonds, 6),
	}, 2)
	if bomb4 == nil || bomb4.Type != Bomb || bomb4.BombCount != 4 {
		t.Fatalf("expected 4-bomb, got %+v", bomb4)
	}

	kings := GetHandType([]Card{
		card(JokerSuit, SmallJoker), card(JokerSuit, SmallJoker),
		card(JokerSuit, BigJoker), card(JokerSuit, BigJoker),
	}, 2)
	if kings == nil || kings.Type != FourKings {
		t.Fatalf("expected four kings, got %+v", kings)
	}

	// Wild completes a bomb
	bombW := GetHandType([]Card{
		card(Spades, 6), card(Hearts, 6), card(Clubs, 6), wild(2),
	}, 2)
	if bombW == nil || bombW.Type != Bomb {
		t.Fatalf("expected wild bomb, got %+v", bombW)
	}
}

func TestTubeAndPlate(t *testing.T) {
	tube := GetHandType([]Card{
		card(Spades, 4), card(Hearts, 4),
		card(Clubs, 5), card(Diamonds, 5),
		card(Spades, 6), card(Hearts, 6),
	}, 2)
	if tube == nil || tube.Type != Tube || tube.Value != 6 {
		t.Fatalf("expected tube 6, got %+v", tube)
	}

	plate := GetHandType([]Card{
		card(Spades, 9), card(Hearts, 9), card(Clubs, 9),
		card(Spades, 10), card(Hearts, 10), card(Clubs, 10),
	}, 2)
	if plate == nil || plate.Type != Plate || plate.Value != 10 {
		t.Fatalf("expected plate 10, got %+v", plate)
	}
}

func TestCompareHands(t *testing.T) {
	pair9 := GetHandType([]Card{card(Spades, 9), card(Hearts, 9)}, 2)
	pairK := GetHandType([]Card{card(Spades, 13), card(Hearts, 13)}, 2)
	if CompareHands(pairK, pair9) <= 0 {
		t.Error("pair K should beat pair 9")
	}

	bomb4 := GetHandType([]Card{card(Spades, 6), card(Hearts, 6), card(Clubs, 6), card(Diamonds, 6)}, 2)
	if CompareHands(bomb4, pairK) <= 0 {
		t.Error("bomb should beat pair")
	}

	sf := GetHandType([]Card{
		card(Spades, 3), card(Spades, 4), card(Spades, 5), card(Spades, 6), card(Spades, 7),
	}, 2)
	bomb5 := GetHandType([]Card{
		card(Spades, 6), card(Hearts, 6), card(Clubs, 6), card(Diamonds, 6), card(Spades, 6),
	}, 2)
	if CompareHands(sf, bomb5) <= 0 {
		t.Error("straight flush should beat 5-bomb")
	}

	bomb6 := GetHandType([]Card{
		card(Spades, 6), card(Hearts, 6), card(Clubs, 6),
		card(Diamonds, 6), card(Spades, 6), card(Hearts, 6),
	}, 2)
	if CompareHands(bomb6, sf) <= 0 {
		t.Error("6-bomb should beat straight flush")
	}

	kings := GetHandType([]Card{
		card(JokerSuit, SmallJoker), card(JokerSuit, SmallJoker),
		card(JokerSuit, BigJoker), card(JokerSuit, BigJoker),
	}, 2)
	if CompareHands(kings, bomb6) != 1 {
		t.Error("four kings beats everything")
	}

	// Level card pair beats Ace pair
	pairA := GetHandType([]Card{card(Spades, RankAce), card(Hearts, RankAce)}, 5)
	pairLevel := GetHandType([]Card{card(Spades, 5), card(Clubs, 5)}, 5)
	if CompareHands(pairLevel, pairA) <= 0 {
		t.Error("level pair should beat Ace pair")
	}
}

func TestDeck(t *testing.T) {
	deck := CreateDeck()
	if len(deck) != 108 {
		t.Fatalf("deck should have 108 cards, got %d", len(deck))
	}
	updated := UpdateCardProperties(deck, 2)
	wilds := 0
	for _, c := range updated {
		if c.IsWild {
			wilds++
		}
	}
	if wilds != 2 {
		t.Errorf("expected 2 wilds (heart level cards), got %d", wilds)
	}
}

