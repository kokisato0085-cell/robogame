import { createRobot, listRobots, challenge, inbox } from "./api";
import { createPlayer } from "./player";
import { deriveStats, checkBuild } from "./buildStats";
import { CHASSIS, PARTS } from "./data";
import type { Battle, Build, Chassis, Condition, ConditionOp, ConditionType, Part, Robot, Ruleset } from "./types";

function el<T extends HTMLElement>(id: string): T {
  return document.getElementById(id) as T;
}

type Channel = "movement" | "weapon" | "special";

const ACTIONS: Record<Channel, string[]> = {
  movement: ["approach", "retreat", "keepDistance", "dashApproach", "dashRetreat", "stop"],
  weapon: ["fire", "hold"],
  special: ["defend"],
};
const CONDITION_TYPES: ConditionType[] = [
  "enemyDistance", "selfHp", "selfShield", "selfHeat", "selfBattery", "dashReady", "lineOfSight",
];
const CONDITION_OPS: Record<ConditionType, ConditionOp[]> = {
  enemyDistance: ["inRange", "outRange", "lt", "gt"],
  selfHp: ["lt", "gt"],
  selfShield: ["exists", "none"],
  selfHeat: ["lt", "gt", "nearOverheat"],
  selfBattery: ["lt", "gt"],
  dashReady: ["exists"],
  lineOfSight: ["clear", "blocked"],
};
const opNeedsValue = (op: ConditionOp): boolean => op === "lt" || op === "gt";

// 編集中のルールセットと、各チャネルの「追加中の条件（AND）」。
const ruleset: Ruleset = { movement: [], weapon: [], special: [] };
const pending: Record<Channel, Condition[]> = { movement: [], weapon: [], special: [] };

// 装着中のパーツ名。
const equipped = new Set<string>();

function selectedChassis(): Chassis {
  const name = el<HTMLSelectElement>("sel-chassis").value;
  return CHASSIS.find((c) => c.name === name) ?? CHASSIS[0];
}

function currentBuild(): Build {
  return { chassis: selectedChassis(), parts: PARTS.filter((p) => equipped.has(p.name)), ruleset };
}

function refreshConstraints(): void {
  const b = currentBuild();
  const c = b.chassis;
  const s = deriveStats(b);
  el("chassis-info").textContent =
    `HP ${c.baseHp} / スロット ${c.slots} / 電力 ${c.batteryCapacity} / 速度 ${c.baseSpeed}` +
    (c.traits.length ? `（${c.traits.join("・")}）` : "");
  el("constraints").textContent =
    `スロット ${s.slotsUsed}/${c.slots} ・ 総重量 ${s.totalWeight} ・ 実効速度 ${s.effSpeed} ・ 利用可能電力 ${s.availPower} ・ シールド ${s.shield}`;
  el("build-warn").textContent = checkBuild(b) ?? "";
}

function describePart(p: Part): string {
  const base = `${p.name}（重${p.weight} 電${p.powerCost} ス${p.slotCost}`;
  if (p.weapon) return `${base} ｜ 攻${p.weapon.power} 射${p.weapon.range} 弾速${p.weapon.projectileSpeed}）`;
  if (p.armor) return `${base} ｜ シールド${p.armor.shield}）`;
  if (p.movement) return `${base} ｜ ダッシュ${p.movement.dashDistance} CD${p.movement.dashCooldown}）`;
  if (p.defense) return `${base} ｜ 防御 耐久${p.defense.charges}回）`;
  return `${base}）`;
}

function renderChassisSelect(): void {
  const sel = el<HTMLSelectElement>("sel-chassis");
  for (const c of CHASSIS) sel.add(new Option(c.name, c.name));
  sel.addEventListener("change", refreshConstraints);
}

function renderPartsCatalog(): void {
  const root = el("parts-catalog");
  root.innerHTML = "";
  for (const p of PARTS) {
    const label = document.createElement("label");
    label.style.display = "block";
    const cb = document.createElement("input");
    cb.type = "checkbox";
    cb.checked = equipped.has(p.name);
    cb.addEventListener("change", () => {
      if (cb.checked) equipped.add(p.name);
      else equipped.delete(p.name);
      refreshConstraints();
    });
    label.append(cb, ` ${describePart(p)}`);
    root.appendChild(label);
  }
}

// ---- DOM 生成ヘルパー ----

function makeSelect(options: string[]): HTMLSelectElement {
  const s = document.createElement("select");
  for (const o of options) s.add(new Option(o, o));
  return s;
}

function makeButton(label: string, onClick: () => void): HTMLButtonElement {
  const b = document.createElement("button");
  b.type = "button";
  b.textContent = label;
  b.addEventListener("click", onClick);
  return b;
}

