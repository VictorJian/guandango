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

  constructor() {
    this.connect();
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
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(payload);
    } else {
      this.queue.push(payload);
    }
  }
}

export const socket = new WSocket();
