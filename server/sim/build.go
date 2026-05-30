package sim

// derived はビルドから算出した、戦闘に使う実効ステータス。
// 都度パーツを走査しないよう、シミュレーション開始時に一度だけ計算する。
type derived struct {
	maxHp            int
	totalWeight      int
	excessWeight     int
	effSpeed         int // ミリ/tick（オーバーヒート前）
	availPower       int
	shield           int
	weapon           *WeaponSpec
	weaponRangeMilli int           // 射程をミリ単位に換算した値（距離比較用）
	dash             *MovementSpec // 移動パーツ（段階2の Booster 等）。無ければ nil
	hasDefense       bool          // 防御ユニットを装備しているか
}

// derive はビルドの実効ステータスを算出する。
// 武器は段階1では1個前提のため、最初に見つかった武器を主武器とする。
func derive(b Build) derived {
	d := derived{maxHp: b.Chassis.BaseHp, totalWeight: b.Chassis.Weight}

	for i := range b.Parts {
		p := &b.Parts[i]
		d.totalWeight += p.Weight
		if p.Armor != nil {
			d.shield += p.Armor.Shield
		}
		if p.Weapon != nil && d.weapon == nil {
			d.weapon = p.Weapon
		}
		if p.Movement != nil && d.dash == nil {
			d.dash = p.Movement
		}
		if p.Category == "defense" {
			d.hasDefense = true
		}
	}

	d.excessWeight = max0(d.totalWeight - b.Chassis.Weight)

	// 実効速度（ミリ）＝ baseSpeed×scale − 重量ペナルティ。下限 SpeedMin。
	speed := b.Chassis.BaseSpeed*PositionScale - WeightSpeedCoeff*d.excessWeight
	if min := SpeedMin * PositionScale; speed < min {
		speed = min
	}
	d.effSpeed = speed

	// 1ティックの利用可能電力（重量負荷を引く）。
	d.availPower = b.Chassis.BatteryCapacity - d.excessWeight/WeightBattDiv

	if d.weapon != nil {
		d.weaponRangeMilli = d.weapon.Range * PositionScale
	}
	return d
}

func max0(x int) int {
	if x < 0 {
		return 0
	}
	return x
}
