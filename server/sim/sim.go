package sim

import "math/bits"

// liveProjectile は飛行中の発射体（シミュレーション内部状態）。
type liveProjectile struct {
	x, y, vx, vy int // ミリ
	source       int
	damage       int
	traveled     int // 飛距離（ミリ）
	rng          int // 飛距離上限（ミリ）
}

// Simulate は a（挑戦者）と b（相手）の戦闘を最後まで計算し、リプレイを返す。
// 同じ (a, b) なら必ず同じ Replay を返す（決定論）。処理順は FunctionalDesign S2-2。
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
	frames = append(frames, Frame{Tick: 0, Robots: st})

	var weaponCd [2]int
	var projs []liveProjectile
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

		// 2) スナップショットから両者の行動を決定（同時解決）。
		snap := st
		dsq := dist2Of(snap[0], snap[1])
		var moveAct, weapAct, defendAct [2]string
		for i := 0; i < 2; i++ {
			ctx := evalContext{self: snap[i], d: der[i], dist2: dsq}
			moveAct[i] = chooseAction(builds[i].Ruleset.Movement, ctx, "approach")
			weapAct[i] = chooseAction(builds[i].Ruleset.Weapon, ctx, "hold")
			defendAct[i] = chooseAction(builds[i].Ruleset.Special, ctx, "none")
		}

		// 3) 移動（ダッシュ・防御を反映）＋壁ずり＋重なり防止。
		for i := 0; i < 2; i++ {
			defending := defendAct[i] == "defend" && der[i].hasDefense
			st[i].Defending = defending

			dashing := (moveAct[i] == "dashApproach" || moveAct[i] == "dashRetreat") &&
				der[i].dash != nil && snap[i].DashCd == 0

			step := der[i].effSpeed
			if dashing {
				step = der[i].dash.DashDistance * PositionScale
			}
			if snap[i].Overheated {
				step /= 2
			}
			if defending {
				step = step * DefendSpeedNum / DefendSpeedDen // 大幅減速
			}

			nx, ny := resolveMovement(moveAct[i], snap[i], snap[1-i], der[i], step)
			nx, ny = clampArena(nx, ny)
			st[i].X, st[i].Y = slideAroundObstacles(snap[i].X, snap[i].Y, nx, ny)

			if dashing {
				st[i].DashCd = der[i].dash.DashCooldown
			}
		}
		separate(&st)

		var events []Event

		// 4) 発射（発射体を生成。即時ダメージは無い）。
		for i := 0; i < 2; i++ {
			w := der[i].weapon
			if weapAct[i] != "fire" || w == nil || weaponCd[i] != 0 || snap[i].Overheated {
				continue
			}
			p, ok := spawnProjectile(snap[i], snap[1-i], w, i)
			if !ok {
				continue // 敵と重なっていて方向が定まらない場合は不発
			}
			projs = append(projs, p)
			weaponCd[i] = w.Cooldown
			st[i].Heat += w.HeatPerShot
			if st[i].Heat >= OverheatThreshold && !st[i].Overheated {
				st[i].Overheated = true
				events = append(events, Event{Type: "overheat", Source: i, Target: i})
			}
		}

		// 5) 発射体の移動と判定（命中・遮蔽物・射程切れ）。
		projs = advanceProjectiles(projs, &st, &events)

		// 6) 破壊イベント＋フレーム記録。
		dead0, dead1 := st[0].Hp <= 0, st[1].Hp <= 0
		if dead0 {
			events = append(events, Event{Type: "destroyed", Source: 0, Target: 0})
		}
		if dead1 {
			events = append(events, Event{Type: "destroyed", Source: 1, Target: 1})
		}
		frames = append(frames, Frame{Tick: tick, Robots: st, Projectiles: snapshotProjectiles(projs), Events: events})

		// 7) 決着判定。
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
	return Replay{
		Builds: builds, Frames: frames,
		Obstacles: obstacles, ArenaW: ArenaW, ArenaH: ArenaH,
		Winner: winner, Reason: reason,
	}
}

// ---- 発射体 ----

// spawnProjectile は発射時の敵位置へ向かう発射体を作る。敵と重なっていれば ok=false。
func spawnProjectile(self, enemy RobotState, w *WeaponSpec, source int) (liveProjectile, bool) {
	dx, dy := enemy.X-self.X, enemy.Y-self.Y
	dist := int(isqrt(int64(dx)*int64(dx) + int64(dy)*int64(dy)))
	if dist == 0 {
		return liveProjectile{}, false
	}
	spd := w.ProjectileSpeed * PositionScale
	return liveProjectile{
		x: self.X, y: self.Y,
		vx: dx * spd / dist, vy: dy * spd / dist,
		source: source, damage: w.Power,
		rng: w.Range * PositionScale,
	}, true
}

