// WebSocket hook — connects to the Go backend and dispatches real-time events.
// Uses a stable ref for the event handler so the connection doesn't
// re-establish when the handler identity changes (e.g. on chat switch).

import { useEffect, useRef, useCallback } from "react";

// Use relative URL — Vite proxy forwards /ws to the Go backend.
const WS_URL = `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}/ws`;

export interface WSEvent {
  type: string;
  data: unknown;
}

type EventHandler = (event: WSEvent) => void;

export function useWebSocket(onEvent: EventHandler) {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  // Store the latest handler in a ref so the WebSocket connection is
  // stable even when the handler changes (avoids reconnect on every
  // activeChat switch).
  const onEventRef = useRef<EventHandler>(onEvent);
  onEventRef.current = onEvent;

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    const ws = new WebSocket(WS_URL);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log("[nod] WebSocket connected");
    };

    ws.onmessage = (msg) => {
      try {
        const evt: WSEvent = JSON.parse(msg.data);
        onEventRef.current(evt);
      } catch (parseErr: unknown) {
        void parseErr;
        console.warn("[nod] Failed to parse WS message:", msg.data);
      }
    };

    ws.onclose = () => {
      console.log("[nod] WebSocket disconnected, reconnecting in 2s...");
      reconnectTimer.current = setTimeout(connect, 2000);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, []); // stable — no dependencies, uses onEventRef

  useEffect(() => {
    connect();
    return () => {
      clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);
}
