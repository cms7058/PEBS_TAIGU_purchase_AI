package app

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func enterpriseTestOperation(t *testing.T, s *Server, a *TrainingAttempt, kind string, extra map[string]any) *TrainingAttempt {
	t.Helper()
	body := map[string]any{"kind": kind, "revision": a.Revision}
	for k, v := range extra {
		body[k] = v
	}
	return decodeAttempt(t, request(s, "POST", "/api/training/attempts/"+a.ID, "TG01", body))
}
func TestEnterpriseCollectionSixModulesAndHistoryBoundary(t *testing.T) {
	s, cfg := newTestServer(t)
	ids := []string{"M01", "M02", "M03", "M04", "M05", "M06"}
	mutate(t, s, "modules-open", "", []string{"TG01", "TG02"}, ids)
	var last *TrainingAttempt
	for _, id := range ids {
		a := decodeAttempt(t, request(s, "POST", "/api/training/"+id+"/start", "TG01", map[string]any{}))
		mustStatus(t, request(s, "GET", "/api/history/"+a.ID+"/review", "TG01", nil), 403)
		a = enterpriseTestOperation(t, s, a, "step", map[string]any{"step": "inspect"})
		a = enterpriseTestOperation(t, s, a, "enterprise-sample", nil)
		if len(a.Acquisition.Sources) != 3 || len(a.Acquisition.Sources[0].Rows) != 24 || a.Acquisition.Assembled {
			t.Fatal("enterprise samples require three sources and explicit assembly")
		}
		mustStatus(t, request(s, "POST", "/api/training/attempts/"+a.ID, "TG01", map[string]any{"kind": "calculate", "revision": a.Revision}), 422)
		cols := []CustomColumn{{Column: Column{Key: "work_package", Label: "维修工作包", Kind: "text"}, Required: true, Purpose: "关联维修工作包和需求责任"}}
		a = enterpriseTestOperation(t, s, a, "configure", map[string]any{"customColumns": cols})
		bindings := append(enterpriseBlueprint(id).Bindings, FieldBinding{Target: "work_package", Source: enterpriseBlueprint(id).Sources[0].ID, Field: "work_package"})
		a = enterpriseTestOperation(t, s, a, "collect", map[string]any{"bindings": bindings})
		if len(a.Rows) != 24 || !a.Acquisition.Assembled || !strings.HasPrefix(a.Rows[0]["work_package"], "WP-") {
			t.Fatal("multi-source field assembly failed")
		}
		a = enterpriseTestOperation(t, s, a, "calculate", nil)
		if a.Result == nil || len(a.Result.Rows) != 24 {
			t.Fatal("collected rows not used by calculation")
		}
		answer := TrainingAnswer{Metrics: a.Result.Metrics, Decision: a.Result.Decision, Analysis: "已比较" + a.Rows[0]["id"] + "与" + a.Rows[1]["id"] + "，并核对工作包、单位、证书、交期及库存约束。仿真数据不能代替企业真实业务判定，需要计划、采购及质量团队人工复核。", Checks: []string{"source", "units", "constraints"}}
		a = enterpriseTestOperation(t, s, a, "submit", map[string]any{"answer": answer})
		mustStatus(t, request(s, "GET", "/api/history/"+a.ID, "TG02", nil), 404)
		w := request(s, "GET", "/api/history/"+a.ID, "TG01", nil)
		mustStatus(t, w, 200)
		if strings.Contains(w.Body.String(), `"score":`) || !strings.Contains(w.Body.String(), "work_package") {
			t.Fatal("history leaks score or lost field snapshot")
		}
		w = request(s, "GET", "/api/history/"+a.ID+"/review", "TG01", nil)
		mustStatus(t, w, 200)
		if strings.Contains(w.Body.String(), "taiguAdvice") || !strings.Contains(w.Body.String(), "summaryReady") {
			t.Fatal("history review must first collect enterprise context")
		}
		mustStatus(t, request(s, "POST", "/api/training/attempts/"+a.ID, "TG01", map[string]any{"kind": "configure", "revision": a.Revision, "customColumns": []CustomColumn{}}), 409)
		w = request(s, "GET", "/api/training/"+id, "TG01", nil)
		mustStatus(t, w, 200)
		if strings.Contains(w.Body.String(), "taiguAdvice") || strings.Contains(w.Body.String(), `"advantages":`) {
			t.Fatal("review enabled in live training payload")
		}
		last = a
	}
	filtered := request(s, "GET", "/api/history?module=M03", "TG01", nil)
	mustStatus(t, filtered, 200)
	var history struct {
		Items []struct {
			Module string `json:"module"`
		} `json:"items"`
	}
	if json.Unmarshal(filtered.Body.Bytes(), &history) != nil || len(history.Items) != 1 || history.Items[0].Module != "M03" {
		t.Fatal("module history includes other modules")
	}
	other := request(s, "GET", "/api/history?module=M03", "TG02", nil)
	mustStatus(t, other, 200)
	if strings.Contains(other.Body.String(), last.ID) {
		t.Fatal("module filter leaked another learner record")
	}
	mustStatus(t, request(s, "GET", "/api/history?module=unknown", "TG01", nil), 422)
	w := request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "请说明软件利弊和太古适配建议"})
	mustStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "仅在历史复盘中开放") {
		t.Fatal("assistant bypasses history gate")
	}
	w = request(s, "POST", "/api/ai/chat", "TG01", ChatRequest{Message: "软件利弊和厦门太古适配建议", HistoryAttempt: last.ID})
	mustStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "历史复盘") || !strings.Contains(w.Body.String(), "先陈述") {
		t.Fatal("history-scoped assistant unavailable")
	}
	mustStatus(t, request(s, "POST", "/api/ai/chat", "TG02", ChatRequest{Message: "软件利弊", HistoryAttempt: last.ID}), 403)
	mutate(t, s, "modules-close", "", []string{"TG01"}, nil)
	mustStatus(t, request(s, "GET", "/api/history/"+last.ID, "TG01", nil), 200)
	s.Close()
	again, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	a, err := again.getAttempt(last.ID)
	if err != nil || len(a.CustomColumns) != 1 || len(a.Acquisition.Sources) != 3 || a.Blueprint == nil {
		t.Fatal("enterprise snapshot did not survive restart")
	}
}
func TestScenarioParametersAndTypedCustomFields(t *testing.T) {
	rows := caseSpec("M02").Rows
	r := calculateConfigured("M02", rows, map[string]float64{"deadlineDays": 4})
	if r.Decision != "SUP-C" || r.Metrics[0] != 18200 {
		t.Fatalf("deadline not applied: %+v", r)
	}
	r = calculateConfigured("M05", caseSpec("M05").Rows, map[string]float64{"serviceZ": 1.65, "holdingRate": 40})
	if math.Abs(r.Metrics[1]-6793.62) > .01 {
		t.Fatal("holding-rate scenario not applied")
	}
	if validateParameters("M03", map[string]float64{"onTimeWeight": 50}) == nil {
		t.Fatal("invalid weighting accepted")
	}
	a := &TrainingAttempt{Module: "M01", CustomColumns: []CustomColumn{{Column: Column{Key: "cutoff_date", Label: "截至日期", Kind: "date"}, Required: true}}}
	bad := caseSpec("M01").Rows
	if len(validateAttemptRows(a, bad)) == 0 {
		t.Fatal("required custom field not checked")
	}
	for _, row := range bad {
		row["cutoff_date"] = "2026-10-27"
	}
	if len(validateAttemptRows(a, bad)) != 0 {
		t.Fatal("valid custom date rejected")
	}
	if validateCustomColumns("M01", []CustomColumn{{Column: Column{Key: "stock", Label: "覆盖库存", Kind: "text"}}}) == nil {
		t.Fatal("core field override accepted")
	}
}
func TestSourceImportMatchingAndStrictJSON(t *testing.T) {
	s, _ := newTestServer(t)
	mutate(t, s, "modules-open", "", []string{"TG01"}, []string{"M01"})
	a := decodeAttempt(t, request(s, "POST", "/api/training/M01/start", "TG01", map[string]any{}))
	a = enterpriseTestOperation(t, s, a, "step", map[string]any{"step": "inspect"})
	csv := sourceSample("M01", "planning")
	in := SourceImport{ID: "planning", System: "内部计划导出", Object: "工作包需求汇总", KeyField: "business_key", Collector: "TG01", AsOf: time.Now().Format(time.RFC3339), Format: "json"}
	raw, _ := json.Marshal(map[string]any{"rows": csv.Rows})
	in.Content = string(raw)
	a = enterpriseTestOperation(t, s, a, "source-import", map[string]any{"source": in})
	if len(a.Acquisition.Sources[0].Rows) != 24 {
		t.Fatal("JSON collection failed")
	}
	mustStatus(t, request(s, "POST", "/api/training/attempts/"+a.ID, "TG02", map[string]any{"kind": "source-import", "revision": a.Revision, "source": in}), 403)
	_, _, err := parseSource(`[{"id":"A"}] {"extra":1}`, "json")
	if err == nil {
		t.Fatal("trailing JSON accepted")
	}
	duplicate := in
	duplicate.Content = `[{"business_key":"A","x":1},{"business_key":"A","x":2}]`
	before := a.Revision
	mustStatus(t, request(s, "POST", "/api/training/attempts/"+a.ID, "TG01", map[string]any{"kind": "source-import", "revision": a.Revision, "source": duplicate}), 422)
	recovered, _ := s.getAttempt(a.ID)
	if recovered.Revision != before || len(recovered.Acquisition.Sources[0].Rows) != 24 {
		t.Fatal("failed collection mutated saved snapshot")
	}
	a = enterpriseTestOperation(t, s, a, "enterprise-sample", nil)
	in.ID = "inventory"
	in.Object = "库存"
	in.Content = `[{"business_key":"wrong","serviceable_available_qty":10,"inbound_before_need_qty":0}]`
	a = enterpriseTestOperation(t, s, a, "source-import", map[string]any{"source": in})
	w := request(s, "POST", "/api/training/attempts/"+a.ID, "TG01", map[string]any{"kind": "collect", "revision": a.Revision, "bindings": enterpriseBlueprint("M01").Bindings})
	mustStatus(t, w, 422)
	if !strings.Contains(w.Body.String(), "没有匹配") {
		t.Fatal(fmt.Sprintf("missing-key error not actionable: %s", w.Body.String()))
	}
}

func TestHistoryExamGate(t *testing.T) {
	s, _ := newTestServer(t)
	exam := &ExamAttempt{ID: "history-exam-gate", Learner: "TG01", Status: "draft", Deadline: time.Now().Add(time.Hour)}
	if err := s.saveExamAttempt(exam); err != nil {
		t.Fatal(err)
	}
	mustStatus(t, request(s, "GET", "/api/history", "TG01", nil), 403)
	mustStatus(t, request(s, "GET", "/api/history", "TG02", nil), 200)
	exam.Status = "submitted"
	if err := s.saveExamAttempt(exam); err != nil {
		t.Fatal(err)
	}
	mustStatus(t, request(s, "GET", "/api/history", "TG01", nil), 200)
}
