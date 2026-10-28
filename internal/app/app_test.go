package app

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func request(s *Server, method, path, token string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func newTestServer(t *testing.T) (*Server, Config) {
	t.Helper()
	cfg := Config{DataDir: t.TempDir(), AdminPassword: "classroom-admin-test-password"}
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	t.Cleanup(func() { s.Close() })
	s.sessions["admin-test"] = session{ID: "admin", Role: "admin", LastSeen: now, Expires: now.Add(time.Hour)}
	for i := 1; i <= 10; i++ {
		id := fmt.Sprintf("TG%02d", i)
		s.sessions[id] = session{ID: id, Role: "learner", LastSeen: now, Expires: now.Add(time.Hour)}
	}
	s.sessions["projector-test"] = session{ID: "projection", Role: "projector", LastSeen: now, Expires: now.Add(time.Hour)}
	return s, cfg
}
func mustStatus(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("got %d want %d: %s", w.Code, status, w.Body.String())
	}
}
func mutate(t *testing.T, s *Server, kind, page string, learners, mods []string) {
	t.Helper()
	w := request(s, "POST", "/api/admin/actions", "admin-test", Action{Revision: s.state.Revision, Kind: kind, Page: page, Learners: learners, Modules: mods})
	mustStatus(t, w, 200)
}

func TestClassroomAuthorizationAndRecovery(t *testing.T) {
	s, cfg := newTestServer(t)
	mustStatus(t, request(s, "GET", "/api/admin/classroom", "", nil), 401)
	mustStatus(t, request(s, "GET", "/api/admin/classroom", "TG01", nil), 403)
	mustStatus(t, request(s, "GET", "/api/slides/P01", "TG01", nil), 403)
	// Login exists but TG09/TG10 have no recent heartbeat: online-only must exclude them.
	for _, id := range []string{"TG09", "TG10"} {
		ss := s.sessions[id]
		ss.LastSeen = time.Now().Add(-time.Minute)
		s.sessions[id] = ss
	}
	mustStatus(t, request(s, "POST", "/api/admin/actions", "admin-test", Action{Revision: s.state.Revision, Kind: "page-grants", Page: "P01", OnlineOnly: true}), 200)
	for i := 1; i <= 8; i++ {
		mustStatus(t, request(s, "GET", "/api/slides/P01", fmt.Sprintf("TG%02d", i), nil), 200)
	}
	mustStatus(t, request(s, "GET", "/api/slides/P01", "TG09", nil), 403)
	mutate(t, s, "modules-open", "", []string{"TG01", "TG04"}, []string{"M01"})
	mutate(t, s, "page-grants", "P02", []string{"TG01", "TG04"}, nil)
	mutate(t, s, "project", "P01", nil, nil)
	// Replace the eight-person list, rather than unioning it with three people.
	mutate(t, s, "page-grants", "P01", []string{"TG01", "TG02", "TG03"}, nil)
	mustStatus(t, request(s, "GET", "/api/slides/P01", "TG04", nil), 403)
	mustStatus(t, request(s, "GET", "/api/slides/P02", "TG04", nil), 200)
	mustStatus(t, request(s, "GET", "/api/modules/M01", "TG04", nil), 200)
	if s.state.Projection != "P01" {
		t.Fatal("learner grants changed projection")
	}
	mutate(t, s, "page-grants", "P01", []string{"TG01", "TG10"}, nil)
	// Close is global even if payload contains only one selected learner.
	mutate(t, s, "page-close", "P01", []string{"TG01"}, nil)
	for _, id := range []string{"TG01", "TG10"} {
		mustStatus(t, request(s, "GET", "/api/slides/P01", id, nil), 403)
	}
	if s.state.Projection != "" {
		t.Fatal("closed page remained projected")
	}
	mustStatus(t, request(s, "GET", "/api/modules/M01", "TG01", nil), 200)
	mutate(t, s, "page-grants", "P02", []string{}, nil)
	mustStatus(t, request(s, "GET", "/api/slides/P02", "TG04", nil), 403)
	// Read-only projection token cannot become an administrator or learner.
	mustStatus(t, request(s, "GET", "/api/projection", "projector-test", nil), 200)
	mustStatus(t, request(s, "GET", "/api/admin/classroom", "projector-test", nil), 403)
	mustStatus(t, request(s, "GET", "/api/slides/P03", "projector-test", nil), 403)
	mustStatus(t, request(s, "GET", "/api/projection", "TG01", nil), 403)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	rev := s.state.Revision
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- request(s, "POST", "/api/admin/actions", "admin-test", Action{Revision: rev, Kind: "project", Page: "P03"}).Code
		}()
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for c := range codes {
		counts[c]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("lost update guard: %v", counts)
	}
	before := s.state.Revision
	s.db.Close()
	mustStatus(t, request(s, "POST", "/api/admin/actions", "admin-test", Action{Revision: before, Kind: "pause"}), 500)
	if s.state.Revision != before || s.state.Projection != "P03" {
		t.Fatal("failed save mutated in-memory state")
	}
	restored, err := New(Config{DataDir: cfg.DataDir})
	if err != nil {
		t.Fatal(err)
	}
	if restored.state.Revision != s.state.Revision || restored.state.Projection != "P03" {
		t.Fatal("persistent state not restored")
	}
	t.Cleanup(func() { restored.Close() })
	if len(restored.state.Audit) == 0 {
		t.Fatal("missing audit")
	}
	mustStatus(t, request(restored, "GET", "/api/admin/classroom", "admin-test", nil), 401)
}

