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
		Weapon: &WeaponSpec{Power: power, Range: rng, Cooldown: cd, HeatPerShot: heat, ProjectileSpeed: 40, Pattern: "single"},
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

func unarmed() Build {
	return Build{Chassis: balancedChassis()}
}

func anyOverheated(r Replay) bool {
	for _, f := range r.Frames {
		if f.Robots[0].Overheated || f.Robots[1].Overheated {
			return true
		}
	}
	return false
}

// ---- テスト本体 ----

func TestDeterminism(t *testing.T) {
	a := aggressive(15, 250, 8, 10)
	b := aggressive(11, 300, 9, 12)
	if !reflect.DeepEqual(Simulate(a, b), Simulate(a, b)) {
		t.Fatal("同じ入力で結果が異なる（非決定的）")
	}
}

func TestInitialFrameAndArena(t *testing.T) {
	r := Simulate(aggressive(10, 250, 8, 5), aggressive(10, 250, 8, 5))
	f0 := r.Frames[0]
	if f0.Robots[0].X != 200*PositionScale || f0.Robots[1].X != 1400*PositionScale {
		t.Errorf("初期X座標が不正: %d, %d", f0.Robots[0].X, f0.Robots[1].X)
	}
	if f0.Robots[0].Hp != 100 || f0.Robots[1].Hp != 100 {
		t.Errorf("初期HPが不正")
	}
	if len(r.Obstacles) != 4 {
		t.Errorf("遮蔽物 = %d 個, want 4", len(r.Obstacles))
	}
	if r.ArenaW != 1600*PositionScale {
		t.Errorf("ArenaW = %d", r.ArenaW)
	}
}

// 攻撃力が低い側は勝てない（点対称配置＋相互遮蔽では引き分けもありうるため「負けない」で検証）。
func TestHigherPowerNeverLoses(t *testing.T) {
	r := Simulate(aggressive(20, 250, 8, 5), aggressive(8, 250, 8, 5))
	if r.Winner == 1 {
		t.Errorf("攻撃力が高い挑戦者が負けた: winner=%d reason=%s", r.Winner, r.Reason)
	}
}

// 武器を持たない相手には負けない。
func TestArmedNeverLosesToUnarmed(t *testing.T) {
	r := Simulate(aggressive(15, 250, 8, 5), unarmed())
	if r.Winner == 1 {
		t.Errorf("武装側が無武装に負けた: winner=%d reason=%s", r.Winner, r.Reason)
	}
}

// 交戦に到達すれば発射体が生成される。
func TestProjectilesAppear(t *testing.T) {
	r := Simulate(aggressive(10, 250, 8, 5), aggressive(10, 250, 8, 5))
	found := false
	for _, f := range r.Frames {
		if len(f.Projectiles) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("発射体が一度も生成されていない（交戦に到達していない可能性）")
	}
}

// 不変条件：ロボの中心が遮蔽物の内部に入らない（壁ずりが効いている）。
func TestRobotsNeverInsideObstacle(t *testing.T) {
	r := Simulate(aggressive(12, 250, 8, 5), aggressive(12, 250, 8, 5))
	for _, f := range r.Frames {
		for i := 0; i < 2; i++ {
			if insideAnyObstacle(f.Robots[i].X, f.Robots[i].Y) {
				t.Fatalf("tick %d: ロボ%d が遮蔽物内 (%d,%d)", f.Tick, i, f.Robots[i].X, f.Robots[i].Y)
			}
		}
	}
}

func minRobotDistance(r Replay) int {
	min := int(^uint(0) >> 1)
	for _, f := range r.Frames {
		if d := int(isqrt(dist2Of(f.Robots[0], f.Robots[1]))); d < min {
			min = d
		}
	}
	return min
}

// 指令なし（デフォルト接近）の2体が中央レーンを通って互いに接近できる（膠着しない）。
func TestDefaultApproachReachesEnemy(t *testing.T) {
	r := Simulate(unarmed(), unarmed())
	if md := minRobotDistance(r); md > 60*PositionScale {
		t.Errorf("接近できていない（膠着の疑い）: 最小距離=%d", md)
	}
}

func TestOverheatOccurs(t *testing.T) {
	r := Simulate(aggressive(10, 250, 3, 60), aggressive(10, 250, 3, 60))
	if !anyOverheated(r) {
		t.Error("高発熱・高連射でオーバーヒートが発生していない")
	}
}

