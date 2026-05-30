package sim

// evalContext は1体のルール評価に必要な、ティック開始時点のスナップショット。
type evalContext struct {
	self  RobotState
	enemy RobotState
	d     derived
	dist2 int64 // 敵との二乗距離（ミリ）
}

// chooseAction はチャネルのルールを上から評価し、最初に全条件を満たす行動を返す。
// どれも合致しなければ def（デフォルト行動）を返す。
func chooseAction(rules []Rule, ctx evalContext, def string) string {
	for _, r := range rules {
		if allConditionsMet(r.Conditions, ctx) {
			return r.Action
		}
	}
	return def
}

func allConditionsMet(cs []Condition, ctx evalContext) bool {
	for _, c := range cs {
		if !condMet(c, ctx) {
			return false
		}
	}
	return true
}

// condMet は1つの条件の真偽を返す（FunctionalDesign §0-6）。未知の条件は偽。
func condMet(c Condition, ctx evalContext) bool {
	switch c.Type {
	case "enemyDistance":
		return enemyDistanceMet(c, ctx)
	case "selfHp":
		if ctx.d.maxHp <= 0 {
			return false
		}
		return cmpInt(c.Op, ctx.self.Hp*100/ctx.d.maxHp, c.Value)
	case "selfShield":
		switch c.Op {
		case "exists":
			return ctx.self.Shield > 0
		case "none":
			return ctx.self.Shield == 0
		}
	case "selfHeat":
		switch c.Op {
		case "lt":
			return ctx.self.Heat < c.Value
		case "gt":
			return ctx.self.Heat > c.Value
		case "nearOverheat":
			return ctx.self.Heat >= OverheatThreshold*8/10
		}
	case "selfBattery":
		return cmpInt(c.Op, ctx.self.Battery, c.Value)
	case "dashReady":
		return ctx.self.DashCd == 0
	case "hitWall":
		return ctx.self.Blocked
	case "lineOfSight":
		blocked := lineOfSightBlocked(ctx.self.X, ctx.self.Y, ctx.enemy.X, ctx.enemy.Y)
		switch c.Op {
		case "blocked":
			return blocked
		case "clear":
			return !blocked
		}
	}
	return false
}

func enemyDistanceMet(c Condition, ctx evalContext) bool {
	switch c.Op {
	case "inRange":
		if ctx.d.weapon == nil {
			return false
		}
		r := int64(ctx.d.weaponRangeMilli)
		return ctx.dist2 <= r*r
	case "outRange":
		if ctx.d.weapon == nil {
			return true
		}
		r := int64(ctx.d.weaponRangeMilli)
		return ctx.dist2 > r*r
	case "lt":
		v := int64(c.Value) * PositionScale
		return ctx.dist2 < v*v
	case "gt":
		v := int64(c.Value) * PositionScale
		return ctx.dist2 > v*v
	}
	return false
}

func cmpInt(op string, a, b int) bool {
	switch op {
	case "lt":
		return a < b
	case "gt":
		return a > b
	}
	return false
}
