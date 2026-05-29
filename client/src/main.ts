import { createRobot, listRobots, challenge } from "./api";
import { createPlayer } from "./player";
import { deriveStats, checkBuild } from "./buildStats";
import { BALANCED, STARTER_CANNON } from "./data";
import type { Build, Condition, ConditionOp, ConditionType, Robot, Ruleset } from "./types";

function el<T extends HTMLElement>(id: string): T {
  return document.getElementById(id) as T;
}

type Channel = "movement" | "weapon";

// 段階1で UI に出す行動・条件（ダッシュ系は段階2のため除外）。
const ACTIONS: Record<Channel, string[]> = {
  movement: ["approach", "retreat", "keepDistance", "stop"],
  weapon: ["fire", "hold"],
};
const CONDITION_TYPES: ConditionType[] = ["enemyDistance", "selfHp", "selfShield", "selfHeat", "selfBattery"];
const CONDITION_OPS: Record<ConditionType, ConditionOp[]> = {
  enemyDistance: ["inRange", "outRange", "lt", "gt"],
  selfHp: ["lt", "gt"],
  selfShield: ["exists", "none"],
  selfHeat: ["lt", "gt", "nearOverheat"],
  selfBattery: ["lt", "gt"],
  dashReady: ["exists"], // 段階1ではUI非表示
};
const opNeedsValue = (op: ConditionOp): boolean => op === "lt" || op === "gt";

// 編集中のルールセットと、各チャネルの「追加中の条件（AND）」。
const ruleset: Ruleset = { movement: [], weapon: [], special: [] };
const pending: Record<Channel, Condition[]> = { movement: [], weapon: [] };

function currentBuild(): Build {
  const parts = el<HTMLInputElement>("eq-weapon").checked ? [STARTER_CANNON] : [];
  return { chassis: BALANCED, parts, ruleset };
}

function refreshConstraints(): void {
  const b = currentBuild();
  const s = deriveStats(b);
  el("constraints").textContent =
    `スロット ${s.slotsUsed}/${b.chassis.slots} ・ 総重量 ${s.totalWeight} ・ 実効速度 ${s.effSpeed} ・ 利用可能電力 ${s.availPower}`;
  el("build-warn").textContent = checkBuild(b) ?? "";
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
  const root = el(channel === "movement" ? "ch-movement" : "ch-weapon");
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
    await refreshRoster(); // 登録した機体を名簿に反映
  } catch (e) {
    msg.textContent = `登録失敗: ${(e as Error).message}`;
  }
}

// ---- 名簿・挑戦・観戦（F2 / F3） ----

const player = createPlayer(el<HTMLCanvasElement>("arena"), el("status"));

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
    player.play(battle.replay, battle.challenger.name, battle.opponent.name);
  } catch (e) {
    msg.textContent = `挑戦失敗: ${(e as Error).message}`;
  }
}

// ---- 初期化 ----

el<HTMLInputElement>("eq-weapon").addEventListener("change", refreshConstraints);
el("btn-register").addEventListener("click", () => void register());
el("btn-refresh").addEventListener("click", () => void refreshRoster());
el("btn-restart").addEventListener("click", () => player.restart());
renderChannel("movement");
renderChannel("weapon");
refreshConstraints();
void refreshRoster();
