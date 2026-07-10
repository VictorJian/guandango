package game

import "sort"

// GetLogicValue mirrors shared/rules.ts getLogicValue.
// Level cards are higher than Ace but lower than Jokers.
func GetLogicValue(rank Rank, level int) int {
	if rank == SmallJoker {
		return 20
	}
	if rank == BigJoker {
		return 21
	}
	if int(rank) == level {
		return 19
	}
	if rank == RankAce {
		return 14
	}
	return int(rank)
}

// SortCards returns a new slice sorted descending by logic value, then suit.
func SortCards(cards []Card, level int) []Card {
	out := append([]Card(nil), cards...)
	sort.SliceStable(out, func(i, j int) bool {
		valA := GetLogicValue(out[i].Rank, level)
		valB := GetLogicValue(out[j].Rank, level)
		if valA != valB {
			return valA > valB
		}
		return out[i].Suit > out[j].Suit
	})
	return out
}

// IsConsecutive checks whether values form a consecutive run (with A-2-3-4-5 special case).
func IsConsecutive(values []int) bool {
	if len(values) < 2 {
		return false
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)

	// Special Case: A-2-3-4-5
	if sorted[len(sorted)-1] == 14 && sorted[0] == 2 {
		if len(sorted) == 5 && sorted[0] == 2 && sorted[1] == 3 && sorted[2] == 4 && sorted[3] == 5 {
			return true
		}
	}

	for i := 0; i < len(sorted)-1; i++ {
		if sorted[i+1] != sorted[i]+1 {
			return false
		}
	}
	return true
}

