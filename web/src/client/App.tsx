import React, { useState, useEffect } from 'react';
import { useGame } from './useGame';
import { Lobby } from './components/Lobby';
import { GameTable } from './components/GameTable';
import { FakeIDE } from './components/FakeIDE';

// 畫面中央的全體公告：倒數 3 秒後淡出
function CenterAnnouncement({ text, onDone }: { text: string, onDone: () => void }) {
  const [remaining, setRemaining] = useState(3);
  const [fading, setFading] = useState(false);

  useEffect(() => {
    setRemaining(3);
    setFading(false);
    const iv = setInterval(() => setRemaining(r => Math.max(r - 1, 1)), 1000);
    const fadeT = setTimeout(() => setFading(true), 2500);
    const doneT = setTimeout(onDone, 3000);
    return () => { clearInterval(iv); clearTimeout(fadeT); clearTimeout(doneT); };
  }, [text]);

  return (
    <div className={`fixed inset-0 flex items-center justify-center z-[70] pointer-events-none transition-opacity duration-500 ${fading ? 'opacity-0' : 'opacity-100'}`}>
      <div className="bg-black/85 border-2 border-yellow-500 rounded-2xl px-8 py-6 text-center shadow-2xl">
        <div className="text-yellow-400 text-xl md:text-3xl font-bold mb-2">{text}</div>
        <div className="text-gray-400 text-2xl font-mono">{remaining}</div>
      </div>
    </div>
  );
}

function App() {
  const {
    inRoom,
    roomState,
    gameState,
    mySeat,
    isSpectator,
    error,
    announcement,
    clearAnnouncement,
    chatMessages,
    roomList,
    actions
  } = useGame();

  
  const [showFakeIDE, setShowFakeIDE] = useState(false);

  useEffect(() => {
      const handleKeyDown = (e: KeyboardEvent) => {
          // Toggle on 'i' key, but avoid input fields
          if (e.key.toLowerCase() === 'i') {
              const target = e.target as HTMLElement;
              if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA') return;
              setShowFakeIDE(prev => !prev);
          }
      };
      
      window.addEventListener('keydown', handleKeyDown);
      return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  return (
    <div className="bg-[#1e1e1e] min-h-screen text-gray-300">
      {showFakeIDE && <FakeIDE />}
      
      {error && (
        <div className="fixed top-4 left-1/2 -translate-x-1/2 bg-red-500 text-white px-6 py-2 rounded-full shadow-lg z-50 font-bold animate-pulse">
          {error}
        </div>
      )}

      {announcement && <CenterAnnouncement text={announcement} onDone={clearAnnouncement} />}

      {!inRoom ? (
        <Lobby onJoin={actions.joinRoom} roomList={roomList} onFetchRoomList={actions.fetchRoomList} />
      ) : (
          roomState && (
            <GameTable 
              gameState={gameState} 
              roomState={roomState}
              mySeat={mySeat}
              onPlay={actions.playHand}
              onPass={actions.passTurn}
              onReady={actions.setReady}
              onStart={actions.startGame}
              onTribute={actions.payTribute}
              onReturnTribute={actions.returnTribute}
              chatMessages={chatMessages}
              onSendChat={actions.sendChat}
              onSwitchSeat={actions.switchSeat}
              onSetGameMode={actions.setGameMode}
              onUseSkill={actions.useSkill}
              onForceEndGame={actions.forceEndGame}
              isSpectator={isSpectator}
              onWatchPlayer={actions.watchPlayer}
              onConfirmStart={actions.confirmStart}
              onSetStartLevel={actions.setStartLevel}
            />
        )
      )}
    </div>
  );
}

export default App;
