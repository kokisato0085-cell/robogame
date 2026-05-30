# 基本設計書

バージョン: v0.1（段階1中心）
方針の根拠は [POLICY.md](./POLICY.md)（前提条件・大方針・中方針）を参照。
本書はシステム全体のアーキテクチャ・データ・API・シミュレーション仕様を定義する。**設計先行**のため、実装は本書に従う。

---

## 1. システム構成

```
[ブラウザ: TypeScript + Canvas]
   │  ① ビルド登録 / 名簿取得 / 挑戦 / 受信箱（REST, JSON）
   ▼
[サーバー: Go]
   ├─ API 層（net/http）            … 検証・ルーティング
   ├─ シミュレーション層（sim）      … 決定論的に戦闘を計算しリプレイ生成
   └─ ストア層                       … 段階1=インメモリ / 段階3=MySQL
   （段階4: WebSocket でライブ同時観戦を追加）
```

- フロントは**リプレイの再生に徹し**、戦闘ロジックを持たない（描画と sim の分離）。
- 戦闘計算はサーバー権威。クライアントは観戦のみ（チート対策）。

---

## 2. データモデル（スキーマ）

JSON 表現。フィールド名は API/保存で共通。

### 2.1 ビルド系

**Chassis（素体）**
| 項目 | 型 | 意味 |
|------|----|------|
| name | string | 素体名 |
| baseHp | int | 基礎HP（0で完全破壊） |
| weight | int | 素体自重 |
| slots | int | スロット数 |
| batteryCapacity | int | 1ティックあたりの利用可能電力（電力スループット上限） |
| baseSpeed | int | 基礎移動速度（units/tick） |
| traits | string[] | 個性（段階2以降。段階1は空） |

**Part（パーツ）** — 共通項目 ＋ カテゴリ固有
| 共通 | 型 | 意味 |
|------|----|------|
| name | string | パーツ名 |
| category | enum | "weapon" / "armor" / "movement" / "power" |
| weight | int | 重量 |
| powerCost | int | 基準消費電力（1ティック稼働あたり） |
| slotCost | int | スロット消費 |

| カテゴリ固有 | 型 | 意味 |
|------|----|------|
| weapon.power | int | 攻撃力 |
| weapon.range | int | 射程 |
| weapon.cooldown | int | 連射間隔（ティック） |
| weapon.heatPerShot | int | 1発の発熱量 |
| weapon.pattern | enum | 攻撃パターン（段階1は "single"） |
| armor.shield | int | シールド値（段階2） |
| movement.dashDistance | int | ダッシュ移動量（段階2） |
| movement.dashCooldown | int | ダッシュCD（段階2） |
| movement.dashPowerCost | int | ダッシュ消費電力（段階2） |

**Ruleset（行動）**
```
Ruleset { channels: { movement: Rule[], weapon: Rule[], special: Rule[] } }
Rule    { conditions: Condition[], action: Action }   // conditions は AND
Condition { type: enum, op: enum, value: number|enum } // 下記パレット
Action    { type: enum }                                // 下記パレット
```

**Build / Robot / Battle / Replay / Frame / Event**
```
Build  { chassis: Chassis, parts: Part[], ruleset: Ruleset }
Robot  { id: string, owner: string, name: string, build: Build }
Battle { id, challenger: Robot, opponent: Robot, winner: int, reason: string, replay: Replay }
Replay { builds: [Build, Build], frames: Frame[], winner: int, reason: string }
Frame  { tick: int, robots: [RobotState, RobotState], events: Event[] }
RobotState { x:int, y:int, hp:int, shield:int, heat:int, battery:int, dashCd:int, overheated:bool }
Event  { type: enum, source:int, target:int, amount:int }   // attack/dash/overheat/destroyed 等
```
- winner: 0=挑戦者 / 1=相手 / -1=引き分け。
- Replay には両者の Build 全体を含め、「どんな構成が勝ったか」を再現可能にする。

### 2.2 条件・行動パレット
- **Condition.type**：`enemyDistance`（op: inRange/outRange/lt/gt + value）／`selfHp`（割合 lt/gt）／`selfShield`（exists/none）／`selfHeat`（lt/gt/nearOverheat）／`selfBattery`（lt/gt）／`dashReady`。
- **Action.type**：移動=`approach`/`retreat`/`keepDistance`/`dashApproach`/`dashRetreat`/`stop`、武器=`fire`/`hold`、特殊=`defend`/`powerToggle`（段階2以降）。

---

## 3. シミュレーション仕様