func TestLoginAndRequestBoundary(t *testing.T) {
	s, cfg := newTestServer(t)
	data, err := os.ReadFile(filepath.Join(cfg.DataDir, "bootstrap-credentials.csv"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil || len(rows) != 11 {
		t.Fatalf("credential generation: %v", err)
	}
	w := request(s, "POST", "/api/login", "", map[string]string{"id": rows[1][0], "password": rows[1][1]})
	mustStatus(t, w, 200)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("cookie flags")
	}
	token := cookies[0].Value
	me := request(s, "GET", "/api/me", token, nil)
	mustStatus(t, me, 200)
	if strings.Contains(me.Body.String(), "hash") || strings.Contains(me.Body.String(), "salt") || strings.Contains(me.Body.String(), rows[1][1]) {
		t.Fatal("credentials leaked")
	}
	state, err := json.Marshal(s.state)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(state, []byte(rows[1][1])) || bytes.Contains(state, []byte(cfg.AdminPassword)) {
		t.Fatal("plaintext password in state")
	}
	mustStatus(t, request(s, "POST", "/api/logout", token, map[string]any{}), 200)
	mustStatus(t, request(s, "GET", "/api/me", token, nil), 401)
	for range 5 {
		mustStatus(t, request(s, "POST", "/api/login", "", map[string]string{"id": "invalid", "password": "bad"}), 401)
	}
	mustStatus(t, request(s, "POST", "/api/login", "", map[string]string{"id": "invalid", "password": "bad"}), 429)
	mustStatus(t, request(s, "POST", "/api/admin/actions", "admin-test", Action{Revision: s.state.Revision, Kind: "page-grants", Page: "P01", Learners: []string{"invalid"}}), 400)
	r := httptest.NewRequest("POST", "/api/logout", strings.NewReader("{}"))
	r.Header.Set("Origin", "https://different.example")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	mustStatus(t, w, 403)
	r = httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"id":"admin","password":"x"}`))
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	mustStatus(t, w, 415)
	for _, path := range []string{"/", "/projector", "/app.js", "/style.css", "/healthz"} {
		mustStatus(t, request(s, "GET", path, "", nil), 200)
	}
}

func TestPortalSessionsAndRoleBoundary(t *testing.T) {
	s, cfg := newTestServer(t)
	data, err := os.ReadFile(filepath.Join(cfg.DataDir, "bootstrap-credentials.csv"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	admin := map[string]string{"id": "admin", "password": cfg.AdminPassword}
	learner := map[string]string{"id": rows[1][0], "password": rows[1][1]}
	for _, tc := range []struct {
		portal string
		input  map[string]string
	}{{"admin", learner}, {"learner", admin}} {
		w := request(s, "POST", "/"+tc.portal+"/api/login", "", tc.input)
		mustStatus(t, w, 403)
		if len(w.Result().Cookies()) != 0 {
			t.Fatal("wrong portal issued a session cookie")
		}
	}
	adminLogin := request(s, "POST", "/admin/api/login", "", admin)
	learnerLogin := request(s, "POST", "/learner/api/login", "", learner)
	mustStatus(t, adminLogin, 200)
	mustStatus(t, learnerLogin, 200)
	cookies := []*http.Cookie{adminLogin.Result().Cookies()[0], learnerLogin.Result().Cookies()[0]}
	for i, role := range []string{"admin", "learner"} {
		if cookies[i].Name != "taigu_"+role+"_session" || cookies[i].Path != "/"+role || !cookies[i].HttpOnly {
			t.Fatalf("wrong portal cookie for %s", role)
		}
	}
	call := func(method, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader("{}"))
		for _, c := range cookies {
			r.AddCookie(c)
		}
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	for _, role := range []string{"admin", "learner"} {
		w := call("GET", "/"+role+"/api/me")
		mustStatus(t, w, 200)
		var me struct {
			Role string `json:"role"`
		}
		if json.Unmarshal(w.Body.Bytes(), &me) != nil || me.Role != role {
			t.Fatalf("portal selected wrong identity: %s", w.Body.String())
		}
	}
	mustStatus(t, call("GET", "/learner/api/admin/classroom"), 403)
	mustStatus(t, request(s, "GET", "/admin/api/me", "TG01", nil), 403)
	mustStatus(t, request(s, "GET", "/learner/api/me", "admin-test", nil), 403)
	mustStatus(t, request(s, "GET", "/admin/api/me", "", nil), 401)
	logout := call("POST", "/admin/api/logout")
	mustStatus(t, logout, 200)
	expired := logout.Result().Cookies()[0]
	if expired.Name != cookies[0].Name || expired.Path != cookies[0].Path || expired.MaxAge != -1 {
		t.Fatal("logout did not clear only the administrator cookie")
	}
	mustStatus(t, call("GET", "/admin/api/me"), 401)
	mustStatus(t, call("GET", "/learner/api/me"), 200)
	for _, path := range []string{"/", "/admin", "/admin/", "/learner", "/learner/"} {
		w := call("GET", path)
		mustStatus(t, w, 200)
		if !strings.Contains(w.Body.String(), "请选择您的身份") {
			t.Fatalf("portal page unavailable: %s", path)
		}
	}
}
