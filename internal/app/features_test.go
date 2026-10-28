package app

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func decodeAttempt(t *testing.T, w *httptest.ResponseRecorder) *TrainingAttempt {
	t.Helper()
	mustStatus(t, w, 200)
	var a TrainingAttempt
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	return &a
}
func operation(t *testing.T, s *Server, a *TrainingAttempt, kind, step string, answer *TrainingAnswer) *TrainingAttempt {
	t.Helper()
	return decodeAttempt(t, request(s, "POST", "/api/training/attempts/"+a.ID, "TG01", map[string]any{"revision": a.Revision, "kind": kind, "step": step, "answer": answer}))
}

func TestSixTrainingCasesAndPrivacy(t *testing.T) {
	s, cfg := newTestServer(t)
	all := []string{"M01", "M02", "M03", "M04", "M05", "M06"}
	mutate(t, s, "modules-open", "", []string{"TG01", "TG02"}, all)
	wants := map[string]Calculation{"M01": {Metrics: [2]float64{18, 45}, Decision: "PN-1002"}, "M02": {Metrics: [2]float64{16140, 8}, Decision: "SUP-B"}, "M03": {Metrics: [2]float64{91.1, 2}, Decision: "SUP-B"}, "M04": {Metrics: [2]float64{2, 5}, Decision: "PO-102"}, "M05": {Metrics: [2]float64{115, 3396.81}, Decision: "service"}, "M06": {Metrics: [2]float64{13000, 5}, Decision: "TRANSFER"}}
	var lastID string
	for _, id := range all {
		t.Run(id, func(t *testing.T) {
			res := request(s, "GET", "/api/training/"+id+"/template?example=1", "TG01", nil)
			mustStatus(t, res, 200)
			records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(res.Body.String(), "\ufeff"))).ReadAll()
			if err != nil || len(records) < 2 {
				t.Fatal("invalid CSV example")
			}
			a := decodeAttempt(t, request(s, "POST", "/api/training/"+id+"/start", "TG01", map[string]any{}))
			same := decodeAttempt(t, request(s, "POST", "/api/training/"+id+"/start", "TG01", map[string]any{}))
			if a.ID != same.ID {
				t.Fatal("start not idempotent")
			}
			mustStatus(t, request(s, "GET", "/api/training/attempts/"+a.ID, "TG02", nil), 403)
			mustStatus(t, request(s, "POST", "/api/training/attempts/"+a.ID, "TG01", map[string]any{"revision": a.Revision, "kind": "submit"}), 422)
			a = operation(t, s, a, "step", "inspect", nil)
			a = operation(t, s, a, "step", "data", nil)
			a = operation(t, s, a, "calculate", "", nil)
			want := wants[id]
			if a.Result == nil || math.Abs(a.Result.Metrics[0]-want.Metrics[0]) > .01 || math.Abs(a.Result.Metrics[1]-want.Metrics[1]) > .01 || a.Result.Decision != want.Decision {
				t.Fatalf("calculation mismatch: %+v", a.Result)
			}
			answer := &TrainingAnswer{Metrics: want.Metrics, Decision: want.Decision, Analysis: "已核对原始数据、单位、交期、证书与库存风险，并逐项比较候选方案。该模拟结果仅依据课程假设，仍需人工确认适用性并制定应急保障措施，不能仅以最低价格作为决策依据。 " + a.Rows[0]["id"] + " " + a.Rows[1]["id"], Checks: []string{"source", "units", "constraints"}}
			a = operation(t, s, a, "submit", "", answer)
			if a.Score != nil {
				t.Fatal("student received private score")
			}
			stored, err := s.getAttempt(a.ID)
			if err != nil || stored.Score == nil || stored.Score.Total != 100 {
				t.Fatalf("expected full rule score, got %+v / %v", stored, err)
			}
			mustStatus(t, request(s, "POST", "/api/training/attempts/"+a.ID, "TG01", map[string]any{"revision": 0, "kind": "submit"}), 200)
			mustStatus(t, request(s, "GET", "/api/admin/reports", "TG01", nil), 403)
			lastID = a.ID
		})
	}
	mutate(t, s, "modules-close", "", []string{"TG01"}, nil)
	mustStatus(t, request(s, "GET", "/api/training/attempts/"+lastID, "TG01", nil), 403)
	report := request(s, "GET", "/api/admin/reports", "admin-test", nil)
	mustStatus(t, report, 200)
	if !strings.Contains(report.Body.String(), "rule-v1") {
		t.Fatal("missing scores")
	}
	s.Close()
	restored, err := New(Config{DataDir: cfg.DataDir})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	a, err := restored.getAttempt(lastID)
	if err != nil || a.Status != "submitted" || a.Score.Total != 100 {
		t.Fatal("work lost on restart")
	}
}

