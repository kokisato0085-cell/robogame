import type { Build } from "./types";

// 制約表示用の実効ステータス計算（サーバー sim.derive と同じ式）。
// サーバーが権威だが、UX のため画面でも同じ計算を行いリアルタイム表示する。
export const POSITION_SCALE = 1000;
export const SPEED_MIN = 3;
export const WEIGHT_SPEED_COEFF = 200; // 0.2 × POSITION_SCALE
export const WEIGHT_BATT_DIV = 10;

export interface BuildStats {
  totalWeight: number;
  slotsUsed: number;
  effSpeed: number; // 表示units/tick
  availPower: number;
  shield: number;
}

export function deriveStats(b: Build): BuildStats {
  let totalWeight = b.chassis.weight;
  let slotsUsed = 0;
  let shield = 0;
  for (const p of b.parts) {
    totalWeight += p.weight;
    slotsUsed += p.slotCost;
    if (p.armor) shield += p.armor.shield;
  }
  const excess = Math.max(0, totalWeight - b.chassis.weight);
  const effSpeedMilli = Math.max(
    SPEED_MIN * POSITION_SCALE,
    b.chassis.baseSpeed * POSITION_SCALE - WEIGHT_SPEED_COEFF * excess,
  );
  const availPower = b.chassis.batteryCapacity - Math.floor(excess / WEIGHT_BATT_DIV);
  return {
    totalWeight,
    slotsUsed,
    effSpeed: Math.floor(effSpeedMilli / POSITION_SCALE),
    availPower,
    shield,
  };
}

// 登録前のクライアント側チェック（サーバー検証の先取り。エラー文 or null）。
export function checkBuild(b: Build): string | null {
  const s = deriveStats(b);
  if (s.slotsUsed > b.chassis.slots) {
    return `スロット超過（使用 ${s.slotsUsed} > 上限 ${b.chassis.slots}）`;
  }
  for (const p of b.parts) {
    if (p.weapon && p.powerCost > s.availPower) {
      return `電力不足（武器消費 ${p.powerCost} > 利用可能 ${s.availPower}）`;
    }
  }
  return null;
}
