package sim

import "math/bits"

// Simulate は a（挑戦者）と b（相手）の戦闘を最後まで計算し、リプレイを返す。
// 同じ (a, b) なら必ず同じ Replay を返す（決定論）。処理順は BasicDesign §3.2 に従う。
func Simulate(a, b Build) Replay {
	builds := [2]Build{a, b}
	der := [2]derived{derive(a), derive(b)}

	var st [2]RobotState
	for i := 0; i < 2; i++ {
		st[i] = RobotState{
			X: startPositions[i][0], Y: startPositions[i][1],
			Hp: der[i].maxHp, Shield: der[i].shield,
			Battery: der[i].availPower,
		}
	}

	frames := make([]Frame, 0, 128)
	frames = append(frames, Frame{Tick: 0, Robots: st}) // 初期スナップショット

	var weaponCd [2]int
	winner, reason := -1, "timeout"

	for tick := 1; tick <= MaxTicks; tick++ {
		// 1) 自然減衰・クールダウン・オーバーヒート回復。
		for i := 0; i < 2; i++ {
			st[i].Heat = max0(st[i].Heat - HeatDecay)
			if weaponCd[i] > 0 {
				weaponCd[i]--
			}
			if st[i].DashCd > 0 {
				st[i].DashCd--
			}
			if st[i].Overheated && st[i].Heat <= OverheatRecover {
				st[i].Overheated = false
			}
		}

		// 2) 減衰後の状態をスナップショットし、両者の行動を決定（同時解決）。
		snap := st
		dsq := dist2Of(snap[0], snap[1])
		var moveAct, weapAct [2]string
		for i := 0; i < 2; i++ {
			ctx := evalContext{self: snap[i], d: der[i], dist2: dsq}
			moveAct[i] = chooseAction(builds[i].Ruleset.Movement, ctx, "approach")
			weapAct[i] = chooseAction(builds[i].Ruleset.Weapon, ctx, "hold")
		}

		// 3) 移動先を算出（スナップショットから）。
		var newPos [2][2]int
		for i := 0; i < 2; i++ {
			step := der[i].effSpeed
			if snap[i].Overheated {
				step /= 2 // オーバーヒート時の速度ペナルティ
			}
			nx, ny := resolveMovement(moveAct[i], snap[i], snap[1-i], der[i], step)
			newPos[i][0], newPos[i][1] = clampArena(nx, ny)
		}

		// 4) 攻撃判定（スナップショットから・同時）。
		var dmg [2]int
		var fired [2]bool
		var events []Event
		for i := 0; i < 2; i++ {
			w := der[i].weapon
			if weapAct[i] != "fire" || w == nil || weaponCd[i] != 0 || snap[i].Overheated {
				continue
			}
			r := int64(der[i].weaponRangeMilli)
			if dsq > r*r {
				continue
			}
			opp := 1 - i
			dmg[opp] += w.Power
			fired[i] = true
			events = append(events, Event{Type: "attack", Source: i, Target: opp, Amount: w.Power})
		}

		// 5) 移動を適用し、重なりを防ぐ。
		for i := 0; i < 2; i++ {
			st[i].X, st[i].Y = newPos[i][0], newPos[i][1]
		}
		separate(&st)

		// 6) 攻撃の副作用（熱・クールダウン・オーバーヒート）。
		for i := 0; i < 2; i++ {
			if !fired[i] {
				continue
			}
			w := der[i].weapon
			weaponCd[i] = w.Cooldown
			st[i].Heat += w.HeatPerShot
			if st[i].Heat >= OverheatThreshold && !st[i].Overheated {
				st[i].Overheated = true
				events = append(events, Event{Type: "overheat", Source: i, Target: i})
			}
		}

		// 7) ダメージ適用（シールド→HP）。
		for i := 0; i < 2; i++ {
			applyDamage(&st[i], dmg[i])
		}

		// 8) 電力（段階1は利用可能電力を表示用に保持）。
		for i := 0; i < 2; i++ {
			st[i].Battery = der[i].availPower
		}

		// 9) 破壊イベント＋フレーム記録。
		dead0, dead1 := st[0].Hp <= 0, st[1].Hp <= 0
		if dead0 {
			events = append(events, Event{Type: "destroyed", Source: 0, Target: 0})
		}
		if dead1 {
			events = append(events, Event{Type: "destroyed", Source: 1, Target: 1})
		}
		frames = append(frames, Frame{Tick: tick, Robots: st, Events: events})

		// 10) 決着判定。
		if dead0 || dead1 {
			reason = "ko"
			switch {
			case dead0 && dead1:
				winner = -1
			case dead1:
				winner = 0
			default:
				winner = 1
			}
			break
		}
	}

	if reason == "timeout" {
		winner = decideTimeout(st[0], st[1])
	}
	return Replay{Builds: builds, Frames: frames, Winner: winner, Reason: reason}
}