// GetHandType analyzes cards and returns the hand, or nil if invalid — mirrors rules.ts getHandType.
func GetHandType(cards []Card, level int) *Hand {
	if len(cards) == 0 {
		return nil
	}

	length := len(cards)

	wildCount := 0
	var nonWilds []Card
	for _, c := range cards {
		if c.IsWild {
			wildCount++
		} else {
			nonWilds = append(nonWilds, c)
		}
	}

	// Counts map for non-wilds (using logic value)
	counts := map[int]int{}
	for _, c := range nonWilds {
		counts[GetLogicValue(c.Rank, level)]++
	}
	uniqueValues := make([]int, 0, len(counts))
	maxCount := 0
	for v, n := range counts {
		uniqueValues = append(uniqueValues, v)
		if n > maxCount {
			maxCount = n
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(uniqueValues))) // Descending

	// 1. Four Kings (Sky Bomb)
	if length == 4 {
		smallJokers, bigJokers := 0, 0
		for _, c := range cards {
			if c.Rank == SmallJoker {
				smallJokers++
			}
			if c.Rank == BigJoker {
				bigJokers++
			}
		}
		if smallJokers == 2 && bigJokers == 2 {
			return &Hand{Type: FourKings, Cards: cards, Value: 999}
		}
	}

	// 2. Single
	if length == 1 {
		val := GetLogicValue(cards[0].Rank, level)
		if cards[0].IsWild {
			val = 19
		}
		return &Hand{Type: Single, Cards: cards, Value: val}
	}

	// 3. Pair
	if length == 2 {
		if wildCount == 2 {
			return &Hand{Type: Pair, Cards: cards, Value: 19}
		}
		if wildCount == 1 {
			if nonWilds[0].Rank > RankAce {
				return nil // wild cannot pair with jokers
			}
			return &Hand{Type: Pair, Cards: cards, Value: uniqueValues[0]}
		}
		if len(uniqueValues) == 1 {
			return &Hand{Type: Pair, Cards: cards, Value: uniqueValues[0]}
		}
	}

	// 4. Trips
	if length == 3 {
		if wildCount == 3 {
			return &Hand{Type: Trips, Cards: cards, Value: 19}
		}
		if len(uniqueValues) == 1 && counts[uniqueValues[0]]+wildCount == 3 {
			if uniqueValues[0] > 19 {
				return nil
			}
			return &Hand{Type: Trips, Cards: cards, Value: uniqueValues[0]}
		}
	}

	// 5. Trips with Pair (Full House) — or 5-card bomb
	if length == 5 {
		if maxCount+wildCount == 5 {
			val := 19
			if len(uniqueValues) > 0 {
				val = uniqueValues[0]
			}
			return &Hand{Type: Bomb, Cards: cards, Value: val, BombCount: 5}
		}

		for _, tVal := range uniqueValues {
			if tVal > 19 {
				continue
			}
			tCount := counts[tVal]
			wildsForTrips := 3 - tCount
			if wildsForTrips < 0 {
				wildsForTrips = 0
			}

			if wildCount >= wildsForTrips {
				remWilds := wildCount - wildsForTrips
				var otherVals []int
				for _, v := range uniqueValues {
					if v != tVal {
						otherVals = append(otherVals, v)
					}
				}

				if len(otherVals) == 0 {
					continue
				}
				if len(otherVals) == 1 {
					pVal := otherVals[0]
					if pVal > 19 {
						continue
					}
					if counts[pVal]+remWilds >= 2 {
						return &Hand{Type: TripsWithPair, Cards: cards, Value: tVal}
					}
				}
			}
		}
	}

	// 6. Straight (5 cards) — possibly Straight Flush
	if length == 5 {
		hasJokerNonWild := false
		for _, c := range nonWilds {
			if c.Rank > RankAce {
				hasJokerNonWild = true
				break
			}
		}
		if !hasJokerNonWild {
			var ranks []int
			for _, c := range nonWilds {
				if c.Rank <= RankAce {
					ranks = append(ranks, int(c.Rank))
				}
			}
			uniqueRanks := map[int]bool{}
			for _, r := range ranks {
				uniqueRanks[r] = true
			}
			if len(uniqueRanks) == len(ranks) {
				if len(ranks) == 0 {
					return &Hand{Type: Straight, Cards: cards, Value: 14}
				}

				val := -1
				maxR, minR := ranks[0], ranks[0]
				for _, r := range ranks {
					if r > maxR {
						maxR = r
					}
					if r < minR {
						minR = r
					}
				}
				if maxR-minR <= 4 {
					top := minR + 4
					if top <= 14 {
						val = top
					}
				}

				hasAce := false
				for _, r := range ranks {
					if r == 14 {
						hasAce = true
					}
				}
				if hasAce {
					// Treat A as 1 for the low straight
					minL, maxL := 15, -1
					for _, r := range ranks {
						low := r
						if r == 14 {
							low = 1
						}
						if low < minL {
							minL = low
						}
						if low > maxL {
							maxL = low
						}
					}
					if maxL-minL <= 4 {
						if val == -1 || 5 > val {
							val = 5
						}
					}
				}

				if val != -1 {
					suits := map[Suit]bool{}
					for _, c := range nonWilds {
						suits[c.Suit] = true
					}
					if len(suits) <= 1 {
						return &Hand{Type: StraightFlush, Cards: cards, Value: val, BombCount: 5}
					}
					return &Hand{Type: Straight, Cards: cards, Value: val}
				}
			}
		}
	}

	// 7. Bomb (4+ cards)
	if length >= 4 {
		if maxCount+wildCount == length {
			if len(uniqueValues) <= 1 {
				val := 19
				if len(uniqueValues) > 0 {
					val = uniqueValues[0]
				}
				if val <= 19 {
					return &Hand{Type: Bomb, Cards: cards, Value: val, BombCount: length}
				}
			}
		}
	}

	// 8. Tube (鋼板) / Plate (木板) — 6 cards, natural only (no wilds)
	if length == 6 && wildCount == 0 {
		s := append([]Card(nil), cards...)
		sort.SliceStable(s, func(i, j int) bool { return s[i].Rank < s[j].Rank })

		if s[0].Rank == s[1].Rank && s[2].Rank == s[3].Rank && s[4].Rank == s[5].Rank {
			if s[2].Rank == s[0].Rank+1 && s[4].Rank == s[2].Rank+1 {
				return &Hand{Type: Tube, Cards: cards, Value: int(s[4].Rank)}
			}
			if s[4].Rank == 14 && s[0].Rank == 2 && s[2].Rank == 3 {
				return &Hand{Type: Tube, Cards: cards, Value: 3} // A-A-2-2-3-3
			}
		}

		if s[0].Rank == s[1].Rank && s[1].Rank == s[2].Rank &&
			s[3].Rank == s[4].Rank && s[4].Rank == s[5].Rank {
			if s[3].Rank == s[0].Rank+1 {
				return &Hand{Type: Plate, Cards: cards, Value: int(s[3].Rank)}
			}
		}
	}

	return nil
}

// CompareHands returns >0 if handA beats handB, <0 if it loses, 0 if not comparable/equal.
func CompareHands(handA, handB *Hand) int {
	if handA.Type == FourKings {
		return 1
	}
	if handB.Type == FourKings {
		return -1
	}

	isBombA := handA.Type == Bomb || handA.Type == StraightFlush
	isBombB := handB.Type == Bomb || handB.Type == StraightFlush

	// Bomb beats normal hands
	if isBombA && !isBombB {
		return 1
	}
	if !isBombA && isBombB {
		return -1
	}

	// Both bombs (or SF)
	if isBombA && isBombB {
		getScore := func(h *Hand) float64 {
			if h.Type == StraightFlush {
				return 5.5 // SF beats 5-bomb, loses to 6-bomb
			}
			return float64(h.BombCount)
		}
		sA, sB := getScore(handA), getScore(handB)
		if sA != sB {
			if sA > sB {
				return 1
			}
			return -1
		}
		return handA.Value - handB.Value
	}

	// Normal hands: different types cannot compare
	if handA.Type != handB.Type {
		return 0
	}
	if len(handA.Cards) != len(handB.Cards) {
		return 0
	}
	return handA.Value - handB.Value
}

// GetLargestCard returns the card with the highest logic value.
func GetLargestCard(cards []Card, level int) Card {
	sorted := SortCards(cards, level)
	return sorted[0]
}