func TestTimeoutDrawWhenUnarmed(t *testing.T) {
	r := Simulate(unarmed(), unarmed())
	if r.Reason != "timeout" {
		t.Errorf("reason=%s, want timeout", r.Reason)
	}
	if r.Winner != -1 {
		t.Errorf("winner=%d, want -1", r.Winner)
	}
}

// ---- ヘルパー単体テスト ----

func TestInsideAnyObstacle(t *testing.T) {
	o := obstacles[0]
	if !insideAnyObstacle(o.X+o.W/2, o.Y+o.H/2) {
		t.Error("遮蔽物中心が内部判定されない")
	}
	if insideAnyObstacle(0, 0) {
		t.Error("原点が遮蔽物内と誤判定")
	}
}

func TestSlideAroundObstacleAvoidsEntry(t *testing.T) {
	o := obstacles[0]
	// 遮蔽物の中心へ直進しようとする（X・Y両方ずれた位置から）。
	oldX, oldY := o.X-100, o.Y-100
	nx, ny := slideAroundObstacles(oldX, oldY, o.X+o.W/2, o.Y+o.H/2)
	if insideAnyObstacle(nx, ny) {
		t.Errorf("壁ずり後も遮蔽物内 (%d,%d)", nx, ny)
	}
}

func TestSegHitsCircle(t *testing.T) {
	if !segHitsCircle(0, 0, 100, 0, 50, 0, 10) {
		t.Error("中心を通る線分が命中しない")
	}
	if segHitsCircle(0, 0, 100, 0, 50, 100, 10) {
		t.Error("遠い線分が命中扱い")
	}
	// 高速で点としては跨ぐ位置でも、線分判定なら命中（トンネリング防止）。
	if !segHitsCircle(0, 0, 40, 0, 20, 5, 10) {
		t.Error("線分近傍が命中しない（トンネリング）")
	}
}

// ---- 段階3：パーツ拡張 ----

func boosterPart() Part {
	return Part{Name: "Booster", Category: "movement", Weight: 5, SlotCost: 1,
		Movement: &MovementSpec{DashDistance: 120, DashCooldown: 30, DashPowerCost: 20}}
}

func guardPart() Part {
	return Part{Name: "Guard", Category: "defense", Weight: 10, SlotCost: 1, PowerCost: 15}
}

func armorPart() Part {
	return Part{Name: "Armor", Category: "armor", Weight: 6, SlotCost: 1, Armor: &ArmorSpec{Shield: 30}}
}

func TestApplyDamageDefendHalves(t *testing.T) {
	s := RobotState{Hp: 100, Defending: true}
	applyDamage(&s, 20)
	if s.Hp != 90 {
		t.Errorf("防御中の被ダメージ半減が効いていない: hp=%d, want 90", s.Hp)
	}
	n := RobotState{Hp: 100}
	applyDamage(&n, 20)
	if n.Hp != 80 {
		t.Errorf("通常被ダメージ: hp=%d, want 80", n.Hp)
	}
}

func TestDeriveDashAndDefense(t *testing.T) {
	d := derive(Build{Chassis: balancedChassis(), Parts: []Part{boosterPart(), guardPart()}})
	if d.dash == nil {
		t.Error("ダッシュパーツが認識されない")
	}
	if !d.hasDefense {
		t.Error("防御ユニットが認識されない")
	}
}

func TestValidateAcceptsDefensiveParts(t *testing.T) {
	b := Build{Chassis: balancedChassis(), Parts: []Part{armorPart(), boosterPart(), guardPart()}}
	if err := ValidateBuild(b); err != nil {
		t.Errorf("有効なビルドが拒否された: %v", err)
	}
}

func TestDashSetsCooldown(t *testing.T) {
	dashBot := Build{
		Chassis: balancedChassis(),
		Parts:   []Part{boosterPart()},
		Ruleset: Ruleset{Movement: []Rule{{Conditions: []Condition{{Type: "dashReady", Op: "exists"}}, Action: "dashApproach"}}},
	}
	r := Simulate(dashBot, unarmed())
	for _, f := range r.Frames {
		if f.Robots[0].DashCd > 0 {
			return // ダッシュが使われ CD が立った
		}
	}
	t.Error("ダッシュが使われていない（DashCd が立たない）")
}
