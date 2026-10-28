package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestConsultationReportStructureAndEvidence(t *testing.T) {
	raw := `一、依据与范围

【依据：学员陈述】目前使用Excel和RPA，企业系统名称待确认。

二、步骤一：识别需求

【优势】能够导出需求。

【局限及实际问题】邮件格式不一致。

【可行建议】先约定字段。

【验证指标】字段完整率需先实测。

三、步骤二：关联数据

【仍待确认】业务主键。

四、实施方案

【阶段一：建立基线】先计时。

【阶段二：受控试点】保留人工审批。

五、验证

【效率指标】建议采用每日核对耗时作为主指标。学员陈述的约2小时可暂作基线假设，减少一半意味着目标约1小时，但须先实测。`
	wrapped, _ := json.Marshal(map[string]string{"analysis": raw})
	incomplete := strings.TrimSuffix(string(wrapped), "}")
	r := buildConsultationReport(incomplete, "测试报告")
	if len(r.Sections) != 5 || len(r.Flows) != 2 || len(r.Charts) != 2 || len(r.Warnings) != 1 {
		t.Fatalf("unexpected structure: sections=%d flows=%d charts=%d warnings=%v", len(r.Sections), len(r.Flows), len(r.Charts), r.Warnings)
	}
	if r.Raw != raw {
		t.Fatal("report changed source text")
	}
	if r.Charts[0].Points[0].Value != 2 || r.Charts[0].Points[1].Value != 1 || !strings.Contains(r.Charts[0].Note, "不代表已经实现") {
		t.Fatal("unverified baseline/target incorrectly represented")
	}
	zero := buildConsultationReport("metrics=[0,0]，warnings=null，不能推断绩效。", "无数值依据")
	if len(zero.Charts) != 0 {
		t.Fatal("fabricated metrics from zero placeholders")
	}
	short := buildConsultationReport("<script>alert(1)</script>\n\n请先核对需求日期。", "短回复")
	if !strings.Contains(short.Raw, "<script>") {
		t.Fatal("source should remain available for escaped frontend rendering")
	}
}
func TestConsultationMultiRoundAndPrivateDrafts(t *testing.T) {
	s, cfg := newTestServer(t)
	a := reviewFixture(t, s)
	v := submittedReview(t, s, a)
	ticket, _ := s.getConsultation(v.TicketID)
	path := "/api/consultations/" + ticket.ID + "/questions"
	body := map[string]string{"text": "请说明如何统计核对耗时？", "context": "验证指标", "clientId": "first-request"}
	mustStatus(t, request(s, "POST", path, "TG01", body), 409)
	ticket = adminTicketOp(t, s, ticket, "publish", "一、实施建议\n\n【可行建议】先记录基线，再做小范围试点。")
	mustStatus(t, request(s, "POST", path, "TG02", body), 404)
	mustStatus(t, request(s, "POST", path, "admin-test", body), 403)
	for round := 0; round < 4; round++ {
		in := map[string]string{"text": "这一轮请进一步解释操作与证据。", "context": "数据校验", "clientId": string(rune('a' + round))}
		w := request(s, "POST", path, "TG01", in)
		mustStatus(t, w, 200)
		mustStatus(t, request(s, "POST", path, "TG01", in), 200)
		ticket, _ = s.getConsultation(ticket.ID)
		if len(ticket.Questions) != round+1 {
			t.Fatal("duplicate question or lost round")
		}
		q := ticket.Questions[round]
		draft := map[string]any{"action": "save-draft", "revision": ticket.Revision, "questionId": q.ID, "draft": "PRIVATE-ROUND-DRAFT"}
		mustStatus(t, request(s, "POST", "/api/admin/consultations/"+ticket.ID, "admin-test", draft), 200)
		public := request(s, "GET", "/api/history/"+a.ID+"/review", "TG01", nil)
		if strings.Contains(public.Body.String(), "PRIVATE-ROUND-DRAFT") || strings.Contains(public.Body.String(), "drafts") {
			t.Fatal("question-specific private draft leaked")
		}
		ticket, _ = s.getConsultation(ticket.ID)
		prompt := request(s, "POST", "/api/admin/consultations/"+ticket.ID, "admin-test", map[string]any{"action": "export-prompt", "revision": ticket.Revision, "questionId": q.ID})
		mustStatus(t, prompt, 200)
		if !strings.Contains(prompt.Body.String(), q.ID) || !strings.Contains(prompt.Body.String(), "先记录基线") {
			t.Fatal("multi-turn context missing")
		}
		mustStatus(t, request(s, "POST", "/api/admin/consultations/"+ticket.ID, "admin-test", map[string]any{"action": "publish", "revision": ticket.Revision, "questionId": q.ID, "draft": "根据本轮追问：记录开始时间、结束时间和处理条数，保留人工复核。"}), 200)
		ticket, _ = s.getConsultation(ticket.ID)
		if ticket.Status != "returned" || pendingQuestions(ticket) != 0 || ticket.Replies[len(ticket.Replies)-1].QuestionID != q.ID {
			t.Fatal("reply not linked to question")
		}
	}
	if len(ticket.Replies) != 5 {
		t.Fatal("not all discussion rounds retained")
	}
	// The next question reopens the queue; a private draft for one topic cannot overwrite another.
	mustStatus(t, request(s, "POST", path, "TG01", map[string]string{"text": "请再解释责任人如何安排？", "clientId": "next-round"}), 200)
	ticket, _ = s.getConsultation(ticket.ID)
	if ticket.Status != "submitted" {
		t.Fatal("follow-up did not re-open administrator queue")
	}
	if len(ticket.Drafts) != 5 {
		t.Fatalf("topic drafts not isolated: %d", len(ticket.Drafts))
	}
	preview := request(s, "POST", "/api/admin/consultations/"+ticket.ID, "admin-test", map[string]any{"action": "preview-report", "revision": ticket.Revision, "draft": "一、建议\n\n【可行建议】先确定责任人。"})
	mustStatus(t, preview, 200)
	saved, _ := s.getConsultation(ticket.ID)
	if saved.Revision != ticket.Revision {
		t.Fatal("preview changed saved draft")
	}
	s.Close()
	again, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	again.sessions = s.sessions
	saved, _ = again.getConsultation(ticket.ID)
	if len(saved.Questions) != 5 || len(saved.Replies) != 5 {
		t.Fatal("discussion history lost after restart")
	}
	exam := &ExamAttempt{ID: "discussion-exam", Learner: "TG01", Status: "draft", Deadline: time.Now().Add(time.Hour)}
	again.saveExamAttempt(exam)
	mustStatus(t, request(again, "POST", path, "TG01", map[string]string{"text": "考试中尝试追问"}), 403)
}
