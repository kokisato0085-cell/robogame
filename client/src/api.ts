import type { Build, Robot, Battle } from "./types";

// ページと同一オリジンの "/api" を叩く（開発時は Vite プロキシが :8080 へ中継）。
const API_BASE = "";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(API_BASE + path, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    let msg = `HTTP ${res.status}`;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) msg = body.error;
    } catch {
      // JSON でないエラー応答はステータスのみ。
    }
    throw new Error(msg);
  }
  return (await res.json()) as T;
}

export function createRobot(owner: string, name: string, build: Build): Promise<Robot> {
  return request<Robot>("/api/robots", {
    method: "POST",
    body: JSON.stringify({ owner, name, build }),
  });
}

export function listRobots(): Promise<Robot[]> {
  return request<Robot[]>("/api/robots");
}

export function challenge(challengerId: string, opponentId: string): Promise<Battle> {
  return request<Battle>("/api/challenge", {
    method: "POST",
    body: JSON.stringify({ challenger_id: challengerId, opponent_id: opponentId }),
  });
}

export function inbox(owner: string): Promise<Battle[]> {
  return request<Battle[]>(`/api/inbox?owner=${encodeURIComponent(owner)}`);
}
