// Package sim は2体のロボットの戦闘を決定論的にシミュレーションし、
// 観戦用のリプレイ（毎ティックのスナップショット列）を生成する。
//
// 設計の根拠は docs/BasicDesign.md・docs/FunctionalDesign.md（段階1）。
// 決定論を最優先するため、座標・速度・距離はミリ単位の整数で扱い（PositionScale=1000）、
// 距離は整数平方根 isqrt で求める。乱数・浮動小数・時刻・map反復順は使わない。
package sim

// ---- ビルド系（ロボットの定義） ----

// Chassis は素体（ベース）。装備なしでは移動のみ可能。
type Chassis struct {
	Name            string   `json:"name"`
	BaseHp          int      `json:"baseHp"`          // 基礎HP（0で完全破壊）
	Weight          int      `json:"weight"`          // 素体自重
	Slots           int      `json:"slots"`           // スロット数
	BatteryCapacity int      `json:"batteryCapacity"` // 1ティックの利用可能電力
	BaseSpeed       int      `json:"baseSpeed"`       // 基礎移動速度（表示units/tick）
	Traits          []string `json:"traits"`          // 個性（段階2以降）
}

// WeaponSpec は武器カテゴリ固有のパラメータ。
type WeaponSpec struct {
	Power           int    `json:"power"`           // 攻撃力
	Range           int    `json:"range"`           // 射程（表示units・発射体の飛距離上限）
	Cooldown        int    `json:"cooldown"`        // 連射間隔（ティック）
	HeatPerShot     int    `json:"heatPerShot"`     // 1発の発熱量
	ProjectileSpeed int    `json:"projectileSpeed"` // 発射体の速度（表示units/tick）
	Pattern         string `json:"pattern"`         // 攻撃パターン（段階1は "single"）
}

// ArmorSpec は装甲カテゴリ固有（段階2）。
type ArmorSpec struct {
	Shield int `json:"shield"`
}

// DefenseSpec は防御ユニット固有。Charges 回まで被ダメージを半減できる（耐久値）。
type DefenseSpec struct {
	Charges int `json:"charges"`
}

// MovementSpec は移動（ダッシュ）カテゴリ固有（段階2）。
type MovementSpec struct {
	DashDistance  int `json:"dashDistance"`
	DashCooldown  int `json:"dashCooldown"`
	DashPowerCost int `json:"dashPowerCost"`
}

// Part はスロットに装着するパーツ。カテゴリ固有は該当ポインタのみ非nil。
type Part struct {
	Name      string        `json:"name"`
	Category  string        `json:"category"` // "weapon"/"armor"/"movement"/"power"
	Weight    int           `json:"weight"`
	PowerCost int           `json:"powerCost"`
	SlotCost  int           `json:"slotCost"`
	Weapon    *WeaponSpec   `json:"weapon,omitempty"`
	Armor     *ArmorSpec    `json:"armor,omitempty"`
	Movement  *MovementSpec `json:"movement,omitempty"`
	Defense   *DefenseSpec  `json:"defense,omitempty"`
}

// Condition は1つの条件（複数ANDでRuleのIF部を構成）。
type Condition struct {
	Type  string `json:"type"`  // "enemyDistance"/"selfHp"/"selfShield"/"selfHeat"/"selfBattery"/"dashReady"
	Op    string `json:"op"`    // "inRange"/"outRange"/"lt"/"gt"/"nearOverheat"/"exists"/"none"
	Value int    `json:"value"` // 比較値（表示単位 or %）。Opによっては未使用
}

// Rule は「条件（AND）→ 行動」。
type Rule struct {
	Conditions []Condition `json:"conditions"`
	Action     string      `json:"action"`
}

// Ruleset はチャネルごとの優先順位ルールリスト。上が高優先。
type Ruleset struct {
	Movement []Rule `json:"movement"`
	Weapon   []Rule `json:"weapon"`
	Special  []Rule `json:"special"`
}

// Build はロボットの定義一式。
type Build struct {
	Chassis Chassis `json:"chassis"`
	Parts   []Part  `json:"parts"`
	Ruleset Ruleset `json:"ruleset"`
}

// ---- 対戦・リプレイ系 ----

