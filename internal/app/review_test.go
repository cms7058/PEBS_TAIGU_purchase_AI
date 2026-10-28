package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func reviewFixture(t *testing.T, s *Server) *TrainingAttempt {
	t.Helper()
	a := &TrainingAttempt{ID: "review-fixture", Learner: "TG01", Module: "M01", Number: 1, Status: "submitted", Rows: caseSpec("M01").Rows, Parameters: defaultParameters("M01"), StartedAt: nowUTC(), UpdatedAt: nowUTC()}
	if err := s.saveAttempt(a, "fixture", nil); err != nil {
		t.Fatal(err)
	}
	return a
}
func readReview(t *testing.T, w *httptest.ResponseRecorder) *EnterpriseReview {
	t.Helper()
	mustStatus(t, w, 200)
	var out struct {
		Review *EnterpriseReview `json:"review"`
	}
	if json.Unmarshal(w.Body.Bytes(), &out) != nil || out.Review == nil {
		t.Fatal("invalid review response")
	}
	return out.Review
}
func reviewOp(t *testing.T, s *Server, v *EnterpriseReview, kind string, extra map[string]any) *EnterpriseReview {
	t.Helper()
	in := map[string]any{"action": kind, "revision": v.Revision}
	for k, x := range extra {
		in[k] = x
	}
	return readReview(t, request(s, "POST", "/api/history/"+v.Attempt+"/review", "TG01", in))
}
func submittedReview(t *testing.T, s *Server, a *TrainingAttempt) *EnterpriseReview {
	t.Helper()
	v := readReview(t, request(s, "GET", "/api/history/"+a.ID+"/review", "TG01", nil))
	v = reviewOp(t, s, v, "save", map[string]any{"transcript": "企业用计划导出和Excel核对，版本变更依靠邮件，存在漏通知和重复录入问题。"})
	v = reviewOp(t, s, v, "manual-summary", nil)
	v = reviewOp(t, s, v, "confirm", map[string]any{"summary": EnterpriseSummary{Systems: "实际系统名称待确认", Functions: "计划导出和Excel核对", Problems: "版本变更漏通知和重复录入", Goals: "减少手工工作", Unknowns: "责任人和时效待确认"}})
	return reviewOp(t, s, v, "submit", nil)
}
func adminTicketOp(t *testing.T, s *Server, v *Consultation, kind, draft string) *Consultation {
	t.Helper()
	w := request(s, "POST", "/api/admin/consultations/"+v.ID, "admin-test", map[string]any{"action": kind, "revision": v.Revision, "draft": draft})
	mustStatus(t, w, 200)
	var out struct {
		Ticket *Consultation `json:"ticket"`
	}
	if json.Unmarshal(w.Body.Bytes(), &out) != nil || out.Ticket == nil {
		t.Fatal("missing ticket")
	}
	return out.Ticket
}
func TestEnterpriseReviewSubmissionLifecycle(t *testing.T) {
	s, cfg := newTestServer(t)
	a := reviewFixture(t, s)
	path := "/api/history/" + a.ID + "/review"
	before, _ := s.getAttempt(a.ID)
	snapshot, _ := json.Marshal(before)
	mustStatus(t, request(s, "GET", path, "TG02", nil), 404)
	mustStatus(t, request(s, "POST", path, "TG01", map[string]any{"action": "submit", "revision": 0}), 409)
	for _, kind := range []string{"analyze", "summarize", "export-analysis", "export-summary", "import-analysis", "import-summary"} {
		mustStatus(t, request(s, "POST", path, "TG01", map[string]any{"action": kind, "revision": 0}), 403)
	}
	v := submittedReview(t, s, a)
	if v.TicketID == "" {
		t.Fatal("submission not linked")
	}
	ticket, _ := s.getConsultation(v.TicketID)
	if ticket.Learner != "TG01" || ticket.Module != "M01" || ticket.Status != "submitted" {
		t.Fatal("incorrect routing")
	}
	v = reviewOp(t, s, v, "submit", nil)
	var count int
	s.db.QueryRow("SELECT count(*) FROM consultations").Scan(&count)
	if count != 1 {
		t.Fatal("duplicate submission")
	}
	mustStatus(t, request(s, "GET", "/api/admin/consultations", "TG01", nil), 403)
	mustStatus(t, request(s, "POST", "/api/admin/consultations/"+ticket.ID, "TG02", map[string]any{}), 403)
	list := request(s, "GET", "/api/admin/consultations", "admin-test", nil)
	mustStatus(t, list, 200)
	if !strings.Contains(list.Body.String(), "M06") {
		t.Fatal("six-module categorization absent")
	}
	ticket = adminTicketOp(t, s, ticket, "save-draft", "PRIVATE-DRAFT-DO-NOT-EXPOSE")
	view := request(s, "GET", path, "TG01", nil)
	if strings.Contains(view.Body.String(), "PRIVATE-DRAFT") || strings.Contains(view.Body.String(), "draftSource") {
		t.Fatal("administrator draft leaked")
	}
	prompt := request(s, "POST", "/api/admin/consultations/"+ticket.ID, "admin-test", map[string]any{"action": "export-prompt", "revision": ticket.Revision, "note": "先评估RPA的适用边界"})
	mustStatus(t, prompt, 200)
	if !strings.Contains(prompt.Body.String(), "版本变更漏通知") || !strings.Contains(prompt.Body.String(), "RPA") {
		t.Fatal("consultation missing submitted facts")
	}
	ticket = adminTicketOp(t, s, ticket, "publish", "管理员核对后的建议：先统一变更版本和通知责任，试点统计漏通知率。")
	if len(ticket.Replies) != 1 || ticket.Status != "returned" {
		t.Fatal("publication failed")
	}
	view = request(s, "GET", path, "TG01", nil)
	if !strings.Contains(view.Body.String(), "漏通知率") || strings.Contains(view.Body.String(), "PRIVATE-DRAFT") {
		t.Fatal("published reply missing or private data leaked")
	}
	chat := request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "软件利弊与适配建议", HistoryAttempt: a.ID})
	mustStatus(t, chat, 200)
	if !strings.Contains(chat.Body.String(), "管理员已返回") || !strings.Contains(chat.Body.String(), "漏通知率") {
		t.Fatal("assistant should show only published reply")
	}
	v = reviewOp(t, s, v, "confirm", map[string]any{"summary": EnterpriseSummary{Functions: "新增导出字段", Problems: "现在最关心的是交接耗时"}})
	v = reviewOp(t, s, v, "submit", nil)
	if v.TicketID == ticket.ID {
		t.Fatal("revised submission overwrote old ticket")
	}
	old, _ := s.getConsultation(ticket.ID)
	if old.Summary.Problems != "版本变更漏通知和重复录入" || len(old.Replies) != 1 {
		t.Fatal("submitted snapshot changed")
	}
	mustStatus(t, request(s, "POST", "/api/admin/consultations/"+ticket.ID, "admin-test", map[string]any{"action": "publish", "revision": 0, "draft": "wrong stale draft"}), 409)
	s.Close()
	again, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	saved, _ := again.getConsultation(ticket.ID)
	if len(saved.Replies) != 1 {
		t.Fatal("published reply lost after restart")
	}
	after, _ := again.getAttempt(a.ID)
	raw, _ := json.Marshal(after)
	if string(raw) != string(snapshot) {
		t.Fatal("review flow modified exercise")
	}
	again.sessions = s.sessions
	e := &ExamAttempt{ID: "consultation-exam", Learner: "TG01", Status: "draft", Deadline: time.Now().Add(time.Hour)}
	again.saveExamAttempt(e)
	mustStatus(t, request(again, "GET", path, "TG01", nil), 403)
}
func TestAdminConsultationModelAndRevision(t *testing.T) {
	s, _ := newTestServer(t)
	a := reviewFixture(t, s)
	v := submittedReview(t, s, a)
	ticket, _ := s.getConsultation(v.TicketID)
	mustStatus(t, request(s, "POST", "/api/admin/consultations/"+ticket.ID, "admin-test", map[string]any{"action": "consult", "revision": ticket.Revision}), 503)
	s.cfg.AIBaseURL = "https://model.example/v1"
	s.cfg.AIKey = "test-key"
	s.cfg.AIModel = "test-model"
	concurrent := false
	s.cfg.AIClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "版本变更漏通知") || !strings.Contains(string(raw), "confirmedSummary") {
			t.Error("model lacks submitted context")
		}
		if concurrent {
			s.mu.Lock()
			newer, _ := s.getConsultation(ticket.ID)
			newer.Draft = "另一个窗口保存的新草稿"
			s.saveConsultation(newer, "admin", "save-draft")
			s.mu.Unlock()
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"analysis\":\"MODEL-PRIVATE-DRAFT：根据学员确认内容提出待验证建议。\"}"}}],"usage":{"total_tokens":120}}`)), Header: http.Header{}}, nil
	})}
	ticket = adminTicketOp(t, s, ticket, "consult", "")
	if ticket.DraftSource != "后台模型 test-model（待管理员核对）" || len(ticket.Replies) != 0 {
		t.Fatal("model output must remain private draft")
	}
	w := request(s, "GET", "/api/history/"+a.ID+"/review", "TG01", nil)
	mustStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "MODEL-PRIVATE-DRAFT") {
		t.Fatal("unpublished model result exposed")
	}
	chat := request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "软件利弊", HistoryAttempt: a.ID})
	mustStatus(t, chat, 200)
	if strings.Contains(chat.Body.String(), "MODEL-PRIVATE") || !strings.Contains(chat.Body.String(), "管理员") {
		t.Fatal("assistant bypass")
	}
	concurrent = true
	mustStatus(t, request(s, "POST", "/api/admin/consultations/"+ticket.ID, "admin-test", map[string]any{"action": "consult", "revision": ticket.Revision}), 409)
	saved, _ := s.getConsultation(ticket.ID)
	if saved.Draft != "另一个窗口保存的新草稿" {
		t.Fatal("stale model result overwrote draft")
	}
}
func TestConsultationSubmissionAtomicity(t *testing.T) {
	s, _ := newTestServer(t)
	a := reviewFixture(t, s)
	v := readReview(t, request(s, "GET", "/api/history/"+a.ID+"/review", "TG01", nil))
	v = reviewOp(t, s, v, "save", map[string]any{"transcript": "目前用Excel整理需求，库存更新不及时，需要采购人员重复核对。"})
	v = reviewOp(t, s, v, "manual-summary", nil)
	v = reviewOp(t, s, v, "confirm", map[string]any{"summary": EnterpriseSummary{Functions: "Excel整理", Problems: "重复核对"}})
	_, err := s.db.Exec("CREATE TRIGGER reject_consultation BEFORE INSERT ON consultations BEGIN SELECT RAISE(ABORT,'test failure'); END")
	if err != nil {
		t.Fatal(err)
	}
	mustStatus(t, request(s, "POST", "/api/history/"+a.ID+"/review", "TG01", map[string]any{"action": "submit", "revision": v.Revision}), 500)
	saved, _ := s.getReview(a.ID)
	if saved.TicketID != "" || saved.Revision != v.Revision {
		t.Fatal("failed submission partially saved")
	}
}
