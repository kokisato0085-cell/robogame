package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"robogame/server/api"
	"robogame/server/sim"
	"robogame/server/store"
)

func validBuild() sim.Build {
	return sim.Build{
		Chassis: sim.Chassis{Name: "Balanced", BaseHp: 100, Weight: 20, Slots: 4, BatteryCapacity: 100, BaseSpeed: 12},
		Parts: []sim.Part{{
			Name: "W", Category: "weapon", Weight: 8, PowerCost: 6, SlotCost: 1,
			Weapon: &sim.WeaponSpec{Power: 12, Range: 250, Cooldown: 8, HeatPerShot: 10, ProjectileSpeed: 40, Pattern: "single"},
		}},
		Ruleset: sim.Ruleset{
			Weapon: []sim.Rule{{Conditions: []sim.Condition{{Type: "enemyDistance", Op: "inRange"}}, Action: "fire"}},
		},
	}
}

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(api.NewServer(store.New()))
	t.Cleanup(ts.Close)
	return ts
}

func postJSON(t *testing.T, url string, payload any) *http.Response {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func createRobot(t *testing.T, ts *httptest.Server, owner, name string) store.Robot {
	t.Helper()
	resp := postJSON(t, ts.URL+"/api/robots", map[string]any{"owner": owner, "name": name, "build": validBuild()})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("作成ステータス = %d, want 201", resp.StatusCode)
	}
	var r store.Robot
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCreateAndListRobots(t *testing.T) {
	ts := newServer(t)
	createRobot(t, ts, "alice", "A")
	createRobot(t, ts, "bob", "B")

	resp, err := http.Get(ts.URL + "/api/robots")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var list []store.Robot
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("一覧件数 = %d, want 2", len(list))
	}
}

func TestChallengeFlow(t *testing.T) {
	ts := newServer(t)
	a := createRobot(t, ts, "alice", "A")
	b := createRobot(t, ts, "bob", "B")

	resp := postJSON(t, ts.URL+"/api/challenge", map[string]any{"challenger_id": a.ID, "opponent_id": b.ID})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("挑戦ステータス = %d, want 200", resp.StatusCode)
	}
	var battle store.Battle
	if err := json.NewDecoder(resp.Body).Decode(&battle); err != nil {
		t.Fatal(err)
	}
	if len(battle.Replay.Frames) == 0 {
		t.Error("リプレイのフレームが空")
	}
	if battle.Winner < -1 || battle.Winner > 1 {
		t.Errorf("勝者 index が不正: %d", battle.Winner)
	}

	resp2, err := http.Get(ts.URL + "/api/inbox?owner=bob")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var inbox []store.Battle
	if err := json.NewDecoder(resp2.Body).Decode(&inbox); err != nil {
		t.Fatal(err)
	}
	if len(inbox) != 1 {
		t.Errorf("bob の受信箱 = %d 件, want 1", len(inbox))
	}
}

func TestCreateRobotRejectsInvalidBuild(t *testing.T) {
	ts := newServer(t)
	bad := validBuild()
	bad.Parts[0].SlotCost = 99 // スロット超過
	resp := postJSON(t, ts.URL+"/api/robots", map[string]any{"owner": "alice", "name": "X", "build": bad})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("不正ビルドのステータス = %d, want 400", resp.StatusCode)
	}
}

func TestChallengeUnknownRobotReturns404(t *testing.T) {
	ts := newServer(t)
	resp := postJSON(t, ts.URL+"/api/challenge", map[string]any{"challenger_id": "nope", "opponent_id": "nope2"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("不明ロボのステータス = %d, want 404", resp.StatusCode)
	}
}
