package app

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func manageLearners(s *Server, kind string, extra map[string]any) *httptest.ResponseRecorder {
	body := map[string]any{"revision": s.state.Revision, "kind": kind}
	for k, v := range extra {
		body[k] = v
	}
	return request(s, "POST", "/api/admin/learners", "admin-test", body)
}
func TestLearnerManagementPasswordsAndPersistence(t *testing.T) {
	s, cfg := newTestServer(t)
	credentials, err := os.ReadFile(filepath.Join(cfg.DataDir, "bootstrap-credentials.csv"))
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := csv.NewReader(bytes.NewReader(credentials)).ReadAll()
	oldPassword := rows[1][1]
	mutate(t, s, "modules-open", "", []string{"TG01"}, []string{"M02"})
	original := decodeAttempt(t, request(s, "POST", "/api/training/M02/start", "TG01", map[string]any{}))
	mustStatus(t, request(s, "GET", "/api/admin/learners", "TG01", nil), 403)
	mustStatus(t, request(s, "POST", "/api/admin/learners", "projector-test", map[string]any{}), 403)
	mustStatus(t, manageLearners(s, "update", map[string]any{"id": "TG01", "name": "张同学", "department": "采购一组"}), 200)
	mustStatus(t, request(s, "POST", "/api/admin/learners", "admin-test", map[string]any{"revision": 1, "kind": "update", "id": "TG01"}), 409)
	mustStatus(t, manageLearners(s, "reset-passwords", map[string]any{"ids": []string{"admin"}}), 422)
	mustStatus(t, manageLearners(s, "reset-passwords", map[string]any{"ids": []string{"TG01", "unknown"}}), 422)
	mustStatus(t, request(s, "GET", "/api/me", "TG01", nil), 200)
	mustStatus(t, manageLearners(s, "reset-passwords", map[string]any{"ids": []string{"TG01"}, "password": "short"}), 422)
	mustStatus(t, manageLearners(s, "reset-passwords", map[string]any{"ids": []string{"TG01"}, "password": "中文三"}), 422)
	mustStatus(t, request(s, "POST", "/api/password", "admin-test", map[string]string{"current": cfg.AdminPassword, "next": "123456"}), 422)
	password := "123456"
	mustStatus(t, manageLearners(s, "reset-passwords", map[string]any{"ids": []string{"TG01", "TG02"}, "password": password}), 422)
	w := manageLearners(s, "reset-passwords", map[string]any{"ids": []string{"TG01"}, "password": password})
	mustStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), password) {
		t.Fatal("new credential not returned once")
	}
	mustStatus(t, request(s, "GET", "/api/me", "TG01", nil), 401)
	mustStatus(t, request(s, "GET", "/api/me", "TG02", nil), 200)
	mustStatus(t, request(s, "POST", "/learner/api/login", "", map[string]string{"id": "TG01", "password": oldPassword}), 401)
	login := request(s, "POST", "/learner/api/login", "", map[string]string{"id": "TG01", "password": password})
	mustStatus(t, login, 200)
	a, err := s.getAttempt(original.ID)
	if err != nil || a.Learner != "TG01" || a.Status != "draft" {
		t.Fatal("password reset changed training")
	}
	view := request(s, "GET", "/api/admin/learners", "admin-test", nil)
	mustStatus(t, view, 200)
	if strings.Contains(view.Body.String(), password) || strings.Contains(view.Body.String(), `"hash"`) || strings.Contains(view.Body.String(), `"salt"`) {
		t.Fatal("credential leaked into management list")
	}
	state, _ := json.Marshal(s.state)
	if bytes.Contains(state, []byte(password)) {
		t.Fatal("plaintext password persisted in state or audit")
	}
	w = manageLearners(s, "reset-passwords", map[string]any{"ids": []string{"TG02", "TG03"}})
	mustStatus(t, w, 200)
	var result struct {
		Credentials []struct {
			ID       string `json:"id"`
			Password string `json:"password"`
		} `json:"credentials"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result.Credentials) != 2 || result.Credentials[0].Password == result.Credentials[1].Password {
		t.Fatal("batch reset requires unique per-user passwords")
	}
	mustStatus(t, request(s, "GET", "/api/me", "TG02", nil), 401)
	s.Close()
	reopened, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.state.Learners[0].Name != "张同学" || reopened.state.Learners[0].Department != "采购一组" {
		t.Fatal("profile not restored")
	}
	login = request(reopened, "POST", "/learner/api/login", "", map[string]string{"id": "TG01", "password": password})
	mustStatus(t, login, 200)
	token := login.Result().Cookies()[0].Value
	mustStatus(t, request(reopened, "POST", "/learner/api/password", token, map[string]string{"current": password, "next": "12345"}), 422)
	mustStatus(t, request(reopened, "POST", "/learner/api/password", token, map[string]string{"current": password, "next": "abcdef"}), 200)
	mustStatus(t, request(reopened, "GET", "/learner/api/me", token, nil), 401)
	mustStatus(t, request(reopened, "POST", "/learner/api/login", "", map[string]string{"id": "TG01", "password": "abcdef"}), 200)
}
func TestLearnerCapacityAndDisable(t *testing.T) {
	s, cfg := newTestServer(t)
	mutate(t, s, "page-grants", "P01", []string{"TG10"}, []string{"M01"})
	w := manageLearners(s, "create", map[string]any{"id": "TG11", "name": "新学员"})
	mustStatus(t, w, 422)
	mustStatus(t, manageLearners(s, "set-enabled", map[string]any{"id": "TG10", "enabled": false}), 200)
	if s.learnerEnabled("TG10") || len(s.state.Learners[9].Pages) != 0 || len(s.state.Learners[9].Modules) != 0 {
		t.Fatal("disable must revoke access")
	}
	mustStatus(t, request(s, "GET", "/api/me", "TG10", nil), 401)
	mustStatus(t, manageLearners(s, "create", map[string]any{"id": "tg01"}), 409)
	mustStatus(t, manageLearners(s, "create", map[string]any{"id": "<bad>"}), 422)
	password := "654321"
	mustStatus(t, manageLearners(s, "create", map[string]any{"id": "TG11", "name": "新学员", "password": password}), 200)
	mustStatus(t, request(s, "POST", "/learner/api/login", "", map[string]string{"id": "TG11", "password": password}), 200)
	mustStatus(t, manageLearners(s, "set-enabled", map[string]any{"id": "TG10", "enabled": true}), 422)
	mustStatus(t, request(s, "POST", "/api/admin/actions", "admin-test", Action{Kind: "page-grants", Revision: s.state.Revision, Page: "P01", Learners: []string{"TG10"}}), 400)
	view := request(s, "GET", "/api/admin/classroom", "admin-test", nil)
	var classroom struct {
		Learners []Learner `json:"learners"`
	}
	json.Unmarshal(view.Body.Bytes(), &classroom)
	if len(classroom.Learners) != 10 || slices.ContainsFunc(classroom.Learners, func(l Learner) bool { return l.Disabled }) {
		t.Fatal("classroom should list only enabled learners")
	}
	s.Close()
	reopened, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if len(reopened.state.Learners) != 11 || reopened.learnerEnabled("TG10") || !reopened.learnerEnabled("TG11") {
		t.Fatal("capacity and disabled states not restored")
	}
}
func TestLearnerManagementWriteFailure(t *testing.T) {
	s, _ := newTestServer(t)
	before, _ := json.Marshal(s.state)
	s.db.Close()
	mustStatus(t, manageLearners(s, "reset-passwords", map[string]any{"ids": []string{"TG01"}}), 500)
	after, _ := json.Marshal(s.state)
	if !bytes.Equal(before, after) {
		t.Fatal("failed reset changed in-memory credentials")
	}
	if _, ok := s.sessions["TG01"]; !ok {
		t.Fatal("failed reset invalidated existing session")
	}
}