### 3.1 基本
- 空間：2D 平面（整数座標）。アリーナ 1000×1000。初期配置は対角または左右両端（座標は機能設計書で確定）。
- 時間：離散ティック。**ティックレート 30/秒**、**最大 1800 ティック（=60秒）**。
- 決定論：整数演算のみ。距離比較は二乗値（`dx*dx+dy*dy` と `range*range`）。乱数・時刻・map 反復順を使わない。
- 解決順序：両者の行動を**ティック開始時点のスナップショットから同時解決**。

### 3.2 1ティックの処理順
1. クールダウン・熱の自然減衰を進める（`heat = max(0, heat - HEAT_DECAY)`、武器CD・ダッシュCDをデクリメント）。
2. 各ロボについて、スナップショット状態から**チャネルごとにルールを上から評価**し、移動/武器/特殊の行動を決定（合致なしはデフォルト：移動=approach、武器=hold）。
3. 電力収支を計算し、**利用可能電力を超える行動は抑制**（段階1=案A前提でほぼ発生しない／段階2=案B で停止）。
4. 移動・攻撃を**同時適用**（攻撃ダメージは同ティックのスナップショットHP/シールド基準）。
5. ダメージをシールド→HP の順で適用。Frame を記録。
6. 決着判定（3.6）。

### 3.3 移動
- 総重量 `totalWeight = chassis.weight + Σ parts.weight`。
- 実効速度 `effSpeed = max(SPEED_MIN, baseSpeed - WEIGHT_SPEED_COEFF * max(0, totalWeight - chassis.weight))`。
- 移動行動：approach=敵方向へ effSpeed、retreat=逆方向、keepDistance=射程帯を保つ、stop=移動なし。
- ダッシュ（段階2）：dashDistance を1ティックで移動、dashPowerCost 消費、dashCooldown 設定。
- 定数：`WEIGHT_SPEED_COEFF=0.2`（装備重量1あたり0.2低下／整数化のため内部は固定小数 or 係数を10倍整数で扱う）、`SPEED_MIN=3`。

### 3.4 攻撃・ダメージ
- 発射条件：武器チャネルが `fire` を選択 ∧ 射程内（distance² ≤ range²）∧ 武器CD=0 ∧ オーバーヒートでない。
- 発射効果：`target.damage += weapon.power`、`weaponCd = weapon.cooldown`、`heat += weapon.heatPerShot`。
- ダメージ適用：`shield` を先に減算し、超過分を `hp` から減算。`hp ≤ 0` で完全破壊。

### 3.5 熱・電力
- 熱の自然減衰：撃たなければ毎ティック `HEAT_DECAY` 低下。
- オーバーヒート：`heat ≥ OVERHEAT_THRESHOLD` で武器ロック＋ペナルティ（移動速度低下 `OVERHEAT_SPEED_MULT`）。`heat ≤ OVERHEAT_RECOVER` で解除。
- 電力（1ティックの利用可能量 = `batteryCapacity - WEIGHT_BATTERY_COEFF * max(0, totalWeight - chassis.weight)`）。
- 消費：稼働パーツの `powerCost` 合計。武器発射時は熱比例で増加：`実消費 = powerCost * (1 + HEAT_BATTERY_COEFF * heat / 100)`。
- 段階1=案A（ビルド時に基準消費 ≤ batteryCapacity を検証）。段階2=案B（超過時はランタイムで電力不足→パーツ抑制、電力割り振りで救済）。
- 定数：`HEAT_DECAY=2`、`OVERHEAT_THRESHOLD=100`、`OVERHEAT_RECOVER=40`、`OVERHEAT_SPEED_MULT=0.5`、`HEAT_BATTERY_COEFF=0.5`、`WEIGHT_BATTERY_COEFF=0.1`。

### 3.6 終了条件
- いずれかの hp ≤ 0：reason="ko"、勝者は生存側（両者0は引き分け -1）。
- 最大ティック到達：reason="timeout"、残HP→残シールドの多い方が勝ち、同値は引き分け。

---

## 4. API 仕様（REST / JSON）

ベース：`/api`。エラーは `{ "error": "メッセージ" }` ＋ HTTP ステータス。CORS は開発時プロキシで同一オリジン化。

| メソッド/パス | リクエスト | レスポンス | 主なステータス |
|---|---|---|---|
| POST `/api/robots` | `{ owner, build }` | `Robot` | 201 / 400(検証) |
| GET `/api/robots` | — | `Robot[]` | 200 |
| POST `/api/challenge` | `{ challenger_id, opponent_id }` | `Battle` | 200 / 404 |
| GET `/api/inbox?owner=` | — | `Battle[]` | 200 / 400 |
| GET `/api/battles/{id}` | — | `Battle` | 200 / 404 |

