// Native-WebSocket replacement for socket.io-client.
// Exposes the same tiny API surface the app uses: on / off / emit / id.
// Wire format matches the Go server: {"event": string, "data": any}

type Handler = (data: any) => void;

class WSocket {
  id: string = '';
  private ws: WebSocket | null = null;
  private handlers = new Map<string, Set<Handler>>();
  private queue: string[] = [];
  private reconnectDelay = 500;
  private joinPayload: string | null = null; // 最後一次 joinRoom，重連後自動重新加入
  private hadConnected = false;

  constructor() {
    this.connect();

    // 手機從背景回到前景時，連線常已被系統悄悄切斷：主動檢查並立即重連
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'visible' && (!this.ws || this.ws.readyState !== WebSocket.OPEN)) {
        try { this.ws?.close(); } catch { /* noop */ }
        if (!this.ws) this.connect();
      }
    });
  }

  private url(): string {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    return `${proto}://${location.host}/ws`;
  }

  private connect() {
    const ws = new WebSocket(this.url());
    this.ws = ws;

    ws.onopen = () => {
      this.reconnectDelay = 500;
      // 斷線重連：自動重新加入原房間（伺服器會以同名接管原座位）
      if (this.hadConnected && this.joinPayload) {
        ws.send(this.joinPayload);
      }
      this.hadConnected = true;
      // Flush queued messages
      for (const msg of this.queue) ws.send(msg);
      this.queue = [];
    };

    ws.onmessage = (ev) => {
      let msg: { event: string; data?: any };
      try {
        msg = JSON.parse(ev.data);
      } catch {
        return;
      }
      if (msg.event === 'connected') {
        this.id = msg.data?.id ?? '';
      }
      const set = this.handlers.get(msg.event);
      if (set) {
        for (const fn of set) fn(msg.data);
      }
    };

    ws.onclose = () => {
      this.ws = null;
      // Auto-reconnect with capped backoff
      setTimeout(() => this.connect(), this.reconnectDelay);
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, 5000);
    };

    ws.onerror = () => {
      ws.close();
    };
  }

  on(event: string, handler: Handler) {
    if (!this.handlers.has(event)) this.handlers.set(event, new Set());
    this.handlers.get(event)!.add(handler);
  }

  off(event: string, handler?: Handler) {
    if (!handler) {
      this.handlers.delete(event);
      return;
    }
    this.handlers.get(event)?.delete(handler);
  }

  emit(event: string, data?: any) {
    const payload = JSON.stringify(data === undefined ? { event } : { event, data });
    if (event === 'joinRoom') {
      this.joinPayload = payload;
    }
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(payload);
    } else {
      this.queue.push(payload);
    }
  }
}

export const socket = new WSocket();
