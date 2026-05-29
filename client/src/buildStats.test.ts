import { describe, it, expect } from "vitest";
import { deriveStats, checkBuild } from "./buildStats";
import { BALANCED, STARTER_CANNON } from "./data";
import type { Build } from "./types";

function emptyRuleset() {
  return { movement: [], weapon: [], special: [] };
}

describe("deriveStats", () => {
  it("素体のみ：重量・速度・電力", () => {
    const b: Build = { chassis: BALANCED, parts: [], ruleset: emptyRuleset() };
    const s = deriveStats(b);
    expect(s.totalWeight).toBe(20);
    expect(s.slotsUsed).toBe(0);
    expect(s.effSpeed).toBe(12); // 重量超過なし
    expect(s.availPower).toBe(100);
  });

  it("武器装着：重量増→速度低下", () => {
    const b: Build = { chassis: BALANCED, parts: [STARTER_CANNON], ruleset: emptyRuleset() };
    const s = deriveStats(b);
    expect(s.totalWeight).toBe(28); // 20 + 8
    expect(s.slotsUsed).toBe(1);
    // excess=8 → effSpeedMilli = 12000 - 200*8 = 10400 → 10
    expect(s.effSpeed).toBe(10);
  });
});

describe("checkBuild", () => {
  it("正常なビルドは null", () => {
    const b: Build = { chassis: BALANCED, parts: [STARTER_CANNON], ruleset: emptyRuleset() };
    expect(checkBuild(b)).toBeNull();
  });

  it("スロット超過を検出", () => {
    const heavy = { ...STARTER_CANNON, slotCost: 99 };
    const b: Build = { chassis: BALANCED, parts: [heavy], ruleset: emptyRuleset() };
    expect(checkBuild(b)).toMatch(/スロット超過/);
  });
});
