package app

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"io"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

type TrainingAnswer struct {
	Metrics  [2]float64 `json:"metrics"`
	Decision string     `json:"decision"`
	Analysis string     `json:"analysis"`
	Checks   []string   `json:"checks"`
}
type ScoreItem struct {
	Name   string  `json:"name"`
	Max    float64 `json:"max"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}
type Score struct {
	Total         float64     `json:"total"`
	Items         []ScoreItem `json:"items"`
	Version       string      `json:"version"`
	Reviewer      string      `json:"reviewer,omitempty"`
	ReviewReason  string      `json:"reviewReason,omitempty"`
	OriginalTotal *float64    `json:"originalTotal,omitempty"`
}
type TrainingAttempt struct {
	ID            string               `json:"id"`
	Learner       string               `json:"learner"`
	Module        string               `json:"module"`
	Number        int                  `json:"number"`
	CaseVersion   string               `json:"caseVersion"`
	Status        string               `json:"status"`
	Revision      int                  `json:"revision"`
	Rows          []map[string]string  `json:"rows"`
	Steps         []string             `json:"steps"`
	Answer        TrainingAnswer       `json:"answer"`
	Result        *Calculation         `json:"result,omitempty"`
	Score         *Score               `json:"score,omitempty"`
	StartedAt     time.Time            `json:"startedAt"`
	UpdatedAt     time.Time            `json:"updatedAt"`
	SubmittedAt   *time.Time           `json:"submittedAt,omitempty"`
	LastPulse     time.Time            `json:"lastPulse"`
	ActiveSeconds float64              `json:"activeSeconds"`
	Active        bool                 `json:"active"`
	CustomColumns []CustomColumn       `json:"customColumns,omitempty"`
	Acquisition   *AcquisitionPlan     `json:"acquisition,omitempty"`
	Parameters    map[string]float64   `json:"parameters,omitempty"`
	Blueprint     *EnterpriseBlueprint `json:"blueprint,omitempty"`
}

func (s *Server) canModule(ss session, id string) bool {
	if ss.Role == "admin" {
		return true
	}
	if ss.Role != "learner" {
		return false
	}
	if !s.learnerEnabled(ss.ID) {
		return false
	}
	for _, l := range s.state.Learners {
		if l.ID == ss.ID {
			return slices.Contains(l.Modules, id)
		}
	}
	return false
}
func learnerAttempt(a *TrainingAttempt) *TrainingAttempt { copy := *a; copy.Score = nil; return &copy }
func (s *Server) featureAPI(w http.ResponseWriter, r *http.Request, ss session) bool {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if r.URL.Path == "/api/history" || strings.HasPrefix(r.URL.Path, "/api/history/") {
		s.historyAPI(w, r, ss, p)
		return true
	}
	if r.URL.Path == "/api/admin/learners" {
		if ss.Role != "admin" {
			fail(w, 403, "仅管理员可管理学员")
		} else {
			s.learnersAPI(w, r, ss)
		}
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/api/slide-images/") {
		s.slideImage(w, r, ss, strings.TrimPrefix(r.URL.Path, "/api/slide-images/"))
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/api/training/") {
		if ss.Role == "projector" {
			fail(w, 403, "无实训权限")
			return true
		}
		if ss.Role == "learner" {
			if exam, err := s.examAttempt(ss.ID); err == nil && exam.Status == "draft" && nowUTC().Before(exam.Deadline) {
				fail(w, 403, "考试期间暂停实训与提示，请先完成结业考试")
				return true
			}
		}
		s.trainingAPI(w, r, ss, p)
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/api/exam") || strings.HasPrefix(r.URL.Path, "/api/admin/exam") {
		s.examAPI(w, r, ss)
		return true
	}
	if r.URL.Path == "/api/admin/reports" || strings.HasPrefix(r.URL.Path, "/api/admin/attempts/") {
		if ss.Role != "admin" {
			fail(w, 403, "仅管理员可查看统计")
			return true
		}
		s.reportAPI(w, r, ss, p)
		return true
	}
	if r.URL.Path == "/api/resources" {
		if ss.Role == "projector" {
			fail(w, 403, "无访问权限")
		} else {
			respond(w, 200, softwareResources)
		}
		return true
	}
	if r.URL.Path == "/api/admin/courseware" {
		if ss.Role != "admin" {
			fail(w, 403, "仅管理员可导入课件")
		} else {
			s.coursewareAPI(w, r, ss)
		}
		return true
	}
	if r.URL.Path == "/api/password" && r.Method == "POST" {
		s.changePassword(w, r, ss)
		return true
	}
	return false
}

func (s *Server) trainingAPI(w http.ResponseWriter, r *http.Request, ss session, p []string) {
	if len(p) < 3 {
		fail(w, 404, "接口不存在")
		return
	}
	id := p[2]
	if id == "attempts" {
		s.attemptAPI(w, r, ss, p)
		return
	}
	spec := caseSpec(id)
	if spec.Title == "" {
		fail(w, 404, "模块不存在")
		return
	}
	if !s.canModule(ss, id) {
		fail(w, 403, "管理员尚未开放此模块")
		return
	}
	if len(p) == 3 && r.Method == "GET" {
		latest, err := s.latestAttempt(ss.ID, id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			fail(w, 500, "读取作业失败")
			return
		}
		respond(w, 200, map[string]any{"case": spec, "attempt": func() any {
			if latest == nil {
				return nil
			}
			return learnerAttempt(latest)
		}()})
		return
	}
	if len(p) == 4 && p[3] == "source-template" && r.Method == "GET" {
		source := sourceSample(id, r.URL.Query().Get("source"))
		if source.ID == "" {
			fail(w, 404, "来源不存在")
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+id+`-`+source.ID+`.csv"`)
		w.Write([]byte{0xef, 0xbb, 0xbf})
		writer := csv.NewWriter(w)
		writer.Write(source.Fields)
		if r.URL.Query().Get("example") == "1" {
			for _, row := range source.Rows {
				values := []string{}
				for _, f := range source.Fields {
					values = append(values, row[f])
				}
				writer.Write(values)
			}
		}
		writer.Flush()
		return
	}
	if len(p) == 4 && p[3] == "template" && r.Method == "GET" {
		if attemptID := r.URL.Query().Get("attempt"); attemptID != "" {
			a, err := s.getAttempt(attemptID)
			if err != nil || a.Learner != ss.ID || a.Module != id {
				fail(w, 403, "无该模板快照权限")
				return
			}
			spec.Columns = attemptColumns(a)
			spec.Rows = a.Rows
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+id+`-`+spec.Version+`.csv"`)
		w.Write([]byte{0xef, 0xbb, 0xbf})
		cw := csv.NewWriter(w)
		header := []string{}
		for _, c := range spec.Columns {
			header = append(header, c.Key)
		}
		cw.Write(header)
		if r.URL.Query().Get("example") == "1" {
			for _, row := range spec.Rows {
				vals := []string{}
				for _, c := range spec.Columns {
					vals = append(vals, row[c.Key])
				}
				cw.Write(vals)
			}
		}
		cw.Flush()
		return
	}
	if len(p) == 4 && p[3] == "start" && r.Method == "POST" {
		var in struct {
			Retry bool `json:"retry"`
		}
		if !decode(w, r, &in) {
			return
		}
		previous, err := s.latestAttempt(ss.ID, id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			fail(w, 500, "读取作业失败")
			return
		}
		if previous != nil && (previous.Status == "draft" || !in.Retry) {
			respond(w, 200, learnerAttempt(previous))
			return
		}
		n := 1
		if previous != nil {
			n = previous.Number + 1
		}
		now := time.Now().UTC()
		a := &TrainingAttempt{ID: randomToken(), Learner: ss.ID, Module: id, Number: n, CaseVersion: spec.Version, Status: "draft", Revision: 1, Rows: spec.Rows, Steps: []string{}, StartedAt: now, UpdatedAt: now, LastPulse: now, Active: true}
		blueprint := spec.Enterprise
		a.Blueprint = &blueprint
		a.Parameters = defaultParameters(id)
		a.CustomColumns = []CustomColumn{}
		if previous != nil {
			a.CustomColumns = slices.Clone(previous.CustomColumns)
			for key, value := range previous.Parameters {
				a.Parameters[key] = value
			}
		}
		a.Acquisition = &AcquisitionPlan{Mode: "sample", System: "课堂基础样例", Collector: ss.ID, AsOf: now.Format(timeFormat), Sources: []SourceSnapshot{}, Bindings: blueprint.Bindings}
		if err := s.saveAttempt(a, "start", map[string]any{"number": n}); err != nil {
			fail(w, 500, "创建作业失败")
			return
		}
		respond(w, 200, learnerAttempt(a))
		return
	}
	fail(w, 404, "接口不存在")
}