func TestImportRevisionAndRetakes(t *testing.T) {
	s, _ := newTestServer(t)
	mutate(t, s, "modules-open", "", []string{"TG01"}, []string{"M02"})
	a := decodeAttempt(t, request(s, "POST", "/api/training/M02/start", "TG01", map[string]any{}))
	a = operation(t, s, a, "step", "inspect", nil)
	r := httptest.NewRequest("POST", "/api/training/attempts/"+a.ID+"/import", strings.NewReader("id,price,qty,fx,freight,lead,certificate\nSUP-A,bad,2,7.2,500,5,0\n"))
	r.Header.Set("Authorization", "Bearer TG01")
	r.Header.Set("X-Attempt-Revision", "2")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	mustStatus(t, w, 422)
	if !strings.Contains(w.Body.String(), "details") {
		t.Fatal("missing row/column errors")
	}
	still, _ := s.getAttempt(a.ID)
	if still.Revision != a.Revision || still.Rows[0]["price"] != "900" {
		t.Fatal("invalid import changed data")
	}
	oldRevision := a.Revision
	a = operation(t, s, a, "step", "data", nil)
	mustStatus(t, request(s, "POST", "/api/training/attempts/"+a.ID, "TG01", map[string]any{"revision": oldRevision, "kind": "calculate"}), 409)
	a = operation(t, s, a, "calculate", "", nil)
	answer := TrainingAnswer{Metrics: [2]float64{16140, 8}, Decision: "SUP-B", Analysis: "核对证书和交期约束后选择 SUP-B，并比较 SUP-C 的价格与运输时间，保留人工复核及风险保障方案。", Checks: []string{"source", "units", "constraints"}}
	a = operation(t, s, a, "submit", "", &answer)
	retry := decodeAttempt(t, request(s, "POST", "/api/training/M02/start", "TG01", map[string]any{"retry": true}))
	if retry.ID == a.ID || retry.Number != 2 {
		t.Fatal("retake overwrote original")
	}
	original, _ := s.getAttempt(a.ID)
	if original.Score == nil {
		t.Fatal("original grade lost")
	}
	w = request(s, "POST", "/api/admin/attempts/"+a.ID, "admin-test", map[string]any{"revision": original.Revision, "total": 82, "reason": "讲师复核并调整分析表达"})
	mustStatus(t, w, 200)
	after, _ := s.getAttempt(a.ID)
	if after.Score.Total != 82 || after.Score.OriginalTotal == nil {
		t.Fatal("manual score history missing")
	}
}

