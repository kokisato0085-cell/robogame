// Package store はロボットと対戦結果をメモリ上で保持する（段階1）。
// 再起動で消える。永続化（MySQL）は段階3で導入する。
// HTTP ハンドラから並行アクセスされるため Mutex で保護する。
package store

import (
	"fmt"
	"sync"

	"robogame/server/sim"
)

// Robot は名簿に登録された1体（所有者・名前付き）。
type Robot struct {
	ID    string    `json:"id"`
	Owner string    `json:"owner"`
	Name  string    `json:"name"`
	Build sim.Build `json:"build"`
}

// Battle は1回の対戦結果。Replay に全経過を含む。
type Battle struct {
	ID         string     `json:"id"`
	Challenger Robot      `json:"challenger"`
	Opponent   Robot      `json:"opponent"`
	Winner     int        `json:"winner"` // 0=挑戦者/1=相手/-1=引き分け
	Reason     string     `json:"reason"`
	Replay     sim.Replay `json:"replay"`
}

// Store はインメモリの保管庫。
type Store struct {
	mu        sync.Mutex
	robots    map[string]Robot
	order     []string // 一覧表示の登録順
	battles   map[string]Battle
	inbox     map[string][]string // 所有者 → 自分のロボが挑まれた対戦ID群
	robotSeq  int
	battleSeq int
}

// New は空の Store を返す。
func New() *Store {
	return &Store{
		robots:  make(map[string]Robot),
		battles: make(map[string]Battle),
		inbox:   make(map[string][]string),
	}
}

// AddRobot はロボットを登録し採番済みの Robot を返す。
func (s *Store) AddRobot(owner, name string, build sim.Build) Robot {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.robotSeq++
	r := Robot{ID: fmt.Sprintf("r%d", s.robotSeq), Owner: owner, Name: name, Build: build}
	s.robots[r.ID] = r
	s.order = append(s.order, r.ID)
	return r
}

// ListRobots は登録順でロボット一覧を返す。
func (s *Store) ListRobots() []Robot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Robot, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.robots[id])
	}
	return out
}

// GetRobot は ID でロボットを引く。
func (s *Store) GetRobot(id string) (Robot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.robots[id]
	return r, ok
}

// RecordBattle は対戦結果を保存し、相手（防衛側）の受信箱に登録する。
func (s *Store) RecordBattle(challenger, opponent Robot, replay sim.Replay) Battle {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.battleSeq++
	b := Battle{
		ID:         fmt.Sprintf("b%d", s.battleSeq),
		Challenger: challenger,
		Opponent:   opponent,
		Winner:     replay.Winner,
		Reason:     replay.Reason,
		Replay:     replay,
	}
	s.battles[b.ID] = b
	s.inbox[opponent.Owner] = append(s.inbox[opponent.Owner], b.ID)
	return b
}

// Inbox は owner の受信箱（自分のロボが挑まれた対戦）を登録順で返す。
func (s *Store) Inbox(owner string) []Battle {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.inbox[owner]
	out := make([]Battle, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.battles[id])
	}
	return out
}
