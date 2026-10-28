package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReviewButtonStatusLifecycle(t *testing.T) {
	s, _ := newTestServer(t)
	a := reviewFixture(t, s)
	path := "/api/history/" + a.ID + "/review-status"
	check := func(code string, replies, pending int) {
		t.Helper()
		w := request(s, "GET", path, "TG01", nil)
		mustStatus(t, w, 200)
		var state ReviewButtonState
		if json.Unmarshal(w.Body.Bytes(), &state) != nil || state.Code != code || state.Replies != replies || state.Pending != pending {
			t.Fatalf("unexpected reply status: %s", w.Body.String())
		}
		if strings.Contains(w.Body.String(), "PRIVATE") || strings.Contains(w.Body.String(), "draft") {
			t.Fatal("private draft leaked in status")
		}
	}
	check("not-submitted", 0, 0)
	v := submittedReview(t, s, a)
	ticket, _ := s.getConsultation(v.TicketID)
	check("waiting", 0, 1)
	ticket = adminTicketOp(t, s, ticket, "save-draft", "PRIVATE-STATUS-DRAFT")
	check("processing", 0, 1)
	ticket = adminTicketOp(t, s, ticket, "publish", "管理员已经核对：请先统一字段与责任，再试点记录效率。")
	check("replied", 1, 0)
	mustStatus(t, request(s, "POST", "/api/consultations/"+ticket.ID+"/questions", "TG01", map[string]string{"text": "怎样安排试点责任人？", "clientId": "status-followup"}), 200)
	check("followup-waiting", 1, 1)
	ticket, _ = s.getConsultation(ticket.ID)
	mustStatus(t, request(s, "POST", "/api/admin/consultations/"+ticket.ID, "admin-test", map[string]any{"action": "publish", "revision": ticket.Revision, "questionId": ticket.Questions[0].ID, "draft": "先明确计划、采购和质量各自的复核责任，并由业务负责人核对。"}), 200)
	check("replied", 2, 0)
	mustStatus(t, request(s, "GET", path, "TG02", nil), 404)
	detail := request(s, "GET", "/api/history/"+a.ID, "TG01", nil)
	mustStatus(t, detail, 200)
	if !strings.Contains(detail.Body.String(), `"reviewState"`) || !strings.Contains(detail.Body.String(), `"code":"replied"`) {
		t.Fatal("history does not include current reply state")
	}
}
