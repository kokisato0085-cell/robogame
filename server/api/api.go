// Package api は RoboGame の HTTP/REST API を提供する（BasicDesign §4）。
// 非同期チャレンジ制（登録 → 名簿 → 挑戦 → 受信箱）のエンドポイントを定義する。
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"robogame/server/sim"
	"robogame/server/store"
)

// Server は API ハンドラ群と保管庫を束ねる。
type Server struct {
	store *store.Store
}

// NewServer はルーティング済みの HTTP ハンドラを返す。
func NewServer(s *store.Store) http.Handler {
	srv := &Server{store: s}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/robots", srv.handleCreateRobot)
	mux.HandleFunc("GET /api/robots", srv.handleListRobots)
	mux.HandleFunc("POST /api/challenge", srv.handleChallenge)
	mux.HandleFunc("GET /api/inbox", srv.handleInbox)
	return withCORS(mux)
}

type createRobotRequest struct {
	Owner string    `json:"owner"`
	Name  string    `json:"name"`
	Build sim.Build `json:"build"`
}

func (s *Server) handleCreateRobot(w http.ResponseWriter, r *http.Request) {
	var req createRobotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON の解析に失敗しました")
		return
	}
	if strings.TrimSpace(req.Owner) == "" {
		writeError(w, http.StatusBadRequest, "owner は必須です")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name は必須です")
		return
	}
	if err := sim.ValidateBuild(req.Build); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	robot := s.store.AddRobot(req.Owner, req.Name, req.Build)
	writeJSON(w, http.StatusCreated, robot)
}

func (s *Server) handleListRobots(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListRobots())
}

type challengeRequest struct {
	ChallengerID string `json:"challenger_id"`
	OpponentID   string `json:"opponent_id"`
}

func (s *Server) handleChallenge(w http.ResponseWriter, r *http.Request) {
	var req challengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON の解析に失敗しました")
		return
	}
	challenger, ok := s.store.GetRobot(req.ChallengerID)
	if !ok {
		writeError(w, http.StatusNotFound, "挑戦者のロボットが見つかりません")
		return
	}
	opponent, ok := s.store.GetRobot(req.OpponentID)
	if !ok {
		writeError(w, http.StatusNotFound, "相手のロボットが見つかりません")
		return
	}
	// 挑戦者=index 0, 相手=index 1 として戦闘を計算する。
	replay := sim.Simulate(challenger.Build, opponent.Build)
	battle := s.store.RecordBattle(challenger, opponent, replay)
	writeJSON(w, http.StatusOK, battle)
}

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	owner := strings.TrimSpace(r.URL.Query().Get("owner"))
	if owner == "" {
		writeError(w, http.StatusBadRequest, "owner クエリは必須です")
		return
	}
	writeJSON(w, http.StatusOK, s.store.Inbox(owner))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// withCORS は開発時の別オリジン（Vite）からの呼び出しを許可する。本番はオリジンを絞る。
func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}
