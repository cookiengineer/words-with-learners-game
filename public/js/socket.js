import { uniqueID } from "./ui.js";

export class GameSocket extends EventTarget {
  constructor() {
    super();
    this.socket = null;
    this.ready = null;
    this.pending = new Map();
    this.manualClose = false;
    this.reconnectDelay = 500;
    this.reconnectTimer = null;
  }

  connect() {
    if (this.socket && (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING)) {
      return this.ready || Promise.resolve();
    }
    clearTimeout(this.reconnectTimer);
    this.manualClose = false;
    const protocol = location.protocol === "https:" ? "wss:" : "ws:";
    this.ready = new Promise((resolve, reject) => {
      const socket = new WebSocket(`${protocol}//${location.host}/ws`);
      this.socket = socket;
      let settled = false;
      socket.addEventListener("open", () => {
        settled = true;
        this.reconnectDelay = 500;
        this.dispatchEvent(new CustomEvent("connected"));
        resolve();
      });
      socket.addEventListener("message", (event) => this.handleMessage(event.data));
      socket.addEventListener("error", () => {
        if (!settled) {
          settled = true;
          reject(new Error("Could not connect to the game server"));
        }
      });
      socket.addEventListener("close", () => {
        this.ready = null;
        for (const [requestId, entry] of this.pending) {
          clearTimeout(entry.timer);
          entry.reject(new Error("Connection closed"));
          this.pending.delete(requestId);
        }
        this.dispatchEvent(new CustomEvent("disconnected"));
        if (!this.manualClose) {
          this.reconnectTimer = setTimeout(() => {
            this.connect().catch(() => {});
          }, this.reconnectDelay);
          this.reconnectDelay = Math.min(this.reconnectDelay * 2, 8000);
        }
      });
    });
    return this.ready;
  }

  handleMessage(raw) {
    let message;
    try {
      message = JSON.parse(raw);
    } catch {
      return;
    }
    const detail = { type: message.type, payload: message.payload, requestId: message.requestId || "" };
    this.dispatchEvent(new CustomEvent("message", { detail }));
    this.dispatchEvent(new CustomEvent(message.type, { detail }));
    if (message.requestId && this.pending.has(message.requestId)) {
      const entry = this.pending.get(message.requestId);
      this.pending.delete(message.requestId);
      clearTimeout(entry.timer);
      if (message.type === "error") {
        const error = new Error(message.payload?.message || "Request failed");
        error.code = message.payload?.code || "request_failed";
        entry.reject(error);
      } else {
        entry.resolve(message.payload);
      }
    }
  }

  async request(type, payload = {}) {
    await this.connect();
    const requestId = uniqueID();
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(requestId);
        reject(new Error("The server did not respond in time"));
      }, 20000);
      this.pending.set(requestId, { resolve, reject, timer });
      this.socket.send(JSON.stringify({ type, requestId, payload }));
    });
  }

  on(type, handler) {
    const listener = (event) => handler(event.detail);
    this.addEventListener(type, listener);
    return () => this.removeEventListener(type, listener);
  }

  close() {
    this.manualClose = true;
    clearTimeout(this.reconnectTimer);
    if (this.socket) {
      this.socket.close();
    }
    this.socket = null;
    this.ready = null;
  }
}

export const socket = new GameSocket();
