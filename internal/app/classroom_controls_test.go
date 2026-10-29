package app

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestBatchPageActivationPreservesOtherGrants(t *testing.T) {
	s, _ := newTestServer(t)
	mutate(t, s, "page-grants", "P01", []string{"TG01", "TG02"}, nil)
	mutate(t, s, "page-close", "P03", nil, nil)
	rev := s.state.Revision
	mustStatus(t, request(s, "POST", "/admin/api/admin/actions", "admin-test", Action{Kind: "pages-activate", Revision: rev, Pages: []string{"P02", "P03"}, Learners: []string{"TG01"}}), 200)
	if s.state.Revision != rev+1 {
		t.Fatal("batch was not a single revision")
	}
	for _, l := range s.state.Learners {
		if l.ID == "TG01" && (!slices.Contains(l.Pages, "P01") || !slices.Contains(l.Pages, "P02") || !slices.Contains(l.Pages, "P03")) {
			t.Fatal("missing granted pages")
		}
		if l.ID == "TG02" && (len(l.Pages) != 1 || l.Pages[0] != "P01") {
			t.Fatal("changed another learner's grants")
		}
	}
	if slices.Contains(s.state.Closed, "P03") {
		t.Fatal("reactivated page still closed")
	}
	rev = s.state.Revision
	for _, pages := range [][]string{{"P02", "unknown"}, {"P02", "P02"}, nil} {
		mustStatus(t, request(s, "POST", "/admin/api/admin/actions", "admin-test", Action{Kind: "pages-activate", Revision: rev, Pages: pages, Learners: []string{"TG01"}}), 400)
		if s.state.Revision != rev {
			t.Fatal("invalid batch changed classroom")
		}
	}
	mustStatus(t, request(s, "POST", "/learner/api/admin/actions", "TG01", Action{Kind: "pages-activate", Revision: rev, Pages: []string{"P04"}, Learners: []string{"TG01"}}), 403)
	mustStatus(t, request(s, "POST", "/admin/api/admin/actions", "admin-test", Action{Kind: "pages-activate", Revision: rev - 1, Pages: []string{"P04"}, Learners: []string{"TG01"}}), 409)
}

func TestProjectionNavigationScopeAndBoundaries(t *testing.T) {
	s, _ := newTestServer(t)
	mutate(t, s, "project", "P01", nil, nil)
	before, _ := json.Marshal(s.state.Learners)
	move := func(token, direction string, revision int) int {
		return request(s, "POST", "/api/projection", token, map[string]any{"revision": revision, "direction": direction}).Code
	}
	if move("TG01", "next", s.state.Revision) != 403 || move("projector-test", "next", s.state.Revision) != 403 {
		t.Fatal("unprivileged projection control allowed")
	}
	w := request(s, "POST", "/admin/api/admin/projection-token", "admin-test", map[string]any{})
	mustStatus(t, w, 200)
	var auth struct {
		Token string `json:"token"`
	}
	json.Unmarshal(w.Body.Bytes(), &auth)
	if move(auth.Token, "previous", s.state.Revision) != 422 {
		t.Fatal("moved before first page")
	}
	rev := s.state.Revision
	if move(auth.Token, "next", rev) != 200 || s.state.Projection != "P02" {
		t.Fatal("failed to advance projection")
	}
	if move(auth.Token, "next", rev) != 409 {
		t.Fatal("accepted stale navigation")
	}
	if move(auth.Token, "previous", s.state.Revision) != 200 || s.state.Projection != "P01" {
		t.Fatal("failed to return to previous page")
	}
	mustStatus(t, request(s, "GET", "/api/admin/classroom", auth.Token, nil), 403)
	after, _ := json.Marshal(s.state.Learners)
	if string(before) != string(after) {
		t.Fatal("projection navigation changed learner grants")
	}
	mutate(t, s, "pause", "", nil, nil)
	if move(auth.Token, "next", s.state.Revision) != 409 {
		t.Fatal("navigated paused projection")
	}
}
