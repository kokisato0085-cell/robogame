import type { Replay, Frame } from "./types";

// Canvas でリプレイを再生する観戦プレイヤー（FunctionalDesign §4 / S2-5）。
// 描画はリプレイの再生に徹し、ゲームロジックは持たない（sim と描画の分離）。

const CANVAS = 600;
const PAD = 20;
const ROBOT_R = 9;
const TICKS_PER_SEC = 30;
const HEAT_MAX = 100;
const COLORS = ["#2d7dd2", "#e8503a"] as const; // 0=青(挑戦者) / 1=赤(相手)

export interface Player {
  play(replay: Replay, labelA: string, labelB: string): void;
  restart(): void;
}

export function createPlayer(canvas: HTMLCanvasElement, statusEl: HTMLElement): Player {
  const ctx = canvas.getContext("2d")!;
  let rafId = 0;
  let last: { replay: Replay; labels: [string, string] } | null = null;

  // アリーナ（ミリ）→ Canvas 座標／長さ。
  const scaleOf = (replay: Replay) => (CANVAS - 2 * PAD) / replay.arenaW;
  const pos = (milli: number, s: number): number => PAD + milli * s;

  function drawBar(x: number, y: number, w: number, ratio: number, color: string): void {
    const r = Math.max(0, Math.min(1, ratio));
    ctx.fillStyle = "#eee";
    ctx.fillRect(x, y, w, 4);
    ctx.fillStyle = color;
    ctx.fillRect(x, y, w * r, 4);
  }

  function drawFrame(replay: Replay, frame: Frame): void {
    const s = scaleOf(replay);
    ctx.clearRect(0, 0, CANVAS, CANVAS);
    ctx.strokeStyle = "#ddd";
    ctx.strokeRect(PAD, PAD, CANVAS - 2 * PAD, CANVAS - 2 * PAD);

    // 遮蔽物。
    ctx.fillStyle = "#bbb";
    for (const o of replay.obstacles) {
      ctx.fillRect(pos(o.x, s), pos(o.y, s), o.w * s, o.h * s);
    }

    // 発射体。
    for (const p of frame.projectiles ?? []) {
      ctx.fillStyle = COLORS[p.source];
      ctx.beginPath();
      ctx.arc(pos(p.x, s), pos(p.y, s), 3, 0, Math.PI * 2);
      ctx.fill();
    }

    // ロボット本体・HP/シールド/熱バー。
    for (let i = 0; i < 2; i++) {
      const st = frame.robots[i];
      const cx = pos(st.x, s);
      const cy = pos(st.y, s);
      const maxHp = replay.builds[i].chassis.baseHp;
      const maxShield = replay.frames[0].robots[i].shield;

      if (st.hp <= 0) {
        ctx.strokeStyle = COLORS[i];
        ctx.beginPath();
        ctx.moveTo(cx - ROBOT_R, cy - ROBOT_R);
        ctx.lineTo(cx + ROBOT_R, cy + ROBOT_R);
        ctx.moveTo(cx + ROBOT_R, cy - ROBOT_R);
        ctx.lineTo(cx - ROBOT_R, cy + ROBOT_R);
        ctx.stroke();
      } else {
        ctx.fillStyle = COLORS[i];
        ctx.beginPath();
        ctx.arc(cx, cy, ROBOT_R, 0, Math.PI * 2);
        ctx.fill();
      }

      const bw = 26;
      const bx = cx - bw / 2;
      drawBar(bx, cy - ROBOT_R - 16, bw, st.hp / maxHp, COLORS[i]);
      if (maxShield > 0) drawBar(bx, cy - ROBOT_R - 11, bw, st.shield / maxShield, "#3aa0e8");
      drawBar(bx, cy - ROBOT_R - 6, bw, st.heat / HEAT_MAX, st.overheated ? "#d00" : "#e8a13a");
    }
  }

  function resultText(replay: Replay, labels: [string, string]): string {
    const who = replay.winner === -1 ? "引き分け" : `勝者: ${labels[replay.winner]}`;
    return `終了 — ${who}（${replay.reason}）`;
  }

  function run(replay: Replay, labels: [string, string]): void {
    cancelAnimationFrame(rafId);
    let startTime = 0;
    const step = (now: number): void => {
      if (startTime === 0) startTime = now;
      const lastIndex = replay.frames.length - 1;
      const idx = Math.min(Math.floor(((now - startTime) / 1000) * TICKS_PER_SEC), lastIndex);
      drawFrame(replay, replay.frames[idx]);
      if (idx < lastIndex) {
        statusEl.textContent = `${labels[0]} vs ${labels[1]} … tick ${replay.frames[idx].tick}`;
        rafId = requestAnimationFrame(step);
      } else {
        statusEl.textContent = resultText(replay, labels);
      }
    };
    rafId = requestAnimationFrame(step);
  }

  return {
    play(replay, labelA, labelB) {
      last = { replay, labels: [labelA, labelB] };
      run(replay, last.labels);
    },
    restart() {
      if (last) run(last.replay, last.labels);
    },
  };
}
