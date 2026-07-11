import { useState, useEffect, useCallback } from 'react';
import { socket } from './socket';
import { Card, GameMode, SkillCard, Hand, HistoryEntry } from '../shared/types';

export interface GameState {
  phase: string;
  level: number;
  currentTurn: number;
  hands: (Card[] | number)[]; 
  lastHand: { playerIndex: number, hand: any } | null;
  roundActions?: { [seat: number]: { type: 'play' | 'pass', cards?: Card[], hand?: any } };
  winners: number[];
  tributeState?: {
      pendingTributes: { from: number, to: number, card?: any }[];
      pendingReturns: { from: number, to: number, card?: any }[];
      completedTributes?: { from: number, to: number, card?: any }[];
  };
  confirmSeat?: number;
  teamLevels?: { [key: number]: number };
  activeTeam?: number;
  // Skill mode fields
  gameMode?: GameMode;
  mySkillCards?: SkillCard[];
  skipNextTurn?: boolean[];
  // New cards to highlight
  newCardIds?: string[];
  // Game history
  history?: HistoryEntry[];
  currentRound?: number;
  // 觀戰模式
  spectating?: boolean;
  watchSeat?: number;
}

export interface RoomState {
  roomId: string;
  players: ({ name: string, seatIndex: number, isReady: boolean } | null)[];
  gameMode?: GameMode;
  spectators?: string[];
}

export function useGame() {
  const [inRoom, setInRoom] = useState(false);
  const [roomState, setRoomState] = useState<RoomState | null>(null);
  const [gameState, setGameState] = useState<GameState | null>(null);
  const [mySeat, setMySeat] = useState<number>(-1);
  const [isSpectator, setIsSpectator] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [chatMessages, setChatMessages] = useState<{sender: string, text: string, time: string, seatIndex: number}[]>([]);
  const [roomList, setRoomList] = useState<Array<{
    id: string;
    playerCount: number;
    maxPlayers: number;
    inGame: boolean;
    gameMode: GameMode;
    hostName: string;
  }>>([]);

  useEffect(() => {
    socket.on('roomState', (state: any) => {
      setRoomState(state);
      setInRoom(true);
      const me = state.players.find((p: any) => p && p.id === socket.id);
      if (me) {
          setMySeat(me.seatIndex);
      }
    });

    socket.on('chatMessage', (msg: any) => {
        setChatMessages(prev => [...prev, msg]);
    });

    socket.on('gameState', (state: GameState) => {
      console.log(`[Client] Received gameState: currentTurn=${state.currentTurn}, phase=${state.phase}, mySeat will compare with ${state.currentTurn}`);
      setGameState(state);
      // 觀戰模式：以被觀看玩家的座位當作視角
      if (state.spectating && typeof state.watchSeat === 'number') {
        setIsSpectator(true);
        setMySeat(state.watchSeat);
      }
    });

    socket.on('spectatorMode', (data: { watchSeat: number }) => {
      console.log(`[Client] Spectator mode, watching seat ${data.watchSeat}`);
      setIsSpectator(true);
      setInRoom(true);
      setMySeat(data.watchSeat);
    });

    socket.on('error', (msg: string) => {
      setError(msg);
      setTimeout(() => setError(null), 3000);
    });
    
    socket.on('gameOver', (data: { winners: number[] }) => {
      console.log(`[Client] Game Over! Winners: ${data.winners.join(', ')}`);
      // The gameState should already be updated via broadcastGameState
      // This event is just a confirmation
      // Note: Game will auto-restart after 3 seconds (handled by Match)
    });
    
    socket.on('matchOver', (data: { winningTeam: number, winners: any[], finalLevels: any }) => {
      console.log(`[Client] MATCH OVER! Team ${data.winningTeam} wins!`);
      alert(`🎉 對局結束！\n獲勝隊伍：${data.winningTeam === 0 ? '0號和2號' : '1號和3號'}\n最終等級：${JSON.stringify(data.finalLevels)}`);
      setGameState(null); // Clear game state to return to lobby
    });

    socket.on('gameTerminated', () => {
        console.log('[Client] Game Terminated by Host');
        setGameState(null); // Clear game state to return to lobby
    });

    socket.on('roomList', (list: any[]) => {
        console.log('[Client] Received room list:', list);
        setRoomList(list);
    });

    return () => {
      socket.off('roomState');
      socket.off('gameState');
      socket.off('spectatorMode');
      socket.off('error');
      socket.off('gameOver');
      socket.off('gameTerminated');
      socket.off('roomList');
    };
  }, []);

  const joinRoom = (name: string, roomId: string) => {
    socket.emit('joinRoom', { playerName: name, roomId });
  };

  const setReady = () => {
    socket.emit('ready');
  };
  
  const startGame = () => {
      socket.emit('start');
  }

  const playHand = (cards: Card[], handType?: Hand) => {
    socket.emit('playHand', { cards, handType });
  };

  const passTurn = () => {
    socket.emit('pass');
  };
  
  const payTribute = (cards: Card[]) => {
      socket.emit('tribute', cards);
  }
  
  const returnTribute = (cards: Card[]) => {
      socket.emit('returnTribute', cards);
  }
  
  const sendChat = (msg: string) => {
      socket.emit('chatMessage', msg);
  }

  const switchSeat = (seatIdx: number) => {
      socket.emit('switchSeat', seatIdx);
  }
  
  const setGameMode = (mode: GameMode) => {
      socket.emit('setGameMode', mode);
  }
  
  const useSkill = (skillId: string, targetSeat?: number) => {
      socket.emit('useSkill', { skillId, targetSeat });
  }

  const forceEndGame = () => {
      socket.emit('forceEndGame');
  }

  const fetchRoomList = () => {
      socket.emit('getRoomList');
  }

  const watchPlayer = (seat: number) => {
      socket.emit('watchPlayer', seat);
  }

  const confirmStart = () => {
      socket.emit('confirmStart');
  }

  return {
    inRoom,
    roomState,
    gameState,
    mySeat,
    setMySeat,
    isSpectator,
    error,
    chatMessages,
    roomList,
    actions: { joinRoom, setReady, playHand, passTurn, startGame, payTribute, returnTribute, sendChat, switchSeat, setGameMode, useSkill, forceEndGame, fetchRoomList, watchPlayer, confirmStart }
  };
}
