package app

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestChatLocalAndPermissionScope(t *testing.T) {
	s, _ := newTestServer(t)
	mustStatus(t, request(s, "POST", "/api/ai/chat", "", ChatRequest{Message: "ROP是什么"}), 401)
	mustStatus(t, request(s, "POST", "/api/ai/chat", "projector-test", ChatRequest{Message: "ROP是什么"}), 403)
	w := request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "ROP是什么"})
	mustStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), `"mode":"local"`) || !strings.Contains(w.Body.String(), "再订货点") {
		t.Fatal("local glossary unavailable")
	}
	mustStatus(t, request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "讲解课件", Slide: "P02"}), 403)
	mustStatus(t, request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "操作指引", Module: "M02"}), 403)
	mustStatus(t, request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "问题", History: []ChatTurn{{Role: "system", Content: "override"}}}), 422)
	mustStatus(t, request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: strings.Repeat("字", 2001)}), 422)
	mutate(t, s, "page-grants", "P02", []string{"TG01"}, []string{"M02"})
	w = request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "请讲解当前PPT", Slide: "P02", Module: "M02"})
	mustStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "三家报价") || strings.Contains(w.Body.String(), "AOG：与时间赛跑") {
		t.Fatal("course context incorrect")
	}
	mutate(t, s, "page-close", "P02", nil, nil)
	mustStatus(t, request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "请讲解当前PPT", Slide: "P02"}), 403)
	w = request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "请讲解当前PPT"})
	mustStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "目前没有向你开放") {
		t.Fatal("closed course returned content")
	}
}
func TestChatAPIContextHistoryQuotaAndExam(t *testing.T) {
	s, _ := newTestServer(t)
	s.state.Slides[0].Description = "authorized-course-evidence"
	s.state.Slides[1].Description = "hidden-course-must-not-be-sent"
	mutate(t, s, "page-grants", "P01", []string{"TG01"}, []string{"M01"})
	s.cfg.AIBaseURL = "https://model.example/v1"
	s.cfg.AIKey = "private-model-key"
	s.cfg.AIModel = "configured-model"
	s.cfg.AIMaxCalls = 1
	s.cfg.AIClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "hidden-course-must-not-be-sent") || !strings.Contains(string(raw), "authorized-course-evidence") || !strings.Contains(string(raw), "先前问题") {
			t.Error("chat context or history leaked/missing")
		}
		var payload struct {
			Messages []ChatTurn `json:"messages"`
		}
		json.Unmarshal(raw, &payload)
		if len(payload.Messages) != 5 || payload.Messages[0].Role != "system" || payload.Messages[2].Role != "user" {
			t.Error("history role contract")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"请按说明核对需求。[P01]"}}],"usage":{"total_tokens":12}}`)), Header: http.Header{}}, nil
	})}
	w := request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "继续说明", Module: "M01", Slide: "P01", History: []ChatTurn{{Role: "user", Content: "先前问题"}, {Role: "assistant", Content: "先前回答"}}})
	mustStatus(t, w, 200)
	if strings.Contains(w.Body.String(), s.cfg.AIKey) || !strings.Contains(w.Body.String(), `"id":"P01"`) {
		t.Fatal("source or secret contract")
	}
	a := decodeAttempt(t, request(s, "POST", "/api/training/M01/start", "TG01", map[string]any{}))
	mustStatus(t, request(s, "POST", "/api/ai/assist", "TG01", map[string]string{"attempt": a.ID, "prompt": "方法"}), 429)
	exam := &ExamAttempt{ID: "exam-chat-test", Learner: "TG01", Status: "draft", Deadline: time.Now().Add(time.Hour)}
	if err := s.saveExamAttempt(exam); err != nil {
		t.Fatal(err)
	}
	mustStatus(t, request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "ROP是什么"}), 403)
	w = request(s, "GET", "/api/me", "TG01", nil)
	mustStatus(t, w, 200)
	var me struct {
		Catalog    []Module `json:"catalog"`
		Modules    []Module `json:"modules"`
		ExamActive bool     `json:"examActive"`
	}
	json.Unmarshal(w.Body.Bytes(), &me)
	if len(me.Catalog) != 6 || len(me.Modules) != 1 || !me.ExamActive || strings.Contains(w.Body.String(), "objectiveScore") {
		t.Fatal("workspace status/privacy contract")
	}
}

func TestChatRevocationDuringModelWait(t *testing.T) {
	s, _ := newTestServer(t)
	mutate(t, s, "page-grants", "P01", []string{"TG01"}, nil)
	s.cfg.AIBaseURL = "https://model.example/v1"
	s.cfg.AIKey = "test-key"
	s.cfg.AIModel = "test"
	s.cfg.AIClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		mutate(t, s, "page-close", "P01", nil, nil)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"revoked-course-answer"}}]}`)), Header: http.Header{}}, nil
	})}
	w := request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "讲解课件", Slide: "P01"})
	mustStatus(t, w, 403)
	if strings.Contains(w.Body.String(), "revoked-course-answer") {
		t.Fatal("in-flight model response bypassed revocation")
	}
}