// RobotState は1ティック終了時点の1体の状態。座標はミリ単位。
type RobotState struct {
	X            int  `json:"x"`
	Y            int  `json:"y"`
	Hp           int  `json:"hp"`
	Shield       int  `json:"shield"`
	Heat         int  `json:"heat"`
	Battery      int  `json:"battery"` // その tick の利用可能電力（段階1は概ね一定）
	DashCd       int  `json:"dashCd"`
	Overheated   bool `json:"overheated"`
	Defending    bool `json:"defending"`    // この tick 防御中（被ダメ半減・大幅減速）
	GuardCharges int  `json:"guardCharges"` // 残りガード回数（0で半減しなくなる）
}

// Event は1ティック内の出来事。
type Event struct {
	Type    string `json:"type"` // "attack"/"overheat"/"destroyed"
	Source  int    `json:"source"`
	Target  int    `json:"target"`
	Amount  int    `json:"amount"`
	Guarded bool   `json:"guarded"` // attack: 実際にガードで半減されたか
}

// Projectile は描画用の発射体スナップショット（ミリ座標）。
type Projectile struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Source int `json:"source"`
}

// Frame は1ティックのスナップショット。
type Frame struct {
	Tick        int           `json:"tick"`
	Robots      [2]RobotState `json:"robots"`
	Projectiles []Projectile  `json:"projectiles"`
	Events      []Event       `json:"events"`
}

// Rect はマップ上の矩形（ミリ・x,y=左上）。遮蔽物に使う。
type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Replay は戦闘の全記録。クライアントはこれを再生して観戦する。
type Replay struct {
	Builds    [2]Build `json:"builds"`
	Frames    []Frame  `json:"frames"`
	Obstacles []Rect   `json:"obstacles"`
	ArenaW    int      `json:"arenaW"` // ミリ
	ArenaH    int      `json:"arenaH"` // ミリ
	Winner    int      `json:"winner"` // 0=挑戦者/1=相手/-1=引き分け
	Reason    string   `json:"reason"` // "ko"/"timeout"
}

// ---- シミュレーション定数（docs/FunctionalDesign.md §0-9 / BasicDesign §3.5） ----

const (
	PositionScale = 1000                 // 表示座標 = 内部ミリ / PositionScale
	ArenaW        = 1600 * PositionScale // 段階2でマップ拡大
	ArenaH        = 1600 * PositionScale
	MaxTicks      = 1800 // 30tick/秒 × 60秒

	SpeedMin         = 3                  // 最低移動速度（表示units/tick）
	WeightSpeedCoeff = 200                // 0.2 × PositionScale（超過重量1あたりの速度低下・ミリ）
	WeightBattDiv    = 10                 // 利用可能電力 = capacity − excessWeight/WeightBattDiv
	MinSep           = 40 * PositionScale // 最小間隔（重なり防止）
	HitRadius        = 18 * PositionScale // 発射体の命中半径

	HeatDecay            = 2   // 毎ティックの自然冷却
	OverheatThreshold    = 100 // この熱でオーバーヒート
	OverheatRecover      = 40  // この熱まで下がると解除
	HeatBattDiv          = 200 // 実消費 = powerCost + powerCost×heat/HeatBattDiv
	OverheatPowerPenalty = 30  // オーバーヒート中の追加電力消費（効果は案B=段階4で有効化）

	// 防御（2-E）：被ダメージ ×1/2、移動量 ×1/4。
	DefendDamageNum = 1
	DefendDamageDen = 2
	DefendSpeedNum  = 1
	DefendSpeedDen  = 4
)

// 初期配置（ミリ）。斜め・中心(800,800)について点対称（FunctionalDesign S2-1）。
var startPositions = [2][2]int{
	{200 * PositionScale, 650 * PositionScale},  // 挑戦者
	{1400 * PositionScale, 950 * PositionScale}, // 相手
}

// 遮蔽物（ミリ）。固定・中心(800,800)について点対称。
// 中央の接近ラインを空け（対称な釣り合いによる膠着を防ぐ）、カバーは脇に配置する（FunctionalDesign S2-1）。
var obstacles = []Rect{
	{X: 700 * PositionScale, Y: 400 * PositionScale, W: 80 * PositionScale, H: 140 * PositionScale},  // 上中央寄り
	{X: 820 * PositionScale, Y: 1060 * PositionScale, W: 80 * PositionScale, H: 140 * PositionScale}, // 下中央寄り（上の点対称）
	{X: 380 * PositionScale, Y: 820 * PositionScale, W: 80 * PositionScale, H: 120 * PositionScale},  // A側カバー
	{X: 1140 * PositionScale, Y: 660 * PositionScale, W: 80 * PositionScale, H: 120 * PositionScale}, // B側カバー（点対称）
}
