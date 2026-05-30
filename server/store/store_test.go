package store

import (
	"testing"

	"robogame/server/sim"
)

func testBuild() sim.Build {
	return sim.Build{
		Chassis: sim.Chassis{Name: "Balanced", BaseHp: 100, Weight: 20, Slots: 4, BatteryCapacity: 100, BaseSpeed: 12},
		Parts: []sim.Part{{
			Name: "W", Category: "weapon", Weight: 8, PowerCost: 6, SlotCost: 1,
			Weapon: &sim.WeaponSpec{Power: 12, Range: 250, Cooldown: 8, HeatPerShot: 10, ProjectileSpeed: 40, Pattern: "single"},
		}},
	}
}

func TestAddAndListPreservesOrder(t *testing.T) {
	s := New()
	r1 := s.AddRobot("alice", "A", testBuild())
	r2 := s.AddRobot("bob", "B", testBuild())

	list := s.ListRobots()
	if len(list) != 2 {
		t.Fatalf("一覧件数 = %d, want 2", len(list))
	}
	if list[0].ID != r1.ID || list[1].ID != r2.ID {
		t.Errorf("登録順が保たれていない: %s, %s", list[0].ID, list[1].ID)
	}
	if r1.ID == r2.ID {
		t.Errorf("ID が重複: %s", r1.ID)
	}
}

func TestRecordBattlePutsResultInDefenderInbox(t *testing.T) {
	s := New()
	challenger := s.AddRobot("alice", "A", testBuild())
	opponent := s.AddRobot("bob", "B", testBuild())

	replay := sim.Simulate(challenger.Build, opponent.Build)
	s.RecordBattle(challenger, opponent, replay)

	if got := s.Inbox("bob"); len(got) != 1 {
		t.Errorf("bob の受信箱 = %d 件, want 1", len(got))
	}
	if got := s.Inbox("alice"); len(got) != 0 {
		t.Errorf("alice の受信箱 = %d 件, want 0", len(got))
	}
}

func TestGetRobotMissing(t *testing.T) {
	if _, ok := New().GetRobot("nope"); ok {
		t.Error("存在しない ID で ok=true になった")
	}
}