func (s *Server) attemptAPI(w http.ResponseWriter, r *http.Request, ss session, p []string) {
	if len(p) < 4 {
		fail(w, 404, "作业不存在")
		return
	}
	a, err := s.getAttempt(p[3])
	if err != nil {
		fail(w, 404, "作业不存在")
		return
	}
	if a.Learner != ss.ID {
		fail(w, 403, "只能操作本人作业")
		return
	}
	if !s.canModule(ss, a.Module) {
		fail(w, 403, "该实训已结束，不能继续操作")
		return
	}
	if r.Method == "GET" {
		respond(w, 200, learnerAttempt(a))
		return
	}
	if r.Method != "POST" {
		fail(w, 405, "不支持此方法")
		return
	}
	if len(p) == 5 && p[4] == "import" {
		if a.Status != "draft" {
			fail(w, 409, "已提交作业不可修改")
			return
		}
		if !slices.Contains(a.Steps, "inspect") {
			fail(w, 422, "请先阅读任务说明")
			return
		}
		revision, err := strconv.Atoi(r.Header.Get("X-Attempt-Revision"))
		if err != nil || revision != a.Revision {
			fail(w, 409, "作业已更新，请重新载入")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			fail(w, 413, "文件超过10 MB")
			return
		}
		reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(raw), "\ufeff")))
		reader.FieldsPerRecord = -1
		records, err := reader.ReadAll()
		if err != nil || len(records) < 2 {
			fail(w, 422, "CSV格式错误，请使用模板")
			return
		}
		spec := caseSpec(a.Module)
		spec.Columns = attemptColumns(a)
		if a.Acquisition != nil && a.Acquisition.Mode == "mapped" && !a.Acquisition.Assembled {
			fail(w, 422, "请先完成来源关联，或明确切换为人工采集")
			return
		}
		if len(records[0]) != len(spec.Columns) {
			fail(w, 422, "表头列数与模板不符")
			return
		}
		for i, c := range spec.Columns {
			if records[0][i] != c.Key {
				fail(w, 422, "表头不匹配：应为 "+c.Key)
				return
			}
		}
		rows := []map[string]string{}
		for i, record := range records[1:] {
			if len(record) != len(spec.Columns) {
				fail(w, 422, "第"+strconv.Itoa(i+2)+"行列数错误")
				return
			}
			row := map[string]string{}
			for j, c := range spec.Columns {
				row[c.Key] = strings.TrimSpace(record[j])
			}
			rows = append(rows, row)
		}
		errs := validateAttemptRows(a, rows)
		if len(errs) > 0 {
			respond(w, 422, map[string]any{"error": "数据预检未通过", "details": errs})
			return
		}
		a.Rows = rows
		a.Result = nil
		a.Steps = slices.DeleteFunc(a.Steps, func(v string) bool { return v == "calculate" || v == "review" })
		if !slices.Contains(a.Steps, "data") {
			a.Steps = append(a.Steps, "data")
		}
		a.Revision++
		a.UpdatedAt = time.Now().UTC()
		if err := s.saveAttempt(a, "import", map[string]int{"rows": len(rows)}); err != nil {
			fail(w, 500, "导入未保存")
			return
		}
		respond(w, 200, learnerAttempt(a))
		return
	}
	var in struct {
		Revision      int                 `json:"revision"`
		Kind          string              `json:"kind"`
		Step          string              `json:"step"`
		Answer        *TrainingAnswer     `json:"answer"`
		Active        bool                `json:"active"`
		CustomColumns *[]CustomColumn     `json:"customColumns"`
		Parameters    map[string]float64  `json:"parameters"`
		DataSource    *AcquisitionHeader  `json:"dataSource"`
		Source        *SourceImport       `json:"source"`
		Bindings      []FieldBinding      `json:"bindings"`
		Rows          []map[string]string `json:"rows"`
	}
	if !decodeLimit(w, r, &in, 12<<20) {
		return
	}
	if a.Status == "submitted" && in.Kind == "submit" {
		respond(w, 200, learnerAttempt(a))
		return
	}
	if a.Status != "draft" {
		fail(w, 409, "已提交作业不可修改，请开始新一次练习")
		return
	}
	if in.Revision != a.Revision {
		fail(w, 409, "作业已更新，请重新载入")
		return
	}
	now := time.Now().UTC()
	gap := now.Sub(a.LastPulse).Seconds()
	if a.Active && gap >= 0 && gap <= 30 {
		a.ActiveSeconds += gap
	}
	a.LastPulse = now
	if in.Answer != nil {
		if len(in.Answer.Analysis) > 12000 || len(in.Answer.Decision) > 160 {
			fail(w, 400, "分析或方案编号过长")
			return
		}
		a.Answer = *in.Answer
	}
	switch in.Kind {
	case "configure", "source-import", "enterprise-sample", "collect":
		if !slices.Contains(a.Steps, "inspect") {
			fail(w, 422, "请先阅读任务和系统边界")
			return
		}
		var err error
		switch in.Kind {
		case "configure":
			if in.Rows != nil {
				if len(in.Rows) < 1 || len(in.Rows) > 5000 {
					fail(w, 422, "业务表需要1–5000行")
					return
				}
				for _, row := range in.Rows {
					if len(row) > 60 {
						fail(w, 422, "业务表列数过多")
						return
					}
					for key, value := range row {
						if len(key) > 100 || len([]rune(value)) > 500 {
							fail(w, 422, "业务表字段过长")
							return
						}
					}
				}
				a.Rows = in.Rows
			}
			err = configureAttempt(a, in.CustomColumns, in.Parameters, in.DataSource)
		case "source-import":
			err = importSource(a, in.Source)
		case "enterprise-sample":
			a.Acquisition = &AcquisitionPlan{Mode: "mapped", System: "企业分源仿真数据", Collector: ss.ID, AsOf: now.Format(timeFormat), Sources: []SourceSnapshot{}, Bindings: enterpriseBlueprint(a.Module).Bindings}
			for _, source := range enterpriseBlueprint(a.Module).Sources {
				snapshot := sourceSample(a.Module, source.ID)
				snapshot.Collector = ss.ID
				a.Acquisition.Sources = append(a.Acquisition.Sources, snapshot)
			}
			invalidateData(a)
		case "collect":
			var rows []map[string]string
			rows, err = assembleSources(a, in.Bindings)
			if err == nil {
				a.Rows = rows
				invalidateData(a)
				a.Acquisition.Bindings = in.Bindings
				a.Acquisition.Mode = "mapped"
				a.Acquisition.Assembled = true
				a.Steps = append(a.Steps, "data")
			}
		}
		if err != nil {
			fail(w, 422, err.Error())
			return
		}
	case "heartbeat":
		a.Active = in.Active
	case "step":
		if in.Step != "inspect" && in.Step != "data" {
			fail(w, 400, "未知操作步骤")
			return
		}
		if in.Step == "data" && !slices.Contains(a.Steps, "inspect") {
			fail(w, 422, "请先阅读任务说明")
			return
		}
		if in.Step == "data" && (len(validateAttemptRows(a, a.Rows)) > 0 || a.Acquisition != nil && a.Acquisition.Mode == "mapped" && !a.Acquisition.Assembled) {
			fail(w, 422, "请先修复数据格式")
			return
		}
		if !slices.Contains(a.Steps, in.Step) {
			a.Steps = append(a.Steps, in.Step)
		}
	case "calculate":
		if !slices.Contains(a.Steps, "data") {
			fail(w, 422, "请先确认数据")
			return
		}
		if errors := validateAttemptRows(a, a.Rows); len(errors) > 0 {
			fail(w, 422, "数据校验未通过，请返回采集与字段配置")
			return
		}
		result := calculateConfigured(a.Module, a.Rows, a.Parameters)
		a.Result = &result
		if !slices.Contains(a.Steps, "calculate") {
			a.Steps = append(a.Steps, "calculate")
		}
	case "save":
	case "submit":
		for _, step := range []string{"inspect", "data", "calculate"} {
			if !slices.Contains(a.Steps, step) {
				fail(w, 422, "请先完成步骤："+step)
				return
			}
		}
		for _, check := range []string{"source", "units", "constraints"} {
			if !slices.Contains(a.Answer.Checks, check) {
				fail(w, 422, "请完成人工复核清单")
				return
			}
		}
		if strings.TrimSpace(a.Answer.Decision) == "" || len([]rune(strings.TrimSpace(a.Answer.Analysis))) < 20 {
			fail(w, 422, "请填写方案编号及至少20字的分析")
			return
		}
		a.Steps = append(a.Steps, "review", "submit")
		a.Status = "submitted"
		a.SubmittedAt = &now
		a.Active = false
		a.Score = scoreAttempt(a)
	default:
		fail(w, 400, "未知操作")
		return
	}
	a.UpdatedAt = now
	a.Revision++
	if err := s.saveAttempt(a, in.Kind, map[string]any{"step": in.Step, "steps": a.Steps}); err != nil {
		fail(w, 500, "作业保存失败")
		return
	}
	respond(w, 200, learnerAttempt(a))
}

