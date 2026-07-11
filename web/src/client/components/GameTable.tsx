import React, { useState, useEffect, useRef } from 'react';
import { Card as CardType, Rank, Suit, GameMode, SkillCard, SkillCardType, Hand } from '../../shared/types';
import { Bot } from '../../shared/bot';
import { Card } from './Card';
import { GameState, RoomState } from '../useGame';
import { getLogicValue, isConsecutive, getAllPossibleHandTypes, getHandDescription } from '../../shared/rules';
import { SkillCardButton } from './SkillCardButton';
import { TargetSelectModal } from './TargetSelectModal';
import { GameHistory } from './GameHistory';

interface Props {
  gameState: GameState | null;
  roomState: RoomState;
  mySeat: number;
  onPlay: (cards: CardType[], handType?: Hand) => void;
  onPass: () => void;
  onReady: () => void;
  onStart: () => void;
  onTribute?: (cards: CardType[]) => void;
  onReturnTribute?: (cards: CardType[]) => void;
  chatMessages: {sender: string, text: string, time: string, seatIndex: number}[];
  onSendChat: (msg: string) => void;
  onSwitchSeat: (seatIdx: number) => void;
  onSetGameMode?: (mode: GameMode) => void;
  onUseSkill?: (skillId: string, targetSeat?: number) => void;
  onForceEndGame?: () => void;
  isSpectator?: boolean;
  onWatchPlayer?: (seat: number) => void;
  onConfirmStart?: () => void;
  onSetStartLevel?: (level: number) => void;
}