func TestExamSnapshotAndLateAnswers(t *testing.T) {
	s, _ := newTestServer(t)
	cfg, _ := s.examConfig()
	loc, _ := time.LoadLocation("Asia/Shanghai")
	cfg.Day2 = time.Now().In(loc).Format("2006-01-02")
	cfg.OpensAt = time.Now().Add(-time.Minute)
	cfg.ClosesAt = time.Now().Add(time.Hour)
	cfg.Published = true
	cfg.Learners = []string{"TG01"}
	cfg.Minutes = 1
	mustStatus(t, request(s, "POST", "/api/admin/exam", "admin-test", cfg), 200)
	mustStatus(t, request(s, "GET", "/api/admin/exam", "TG01", nil), 403)
	mustStatus(t, request(s, "POST", "/api/exam", "TG02", map[string]any{"kind": "start"}), 403)
	w := request(s, "POST", "/api/exam", "TG01", map[string]any{"kind": "start"})
	mustStatus(t, w, 200)
	if strings.Contains(w.Body.String(), `"answer":`) || strings.Contains(w.Body.String(), `"rubric"`) || strings.Contains(w.Body.String(), "objectiveScore") {
		t.Fatal("answer leak")
	}
	a, _ := s.examAttempt("TG01")
	mustStatus(t, request(s, "POST", "/api/exam", "TG01", map[string]any{"kind": "save", "revision": a.Revision, "answers": map[string]string{"Q1": "1", "Q2": "1", "Q3": "先满足证书、适用性和到达时间，再比较成本并由专业人员复核。"}}), 200)
	cfg, _ = s.examConfig()
	cfg.Title = "change"
	mustStatus(t, request(s, "POST", "/api/admin/exam", "admin-test", cfg), 409)
	a, _ = s.examAttempt("TG01")
	a.Deadline = time.Now().Add(-time.Second)
	s.saveExamAttempt(a)
	mustStatus(t, request(s, "POST", "/api/exam", "TG01", map[string]any{"kind": "submit", "revision": a.Revision, "answers": map[string]string{"Q1": "0"}}), 200)
	a, _ = s.examAttempt("TG01")
	if a.Answers["Q1"] != "1" || a.ObjectiveScore != 60 || a.Status != "submitted" {
		t.Fatal("late answer changed final exam")
	}
	w = request(s, "POST", "/api/admin/exam/review", "admin-test", map[string]any{"learner": "TG01", "revision": a.Revision, "scores": map[string]float64{"Q3": 40}, "reason": "论据完整且符合课程要求"})
	mustStatus(t, w, 200)
	a, _ = s.examAttempt("TG01")
	total, complete := examTotal(a)
	if !complete || total != 100 || len(a.Reviews) != 1 {
		t.Fatal("manual exam review failed")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAIAssistReplayAndAPI(t *testing.T) {
	s, _ := newTestServer(t)
	mutate(t, s, "modules-open", "", []string{"TG01"}, []string{"M02"})
	a := decodeAttempt(t, request(s, "POST", "/api/training/M02/start", "TG01", map[string]any{}))
	w := request(s, "POST", "/api/ai/assist", "TG01", map[string]string{"attempt": a.ID, "prompt": "方法提示"})
	mustStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), `"replay"`) {
		t.Fatal("replay mislabeled")
	}
	s.cfg.AIKey = "test-key"
	s.cfg.AIBaseURL = "https://model.example/v1"
	s.cfg.AIModel = "configured-model"
	s.cfg.AIMaxCalls = 1
	s.cfg.AIClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer test-key" || r.URL.Path != "/v1/chat/completions" {
			t.Error("request contract")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"先核对证书与交期。"}}],"usage":{"total_tokens":30}}`)), Header: http.Header{}}, nil
	})}
	w = request(s, "POST", "/api/ai/assist", "TG01", map[string]string{"attempt": a.ID, "prompt": "方法提示"})
	mustStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), `"api"`) || strings.Contains(w.Body.String(), "test-key") {
		t.Fatal("API result mode or secret leak")
	}
	mustStatus(t, request(s, "POST", "/api/ai/assist", "TG01", map[string]string{"attempt": a.ID}), 429)
}

func TestCoursewareOrderAndMigration(t *testing.T) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	files := map[string]string{"ppt/presentation.xml": `<p:presentation xmlns:p="p" xmlns:r="relationships"><p:sldIdLst><p:sldId id="100" r:id="rSecond"/><p:sldId id="101" r:id="rFirst"/></p:sldIdLst></p:presentation>`, "ppt/_rels/presentation.xml.rels": `<Relationships><Relationship Id="rFirst" Target="slides/slide1.xml"/><Relationship Id="rSecond" Target="slides/slide2.xml"/></Relationships>`, "ppt/slides/slide1.xml": `<sld><t>后讲授</t><t>正文B</t></sld>`, "ppt/slides/slide2.xml": `<sld><t>先讲授</t><t>正文A</t></sld>`}
	for name, content := range files {
		f, _ := z.Create(name)
		f.Write([]byte(content))
	}
	z.Close()
	pages, err := pptxSlides(buf.Bytes())
	if err != nil || len(pages) != 2 || pages[0].Title != "先讲授" {
		t.Fatalf("PPTX order: %+v %v", pages, err)
	}
	s, cfg := newTestServer(t)
	mutate(t, s, "page-grants", "P01", []string{"TG01"}, []string{"M01"})
	mustStatus(t, request(s, "POST", "/api/admin/courseware", "admin-test", map[string]any{"revision": s.state.Revision, "slides": pages}), 200)
	if len(s.state.Learners[0].Pages) != 0 || !slices.Contains(s.state.Learners[0].Modules, "M01") {
		t.Fatal("courseware replacement affected modules")
	}
	legacyDir := t.TempDir()
	b, _ := json.Marshal(s.state)
	os.WriteFile(filepath.Join(legacyDir, "classroom.json"), b, 0600)
	migrated, err := New(Config{DataDir: legacyDir})
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	if migrated.state.Slides[0].Title != "先讲授" {
		t.Fatal("legacy migration lost state")
	}
	if _, err = os.Stat(filepath.Join(cfg.DataDir, "training.sqlite")); err != nil {
		t.Fatal("SQLite storage absent")
	}
}

func TestSlideImageAuthorization(t *testing.T) {
	s, cfg := newTestServer(t)
	folder := filepath.Join(cfg.DataDir, "courseware", "pdf-test")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "page-1.png"), []byte("test-image"), 0600); err != nil {
		t.Fatal(err)
	}
	s.state.Slides[0].Asset = "pdf-test/page-1.png"
	mustStatus(t, request(s, "GET", "/api/slide-images/P01", "TG01", nil), 403)
	mustStatus(t, request(s, "GET", "/api/slide-images/P01", "projector-test", nil), 403)
	mustStatus(t, request(s, "GET", "/api/slide-images/P01", "admin-test", nil), 200)
	mutate(t, s, "page-grants", "P01", []string{"TG01"}, nil)
	mustStatus(t, request(s, "GET", "/api/slide-images/P01", "TG01", nil), 200)
	mutate(t, s, "project", "P01", nil, nil)
	mustStatus(t, request(s, "GET", "/api/slide-images/P01", "projector-test", nil), 200)
	mutate(t, s, "page-close", "P01", nil, nil)
	mustStatus(t, request(s, "GET", "/api/slide-images/P01", "TG01", nil), 403)
	mustStatus(t, request(s, "GET", "/api/slide-images/P01", "projector-test", nil), 403)
	bad := slices.Clone(s.state.Slides)
	bad[0].Asset = "../../private.png"
	mustStatus(t, request(s, "POST", "/api/admin/courseware", "admin-test", map[string]any{"revision": s.state.Revision, "slides": bad}), 422)
}