function formatCondition(c: Condition): string {
  return opNeedsValue(c.op) ? `${c.type} ${c.op} ${c.value}` : `${c.type} ${c.op}`;
}

function formatRule(action: string, conditions: Condition[]): string {
  if (conditions.length === 0) return `常に → ${action}`;
  return `もし ${conditions.map(formatCondition).join(" かつ ")} → ${action}`;
}

// ---- チャネル描画 ----

function moveRule(channel: Channel, idx: number, dir: number): void {
  const rules = ruleset[channel];
  const j = idx + dir;
  if (j < 0 || j >= rules.length) return;
  [rules[idx], rules[j]] = [rules[j], rules[idx]];
  renderChannel(channel);
}

function renderChannel(channel: Channel): void {
  const root = el(`ch-${channel}`);
  root.innerHTML = "";

  const list = document.createElement("ul");
  const rules = ruleset[channel];
  if (rules.length === 0) {
    const li = document.createElement("li");
    li.textContent = "（ルールなし → デフォルト動作）";
    list.appendChild(li);
  } else {
    rules.forEach((rule, idx) => {
      const li = document.createElement("li");
      li.append(
        `${formatRule(rule.action, rule.conditions)} `,
        makeButton("↑", () => moveRule(channel, idx, -1)),
        makeButton("↓", () => moveRule(channel, idx, 1)),
        makeButton("✕", () => {
          rules.splice(idx, 1);
          renderChannel(channel);
        }),
      );
      list.appendChild(li);
    });
  }
  root.appendChild(list);
  root.appendChild(buildAddForm(channel));
}

function buildAddForm(channel: Channel): HTMLElement {
  const wrap = document.createElement("div");
  wrap.className = "addform";

  const typeSel = makeSelect(CONDITION_TYPES);
  const opSel = makeSelect(CONDITION_OPS[typeSel.value as ConditionType]);
  const valInput = document.createElement("input");
  valInput.type = "number";
  valInput.value = "50";
  valInput.style.width = "70px";

  const syncValVisibility = (): void => {
    valInput.style.display = opNeedsValue(opSel.value as ConditionOp) ? "" : "none";
  };
  const syncOps = (): void => {
    opSel.innerHTML = "";
    for (const o of CONDITION_OPS[typeSel.value as ConditionType]) opSel.add(new Option(o, o));
    syncValVisibility();
  };
  typeSel.addEventListener("change", syncOps);
  opSel.addEventListener("change", syncValVisibility);
  syncValVisibility();

  // 追加中の条件は、個別に ✕ で取り消せるようにする（誤操作の救済）。
  const pendingLabel = document.createElement("span");
  pendingLabel.append(" 追加中: ");
  if (pending[channel].length === 0) {
    pendingLabel.append("（条件なし＝常に実行）");
  } else {
    pending[channel].forEach((c, i) => {
      if (i > 0) pendingLabel.append(" かつ ");
      pendingLabel.append(
        formatCondition(c),
        makeButton("✕", () => {
          pending[channel].splice(i, 1);
          renderChannel(channel);
        }),
      );
    });
  }

  const addCondBtn = makeButton("条件追加", () => {
    const op = opSel.value as ConditionOp;
    pending[channel].push({
      type: typeSel.value as ConditionType,
      op,
      value: opNeedsValue(op) ? Number(valInput.value) : 0,
    });
    renderChannel(channel);
  });

  const actionSel = makeSelect(ACTIONS[channel]);
  const addRuleBtn = makeButton("ルール追加", () => {
    ruleset[channel].push({ conditions: [...pending[channel]], action: actionSel.value });
    pending[channel] = [];
    renderChannel(channel);
  });

  wrap.append("条件: ", typeSel, opSel, valInput, addCondBtn, pendingLabel,
    document.createElement("br"), "行動: ", actionSel, addRuleBtn);
  return wrap;
}

// ---- 登録 ----

async function register(): Promise<void> {
  const owner = el<HTMLInputElement>("in-owner").value.trim();
  const name = el<HTMLInputElement>("in-name").value.trim();
  const msg = el("register-msg");
  msg.textContent = "";

  const build = currentBuild();
  const warn = checkBuild(build);
  if (warn) {
    msg.textContent = `登録不可: ${warn}`;
    return;
  }
  try {
    const robot = await createRobot(owner, name, build);
    msg.textContent = `登録成功: ${robot.name}（${robot.id}）`;
    el<HTMLInputElement>("in-inbox-owner").value = owner; // 受信箱の所有者欄も自分に合わせる
    localStorage.setItem("rg-owner", owner); // owner を記憶
    await refreshRoster(); // 登録した機体を名簿に反映
  } catch (e) {
    msg.textContent = `登録失敗: ${(e as Error).message}`;
  }
}

