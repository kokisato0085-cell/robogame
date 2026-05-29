import type { Chassis, Part } from "./types";

// 段階1 のカタログ（BasicDesign §5）。素体1種・武器1種。
// 値はサーバーの基本設計書と一致させる。

export const BALANCED: Chassis = {
  name: "Balanced",
  baseHp: 100,
  weight: 20,
  slots: 4,
  batteryCapacity: 100,
  baseSpeed: 12,
  traits: [],
};

export const STARTER_CANNON: Part = {
  name: "Starter Cannon",
  category: "weapon",
  weight: 8,
  powerCost: 6,
  slotCost: 1,
  weapon: { power: 12, range: 250, cooldown: 8, heatPerShot: 18, pattern: "single" },
};