- 登録時の検証（中9）：ステータス正値、`Σ slotCost ≤ slots`、`totalWeight` 妥当、案A の電力収支、Ruleset の条件/行動が既知種別・チャネル妥当。

---

## 5. パーツ初期ラインナップ（段階1）

**素体「Balanced」**：baseHp=100, weight=20, slots=4, batteryCapacity=100, baseSpeed=12, traits=[]。

**武器「Starter Cannon」**：category=weapon, weight=8, powerCost=6, slotCost=1, power=12, range=250, cooldown=8, heatPerShot=18, pattern="single"。

※段階1はこの素体1種＋武器1種、通常移動のみ。装甲・ダッシュ・防御・複数素体・個性は段階2。

---

## 6. セキュリティ・インフラ
- サーバー権威の決定論 sim、リプレイはサーバー生成（改ざん不可）。
- 入力検証は §4 の登録時検証に従う。
- 認証は段階3以降（基礎は owner 名のみ）。
- ホスティング：フロント=静的、サーバー=常時接続可能な PaaS（WebSocket 前提）→将来クラウド。

---

## 7. 段階1で実装する範囲
データモデル（§2、段階1利用分）／sim（§3 の移動・攻撃・ダメージ・熱・終了条件）／REST（§4 の robots・challenge・inbox）／パーツ初期ラインナップ（§5）／Canvas 観戦（機能設計書で詳細）。装甲・ダッシュ・案B・複数素体・WebSocket・MySQL・ビジュアルプログラミングは対象外（後段）。

---

## 8. 未確定事項（機能設計書で確定する）
Pass 2（実装者テスト）で洗い出した、本書だけでは実装できない／曖昧な点。機能設計書で確定する。
1. **2D 移動の決定論的整数化**：敵方向への移動ベクトルの算出方法と丸め規則（整数のまま方向を出す方式）。
2. **小数係数の固定小数点表現**：`WEIGHT_SPEED_COEFF` などを整数演算で扱うスケール（例：1000倍固定小数）。
3. **初期配置座標**：1v1 の開始位置。
4. **keepDistance（カイティング）の保持距離帯**：どの距離を維持するか（例：自武器射程の○割）。
5. **条件 op の正確な意味**：`selfHp` 割合の基準、`enemyDistance` の inRange が指す射程基準（自武器射程か固定値か）。
6. **パーツの「稼働」定義**：毎ティックどのパーツが電力を消費するか（常時消費 vs 行動時のみ）。
7. **重なりの扱い**：同ティックで両者が同座標に重なる場合の最小間隔処理。
8. **案A の電力検証基準**：検証は「利用可能電力（重量負荷を引いた後）」に対して行う。

## 9. 段階2 追加設計

### 2-A 装甲（シールド）パーツ（確定）
- 装甲パーツ2種（`category="armor"`、powerCost=0）。シールドは回復なし・ダメージはシールド→HP の順（sim 実装済み）。

  | パーツ | shield | weight | slotCost |
  |--------|-------:|------:|--------:|
  | Light Plating | 30 | 6 | 1 |
  | Heavy Plating | 70 | 16 | 2 |

- 重い装甲ほどシールドは厚いが、総重量増 → 移動速度低下・バッテリー負荷増（既存式 §3.3/§3.5 に乗る）。
- **ビルダー UI を汎用化**：武器トグルではなく、**パーツカタログ（武器＋装甲）から複数装着**できる形にする（スロット/重量/電力の制約内）。観戦のシールドバーは実装済み。

### 2-B ダッシュ（確定）
- ダッシュパーツ「Booster」（`category="movement"`）。

  | パーツ | dashDistance | dashCooldown | dashPowerCost | weight | slotCost |
  |--------|-------------:|-------------:|--------------:|------:|--------:|
  | Booster | 120 | 30 | 20 | 5 | 1 |

- 行動 `dashApproach`／`dashRetreat`：Booster 装備済み ∧ `dashCd==0` なら 1 ティックで dashDistance（120units＝通常移動の十数倍）を敵方向／逆方向へ瞬間移動し、`dashCd=dashCooldown` を設定。
- 未装備 or CD 中は通常移動にフォールバック。条件 `dashReady`（dashCd==0）が有効化される。
- `dashPowerCost` は定義のみ。**電力不足での抑制は案B（2-F）**で実装。2-B は CD のみでゲート（案A）。

