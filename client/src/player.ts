import type { Replay, Frame, Build } from "./types";

// 弾の見た目を武器ごとに変える（色・サイズ）。
const PROJECTILE_STYLE: Record<string, { color: string; size: number }> = {
  "Starter Cannon": { color: "#e8a13a", size: 5 },
  "Rapid SMG": { color: "#f2d24b", size: 2 },
  "Rail Sniper": { color: "#e8503a", size: 3 },
  Laser: { color: "#19c2c2", size: 2 },
  Scatter: { color: "#b070ff", size: 2 },
};
function weaponNameOf(build: Build): string {
  return build.parts.find((p) => p.category === "weapon")?.name ?? "";
}

// Canvas でリプレイを再生する観戦プレイヤー（FunctionalDesign §4 / S2-5）。
// 描画はリプレイの再生に徹し、ゲームロジックは持たない（sim と描画の分離）。

const CANVAS = 760;
const PAD = 24;
const TICKS_PER_SEC = 30;
const HEAT_MAX = 100;
const FLOATER_LIFE = 0.6; // ダメージ数値の表示秒数
const SPRITE_HALF_MILLI = 55000; // = sim MinSep/2。画像範囲＝当たり(分離)半径。スプライトはマップ縮尺に連動
const COLORS = ["#2d7dd2", "#e8503a"] as const; // 0=青(挑戦者) / 1=赤(相手)

// ロボのスプライト（正面／後ろ向き）。読み込み前は円でフォールバック。
const frontImg = new Image();
frontImg.src = "/robot-front.png";
const backImg = new Image();
backImg.src = "/robot-back.png";

interface Floater {
  cx: number;
  cy: number;
  amount: number;
  guarded: boolean;
  born: number;
}

export interface Player {
  play(replay: Replay, labelA: string, labelB: string): void;
  restart(): void;
}

