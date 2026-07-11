import React from 'react';
import { Card as CardType, Suit, Rank } from '../../shared/types';

interface Props {
  card: CardType;
  selected?: boolean;
  onClick?: () => void;
  small?: boolean;
  isHighlighted?: boolean;  // For new card highlight animation
}

// 傳統紅黑二色：黑桃/梅花=黑、紅心/紅磚=紅，靠形狀區分花色
const SUIT_COLORS: { [key in Suit]?: string } = {
  [Suit.Spades]: '#1f2937',
  [Suit.Hearts]: '#dc2626',
  [Suit.Clubs]: '#1f2937',
  [Suit.Diamonds]: '#dc2626',
};

// 內嵌 SVG 花色圖示：形狀差異明顯（紅磚為尖角菱形、梅花為三圓一桿）
const SUIT_PATHS: { [key in Suit]?: React.ReactNode } = {
  [Suit.Spades]: (
    <path d="M12 2C9 7 4 9.5 4 13.5c0 2.5 2 4 4 4 1 0 2-.3 2.8-1-.3 1.8-1 3-2.3 4.2V22h7v-1.3c-1.3-1.2-2-2.4-2.3-4.2.8.7 1.8 1 2.8 1 2 0 4-1.5 4-4C20 9.5 15 7 12 2z" />
  ),
  [Suit.Hearts]: (
    <path d="M12 21S3.5 15.5 1.9 10.6C.7 6.9 3.1 3.5 6.6 3.5c2.2 0 4 1.2 5.4 3.1 1.4-1.9 3.2-3.1 5.4-3.1 3.5 0 5.9 3.4 4.7 7.1C20.5 15.5 12 21 12 21z" />
  ),
  [Suit.Clubs]: (
    <>
      <circle cx="12" cy="6.8" r="4.6" />
      <circle cx="6.4" cy="13.6" r="4.6" />
      <circle cx="17.6" cy="13.6" r="4.6" />
      <path d="M10.6 14h2.8c-.2 3 .5 5 2 6.6V22H8.6v-1.4c1.5-1.6 2.2-3.6 2-6.6z" />
    </>
  ),
  [Suit.Diamonds]: (
    <path d="M12 1.5 20.5 12 12 22.5 3.5 12 12 1.5z" />
  ),
};

const SuitIcon: React.FC<{ suit: Suit; className?: string }> = ({ suit, className }) => (
  <svg viewBox="0 0 24 24" className={className} fill={SUIT_COLORS[suit] ?? 'currentColor'} aria-hidden>
    {SUIT_PATHS[suit]}
  </svg>
);

const getRankLabel = (rank: Rank) => {
  switch (rank) {
    case Rank.Two: return '2';
    case Rank.Three: return '3';
    case Rank.Four: return '4';
    case Rank.Five: return '5';
    case Rank.Six: return '6';
    case Rank.Seven: return '7';
    case Rank.Eight: return '8';
    case Rank.Nine: return '9';
    case Rank.Ten: return '10';
    case Rank.Jack: return 'J';
    case Rank.Queen: return 'Q';
    case Rank.King: return 'K';
    case Rank.Ace: return 'A';
    case Rank.SmallJoker: return 'Small Joker';
    case Rank.BigJoker: return 'Big Joker';
  }
};

export const Card: React.FC<Props> = ({ card, selected, onClick, small, isHighlighted }) => {
  const isJoker = card.suit === Suit.Joker;
  const suitColor = card.rank === Rank.BigJoker ? '#dc2626' : SUIT_COLORS[card.suit] ?? '#1f2937';

  const baseClasses = "relative bg-white rounded shadow-md border border-gray-300 flex flex-col justify-between select-none cursor-pointer transition-transform";
  const sizeClasses = small
    ? "w-8 h-12 text-xs p-1"
    : "w-12 h-[4.5rem] text-sm p-1.5 md:w-16 md:h-24 md:text-base md:p-2 hover:-translate-y-2";
  const selectClasses = selected ? "ring-2 ring-blue-500 -translate-y-4" : "";
  // Highlight animation for new cards - glowing green border with pulse
  const highlightClasses = isHighlighted
    ? "ring-4 ring-green-400 shadow-[0_0_15px_rgba(74,222,128,0.7)] animate-pulse"
    : "";

  if (isJoker) {
     return (
        <div
          className={`${baseClasses} ${sizeClasses} ${selectClasses} ${highlightClasses}`}
          style={{ color: suitColor }}
          onClick={onClick}
        >
           <div className="text-center w-full h-full flex items-center justify-center font-bold writing-vertical">
               {card.rank === Rank.SmallJoker ? '黑鬼' : '紅鬼'}
           </div>
        </div>
     );
  }

  return (
    <div
      className={`${baseClasses} ${sizeClasses} ${selectClasses} ${highlightClasses}`}
      style={{ color: suitColor }}
      onClick={onClick}
    >
      {/* 左上角：牌值 + 花色（牌重疊時仍看得到） */}
      <div className="flex flex-col items-start leading-none">
        <div className="font-bold">{getRankLabel(card.rank)}</div>
        <SuitIcon suit={card.suit} className={small ? "w-2.5 h-2.5 mt-0.5" : "w-3 h-3 md:w-4 md:h-4 mt-0.5"} />
      </div>

      {/* 中央浮水印 */}
      <div className="absolute inset-0 flex items-center justify-center pointer-events-none opacity-25">
        <SuitIcon suit={card.suit} className={small ? "w-5 h-5" : "w-7 h-7 md:w-9 md:h-9"} />
      </div>

      {/* 右下角花色 */}
      <SuitIcon suit={card.suit} className={`self-end ${small ? "w-2.5 h-2.5" : "w-3 h-3 md:w-4 md:h-4"}`} />

      {card.isLevelCard && (
          <div className="absolute top-0 right-0 w-2 h-2 bg-yellow-400 rounded-full"></div>
      )}
    </div>
  );
};
