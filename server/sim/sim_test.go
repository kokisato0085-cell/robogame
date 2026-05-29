package sim

import (
	"reflect"
	"testing"
)

// ---- テスト用ビルド生成ヘルパー ----

func balancedChassis() Chassis {
	return Chassis{Name: "Balanced", BaseHp: 100, Weight: 20, Slots: 4, BatteryCapacity: 100, BaseSpeed: 12}
}

func weaponPart(power, rng, cd, heat int) Part {
	return Part{
		Name: "W", Category: "weapon", Weight: 8, PowerCost: 6, SlotCost: 1,
		Weapon: &WeaponSpec{Power: power, Range: rng, Cooldown: cd, HeatPerShot: heat, Pattern: "single"},
	}
}

// aggressive は「射程内なら撃つ／移動はデフォルト=接近」のビルド。
func aggressive(power, rng, cd, heat int) Build {
	return Build{
		Chassis: balancedChassis(),
		Parts:   []Part{weaponPart(power, rng, cd, heat)},
		Ruleset: Ruleset{
			Weapon: []Rule{{Conditions: []Condition{{Type: "enemyDistance", Op: "inRange"}}, Action: "fire"}},
		},
	}
}

func dist2(a, b RobotState) int64 {
	dx, dy := int64(a.X-b.X), int64(a.Y-b.Y)
	return dx*dx + dy*dy
}

func anyOverheated(r Replay) bool {
	for _, f := range r.Frames {
		if f.Robots[0].Overheated || f.Robots[1].Overheated {
			return true
		}
	}
	return false
}

// ---- テスト本体（設計書 §3 / §0 に基づく） ----

// 決定論：同じ入力なら必ず同じリプレイ。最重要要件。
func TestDeterminism(t *testing.T) {
	a := aggressive(15, 250, 8, 10)
	b := aggressive(11, 300, 9, 12)
	if !reflect.DeepEqual(Simulate(a, b), Simulate(a, b)) {
		t.Fatal("同じ入力で結果が異なる（非決定的）")
	}
}

// 初期フレーム：開始座標と初期HP（FunctionalDesign §0-4）。
func TestInitialFrame(t *testing.T) {
	r := Simulate(aggressive(10, 250, 8, 5), aggressive(10, 250, 8, 5))
	f0 := r.Frames[0]
	if f0.Tick != 0 {
		t.Errorf("最初のフレーム tick=%d, want 0", f0.Tick)
	}
	if f0.Robots[0].X != startPositions[0][0] || f0.Robots[1].X != startPositions[1][0] {
		t.Errorf("初期X座標が不正: %d, %d", f0.Robots[0].X, f0.Robots[1].X)
	}
	if f0.Robots[0].Hp != 100 || f0.Robots[1].Hp != 100 {
		t.Errorf("初期HPが不正: %d, %d", f0.Robots[0].Hp, f0.Robots[1].Hp)
	}
}

// 射程外からはデフォルトで接近する（距離が縮む）。
func TestApproachFromOutOfRange(t *testing.T) {
	r := Simulate(aggressive(10, 100, 8, 5), aggressive(10, 100, 8, 5))
	if len(r.Frames) < 12 {
		t.Fatalf("フレームが少なすぎる: %d", len(r.Frames))
	}
	if dist2(r.Frames[11].Robots[0], r.Frames[11].Robots[1]) >= dist2(r.Frames[0].Robots[0], r.Frames[0].Robots[1]) {
		t.Error("接近していない（距離が縮んでいない）")
	}
}

// 同条件で攻撃力が高い方が勝つ。
func TestHigherPowerWins(t *testing.T) {
	r := Simulate(aggressive(20, 250, 8, 5), aggressive(8, 250, 8, 5))
	if r.Winner != 0 {
		t.Errorf("攻撃力が高い側が勝つはず: winner=%d reason=%s", r.Winner, r.Reason)
	}
}

// 過剰な連射はオーバーヒートする（熱管理がアルゴリズムの役割＝柱）。
func TestOverheatOccurs(t *testing.T) {
	r := Simulate(aggressive(10, 250, 3, 60), aggressive(10, 250, 3, 60))
	if !anyOverheated(r) {
		t.Error("高発熱・高連射でオーバーヒートが発生していない")
	}
}

// ダメージが出ない場合はタイムアウト・引き分け。
func TestTimeoutDrawWhenNoDamage(t *testing.T) {
	r := Simulate(aggressive(0, 250, 8, 0), aggressive(0, 250, 8, 0))
	if r.Reason != "timeout" {
		t.Errorf("reason=%s, want timeout", r.Reason)
	}
	if r.Winner != -1 {
		t.Errorf("winner=%d, want -1（引き分け）", r.Winner)
	}
	if want := MaxTicks + 1; len(r.Frames) != want {
		t.Errorf("フレーム数=%d, want %d", len(r.Frames), want)
	}
}