func scoreAttempt(a *TrainingAttempt) *Score {
	items := []ScoreItem{{Name: "操作步骤", Max: 20, Score: 20, Reason: "按要求完成阅读、数据检查、计算和复核"}, {Name: "数据质量", Max: 20, Score: 20, Reason: "数据通过列、类型、范围与唯一性校验"}, {Name: "计算准确性", Max: 25, Reason: "两项指标分别按允许误差1%或0.05核对"}, {Name: "业务决策", Max: 15, Reason: "依据案例的确定规则评价推荐方案"}, {Name: "分析表达", Max: 10, Reason: "规则检查：篇幅、案例证据编号和业务约束词；语义质量待讲师复核"}, {Name: "操作效率", Max: 10, Score: 10, Reason: "按有效活动时间计，不含长时间断线；参考时间15分钟"}}
	r := calculateConfigured(a.Module, a.Rows, a.Parameters)
	for i, want := range r.Metrics {
		if math.Abs(a.Answer.Metrics[i]-want) <= math.Max(.05, math.Abs(want)*.01) {
			items[2].Score += 12.5
		}
	}
	if strings.EqualFold(strings.TrimSpace(a.Answer.Decision), r.Decision) {
		items[3].Score = 15
	}
	if len([]rune(a.Answer.Analysis)) >= 60 {
		items[4].Score += 4
	}
	evidence := 0
	for _, row := range a.Rows {
		if strings.Contains(a.Answer.Analysis, row["id"]) {
			evidence++
		}
	}
	if evidence >= 2 {
		items[4].Score += 3
	}
	for _, word := range []string{"风险", "交期", "证书", "假设", "库存", "延期", "约束"} {
		if strings.Contains(a.Answer.Analysis, word) {
			items[4].Score += 3
			break
		}
	}
	if a.ActiveSeconds > 1800 {
		items[5].Score = 5
	} else if a.ActiveSeconds > 900 {
		items[5].Score = 8
	}
	score := &Score{Items: items, Version: "rule-v1", Total: 0}
	for _, v := range items {
		score.Total += v.Score
	}
	return score
}