export const GameTable: React.FC<Props> = ({
  gameState, roomState, mySeat, onPlay, onPass, onReady, onStart,
  onTribute, onReturnTribute, chatMessages, onSendChat, onSwitchSeat,
  onSetGameMode, onUseSkill, onForceEndGame, isSpectator, onWatchPlayer, onConfirmStart, onSetStartLevel
}) => {
  const [selectedCardIds, setSelectedCardIds] = useState<string[]>([]);
  const [chatInput, setChatInput] = useState('');
  const [showChat, setShowChat] = useState(false); // 手機版聊天室開關（桌機恆顯示）
  const [viewMode, setViewMode] = useState<'normal' | 'stacked'>('normal'); 
  const chatEndRef = useRef<HTMLDivElement>(null);
  const [showEmojiPicker, setShowEmojiPicker] = useState(false);
  
  // Common emojis for quick selection
  const quickEmojis = ['😀', '😂', '🤣', '😎', '🥳', '😭', '😡', '🤔', '👍', '👎', '❤️', '🔥', '💯', '🎉', '🤝', '✌️', '💪', '🙏', '😱', '🤯'];
  
  // Skill card state
  const [pendingSkill, setPendingSkill] = useState<SkillCard | null>(null);
  const [showTargetSelect, setShowTargetSelect] = useState(false);
  
  // New card highlight state
  const [highlightedCardIds, setHighlightedCardIds] = useState<Set<string>>(new Set());
  
  // Chat bubble state for each seat (seat -> message)
  const [chatBubbles, setChatBubbles] = useState<{ [seat: number]: string }>({});
  
  // History window state
  const [showHistory, setShowHistory] = useState(false);
  
  // Hand type selection state (for wild cards with multiple interpretations)
  const [possibleHands, setPossibleHands] = useState<Hand[]>([]);
  const [showHandSelector, setShowHandSelector] = useState(false);
  
  // Track new cards and set up highlight timer
  useEffect(() => {
      if (gameState?.newCardIds && gameState.newCardIds.length > 0) {
          const newIds = new Set(gameState.newCardIds);
          setHighlightedCardIds(prev => new Set([...prev, ...newIds]));
          
          // Clear highlight after 3 seconds
          const timer = setTimeout(() => {
              setHighlightedCardIds(prev => {
                  const updated = new Set(prev);
                  gameState.newCardIds!.forEach(id => updated.delete(id));
                  return updated;
              });
          }, 3000);
          
          return () => clearTimeout(timer);
      }
  }, [gameState?.newCardIds]);
  
  // Track chat messages and show bubbles
  useEffect(() => {
      if (chatMessages.length > 0) {
          const lastMsg = chatMessages[chatMessages.length - 1];
          if (lastMsg.seatIndex !== undefined) {
              // Show bubble for this seat
              setChatBubbles(prev => ({
                  ...prev,
                  [lastMsg.seatIndex]: lastMsg.text
              }));
              
              // Clear bubble after 5 seconds
              const timer = setTimeout(() => {
                  setChatBubbles(prev => {
                      const updated = { ...prev };
                      delete updated[lastMsg.seatIndex];
                      return updated;
                  });
              }, 5000);
              
              return () => clearTimeout(timer);
          }
      }
  }, [chatMessages.length]);
  
  // Auto-scroll chat
  useEffect(() => {
      chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatMessages]);

  const getPlayerAt = (offset: number) => {
    const seat = (mySeat + offset) % 4;
    const player = roomState.players.find(p => p && p.seatIndex === seat);
    // hands[seat] 可能是牌陣列（自己/被觀看者）或張數數字（其他人）。
    // 觀戰切換視角時會有一瞬間座位與 gameState 不同步，必須兩種形態都能處理，
    // 否則把牌陣列當數字渲染會讓 React 整頁崩潰。
    const rawHand = gameState ? gameState.hands[seat] : 0;
    const handCount = Array.isArray(rawHand) ? rawHand.length : (rawHand ?? 0);
    
    // Team identification
    const isTeammate = (mySeat + 2) % 4 === seat;
    const isOpponent = !isTeammate && seat !== mySeat;
    
    return { player, handCount, seat, isTeammate, isOpponent };
  };

  const top = getPlayerAt(2);
  const left = getPlayerAt(3);
  const right = getPlayerAt(1);
  const me = getPlayerAt(0);

  const myHandRaw = gameState ? gameState.hands[mySeat] : [];
  const myHandOriginal: CardType[] = Array.isArray(myHandRaw) ? myHandRaw : [];
  const [sortedHand, setSortedHand] = useState<CardType[]>([]);
  const [straightFlushIds, setStraightFlushIds] = useState<Set<string>>(new Set());

  useEffect(() => {
      if (myHandOriginal.length > 0 && gameState) {
          setSortedHand(myHandOriginal);
          
          // Detect Straight Flushes for Highlighting
          const sfSet = new Set<string>();
          // Logic: Group by Suit -> Sort by Rank -> Check consecutive 5+
          const suits = [Suit.Spades, Suit.Hearts, Suit.Clubs, Suit.Diamonds];
          
          suits.forEach(s => {
              const suitCards = myHandOriginal.filter(c => c.suit === s && !c.isWild && c.rank <= Rank.Ace);
              // Sort by Rank Ascending
              suitCards.sort((a, b) => a.rank - b.rank);
              
              // Find sequences
              let seq: CardType[] = [];
              for (let i = 0; i < suitCards.length; i++) {
                  if (seq.length === 0) {
                      seq.push(suitCards[i]);
                  } else {
                      const last = seq[seq.length - 1];
                      if (suitCards[i].rank === last.rank + 1) {
                          seq.push(suitCards[i]);
                      } else if (suitCards[i].rank === last.rank) {
                          // Duplicate rank? Skip or fork? 
                          // For visualization, just highlight one path or all?
                          // Simple: Reset sequence if gap
                          // Actually duplicates break strict sequence check if we just use prev.
                          // But if it is duplicate rank, we can still form SF if we have 5 unique ranks.
                          // Simplification: Check strict consecutive ranks.
                          // If gap > 1, reset.
                      } else {
                          // Gap
                          if (seq.length >= 5) {
                              seq.forEach(c => sfSet.add(c.id));
                          }
                          seq = [suitCards[i]];
                      }
                  }
              }
              if (seq.length >= 5) {
                  seq.forEach(c => sfSet.add(c.id));
              }
          });
          setStraightFlushIds(sfSet);

      } else {
          setSortedHand([]);
          setStraightFlushIds(new Set());
      }
  }, [myHandOriginal, gameState?.level]); 

  const toggleViewMode = () => {
      setViewMode(prev => prev === 'normal' ? 'stacked' : 'normal');
  };

  const toggleSelect = (id: string) => {
    setSelectedCardIds(prev => 
      prev.includes(id) ? prev.filter(x => x !== id) : [...prev, id]
    );
  };

  const handlePlay = () => {
    const cards = sortedHand.filter(c => selectedCardIds.includes(c.id));
    
    // Check if cards contain wild cards
    const hasWild = cards.some(c => c.isWild);
    
    if (hasWild && gameState) {
      // Get all possible hand types
      const possibilities = getAllPossibleHandTypes(cards, gameState.level);
      
      if (possibilities.length > 1) {
        // Multiple interpretations - show selector
        setPossibleHands(possibilities);
        setShowHandSelector(true);
        return;
      } else if (possibilities.length === 1) {
        // Single interpretation - play directly
        onPlay(cards, possibilities[0]);
        setSelectedCardIds([]);
        return;
      }
    }
    
    // No wild cards or no valid interpretation - play as normal
    onPlay(cards);
    setSelectedCardIds([]);
  };
  
  const handleHandTypeSelect = (hand: Hand) => {
    const cards = sortedHand.filter(c => selectedCardIds.includes(c.id));
    onPlay(cards, hand);
    setSelectedCardIds([]);
    setShowHandSelector(false);
    setPossibleHands([]);
  };
  
  const handleHint = () => {
      if (!gameState) return;
      const bot = new Bot(sortedHand, gameState.level);
      const target = gameState.lastHand && gameState.lastHand.playerIndex !== mySeat ? gameState.lastHand.hand : null;
      const move = bot.decideMove(target);
      
      if (move) {
          setSelectedCardIds(move.map(c => c.id));
      } else {
          setSelectedCardIds([]);
      }
  };
  
  const handleTributeAction = () => {
      const cards = sortedHand.filter(c => selectedCardIds.includes(c.id));
      if (cards.length !== 1) {
          alert("請選擇一張牌");
          return;
      }
      if (gameState.phase === 'ReturnTribute') {
          // 還貢限制：不可大於10、不可是當前等級的牌（除非手上完全沒有合規的牌）
          const isEligible = (c: CardType) => c.rank <= 10 && c.rank !== gameState.level;
          if (sortedHand.some(isEligible) && !isEligible(cards[0])) {
              alert(cards[0].rank === gameState.level ? '還貢的牌不能是當前等級的牌' : '還貢的牌不能大於10');
              return;
          }
      }
      if (gameState.phase === 'Tribute' && onTribute) onTribute(cards);
      if (gameState.phase === 'ReturnTribute' && onReturnTribute) onReturnTribute(cards);
      setSelectedCardIds([]);
  };
  
  const handleChatSubmit = (e: React.FormEvent) => {
      e.preventDefault();
      if (chatInput.trim()) {
          onSendChat(chatInput.trim());
          setChatInput('');
      }
  }
  
  // Skill card handlers
  const handleSkillClick = (skill: SkillCard) => {
      const needsTarget = [SkillCardType.Steal, SkillCardType.Discard, SkillCardType.Skip];
      if (needsTarget.includes(skill.type)) {
          setPendingSkill(skill);
          setShowTargetSelect(true);
      } else {
          // No target needed, use immediately
          onUseSkill?.(skill.id);
      }
  };
  
  const handleTargetSelect = (targetSeat: number) => {
      if (pendingSkill) {
          onUseSkill?.(pendingSkill.id, targetSeat);
          setPendingSkill(null);
          setShowTargetSelect(false);
      }
  };
  
  const handleTargetCancel = () => {
      setPendingSkill(null);
      setShowTargetSelect(false);
  };
  
  // Get players for target selection
  const getPlayersForTargeting = () => {
      return roomState.players
          .filter((p): p is NonNullable<typeof p> => p !== null)
          .map(p => ({
              name: p.name,
              seatIndex: p.seatIndex,
              handCount: gameState 
                  ? (typeof gameState.hands[p.seatIndex] === 'number' 
                      ? gameState.hands[p.seatIndex] as number 
                      : (gameState.hands[p.seatIndex] as CardType[]).length)
                  : 0
          }));
  };

  const isTributePhase = gameState && (gameState.phase === 'Tribute' || gameState.phase === 'ReturnTribute');
  const amIPaying = !isSpectator && isTributePhase && gameState.tributeState && (
      (gameState.phase === 'Tribute' && gameState.tributeState.pendingTributes.some((t: any) => t.from === mySeat)) ||
      (gameState.phase === 'ReturnTribute' && gameState.tributeState.pendingReturns.some((t: any) => t.from === mySeat))
  );

  // 進貢/還貢狀態畫面：所有人可見，貢牌完成後由上一局第四名確認開局
  const renderTributePanel = () => {
    if (!gameState || !gameState.tributeState) return null;
    if (!['Tribute', 'ReturnTribute', 'TributeConfirm'].includes(gameState.phase)) return null;

    const ts = gameState.tributeState;
    const nameOf = (seat: number) =>
        roomState.players.find(p => p && p.seatIndex === seat)?.name || `座位${seat}`;
    const tributes = [...(ts.completedTributes ?? []), ...(ts.pendingTributes ?? [])];
    const returns = ts.pendingReturns ?? [];

    const titles: { [key: string]: string } = {
        Tribute: '進貢階段',
        ReturnTribute: '還貢階段',
        TributeConfirm: '貢牌完成',
    };

    const renderExchange = (label: string, labelColor: string, entries: { from: number, to: number, card?: CardType }[]) =>
        entries.map((e, i) => (
            <div key={`${label}-${i}`} className="flex items-center gap-3 text-white">
                <span className={`${labelColor} font-bold w-10`}>{label}</span>
                <span>{nameOf(e.from)} → {nameOf(e.to)}</span>
                {e.card
                    ? <Card card={e.card} small />
                    : <span className="text-gray-400 animate-pulse">選牌中...</span>}
            </div>
        ));

    return (
      <div className="bg-[#252526] border border-yellow-500/50 rounded-lg p-6 shadow-2xl flex flex-col items-center gap-3 mb-4">
        <div className="text-yellow-400 font-bold text-2xl">{titles[gameState.phase]}</div>
        {renderExchange('進貢', 'text-red-400', tributes)}
        {renderExchange('還貢', 'text-green-400', returns)}
        {gameState.phase === 'TributeConfirm' && (
            gameState.confirmSeat === mySeat && !isSpectator ? (
                <button
                    onClick={onConfirmStart}
                    className="mt-2 bg-yellow-500 hover:bg-yellow-600 text-black px-8 py-2 rounded-full font-bold shadow-lg animate-pulse"
                >
                    確定開始
                </button>
            ) : (
                <div className="text-gray-400 mt-2 animate-pulse">
                    等待 {nameOf(gameState.confirmSeat ?? -1)}（上局第四名）確認開始...
                </div>
            )
        )}
      </div>
    );
  };

  const renderLastHand = () => {
    if (!gameState || !gameState.lastHand) return null;
    const { playerIndex, hand } = gameState.lastHand;
    const playerName = roomState.players.find(p => p && p.seatIndex === playerIndex)?.name || `Seat ${playerIndex}`;
    
    return (
      <div className="bg-green-700/50 p-4 rounded-lg flex flex-col items-center">
        <div className="text-white mb-2 font-bold">{playerName} 出牌:</div>
        <div className="flex -space-x-8">
           {hand.cards.map((c: CardType) => (
             <Card key={c.id} card={c} />
           ))}
        </div>
        <div className="text-yellow-300 font-bold mt-2">{hand.type}</div>
      </div>
    );
  };

  // Helper to render cards played in round action
  const renderActionCards = (cards: CardType[] | undefined) => {
      if (!cards || cards.length === 0) return null;
      return (
          <div className="flex gap-0.5 mt-1">
              {cards.slice(0, 6).map((card, i) => (
                  <div key={i} className="w-6 h-8 bg-white rounded text-xs flex items-center justify-center font-bold border border-gray-300"
                       style={{ color: (card.suit === Suit.Hearts || card.suit === Suit.Diamonds) ? 'red' : 'black' }}>
                      {card.rank === Rank.SmallJoker ? '🃏' : card.rank === Rank.BigJoker ? '🃟' : 
                       ['', '', '2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K', 'A'][card.rank] || '?'}
                  </div>
              ))}
              {cards.length > 6 && <span className="text-white text-xs">+{cards.length - 6}</span>}
          </div>
      );
  };

  // 觀看指定座位的觀戰者名單
  const watchersOf = (seat: number) =>
      (roomState.spectators ?? []).filter(s => s.watchSeat === seat).map(s => s.name);

  const PlayerArea = ({ data, pos }: { data: any, pos: string }) => {
    const action = gameState?.roundActions?.[data.seat];
    const bubble = chatBubbles[data.seat];
    const watchers = watchersOf(data.seat);
    
    // Get winner position (第一名、第二名、第三名、第四名)
    const getWinnerPosition = () => {
      if (!gameState || !gameState.winners) return null;
      const position = gameState.winners.indexOf(data.seat);
      if (position === -1) return null;
      const labels = ['第一名', '第二名', '第三名', '第四名'];
      const colors = ['bg-yellow-500', 'bg-orange-500', 'bg-purple-500', 'bg-gray-500'];
      return { label: labels[position], color: colors[position] };
    };
    
    const winnerPos = getWinnerPosition();
    
    return (
      <div
          className={`absolute ${pos} flex flex-col items-center p-2 md:p-4 rounded-lg transition-colors ${data.isTeammate ? 'bg-blue-900/40 border-2 border-blue-400' : 'bg-black/20'} ${!gameState && !data.player ? 'cursor-pointer hover:bg-white/10' : ''}`}
          onClick={() => !isSpectator && !gameState && !data.player && onSwitchSeat(data.seat)}
      >
         {/* Chat Bubble */}
         {bubble && (
           <div className="absolute -top-16 left-1/2 -translate-x-1/2 z-50 animate-bounce-in">
             <div className="relative bg-white text-gray-800 px-4 py-2 rounded-xl shadow-lg max-w-48 text-sm font-medium whitespace-pre-wrap">
               {bubble}
               {/* Bubble arrow */}
               <div className="absolute -bottom-2 left-1/2 -translate-x-1/2 w-0 h-0 border-l-8 border-r-8 border-t-8 border-l-transparent border-r-transparent border-t-white"></div>
             </div>
           </div>
         )}
         
         <div className="w-9 h-9 md:w-12 md:h-12 bg-gray-300 rounded-full flex items-center justify-center mb-2 relative">
           {data.player ? data.player.name[0].toUpperCase() : (gameState ? '?' : '+')}
           {data.isTeammate && <div className="absolute -top-1 -right-1 bg-blue-500 text-xs text-white px-1 rounded">友</div>}
           {data.isOpponent && <div className="absolute -top-1 -right-1 bg-red-500 text-xs text-white px-1 rounded">敵</div>}
           {data.player && data.player.seatIndex === 0 && (
               <div className="absolute -bottom-1 -right-1 text-xs bg-yellow-500 text-black px-1 rounded font-bold border border-white">
                   Host
               </div>
           )}
           {/* Winner Position Badge */}
           {winnerPos && (
               <div className={`absolute -top-2 left-1/2 -translate-x-1/2 ${winnerPos.color} text-white text-xs px-2 py-0.5 rounded-full font-bold shadow-lg border-2 border-white animate-pulse`}>
                   {winnerPos.label}
               </div>
           )}
         </div>
         <div className="text-white font-bold flex items-center gap-2 text-sm md:text-base">
             {data.player ? data.player.name : (gameState ? '等待中...' : '點選入座')}
             {data.player && (data.player as any).isDisconnected && (
                 <span className="text-red-500 text-xs font-bold bg-white px-1 rounded animate-pulse">OFF</span>
             )}
         </div>
         {gameState && <div className="text-yellow-400 text-sm md:text-base">張數: {data.handCount}</div>}
         {data.player && data.player.isReady && !gameState && <div className="text-green-400 text-sm">Ready</div>}
         {watchers.length > 0 && (
             <div className="text-purple-400 text-xs mt-1 max-w-32 text-center">
                 👁 {watchers.join('、')} 觀看中
             </div>
         )}
         
         {/* Show current round action */}
         {gameState && action && (
             <div className="mt-2 flex flex-col items-center">
                 {action.type === 'pass' ? (
                     <div className="text-gray-400 font-bold text-sm bg-gray-700/50 px-3 py-1 rounded">過</div>
                 ) : (
                     <div className="flex flex-col items-center">
                         <div className="text-green-400 text-xs mb-1">{action.hand?.type || '出牌'}</div>
                         {renderActionCards(action.cards)}
                     </div>
                 )}
             </div>
         )}
         
         {gameState && gameState.currentTurn === data.seat && !action && (
             <div className="animate-bounce text-red-500 font-bold mt-2">Thinking...</div>
         )}
      </div>
    );
  };

  const getStackedMatrix = () => {
      if (!gameState) return [];
      
      const matrix: { [key: number]: { [key: number]: CardType } } = {};
      const suits = [Suit.Spades, Suit.Hearts, Suit.Clubs, Suit.Diamonds, Suit.Joker];
      
      const presentValues = new Set<number>();
      
      sortedHand.forEach(c => {
          const val = getLogicValue(c.rank, gameState.level);
          presentValues.add(val);
      });

      const sortedVals = Array.from(presentValues).sort((a, b) => b - a);
      
      return sortedVals.map(val => {
          const cardsOfRank = sortedHand.filter(c => getLogicValue(c.rank, gameState.level) === val);
          
          const slots: { [key: number]: CardType[] } = {
              [Suit.Joker]: [],
              [Suit.Spades]: [],
              [Suit.Hearts]: [],
              [Suit.Clubs]: [],
              [Suit.Diamonds]: []
          };
          
          cardsOfRank.forEach(c => {
             if (c.rank === Rank.SmallJoker || c.rank === Rank.BigJoker) {
                 slots[Suit.Joker].push(c);
             } else {
                 slots[c.suit].push(c);
             }
          });
          
          return { val, slots };
      });
  };

  return (
    <div className="relative w-full h-screen bg-[#1e1e1e] overflow-hidden flex items-center justify-center font-mono">
      <div className="absolute inset-4 md:inset-20 border-2 border-[#333333] rounded-xl opacity-50 pointer-events-none"></div>

      {/* 手機時上方玩家往右挪，避免和左上角等級/歷史紀錄面板重疊 */}
      <PlayerArea data={top} pos="top-4 left-[64%] md:left-1/2 -translate-x-1/2" />
      <PlayerArea data={left} pos="left-1 md:left-8 top-1/2 -translate-y-1/2" />
      <PlayerArea data={right} pos="right-1 md:right-8 top-1/2 -translate-y-1/2" />

      {/* Spectator Panel */}
      {isSpectator && (
          <div className="absolute bottom-4 left-4 z-50 bg-[#252526] border border-purple-500/50 rounded-lg p-3 shadow-lg pointer-events-auto">
              <div className="text-purple-400 font-bold text-sm mb-2">👁 觀戰模式</div>
              <div className="text-xs text-gray-400 mb-1">觀看玩家:</div>
              <div className="flex gap-1">
                  {roomState.players.map(p => p && (
                      <button
                          key={p.seatIndex}
                          onClick={() => onWatchPlayer?.(p.seatIndex)}
                          className={`px-2 py-1 rounded text-xs font-bold transition-colors ${
                              p.seatIndex === mySeat
                                  ? 'bg-purple-600 text-white'
                                  : 'bg-[#3c3c3c] text-gray-300 hover:bg-[#4a4a4a]'
                          }`}
                      >
                          {p.name}
                      </button>
                  ))}
              </div>
          </div>
      )}
      
      {/* Chat toggle (mobile only) */}
      <button
          onClick={() => setShowChat(!showChat)}
          className="md:hidden absolute top-4 right-4 z-30 bg-[#252526] border border-[#333333] rounded-full w-10 h-10 text-lg shadow-lg"
      >
          💬
      </button>

      {/* Chat Box — always visible on desktop, togglable on mobile */}
      <div className={`absolute top-16 md:top-4 right-4 w-72 h-56 bg-[#252526] border border-[#333333] rounded ${showChat ? 'flex' : 'hidden'} md:flex flex-col pointer-events-auto z-20 shadow-lg`}>
          <div className="flex-1 overflow-y-auto p-2 text-sm text-[#d4d4d4] scrollbar-thin">
              {chatMessages.map((msg, i) => (
                  <div key={i} className="mb-1">
                      <span className="text-[#858585] text-xs">[{msg.time}] </span>
                      <span className="font-bold text-[#569cd6]">{msg.sender}: </span>
                      <span className="break-words">{msg.text}</span>
                  </div>
              ))}
              <div ref={chatEndRef} />
          </div>
          
          {/* Emoji Picker */}
          {showEmojiPicker && (
              <div className="p-2 border-t border-[#333333] bg-[#1e1e1e] grid grid-cols-10 gap-1">
                  {quickEmojis.map((emoji, i) => (
                      <button 
                          key={i} 
                          type="button"
                          onClick={() => {
                              setChatInput(prev => prev + emoji);
                              setShowEmojiPicker(false);
                          }}
                          className="text-lg hover:bg-[#3c3c3c] rounded p-1 transition-colors"
                      >
                          {emoji}
                      </button>
                  ))}
              </div>
          )}
          
          <form onSubmit={handleChatSubmit} className="p-2 border-t border-[#333333] flex items-center gap-1">
              <button 
                  type="button" 
                  onClick={() => setShowEmojiPicker(!showEmojiPicker)}
                  className="text-lg hover:bg-[#3c3c3c] rounded p-1"
                  title="表情"
              >
                  😊
              </button>
              <input 
                  className="flex-1 bg-[#3c3c3c] border-none text-white text-sm focus:outline-none rounded px-2 py-1" 
                  placeholder="輸入訊息..." 
                  value={chatInput}
                  onChange={e => setChatInput(e.target.value)}
              />
              <button type="submit" className="text-[#0e639c] font-bold text-sm hover:text-[#1177bb]">發送</button>
          </form>
      </div>

      {gameState && (
          <div className="absolute top-4 left-4 flex flex-col gap-2 items-start z-50">
              <div className="bg-[#252526] border border-[#333333] px-3 py-1.5 md:px-4 md:py-2 rounded shadow-lg flex flex-col gap-1 max-w-[46vw] md:max-w-none">
                  {gameState.teamLevels && (
                      <div className="text-xs md:text-sm flex flex-col gap-0.5">
                          {[0, 1].map(team => {
                              const members = roomState.players
                                  .filter(p => p && p.seatIndex % 2 === team)
                                  .map(p => p!.name)
                                  .join('、') || `隊伍${team}`;
                              const isActive = gameState.activeTeam === team;
                              return (
                                  <div key={team} className={isActive ? 'text-yellow-400 font-bold' : 'text-gray-400'}>
                                      {members}：{gameState.teamLevels![team]}階{isActive && '（目前）'}
                                  </div>
                              );
                          })}
                      </div>
                  )}
              </div>

              {/* History Button — below the level panel */}
              <button
                  onClick={() => setShowHistory(true)}
                  className="bg-blue-600 hover:bg-blue-700 text-white px-3 py-1.5 text-sm md:px-4 md:py-2 md:text-base rounded-lg shadow-lg font-medium transition flex items-center gap-2"
                  title="查看遊戲歷史紀錄"
              >
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  歷史紀錄
                  {gameState.history && gameState.history.length > 0 && (
                      <span className="bg-red-500 text-xs px-2 py-0.5 rounded-full">
                          {gameState.history.length}
                      </span>
                  )}
              </button>

              {/* Host Force End Button */}
              {!isSpectator && me.player && me.player.seatIndex === 0 && (
                <button 
                    onClick={() => {
                        if (confirm('⚠️ 確定要強制結束目前的遊戲嗎？所有進度將遺失。')) {
                            onForceEndGame?.();
                        }
                    }}
                    className="bg-red-900/80 hover:bg-red-600 text-white text-xs px-3 py-1 rounded border border-red-500/50 shadow-lg backdrop-blur-sm transition-all flex items-center gap-1"
                >
                    <span>⛔</span> 強制結束
                </button>
              )}
          </div>
      )}
      
      <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2">
        {renderTributePanel()}
        {renderLastHand()}
        {!gameState && (
            <div className="flex flex-col gap-4 mt-8 items-center">
               <div className="text-white text-xl">等待玩家加入...</div>
               
               {/* 起始階層選項（開發環境使用，僅房主可調整） */}
               {roomState.devMode && (
                   <div className="flex items-center gap-2 bg-[#252526] border border-orange-500/60 rounded px-3 py-2">
                       <span className="text-orange-400 text-sm font-bold">起始階層</span>
                       {!isSpectator && me.player?.seatIndex === 0 ? (
                           <select
                               value={roomState.startLevel ?? 2}
                               onChange={e => onSetStartLevel?.(parseInt(e.target.value))}
                               className="bg-[#3c3c3c] text-white rounded px-2 py-1 text-sm focus:outline-none"
                           >
                               {[2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14].map(v => (
                                   <option key={v} value={v}>
                                       {v <= 10 ? v : ['J', 'Q', 'K', 'A'][v - 11]}
                                   </option>
                               ))}
                           </select>
                       ) : (
                           <span className="text-white text-sm">
                               {(roomState.startLevel ?? 2) <= 10
                                   ? (roomState.startLevel ?? 2)
                                   : ['J', 'Q', 'K', 'A'][(roomState.startLevel ?? 2) - 11]}
                           </span>
                       )}
                       <span className="text-gray-500 text-xs">（開發環境使用）</span>
                   </div>
               )}
               {/* Game mode toggle hidden — Skill mode not available yet (Go server is Normal-mode only) */}
               {!isSpectator && me.player && !me.player.isReady && (
                   <button onClick={onReady} className="bg-blue-500 text-white px-6 py-2 rounded font-bold">準備</button>
               )}
               {!isSpectator && me.player && me.player.seatIndex === 0 && (
                   <button onClick={onStart} className="bg-yellow-500 text-black px-6 py-2 rounded font-bold">開始遊戲（房主）</button>
               )}
            </div>
        )}
      </div>

      <div className="absolute bottom-0 w-full flex flex-col items-center pb-4 z-20 pointer-events-none">
        {/* Debug Info */}
        {gameState && (
            <div className="text-xs text-gray-500 mb-1 pointer-events-auto">
                [Debug] mySeat={mySeat}, currentTurn={gameState.currentTurn}, phase={gameState.phase}, isMyTurn={String(gameState.currentTurn === mySeat)}, myCards={Array.isArray(gameState.hands[mySeat]) ? (gameState.hands[mySeat] as any[]).length : '?'}
            </div>
        )}

        {/* Skill Cards Area */}
        {gameState && gameState.gameMode === GameMode.Skill && gameState.mySkillCards && gameState.mySkillCards.length > 0 && (
            <div className="mb-4 pointer-events-auto flex flex-col items-center">
                <div className="text-purple-400 text-sm mb-2 font-bold">我的技能卡</div>
                <div className="flex gap-3">
                    {gameState.mySkillCards.map((skill) => (
                        <SkillCardButton 
                            key={skill.id} 
                            skill={skill} 
                            onClick={() => handleSkillClick(skill)}
                            disabled={gameState.currentTurn !== mySeat || gameState.phase !== 'Playing'}
                        />
                    ))}
                </div>
                {gameState.currentTurn === mySeat && gameState.phase === 'Playing' && (
                    <div className="text-xs text-gray-400 mt-1">點選技能卡使用（使用後仍可出牌）</div>
                )}
            </div>
        )}

        {/* Controls Container */}
        <div className="mb-4 md:mb-8 pointer-events-auto">
            {!isSpectator && gameState && gameState.currentTurn === mySeat && gameState.phase === 'Playing' && (
                <div className="flex flex-wrap justify-center gap-2 md:gap-4">
                    <button
                      onClick={toggleViewMode}
                      className="bg-gray-600 hover:bg-gray-700 text-white px-3 md:px-4 py-2 rounded-full font-bold shadow-lg text-sm md:text-base"
                    >
                      {viewMode === 'normal' ? '切換同花順檢視' : '切換普通檢視'}
                    </button>
                    <button
                      onClick={handleHint}
                      className="bg-yellow-500 hover:bg-yellow-600 text-black px-3 md:px-4 py-2 rounded-full font-bold shadow-lg text-sm md:text-base"
                    >
                      提示
                    </button>
                    <button
                      onClick={handlePlay}
                      disabled={selectedCardIds.length === 0}
                      className="bg-blue-600 hover:bg-blue-700 text-white px-6 md:px-8 py-2 rounded-full font-bold shadow-lg disabled:opacity-50 text-sm md:text-base"
                    >
                      出牌
                    </button>
                    <button
                      onClick={onPass}
                      className="bg-red-600 hover:bg-red-700 text-white px-6 md:px-8 py-2 rounded-full font-bold shadow-lg text-sm md:text-base"
                    >
                      過
                    </button>
                </div>
            )}
            
            {amIPaying && (
                <div className="flex gap-4">
                   <div className="text-yellow-400 font-bold text-xl animate-pulse">
                       {gameState!.phase === 'Tribute' ? '請進貢最大牌' : `請還貢一張牌（不可大於10、不可為${gameState!.level}）`}
                   </div>
                   <button 
                      onClick={handleTributeAction} 
                      className="bg-purple-600 hover:bg-purple-700 text-white px-8 py-2 rounded-full font-bold shadow-lg"
                   >
                      確認
                   </button>
                </div>
            )}
        </div>

        {/* Hand Area - Compact Grid (horizontal scroll on small screens) */}
        <div className={`px-2 md:px-8 max-w-full overflow-x-auto flex items-end justify-start md:justify-center pointer-events-auto transition-all duration-300 ${viewMode === 'normal' ? 'h-32 -space-x-6 md:-space-x-8' : 'h-64 gap-1'}`}>
          {viewMode === 'normal' ? (
              // Normal View
              sortedHand.map((card: CardType) => (
                <Card
                  key={card.id}
                  card={card}
                  selected={selectedCardIds.includes(card.id)}
                  onClick={() => !isSpectator && toggleSelect(card.id)}
                  isHighlighted={highlightedCardIds.has(card.id)}
                />
              ))
          ) : (
              // Stacked Matrix View (Compact columns)
              getStackedMatrix().map((col, cIdx) => (
                  <div key={cIdx} className="relative w-16 h-64 flex-shrink-0">
                      {[Suit.Joker, Suit.Spades, Suit.Hearts, Suit.Clubs, Suit.Diamonds].map((suit, sIdx) => {
                          const cards = col.slots[suit];
                          if (!cards || cards.length === 0) return null;
                          
                          return cards.map((card, idx) => (
                              <div 
                                key={card.id} 
                                className={`absolute transition-transform ${straightFlushIds.has(card.id) ? 'ring-2 ring-yellow-400 shadow-[0_0_10px_rgba(250,204,21,0.5)] rounded' : ''}`}
                                style={{ 
                                    bottom: `${(4 - sIdx) * 30 + (idx * 5)}px`, 
                                    zIndex: sIdx * 10 + idx 
                                }}
                              >
                                <Card
                                    card={card}
                                    selected={selectedCardIds.includes(card.id)}
                                    onClick={() => !isSpectator && toggleSelect(card.id)}
                                    small
                                    isHighlighted={highlightedCardIds.has(card.id)}
                                />
                              </div>
                          ));
                      })}
                  </div>
              ))
          )}
        </div>
        <div className="relative">
          {/* My Chat Bubble */}
          {chatBubbles[mySeat] && (
            <div className="absolute -top-12 left-1/2 -translate-x-1/2 z-50 animate-bounce-in">
              <div className="relative bg-white text-gray-800 px-4 py-2 rounded-xl shadow-lg max-w-48 text-sm font-medium whitespace-pre-wrap">
                {chatBubbles[mySeat]}
                <div className="absolute -bottom-2 left-1/2 -translate-x-1/2 w-0 h-0 border-l-8 border-r-8 border-t-8 border-l-transparent border-r-transparent border-t-white"></div>
              </div>
            </div>
          )}
          <div className="text-white font-bold mt-2 flex items-center gap-2">
            <span>{me.player?.name} {isSpectator ? <span className="text-purple-400">(觀戰中)</span> : '(Me)'}</span>
            {watchersOf(mySeat).length > 0 && (
                <span className="text-purple-400 text-xs font-normal">👁 {watchersOf(mySeat).join('、')} 觀看中</span>
            )}
          </div>
        </div>
      </div>
      
      {gameState && gameState.phase === 'Score' && (
          <div className="absolute inset-0 bg-black/80 flex flex-col items-center justify-center text-white z-50">
              <h1 className="text-6xl font-bold mb-8 text-yellow-400">本局結束</h1>
              <div className="text-2xl mb-4">
                  獲勝順序: {gameState.winners.map(w => {
                      const p = roomState.players.find(pl => pl && pl.seatIndex === w);
                      return p ? p.name : `Seat ${w}`;
                  }).join(' → ')}
              </div>
              {gameState.teamLevels && (
                  <div className="text-xl text-gray-300 mb-4">
                      目前等級 - 隊伍0: {gameState.teamLevels[0]} | 隊伍1: {gameState.teamLevels[1]}
                  </div>
              )}
              <div className="text-lg text-yellow-300 animate-pulse">
                  ⏳ 3秒後自動開始下一局...
              </div>
              <div className="text-sm text-gray-400 mt-4">
                  (對局將持續到某隊打到A並連勝兩次)
              </div>
          </div>
      )}
      
      {/* Hand Type Selection Modal (for wild cards) */}
      {showHandSelector && possibleHands.length > 0 && gameState && (
          <div className="absolute inset-0 bg-black/80 flex items-center justify-center z-50">
              <div className="bg-[#252526] border border-[#333333] rounded-lg p-6 max-w-md shadow-2xl">
                  <h2 className="text-2xl font-bold text-[#9cdcfe] mb-4">選擇牌型</h2>
                  <p className="text-gray-400 mb-4">您的牌包含紅心{gameState.level}（萬能牌），可以組成以下牌型：</p>
                  <div className="flex flex-col gap-3">
                      {possibleHands.map((hand, idx) => (
                          <button
                              key={idx}
                              onClick={() => handleHandTypeSelect(hand)}
                              className="bg-[#3c3c3c] hover:bg-[#4c4c4c] text-white px-6 py-3 rounded-lg font-bold transition-colors text-left"
                          >
                              <div className="text-lg">{getHandDescription(hand, gameState.level)}</div>
                              <div className="text-sm text-gray-400 mt-1">
                                  {hand.type} - 值: {hand.value}
                                  {hand.bombCount && ` (${hand.bombCount}張炸彈)`}
                              </div>
                          </button>
                      ))}
                  </div>
                  <button
                      onClick={() => {
                          setShowHandSelector(false);
                          setPossibleHands([]);
                      }}
                      className="mt-4 w-full bg-gray-600 hover:bg-gray-700 text-white px-4 py-2 rounded-lg"
                  >
                      取消
                  </button>
              </div>
          </div>
      )}
      
      {/* Target Selection Modal for Skills */}
      {showTargetSelect && pendingSkill && (
          <TargetSelectModal
              skillType={pendingSkill.type}
              players={getPlayersForTargeting()}
              mySeat={mySeat}
              onSelect={handleTargetSelect}
              onCancel={handleTargetCancel}
          />
      )}
      
      {/* Game History Window */}
      {gameState && gameState.history && (
          <GameHistory
              history={gameState.history}
              currentRound={gameState.currentRound || 1}
              isOpen={showHistory}
              onClose={() => setShowHistory(false)}
          />
      )}
      
    </div>
  );
};