export function createPlayer(canvas: HTMLCanvasElement, statusEl: HTMLElement): Player {
  const ctx = canvas.getContext("2d")!;
  let rafId = 0;
  let last: { replay: Replay; labels: [string, string] } | null = null;

  const scaleOf = (replay: Replay) => (CANVAS - 2 * PAD) / replay.arenaW;
  const pos = (milli: number, s: number): number => PAD + milli * s;

  function drawBar(x: number, y: number, w: number, ratio: number, color: string): void {
    const r = Math.max(0, Math.min(1, ratio));
    ctx.fillStyle = "#eee";
    ctx.fillRect(x, y, w, 4);
    ctx.fillStyle = color;
    ctx.fillRect(x, y, w * r, 4);
  }

  // ガードを「分割リング」で表現。残り current/max のセグメントを描き、欠けていく様子を見せる。
  function drawGuardRing(cx: number, cy: number, current: number, max: number, active: boolean, baseR: number): void {
    if (max <= 0 || current <= 0) return;
    const r = baseR + 4;
    const seg = (Math.PI * 2) / max;
    const gap = seg * 0.25;
    ctx.strokeStyle = "#19c2c2";
    ctx.lineWidth = active ? 3.5 : 2;
    ctx.globalAlpha = active ? 1 : 0.45;
    for (let k = 0; k < current; k++) {
      const a0 = -Math.PI / 2 + k * seg + gap / 2;
      ctx.beginPath();
      ctx.arc(cx, cy, r, a0, a0 + seg - gap);
      ctx.stroke();
    }
    ctx.globalAlpha = 1;
    ctx.lineWidth = 1;
  }

  function drawFrame(replay: Replay, frame: Frame): void {
    const s = scaleOf(replay);
    ctx.clearRect(0, 0, CANVAS, CANVAS);
    ctx.strokeStyle = "#ddd";
    ctx.strokeRect(PAD, PAD, CANVAS - 2 * PAD, CANVAS - 2 * PAD);

    ctx.fillStyle = "#bbb";
    for (const o of replay.obstacles) ctx.fillRect(pos(o.x, s), pos(o.y, s), o.w * s, o.h * s);

    for (const p of frame.projectiles ?? []) {
      const style = PROJECTILE_STYLE[weaponNameOf(replay.builds[p.source])];
      ctx.fillStyle = style ? style.color : COLORS[p.source];
      ctx.beginPath();
      ctx.arc(pos(p.x, s), pos(p.y, s), style ? style.size : 3, 0, Math.PI * 2);
      ctx.fill();
    }

    for (let i = 0; i < 2; i++) {
      const st = frame.robots[i];
      const enemy = frame.robots[1 - i];
      const cx = pos(st.x, s);
      const cy = pos(st.y, s);
      const rpx = SPRITE_HALF_MILLI * s; // 画像半径(px)＝当たり半径に連動
      const maxHp = replay.builds[i].chassis.baseHp;
      const maxShield = replay.frames[0].robots[i].shield;

      if (st.hp <= 0) {
        ctx.strokeStyle = COLORS[i];
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.moveTo(cx - rpx, cy - rpx);
        ctx.lineTo(cx + rpx, cy + rpx);
        ctx.moveTo(cx + rpx, cy - rpx);
        ctx.lineTo(cx - rpx, cy + rpx);
        ctx.stroke();
        ctx.lineWidth = 1;
      } else {
        // 敵が下にいれば正面、上にいれば後ろ向き。読み込み前は円でフォールバック。
        const img = enemy.y > st.y ? frontImg : backImg;
        if (img.complete && img.naturalWidth > 0) {
          ctx.drawImage(img, cx - rpx, cy - rpx, rpx * 2, rpx * 2);
        } else {
          ctx.fillStyle = COLORS[i];
          ctx.beginPath();
          ctx.arc(cx, cy, rpx, 0, Math.PI * 2);
          ctx.fill();
        }
        drawGuardRing(cx, cy, st.guardCharges, replay.frames[0].robots[i].guardCharges, st.defending, rpx);
      }

      const bw = 30;
      const bx = cx - bw / 2;
      drawBar(bx, cy - rpx - 16, bw, st.hp / maxHp, COLORS[i]);
      if (maxShield > 0) drawBar(bx, cy - rpx - 11, bw, st.shield / maxShield, "#3aa0e8");
      drawBar(bx, cy - rpx - 6, bw, st.heat / HEAT_MAX, st.overheated ? "#d00" : "#e8a13a");
    }
  }

  function drawFloaters(floaters: Floater[], now: number): void {
    ctx.textAlign = "center";
    ctx.font = "bold 13px sans-serif";
    for (const f of floaters) {
      const age = (now - f.born) / 1000;
      ctx.globalAlpha = Math.max(0, 1 - age / FLOATER_LIFE);
      ctx.fillStyle = f.guarded ? "#19c2c2" : "#d11";
      ctx.fillText(`-${f.amount}${f.guarded ? " G" : ""}`, f.cx, f.cy - 18 - age * 26);
    }
    ctx.globalAlpha = 1;
  }

  function resultText(replay: Replay, labels: [string, string]): string {
    const who = replay.winner === -1 ? "引き分け" : `勝者: ${labels[replay.winner]}`;
    return `終了 — ${who}（${replay.reason}）`;
  }

  function run(replay: Replay, labels: [string, string]): void {
    cancelAnimationFrame(rafId);
    const s = scaleOf(replay);
    const lastIndex = replay.frames.length - 1;
    let startTime = 0;
    let processed = 0; // 既にダメージ数値を生成したフレーム index
    const floaters: Floater[] = [];

    const step = (now: number): void => {
      if (startTime === 0) startTime = now;
      const idx = Math.min(Math.floor(((now - startTime) / 1000) * TICKS_PER_SEC), lastIndex);

      // 新しく通過したフレームの attack イベントからダメージ数値を生成。
      for (let fi = processed + 1; fi <= idx; fi++) {
        const frame = replay.frames[fi];
        const atks = (frame.events ?? []).filter((e) => e.type === "attack" && e.amount > 0);
        // 同時ヒット（拡散など）は横に並べて個別に表示する。
        atks.forEach((ev, k) => {
          const t = frame.robots[ev.target];
          const off = (k - (atks.length - 1) / 2) * 14;
          floaters.push({ cx: pos(t.x, s) + off, cy: pos(t.y, s), amount: ev.amount, guarded: ev.guarded, born: now });
        });
      }
      processed = idx;
      while (floaters.length && (now - floaters[0].born) / 1000 > FLOATER_LIFE) floaters.shift();

      drawFrame(replay, replay.frames[idx]);
      drawFloaters(floaters, now);

      if (idx < lastIndex || floaters.length) {
        statusEl.textContent =
          idx < lastIndex ? `${labels[0]} vs ${labels[1]} … tick ${replay.frames[idx].tick}` : resultText(replay, labels);
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
