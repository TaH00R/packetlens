
"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { isSnapshot, mergeHistory } from "@/lib/monitoring";

import type {
  ConnectionStatus,
  MonitoringSnapshot,
} from "@/types/metrics";

const WS_URL =
  process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8080/api/ws";

const INITIAL_RECONNECT_DELAY = 1000;
const MAX_RECONNECT_DELAY = 15000;

export function usePacketLensSocket() {
  const [status, setStatus] =
    useState<ConnectionStatus>("connecting");
  const [connectionVersion, setConnectionVersion] = useState(0);

  const [latest, setLatest] =
    useState<MonitoringSnapshot | null>(null);

  const [history, setHistory] =
    useState<MonitoringSnapshot[]>([]);

  const socketRef = useRef<WebSocket | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const attemptsRef = useRef(0);
  const mountedRef = useRef(false);

  const connect = useCallback(function connectSocket() {
    if (!mountedRef.current) return;

    if (
      socketRef.current &&
      (socketRef.current.readyState === WebSocket.OPEN ||
        socketRef.current.readyState === WebSocket.CONNECTING)
    ) {
      return;
    }

    setStatus(
      attemptsRef.current === 0 ? "connecting" : "reconnecting"
    );

    let socket: WebSocket;
    try {
      socket = new WebSocket(WS_URL);
    } catch (error) {
      console.error("Invalid PacketLens WebSocket URL:", error);
      setStatus("disconnected");
      return;
    }
    socketRef.current = socket;

    socket.onopen = () => {
      if (socketRef.current !== socket) return;

      attemptsRef.current = 0;
      setStatus("connected");
      setConnectionVersion((version) => version + 1);
    };

    socket.onmessage = (event) => {
      if (socketRef.current !== socket) return;

      try {
        const snapshot: unknown = JSON.parse(event.data);

        if (!isSnapshot(snapshot)) {
          throw new Error("Invalid snapshot structure");
        }

        setLatest(snapshot);

        setHistory((previous) =>
          mergeHistory(previous, [snapshot])
        );
      } catch (error) {
        console.error("Invalid PacketLens snapshot:", error);
      }
    };

    socket.onerror = () => {
      if (socketRef.current !== socket) return;
      console.error("PacketLens WebSocket error");
    };

    socket.onclose = () => {
      if (socketRef.current !== socket) return;

      socketRef.current = null;

      if (!mountedRef.current) return;

      attemptsRef.current += 1;
      setStatus("reconnecting");

      const delay = Math.min(
        INITIAL_RECONNECT_DELAY * 2 ** (attemptsRef.current - 1),
        MAX_RECONNECT_DELAY
      );

      timerRef.current = setTimeout(connectSocket, delay);
    };
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    connect();

    return () => {
      mountedRef.current = false;

      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }

      const socket = socketRef.current;
      socketRef.current = null;
      socket?.close();
    };
  }, [connect]);

  const reconnect = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }

    attemptsRef.current = 0;

    const socket = socketRef.current;
    socketRef.current = null;
    socket?.close();

    setStatus("connecting");
    connect();
  }, [connect]);

  return {
    status,
    latest,
    history,
    reconnect,
    connectionVersion,
  };
}
