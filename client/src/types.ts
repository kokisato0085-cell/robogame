// サーバー（Go の sim パッケージ）の JSON と 1:1 対応する型。
// 将来は型の二重管理を避けるため Protocol Buffers での自動生成を検討する。

export interface Chassis {
  name: string;
  baseHp: number;
  weight: number;
  slots: number;
  batteryCapacity: number;
  baseSpeed: number;
  traits: string[];
}

export interface WeaponSpec {
  power: number;
  range: number;
  cooldown: number;
  heatPerShot: number;
  pattern: string;
}

export interface ArmorSpec {
  shield: number;
}

export interface MovementSpec {
  dashDistance: number;
  dashCooldown: number;
  dashPowerCost: number;
}

export type PartCategory = "weapon" | "armor" | "movement" | "power";

export interface Part {
  name: string;
  category: PartCategory;
  weight: number;
  powerCost: number;
  slotCost: number;
  weapon?: WeaponSpec;
  armor?: ArmorSpec;
  movement?: MovementSpec;
}

export type ConditionType =
  | "enemyDistance"
  | "selfHp"
  | "selfShield"
  | "selfHeat"
  | "selfBattery"
  | "dashReady";

export type ConditionOp =
  | "inRange"
  | "outRange"
  | "lt"
  | "gt"
  | "nearOverheat"
  | "exists"
  | "none";

export interface Condition {
  type: ConditionType;
  op: ConditionOp;
  value: number;
}

export interface Rule {
  conditions: Condition[];
  action: string;
}

export interface Ruleset {
  movement: Rule[];
  weapon: Rule[];
  special: Rule[];
}

export interface Build {
  chassis: Chassis;
  parts: Part[];
  ruleset: Ruleset;
}

export interface Robot {
  id: string;
  owner: string;
  name: string;
  build: Build;
}

// ---- 観戦（リプレイ）系：T4 で使用 ----

export interface RobotState {
  x: number;
  y: number;
  hp: number;
  shield: number;
  heat: number;
  battery: number;
  dashCd: number;
  overheated: boolean;
}

export interface BattleEvent {
  type: string; // "attack" / "overheat" / "destroyed"
  source: number;
  target: number;
  amount: number;
}

export interface Frame {
  tick: number;
  robots: [RobotState, RobotState];
  events: BattleEvent[] | null;
}

export interface Replay {
  builds: [Build, Build];
  frames: Frame[];
  winner: number;
  reason: string;
}

export interface Battle {
  id: string;
  challenger: Robot;
  opponent: Robot;
  winner: number;
  reason: string;
  replay: Replay;
}