### 2-C オーバーヒート完全版（確定）
- 既存（段階1実装済）：`heat ≥ 100` で発動 → **武器ロック ＋ 移動速度半減**、`heat ≤ 40` で解除。冷却 `HEAT_DECAY=2/tick`（=60/秒）据え置き。
- 追加：**オーバーヒート中は一定の追加電力消費 `OVERHEAT_POWER_PENALTY=30/tick`**。電力逼迫の実効化は案B（2-F）と連動（自滅の連鎖を作り、無理な連射への罰を強める）。
- 時間感覚：1 tick = 1/30 秒。オーバーヒートから回復まで約1秒、その間は撃てず移動半減。

### 2-D 複数素体＆個性（確定）
素体を3種に。個性は数値プロフィールで表現する（`weight`＝重量許容。これを超えた装備重量だけ速度低下・電力負荷）。

| 素体 | baseHp | weight | slots | batteryCapacity | baseSpeed | 個性 |
|------|-------:|------:|------:|----------------:|----------:|------|
| Balanced | 100 | 20 | 4 | 100 | 12 | 万能（既存） |
| Fortress | 160 | 35 | 5 | 120 | 9 | 高HP・多スロット・重装備OK だが鈍足 |
| Runner | 70 | 12 | 3 | 90 | 16 | 高速・回避向き だが低HP・積載少 |

※最終バランスは武器（発射体）次第。数値は小方針として後から調整する。

### 2-G 戦闘モデルの進化：発射体・遮蔽物・マップ（段階2・確定）
hitscan（射程内で即命中）を廃し、**回避可能な発射体**に進化させる。これが「アルゴリズム > パーツ」の柱を真に成立させる（長射程＋装甲ゴリ押しを無効化）。

- **マップ**：アリーナ 1600×1600（内部 1,600,000 ミリ）。
- **初期配置**：**斜め・点対称**（中心(800,800)）。挑戦者(200,650)／相手(1400,950)。初期は両者間≈1237 > 射程250 で、開幕は射程で攻撃が通らない。
- **遮蔽物**：固定の矩形 **4個**（点対称・毎戦同一）。**中央の接近ラインは空ける**（対称な釣り合いによる膠着を防ぐ）。カバーは脇に配置。弾を止める／移動は**壁ずり**（FunctionalDesign S2-3。正面衝突時は垂直へ回り込む）。Replay に含めクライアントが描画。
  - 具体座標は FunctionalDesign S2-1。前方カバーは長射程武器を入れる段階3で再設置。
- **発射体（プロジェクタイル）**：`WeaponSpec.projectileSpeed`（units/tick）を追加。命中判定は共通定数 `HitRadius`。
  - 発射：自機位置に発射体を生成。速度＝**発射時の敵位置**への方向 × projectileSpeed（リード射撃は将来）。damage＝発射時の weapon.power。
  - 毎tick：全発射体を移動 →（敵に HitRadius 内で命中：シールド→HP ＋消滅）／（遮蔽物に入れば消滅）／（飛距離 > weapon.range で消滅）。
  - 命中が「弾が届くか」になり、速度・ダッシュ・カバーで**回避可能**。
- **スキーマ追加**：`Frame.projectiles: [{x,y,source}]`（描画用）。`Replay` に `obstacles: Rect[]` と arena 寸法。
- **定数（小方針・調整可）**：`HitRadius=18`、Starter Cannon の `projectileSpeed=40`。
- **※ §3.4 の hitscan 攻撃は段階2でこの発射体モデルに置き換わる。**条件「カバー退避」等のパレット拡張は将来。

### 2-E 防御ユニット（確定）
- パーツ「Guard Unit」（`category="defense"`、weight 10・slotCost 1・powerCost 15）。装備すると特殊チャネルで `defend` を選べる。
- **defend 中の tick**：被ダメージ **×1/2（50%軽減）**、移動速度 **×1/4（大幅減速・移動不可ではない）**、電力 powerCost を消費。
- ＝「避けられる時は動く／避けられない時は耐える」をアルゴリズムが判断するトレードオフ（柱に効く）。
- 定数（小方針・調整可）：`DefendDamageMult=1/2`、`DefendSpeedMult=1/4`。

## 検証状況
- Pass 1（ソース突合）：実装着手後に実施（設計書先行のため、コードが本書に従う）。
- Pass 2（実装者テスト）：自己実施。未確定点を §8 に列挙（機能設計書で解消）。
- Pass 3（矛盾検出）：自己実施。`batteryCapacity`＝1ティック利用可能電力と統一、案A 検証基準を §8-8 に明記し整合化。