// ---- 名簿・挑戦・観戦（F2 / F3） ----

const playerBattle = createPlayer(el<HTMLCanvasElement>("arena-battle"), el("status-battle"));
const playerInbox = createPlayer(el<HTMLCanvasElement>("arena-inbox"), el("status-inbox"));

async function refreshRoster(): Promise<void> {
  const roster = await listRobots();
  const sel = el<HTMLSelectElement>("sel-challenger");
  const prev = sel.value;
  sel.innerHTML = "";
  for (const r of roster) sel.add(new Option(`${r.name}（${r.id} / ${r.owner}）`, r.id));
  if (roster.some((r) => r.id === prev)) sel.value = prev;

  const ul = el("roster");
  ul.innerHTML = "";
  if (roster.length === 0) {
    ul.innerHTML = "<li>まだロボットがいません。上で登録してください。</li>";
    return;
  }
  for (const r of roster) {
    const li = document.createElement("li");
    li.append(`${r.name}（${r.owner} / ${r.id}） `, makeButton("挑戦", () => void startChallenge(r)));
    ul.appendChild(li);
  }
}

async function startChallenge(opponent: Robot): Promise<void> {
  const msg = el("challenge-msg");
  msg.textContent = "";
  const challengerId = el<HTMLSelectElement>("sel-challenger").value;
  if (!challengerId) {
    msg.textContent = "先に挑戦者（自分のロボット）を登録・選択してください。";
    return;
  }
  try {
    const battle = await challenge(challengerId, opponent.id);
    playerBattle.play(battle.replay, battle.challenger.name, battle.opponent.name);
  } catch (e) {
    msg.textContent = `挑戦失敗: ${(e as Error).message}`;
  }
}

// ---- 受信箱（F4） ----

// 防衛側（受信箱の所有者）視点の勝敗。winner: 1=相手(防衛側)勝ち / 0=挑戦者勝ち / -1=引分。
function defenderResult(b: Battle): string {
  if (b.winner === -1) return "引き分け";
  return b.winner === 1 ? "勝ち" : "負け";
}

async function showInbox(): Promise<void> {
  const owner = el<HTMLInputElement>("in-inbox-owner").value.trim();
  const list = el("inbox");
  const msg = el("inbox-msg");
  msg.textContent = "";
  list.innerHTML = "";
  if (!owner) {
    msg.textContent = "所有者を入力してください。";
    return;
  }
  try {
    const battles = await inbox(owner);
    if (battles.length === 0) {
      list.innerHTML = "<li>まだ挑戦されていません。</li>";
      return;
    }
    for (const b of battles) {
      const li = document.createElement("li");
      li.append(
        `${b.challenger.name}（${b.challenger.owner}）からの挑戦 → ${defenderResult(b)} `,
        makeButton("観戦", () => playerInbox.play(b.replay, b.challenger.name, b.opponent.name)),
      );
      list.appendChild(li);
    }
  } catch (e) {
    msg.textContent = `受信箱の取得に失敗: ${(e as Error).message}`;
  }
}

// ---- 初期化 ----

// タブ切替。
function setActiveTab(name: string): void {
  for (const btn of document.querySelectorAll<HTMLElement>(".tab")) {
    btn.classList.toggle("active", btn.dataset.tab === name);
  }
  for (const panel of document.querySelectorAll<HTMLElement>(".panel")) {
    panel.classList.toggle("active", panel.id === `panel-${name}`);
  }
}
for (const btn of document.querySelectorAll<HTMLElement>(".tab")) {
  btn.addEventListener("click", () => setActiveTab(btn.dataset.tab ?? "create"));
}

// owner 名を記憶して自動入力（毎回入力の手間を緩和）。
const savedOwner = localStorage.getItem("rg-owner");
if (savedOwner) {
  el<HTMLInputElement>("in-owner").value = savedOwner;
  el<HTMLInputElement>("in-inbox-owner").value = savedOwner;
}

el("btn-register").addEventListener("click", () => void register());
el("btn-refresh").addEventListener("click", () => void refreshRoster());
el("btn-restart-battle").addEventListener("click", () => playerBattle.restart());
el("btn-restart-inbox").addEventListener("click", () => playerInbox.restart());
el("btn-inbox").addEventListener("click", () => void showInbox());
renderChassisSelect();
renderPartsCatalog();
renderChannel("movement");
renderChannel("weapon");
renderChannel("special");
refreshConstraints();
void refreshRoster();
