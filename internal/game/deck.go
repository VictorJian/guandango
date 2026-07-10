package game

import (
	"fmt"
	"math/rand"
)

// CreateDeck builds two full decks (108 cards) — mirrors shared/deck.ts createDeck.
func CreateDeck() []Card {
	cards := make([]Card, 0, 108)
	suits := []Suit{Spades, Hearts, Clubs, Diamonds}

	for i := 0; i < 2; i++ {
		for _, suit := range suits {
			for rank := RankTwo; rank <= RankAce; rank++ {
				cards = append(cards, Card{
					Suit: suit,
					Rank: rank,
					ID:   fmt.Sprintf("%d-%d-%d", suit, rank, i),
				})
			}
		}
		cards = append(cards, Card{Suit: JokerSuit, Rank: SmallJoker, ID: fmt.Sprintf("joker-small-%d", i)})
		cards = append(cards, Card{Suit: JokerSuit, Rank: BigJoker, ID: fmt.Sprintf("joker-big-%d", i)})
	}
	return cards
}

func ShuffleDeck(cards []Card) []Card {
	rand.Shuffle(len(cards), func(i, j int) {
		cards[i], cards[j] = cards[j], cards[i]
	})
	return cards
}

// UpdateCardProperties sets isLevelCard/isWild flags based on the current level.
func UpdateCardProperties(cards []Card, currentLevel int) []Card {
	out := make([]Card, len(cards))
	for i, card := range cards {
		card.IsLevelCard = int(card.Rank) == currentLevel
		card.IsWild = card.IsLevelCard && card.Suit == Hearts
		out[i] = card
	}
	return out
}