// advanceProjectiles は全発射体を1tick進め、命中/遮蔽/射程切れを処理し、生存分を返す。
func advanceProjectiles(projs []liveProjectile, st *[2]RobotState, events *[]Event) []liveProjectile {
	kept := make([]liveProjectile, 0, len(projs))
	for _, p := range projs {
		ox, oy := p.x, p.y
		p.x += p.vx
		p.y += p.vy
		p.traveled += int(isqrt(int64(p.vx)*int64(p.vx) + int64(p.vy)*int64(p.vy)))

		if p.traveled >= p.rng || insideAnyObstacle(p.x, p.y) {
			continue // 射程切れ or 遮蔽物で消滅
		}
		// 移動線分と敵円のスイープ判定（高速弾のすり抜けを防ぐ）。
		target := 1 - p.source
		if segHitsCircle(ox, oy, p.x, p.y, st[target].X, st[target].Y, HitRadius) {
			applyDamage(&st[target], p.damage)
			*events = append(*events, Event{Type: "attack", Source: p.source, Target: target, Amount: p.damage})
			continue // 命中で消滅
		}
		kept = append(kept, p)
	}
	return kept
}

// segHitsCircle は線分 (ax,ay)-(bx,by) が中心 (cx,cy)・半径 r の円に交差するか（整数）。
// 円の中心から線分への最近点距離が r 以下かで判定する。
func segHitsCircle(ax, ay, bx, by, cx, cy, r int) bool {
	abx, aby := int64(bx-ax), int64(by-ay)
	ab2 := abx*abx + aby*aby
	var qx, qy int64
	if ab2 == 0 {
		qx, qy = int64(ax), int64(ay)
	} else {
		t := int64(cx-ax)*abx + int64(cy-ay)*aby // AC・AB
		if t < 0 {
			t = 0
		} else if t > ab2 {
			t = ab2
		}
		qx = int64(ax) + abx*t/ab2
		qy = int64(ay) + aby*t/ab2
	}
	dx, dy := int64(cx)-qx, int64(cy)-qy
	return dx*dx+dy*dy <= int64(r)*int64(r)
}

func snapshotProjectiles(projs []liveProjectile) []Projectile {
	if len(projs) == 0 {
		return nil
	}
	out := make([]Projectile, len(projs))
	for i, p := range projs {
		out[i] = Projectile{X: p.x, Y: p.y, Source: p.source}
	}
	return out
}

// ---- 遮蔽物 ----

func insideAnyObstacle(x, y int) bool {
	for _, o := range obstacles {
		if x >= o.X && x <= o.X+o.W && y >= o.Y && y <= o.Y+o.H {
			return true
		}
	}
	return false
}

// slideAroundObstacles は壁ずり（FunctionalDesign S2-3）。
// 直進が塞がれたら軸別に通れる方へ。正面衝突（軸別も不可）なら進行方向に垂直へ回り込む（壁伝い）。
func slideAroundObstacles(oldX, oldY, nx, ny int) (int, int) {
	if !insideAnyObstacle(nx, ny) {
		return nx, ny
	}
	if !insideAnyObstacle(nx, oldY) {
		return nx, oldY
	}
	if !insideAnyObstacle(oldX, ny) {
		return oldX, ny
	}
	// 正面から壁に当たり軸スライドも不可なら、進行方向に垂直な向きへ壁伝いに動く。
	dx, dy := nx-oldX, ny-oldY
	for _, c := range [2][2]int{{-dy, dx}, {dy, -dx}} {
		px, py := clampArena(oldX+c[0], oldY+c[1])
		if (px != oldX || py != oldY) && !insideAnyObstacle(px, py) {
			return px, py
		}
	}
	return oldX, oldY
}

// ---- 移動 ----

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
	if s.Defending {
		dmg = dmg * DefendDamageNum / DefendDamageDen // 防御中は被ダメージ半減
	}
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
		return x + step, y
	}
	return x + dx*step/dist, y + dy*step/dist
}

func clampArena(x, y int) (int, int) {
	return clamp(x, 0, ArenaW), clamp(y, 0, ArenaH)
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
