package sim

import "fmt"

// 妥当な種別の集合（behavior.go の評価対象と対応）。検証時のみ使用するため
// map を用いてよい（Simulate 内では使わないので決定論には影響しない）。
var (
	validCategories      = map[string]bool{"weapon": true, "armor": true, "movement": true, "power": true}
	validMovementActions = map[string]bool{"approach": true, "retreat": true, "keepDistance": true, "dashApproach": true, "dashRetreat": true, "stop": true}
	validWeaponActions   = map[string]bool{"fire": true, "hold": true}
	validSpecialActions  = map[string]bool{"defend": true, "powerToggle": true}
	validConditionTypes  = map[string]bool{"enemyDistance": true, "selfHp": true, "selfShield": true, "selfHeat": true, "selfBattery": true, "dashReady": true}
	validConditionOps    = map[string]bool{"inRange": true, "outRange": true, "lt": true, "gt": true, "nearOverheat": true, "exists": true, "none": true}
)

// ValidateBuild はビルドが登録可能か検証する（BasicDesign §4 / FunctionalDesign §0-7）。
// 不正があれば理由付きの error を返す。
func ValidateBuild(b Build) error {
	c := b.Chassis
	switch {
	case c.BaseHp <= 0:
		return fmt.Errorf("baseHp は1以上である必要があります")
	case c.BatteryCapacity <= 0:
		return fmt.Errorf("batteryCapacity は1以上である必要があります")
	case c.BaseSpeed <= 0:
		return fmt.Errorf("baseSpeed は1以上である必要があります")
	case c.Slots < 0 || c.Weight < 0:
		return fmt.Errorf("素体のスロット数・重量が不正です")
	}

	totalSlot, totalWeight := 0, c.Weight
	for _, p := range b.Parts {
		if p.Weight < 0 || p.PowerCost < 0 || p.SlotCost < 0 {
			return fmt.Errorf("パーツのステータスに負値があります")
		}
		if !validCategories[p.Category] {
			return fmt.Errorf("不明なパーツカテゴリです: %q", p.Category)
		}
		if err := validatePartSpec(p); err != nil {
			return err
		}
		totalSlot += p.SlotCost
		totalWeight += p.Weight
	}
	if totalSlot > c.Slots {
		return fmt.Errorf("スロット超過です（使用 %d > 上限 %d）", totalSlot, c.Slots)
	}

	// 案A：武器の基準消費電力が利用可能電力以下であること。
	excess := max0(totalWeight - c.Weight)
	avail := c.BatteryCapacity - excess/WeightBattDiv
	for _, p := range b.Parts {
		if p.Weapon != nil && p.PowerCost > avail {
			return fmt.Errorf("電力不足です（武器消費 %d > 利用可能 %d）", p.PowerCost, avail)
		}
	}

	return validateRuleset(b.Ruleset)
}

func validatePartSpec(p Part) error {
	switch p.Category {
	case "weapon":
		w := p.Weapon
		if w == nil {
			return fmt.Errorf("武器パーツに weapon 仕様がありません")
		}
		if w.Power < 0 || w.Range <= 0 || w.Cooldown <= 0 || w.HeatPerShot < 0 || w.ProjectileSpeed <= 0 {
			return fmt.Errorf("武器パラメータが不正です")
		}
	case "armor":
		if p.Armor == nil || p.Armor.Shield < 0 {
			return fmt.Errorf("装甲パーツの仕様が不正です")
		}
	case "movement":
		if p.Movement == nil {
			return fmt.Errorf("移動パーツに movement 仕様がありません")
		}
	}
	return nil
}

func validateRuleset(rs Ruleset) error {
	if err := validateRules(rs.Movement, validMovementActions); err != nil {
		return err
	}
	if err := validateRules(rs.Weapon, validWeaponActions); err != nil {
		return err
	}
	return validateRules(rs.Special, validSpecialActions)
}

func validateRules(rules []Rule, validActions map[string]bool) error {
	for _, r := range rules {
		if !validActions[r.Action] {
			return fmt.Errorf("このチャネルで使えない行動です: %q", r.Action)
		}
		for _, c := range r.Conditions {
			if !validConditionTypes[c.Type] {
				return fmt.Errorf("不明な条件です: %q", c.Type)
			}
			if !validConditionOps[c.Op] {
				return fmt.Errorf("不明な条件演算子です: %q", c.Op)
			}
		}
	}
	return nil
}
