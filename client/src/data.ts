import type { Chassis, Part } from "./types";

// 段階3 のカタログ（FunctionalDesign S3-0 / BasicDesign §9）。
// 値はサーバーの設計と一致させる。

export const CHASSIS: Chassis[] = [
  { name: "Balanced", baseHp: 100, weight: 20, slots: 4, batteryCapacity: 100, baseSpeed: 12, traits: [] },
  { name: "Fortress", baseHp: 160, weight: 35, slots: 5, batteryCapacity: 120, baseSpeed: 9, traits: ["高HP・鈍足"] },
  { name: "Runner", baseHp: 70, weight: 12, slots: 3, batteryCapacity: 90, baseSpeed: 16, traits: ["高速・低HP"] },
];

export const PARTS: Part[] = [
  {
    name: "Starter Cannon",
    category: "weapon",
    weight: 8,
    powerCost: 6,
    slotCost: 1,
    weapon: { power: 14, range: 350, cooldown: 30, heatPerShot: 16, projectileSpeed: 36, pellets: 1, spreadDeg: 0, pattern: "single" },
  },
  {
    name: "Rapid SMG",
    category: "weapon",
    weight: 7,
    powerCost: 5,
    slotCost: 1,
    weapon: { power: 4, range: 200, cooldown: 4, heatPerShot: 5, projectileSpeed: 48, pellets: 1, spreadDeg: 0, pattern: "single" },
  },
  {
    name: "Rail Sniper",
    category: "weapon",
    weight: 14,
    powerCost: 12,
    slotCost: 2,
    weapon: { power: 40, range: 600, cooldown: 60, heatPerShot: 45, projectileSpeed: 100, pellets: 1, spreadDeg: 0, pattern: "single" },
  },
  {
    name: "Laser",
    category: "weapon",
    weight: 9,
    powerCost: 10,
    slotCost: 1,
    weapon: { power: 9, range: 380, cooldown: 6, heatPerShot: 26, projectileSpeed: 130, pellets: 1, spreadDeg: 0, pattern: "single" },
  },
  {
    name: "Scatter",
    category: "weapon",
    weight: 10,
    powerCost: 8,
    slotCost: 2,
    weapon: { power: 6, range: 170, cooldown: 26, heatPerShot: 22, projectileSpeed: 32, pellets: 3, spreadDeg: 7, pattern: "spread" },
  },
  { name: "Light Plating", category: "armor", weight: 6, powerCost: 0, slotCost: 1, armor: { shield: 30 } },
  { name: "Heavy Plating", category: "armor", weight: 16, powerCost: 0, slotCost: 2, armor: { shield: 70 } },
  {
    name: "Booster",
    category: "movement",
    weight: 5,
    powerCost: 0,
    slotCost: 1,
    movement: { dashDistance: 120, dashCooldown: 30, dashPowerCost: 20 },
  },
  { name: "Guard Unit", category: "defense", weight: 10, powerCost: 15, slotCost: 1, defense: { charges: 5 } },
];
