export enum Suit {
  Spades = 0, 
  Hearts = 1, 
  Clubs = 2,  
  Diamonds = 3, 
  Joker = 4   
}

export enum Rank {
  Two = 2,
  Three = 3,
  Four = 4,
  Five = 5,
  Six = 6,
  Seven = 7,
  Eight = 8,
  Nine = 9,
  Ten = 10,
  Jack = 11,
  Queen = 12,
  King = 13,
  Ace = 14,
  SmallJoker = 15,
  BigJoker = 16
}

export interface Card {
  suit: Suit;
  rank: Rank;
  id: string; 
  isLevelCard?: boolean; 
  isWild?: boolean; 
}

export enum HandType {
  Single = 'Single',
  Pair = 'Pair',
  Trips = 'Trips',
  TripsWithPair = 'TripsWithPair',
  Straight = 'Straight',
  Tube = 'Tube',
  Plate = 'Plate',
  Bomb = 'Bomb',
  StraightFlush = 'StraightFlush',
  FourKings = 'FourKings'
}

export interface Hand {
  type: HandType;
  cards: Card[];
  value: number; 
  bombCount?: number;
}

// Hand interpretation with wild card usage
export interface HandInterpretation {
  hand: Hand;
  description: string; // Human-readable description of how wilds are used
  wildUsage?: { [cardId: string]: { asRank: Rank, asSuit?: Suit } }; // How each wild is interpreted
}

// Game Mode
export enum GameMode {
  Normal = 'Normal',
  Skill = 'Skill'
}

// Skill Card Types
export enum SkillCardType {
  DrawTwo = 'DrawTwo',           // 無中生有：獲得隨機兩張牌
  Steal = 'Steal',               // 順手牽羊：從目標玩家隨機獲得一張牌
  Discard = 'Discard',           // 過河拆橋：讓目標玩家隨機棄一張牌
  Skip = 'Skip',                 // 樂不思蜀：讓目標玩家下回合跳過
  Harvest = 'Harvest'            // 五穀豐登：每個玩家各獲得一張隨機牌
}

export interface SkillCard {
  id: string;
  type: SkillCardType;
}

// Skill card display names
export const SkillCardNames: { [key in SkillCardType]: string } = {
  [SkillCardType.DrawTwo]: '無中生有',
  [SkillCardType.Steal]: '順手牽羊',
  [SkillCardType.Discard]: '過河拆橋',
  [SkillCardType.Skip]: '樂不思蜀',
  [SkillCardType.Harvest]: '五穀豐登'
};

// Game History Log Entry Types
export enum HistoryEventType {
  GameStart = 'GameStart',
  PhaseChange = 'PhaseChange',
  Play = 'Play',
  Pass = 'Pass',
  Tribute = 'Tribute',
  ReturnTribute = 'ReturnTribute',
  SkillUse = 'SkillUse',
  RoundEnd = 'RoundEnd',
  PlayerFinish = 'PlayerFinish',
  GameEnd = 'GameEnd',
  LevelUp = 'LevelUp'
}

export interface HistoryEntry {
  id: string;
  timestamp: number;
  type: HistoryEventType;
  playerIndex?: number;
  playerName?: string;
  message: string; // Human-readable message
  details?: any; // Additional data (cards, skill type, etc.)
}

// Client-side game history state
export interface GameHistory {
  entries: HistoryEntry[];
  currentRound: number;
  currentLevel: number;
}
