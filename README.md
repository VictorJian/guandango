# GuanDanGo — 摜蛋 4 人連線遊戲（Go 版）

從 [GuanDanInOffice](../GuanDanInOffice)（Node.js + Socket.IO）移植的 Go 後端版本。
前端沿用原本的 React UI，連線層改為原生 WebSocket。

## 目前範圍

- ✅ 普通模式完整流程：發牌、出牌、牌型判斷（含逢人配）、進貢/還貢、抗貢、接風、升級、整場打到 A
- ✅ 房間系統：建房、入座、換座、聊天、斷線重連、房主強制結束
- ❌ 技能模式：暫不提供（依需求移除）
- ❌ Bot 電腦玩家：暫不提供（開局需 4 位真人）

## 專案結構

```
cmd/server/          # 進入點（HTTP + WebSocket + 靜態檔案）
internal/game/       # 純遊戲邏輯：牌、牌型規則（可獨立測試）
internal/server/     # 房間 / 對局 / 單局引擎 / WebSocket 客戶端封裝
web/                 # React 前端（Vite + Tailwind）
```

## 開發

```bash
# 後端（port 3000）
go run ./cmd/server

# 前端開發模式（port 5173，/ws 會 proxy 到 3000）
cd web && npm install && npm run dev

# 測試（含完整對局模擬與 WebSocket 整合測試）
go test ./... -race
```

## 部署

```bash
cd web && npm run build     # 產出 web/dist
go build -o guandan ./cmd/server
PORT=3000 ./guandan          # STATIC_DIR 可覆寫靜態檔案路徑（預設 web/dist）
```

## WebSocket 協定

訊息格式：`{"event": string, "data": any}`，事件名稱與原 Socket.IO 版本一致
（joinRoom / ready / start / playHand / pass / tribute / returnTribute / chatMessage / switchSeat / forceEndGame / getRoomList，
伺服器回送 connected / roomState / gameState / chatMessage / error / gameOver / matchOver / matchStarted / gameTerminated / historyUpdate / roomList）。