// resolveMovement は行動に応じた移動後座標を返す。
func resolveMovement(action string, self, enemy RobotState, d derived, step int) (int, int) {
	switch action {
	case "retreat", "dashRetreat":
		return stepAway(self.X, self.Y, enemy.X, enemy.Y, step)
	case "keepDistance":
		return resolveKeepDistance(self, enemy, d, step)
	case "stop":
		return self.X, self.Y
	default: // approach / dashApproach / 未知 → 接近
		return stepToward(self.X, self.Y, enemy.X, enemy.Y, step)
	}
}

// resolveKeepDistance は自武器射程の 80〜100% を保つ（FunctionalDesign §0-5）。
func resolveKeepDistance(self, enemy RobotState, d derived, step int) (int, int) {
	if d.weapon == nil {
		return stepToward(self.X, self.Y, enemy.X, enemy.Y, step)
	}
	dist := int(isqrt(dist2Of(self, enemy)))
	r := d.weaponRangeMilli
	switch {
	case dist < r*8/10:
		return stepAway(self.X, self.Y, enemy.X, enemy.Y, step)
	case dist > r:
		return stepToward(self.X, self.Y, enemy.X, enemy.Y, step)
	default:
		return self.X, self.Y
	}
}

func applyDamage(s *RobotState, dmg int) {
	if dmg <= 0 {
		return
	}
	if s.Shield > 0 {
		if dmg <= s.Shield {
			s.Shield -= dmg
			return
		}
		dmg -= s.Shield
		s.Shield = 0
	}
	s.Hp -= dmg
}

// separate は両者が MinSep より近づいた場合、中点を保ったまま MinSep まで引き離す。
func separate(st *[2]RobotState) {
	dx, dy := st[1].X-st[0].X, st[1].Y-st[0].Y
	dist := int(isqrt(int64(dx)*int64(dx) + int64(dy)*int64(dy)))
	if dist >= MinSep {
		return
	}
	midX, midY := (st[0].X+st[1].X)/2, (st[0].Y+st[1].Y)/2
	half := MinSep / 2
	if dist == 0 {
		st[0].X, st[1].X = midX-half, midX+half
		st[0].Y, st[1].Y = midY, midY
		return
	}
	st[0].X, st[0].Y = midX-dx*half/dist, midY-dy*half/dist
	st[1].X, st[1].Y = midX+dx*half/dist, midY+dy*half/dist
}

func decideTimeout(a, b RobotState) int {
	switch {
	case a.Hp != b.Hp:
		return boolToWinner(a.Hp > b.Hp)
	case a.Shield != b.Shield:
		return boolToWinner(a.Shield > b.Shield)
	default:
		return -1
	}
}

func boolToWinner(aWins bool) int {
	if aWins {
		return 0
	}
	return 1
}

// ---- 幾何ヘルパー（整数のみ・決定論的） ----

func dist2Of(a, b RobotState) int64 {
	dx, dy := int64(a.X-b.X), int64(a.Y-b.Y)
	return dx*dx + dy*dy
}

func stepToward(x, y, tx, ty, step int) (int, int) {
	dx, dy := tx-x, ty-y
	dist := int(isqrt(int64(dx)*int64(dx) + int64(dy)*int64(dy)))
	if dist == 0 {
		return x, y
	}
	if dist <= step {
		return tx, ty
	}
	return x + dx*step/dist, y + dy*step/dist
}

func stepAway(x, y, fromX, fromY, step int) (int, int) {
	dx, dy := x-fromX, y-fromY
	dist := int(isqrt(int64(dx)*int64(dx) + int64(dy)*int64(dy)))
	if dist == 0 {
		return x + step, y // 重なっている場合は任意方向（+X）へ
	}
	return x + dx*step/dist, y + dy*step/dist
}

func clampArena(x, y int) (int, int) {
	x = clamp(x, 0, ArenaW)
	y = clamp(y, 0, ArenaH)
	return x, y
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// isqrt は floor(√n) を整数で返す（ニュートン法・決定論的）。
func isqrt(n int64) int64 {
	if n <= 0 {
		return 0
	}
	x := int64(1) << uint((bits.Len64(uint64(n))+1)/2) // x ≥ √n
	for {
		y := (x + n/x) / 2
		if y >= x {
			return x
		}
		x = y
	}
}
