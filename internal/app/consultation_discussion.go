package app

import (
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

func draftKey(questionID string) string {
	if questionID == "" {
		return "initial"
	}
	return questionID
}
func pendingQuestions(v *Consultation) int {
	n := 0
	for _, q := range v.Questions {
		answered := false
		for _, r := range v.Replies {
			if r.QuestionID == q.ID {
				answered = true
				break
			}
		}
		if !answered {
			n++
		}
	}
	return n
}
func normalizeConsultation(v *Consultation) {
	if v.Questions == nil {
		v.Questions = []ConsultationQuestion{}
	}
	if v.Drafts == nil {
		v.Drafts = map[string]ConsultationDraft{}
	}
	if v.Draft != "" {
		key := draftKey(v.DraftQuestionID)
		if _, ok := v.Drafts[key]; !ok {
			v.Drafts[key] = ConsultationDraft{v.Draft, v.DraftSource, v.PromptNote}
		}
	}
	for i := range v.Replies {
		r := &v.Replies[i]
		if r.ID == "" {
			r.ID = fmt.Sprintf("legacy-reply-%d", i+1)
		}
		if r.Report == nil {
			r.Report = buildConsultationReport(r.Text, "问题复盘与改进建议")
		}
	}
}
func (s *Server) learnerDiscussion(w http.ResponseWriter, r *http.Request, ss session) {
	if ss.Role != "learner" {
		fail(w, 403, "仅学员本人可发起追问")
		return
	}
	if r.Method != "POST" {
		fail(w, 405, "请使用提交追问操作")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[3] != "questions" {
		fail(w, 404, "讨论入口不存在")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.getConsultation(parts[2])
	if err != nil || v.Learner != ss.ID {
		fail(w, 404, "问题不存在")
		return
	}
	if _, code, msg := s.reviewAllowed(ss, v.Attempt); code != 200 {
		fail(w, code, msg)
		return
	}
	var in struct {
		Text     string `json:"text"`
		Context  string `json:"context"`
		ClientID string `json:"clientId"`
	}
	if !decode(w, r, &in) {
		return
	}
	text := strings.TrimSpace(in.Text)
	if utf8.RuneCountInString(text) < 2 || utf8.RuneCountInString(text) > 2000 || utf8.RuneCountInString(in.Context) > 200 || len(in.ClientID) > 100 {
		fail(w, 422, "追问需2–2000字，关联内容最多200字")
		return
	}
	if len(v.Replies) == 0 {
		fail(w, 409, "请等待管理员首次回复后，再围绕答复继续讨论")
		return
	}
	if in.ClientID != "" {
		for _, q := range v.Questions {
			if q.ClientID == in.ClientID {
				if q.Text != text || q.Context != strings.TrimSpace(in.Context) {
					fail(w, 409, "这条发送请求已用于其他内容，请重新提交")
					return
				}
				respond(w, 200, publicConsultation(v))
				return
			}
		}
	}
	v.Questions = append(v.Questions, ConsultationQuestion{ID: randomToken(), At: nowUTC().Format(timeFormat), Text: text, Context: strings.TrimSpace(in.Context), ClientID: in.ClientID})
	v.Status = "submitted"
	if s.saveConsultation(v, ss.ID, "ask") != nil {
		fail(w, 500, "追问未保存，请保留文字后重试")
		return
	}
	respond(w, 200, publicConsultation(v))
}
