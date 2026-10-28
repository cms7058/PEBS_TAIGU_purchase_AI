package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	_ "time/tzdata"
)

type Question struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Prompt  string   `json:"prompt"`
	Options []string `json:"options,omitempty"`
	Points  float64  `json:"points"`
	Answer  string   `json:"answer,omitempty"`
	Rubric  string   `json:"rubric,omitempty"`
	Slide   string   `json:"slide"`
}
type ExamConfig struct {
	Revision  int        `json:"revision"`
	Title     string     `json:"title"`
	Day2      string     `json:"day2"`
	OpensAt   time.Time  `json:"opensAt"`
	ClosesAt  time.Time  `json:"closesAt"`
	Minutes   int        `json:"minutes"`
	PassScore float64    `json:"passScore"`
	Published bool       `json:"published"`
	Example   bool       `json:"example"`
	Learners  []string   `json:"learners"`
	Questions []Question `json:"questions"`
}
type ExamReview struct {
	At     time.Time          `json:"at"`
	Actor  string             `json:"actor"`
	Before map[string]float64 `json:"before"`
	After  map[string]float64 `json:"after"`
	Reason string             `json:"reason"`
}
type ExamAttempt struct {
	ID             string             `json:"id"`
	Learner        string             `json:"learner"`
	Status         string             `json:"status"`
	Revision       int                `json:"revision"`
	StartedAt      time.Time          `json:"startedAt"`
	Deadline       time.Time          `json:"deadline"`
	SubmittedAt    *time.Time         `json:"submittedAt,omitempty"`
	Questions      []Question         `json:"questions"`
	Answers        map[string]string  `json:"answers"`
	ObjectiveScore float64            `json:"objectiveScore"`
	ManualScores   map[string]float64 `json:"manualScores"`
	ReviewReason   string             `json:"reviewReason"`
	PassScore      float64            `json:"passScore"`
	Reviews        []ExamReview       `json:"reviews"`
}

func (s *Server) initExam() error {
	var n int
	if err := s.db.QueryRow("SELECT count(*) FROM exam_config").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	c := ExamConfig{Revision: 1, Title: "第二天结业考试（示例草稿）", Minutes: 45, PassScore: 60, Example: true, Learners: []string{}, Questions: []Question{{ID: "Q1", Kind: "choice", Prompt: "报价最低但证书缺失时，应优先采取哪项行动？", Options: []string{"直接下单", "核实证书与适用性后再比较", "忽略交期"}, Answer: "1", Points: 30, Slide: "P02"}, {ID: "Q2", Kind: "choice", Prompt: "再订货点应考虑哪些因素？", Options: []string{"仅单位价格", "交期内需求与安全库存", "仅仓库面积"}, Answer: "1", Points: 30, Slide: "P05"}, {ID: "Q3", Kind: "text", Prompt: "请解释AOG采购中价格、到达时间、证书及适用性的取舍。", Points: 40, Rubric: "检查硬性约束、方案比较、时间与成本证据及人工复核安排", Slide: "P06"}}}
	return s.saveExam(c)
}
func (s *Server) examConfig() (ExamConfig, error) {
	var b []byte
	var c ExamConfig
	err := s.db.QueryRow("SELECT payload FROM exam_config WHERE id=1").Scan(&b)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}
func (s *Server) saveExam(c ExamConfig) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO exam_config(id,payload) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", b)
	return err
}
func (s *Server) examAttempt(learner string) (*ExamAttempt, error) {
	var b []byte
	err := s.db.QueryRow("SELECT payload FROM exam_attempts WHERE learner=?", learner).Scan(&b)
	if err != nil {
		return nil, err
	}
	var a ExamAttempt
	err = json.Unmarshal(b, &a)
	return &a, err
}
func (s *Server) saveExamAttempt(a *ExamAttempt) error {
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO exam_attempts(id,learner,payload) VALUES(?,?,?) ON CONFLICT(learner) DO UPDATE SET payload=excluded.payload", a.ID, a.Learner, b)
	return err
}
func publicExamAttempt(a *ExamAttempt) any {
	if a == nil {
		return nil
	}
	qs := slices.Clone(a.Questions)
	for i := range qs {
		qs[i].Answer = ""
		qs[i].Rubric = ""
	}
	return map[string]any{"id": a.ID, "status": a.Status, "revision": a.Revision, "startedAt": a.StartedAt, "deadline": a.Deadline, "submittedAt": a.SubmittedAt, "questions": qs, "answers": a.Answers}
}
func finishExam(a *ExamAttempt, now time.Time) {
	if a.Status != "draft" {
		return
	}
	a.Status = "submitted"
	a.SubmittedAt = &now
	a.Revision++
	a.ObjectiveScore = 0
	for _, q := range a.Questions {
		if q.Kind == "choice" && a.Answers[q.ID] == q.Answer {
			a.ObjectiveScore += q.Points
		}
	}
}
func examTotal(a *ExamAttempt) (float64, bool) {
	score := a.ObjectiveScore
	complete := a.Status == "submitted"
	for _, q := range a.Questions {
		if q.Kind == "text" {
			p, ok := a.ManualScores[q.ID]
			if !ok {
				complete = false
			}
			score += p
		}
	}
	return score, complete
}
func validateExam(c ExamConfig, learners []Learner) error {
	if len(c.Title) == 0 || len(c.Title) > 160 || c.Minutes < 1 || c.Minutes > 180 || len(c.Questions) < 1 || len(c.Questions) > 100 {
		return fmt.Errorf("标题、时长或题目数量无效")
	}
	ids := map[string]bool{}
	sum := 0.0
	for _, q := range c.Questions {
		if q.ID == "" || ids[q.ID] || len(q.Prompt) == 0 || len(q.Prompt) > 6000 || q.Points <= 0 || q.Points > 100 {
			return fmt.Errorf("题号、题干或分值无效")
		}
		ids[q.ID] = true
		sum += q.Points
		if q.Kind == "choice" {
			if len(q.Options) < 2 || len(q.Options) > 10 {
				return fmt.Errorf("选择题需要2–10个选项")
			}
			valid := false
			for i := range q.Options {
				if q.Answer == fmt.Sprint(i) {
					valid = true
				}
			}
			if !valid {
				return fmt.Errorf("选择题答案应为从0开始的选项索引")
			}
		} else if q.Kind != "text" {
			return fmt.Errorf("题型仅支持 choice 和 text")
		}
	}
	if sum != 100 || c.PassScore < 0 || c.PassScore > 100 {
		return fmt.Errorf("总分必须100分，通过线0–100")
	}
	for _, id := range c.Learners {
		if !slices.ContainsFunc(learners, func(l Learner) bool { return l.ID == id }) {
			return fmt.Errorf("学员名单无效")
		}
	}
	if c.Published {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		if c.Day2 == "" || c.OpensAt.In(loc).Format("2006-01-02") != c.Day2 || !c.ClosesAt.After(c.OpensAt) || c.ClosesAt.Sub(c.OpensAt) > 24*time.Hour || len(c.Learners) == 0 {
			return fmt.Errorf("请设置第二天日期、有效考试窗口及学员名单")
		}
	}
	return nil
}

func (s *Server) examAPI(w http.ResponseWriter, r *http.Request, ss session) {
	if ss.Role == "projector" {
		fail(w, 403, "无考试权限")
		return
	}
	cfg, err := s.examConfig()
	if err != nil {
		fail(w, 500, "读取考试失败")
		return
	}
	now := time.Now().UTC()
	if r.URL.Path == "/api/admin/exam/review" {
		if ss.Role != "admin" {
			fail(w, 403, "仅管理员可评分")
			return
		}
		var in struct {
			Learner  string             `json:"learner"`
			Revision int                `json:"revision"`
			Scores   map[string]float64 `json:"scores"`
			Reason   string             `json:"reason"`
		}
		if !decodeLimit(w, r, &in, 2<<20) {
			return
		}
		a, err := s.examAttempt(in.Learner)
		if err != nil {
			fail(w, 404, "考试记录不存在")
			return
		}
		if a.Status != "submitted" || a.Revision != in.Revision {
			fail(w, 409, "考试未交卷或已被更新")
			return
		}
		if len([]rune(in.Reason)) < 5 {
			fail(w, 422, "请填写至少5字评分理由")
			return
		}
		for id, score := range in.Scores {
			var q *Question
			for i := range a.Questions {
				if a.Questions[i].ID == id {
					q = &a.Questions[i]
				}
			}
			if q == nil || q.Kind != "text" || score < 0 || score > q.Points {
				fail(w, 422, "题号或分值无效")
				return
			}
		}
		before := map[string]float64{}
		for k, v := range a.ManualScores {
			before[k] = v
		}
		after := map[string]float64{}
		for k, v := range in.Scores {
			after[k] = v
			a.ManualScores[k] = v
		}
		a.Reviews = append(a.Reviews, ExamReview{At: now, Actor: ss.ID, Before: before, After: after, Reason: in.Reason})
		a.ReviewReason = in.Reason
		a.Revision++
		if s.saveExamAttempt(a) != nil {
			fail(w, 500, "保存评分失败")
			return
		}
		total, complete := examTotal(a)
		respond(w, 200, map[string]any{"total": total, "complete": complete})
		return
	}
	if r.URL.Path == "/api/admin/exam" {
		if ss.Role != "admin" {
			fail(w, 403, "仅管理员管理考试")
			return
		}
		if r.Method == "GET" {
			respond(w, 200, cfg)
			return
		}
		var in ExamConfig
		if !decodeLimit(w, r, &in, 2<<20) {
			return
		}
		if in.Revision != cfg.Revision {
			fail(w, 409, "试卷已更新，请重新载入")
			return
		}
		var n int
		if err := s.db.QueryRow("SELECT count(*) FROM exam_attempts").Scan(&n); err != nil {
			fail(w, 500, "读取考试状态失败")
			return
		}
		if n > 0 {
			fail(w, 409, "已有学员开考，试卷快照与考试设置已锁定")
			return
		}
		if err := validateExam(in, s.state.Learners); err != nil {
			fail(w, 422, err.Error())
			return
		}
		in.Revision++
		if err := s.saveExam(in); err != nil {
			fail(w, 500, "保存考试失败")
			return
		}
		respond(w, 200, in)
		return
	}
	if ss.Role != "learner" {
		fail(w, 403, "此入口仅限学员")
		return
	}
	allowed := cfg.Published && slices.Contains(cfg.Learners, ss.ID)
	a, err := s.examAttempt(ss.ID)
	if err != nil && err != sql.ErrNoRows {
		fail(w, 500, "读取考试作答失败")
		return
	}
	if a != nil && a.Status == "draft" && !now.Before(a.Deadline) {
		finishExam(a, a.Deadline)
		if err := s.saveExamAttempt(a); err != nil {
			fail(w, 500, "到时交卷保存失败")
			return
		}
	}
	if r.Method == "GET" {
		respond(w, 200, map[string]any{"title": cfg.Title, "available": allowed, "opensAt": cfg.OpensAt, "closesAt": cfg.ClosesAt, "minutes": cfg.Minutes, "example": cfg.Example, "serverNow": now, "attempt": publicExamAttempt(a)})
		return
	}
	if !allowed {
		fail(w, 403, "结业考试尚未向你开放")
		return
	}
	var in struct {
		Kind     string            `json:"kind"`
		Revision int               `json:"revision"`
		Answers  map[string]string `json:"answers"`
	}
	if !decodeLimit(w, r, &in, 2<<20) {
		return
	}
	if in.Kind == "start" {
		if a != nil {
			respond(w, 200, publicExamAttempt(a))
			return
		}
		if now.Before(cfg.OpensAt) || !now.Before(cfg.ClosesAt) {
			fail(w, 403, "当前不在考试窗口内")
			return
		}
		deadline := now.Add(time.Duration(cfg.Minutes) * time.Minute)
		if deadline.After(cfg.ClosesAt) {
			deadline = cfg.ClosesAt
		}
		qs := slices.Clone(cfg.Questions) // deterministic per-attempt order is snapshotted once.
		for i := len(qs) - 1; i > 0; i-- {
			random := []byte(randomToken())
			j := int(random[0]) % (i + 1)
			qs[i], qs[j] = qs[j], qs[i]
		}
		a = &ExamAttempt{ID: randomToken(), Learner: ss.ID, Status: "draft", Revision: 1, StartedAt: now, Deadline: deadline, Questions: qs, Answers: map[string]string{}, ManualScores: map[string]float64{}, PassScore: cfg.PassScore}
		if err := s.saveExamAttempt(a); err != nil {
			fail(w, 500, "开考失败")
			return
		}
		respond(w, 200, publicExamAttempt(a))
		return
	}
	if a == nil {
		fail(w, 409, "请先开始考试")
		return
	}
	if a.Status == "submitted" {
		respond(w, 200, publicExamAttempt(a))
		return
	}
	if in.Revision != a.Revision {
		fail(w, 409, "作答已更新，请重新载入")
		return
	}
	if in.Kind != "save" && in.Kind != "submit" {
		fail(w, 400, "未知考试操作")
		return
	}
	for id, answer := range in.Answers {
		var question *Question
		for i := range a.Questions {
			if a.Questions[i].ID == id {
				question = &a.Questions[i]
			}
		}
		if question == nil || len(answer) > 12000 {
			fail(w, 422, "题号或答案格式错误")
			return
		}
		if question.Kind == "choice" && answer != "" {
			valid := false
			for i := range question.Options {
				if answer == fmt.Sprint(i) {
					valid = true
				}
			}
			if !valid {
				fail(w, 422, "选项无效")
				return
			}
		}
		a.Answers[id] = strings.TrimSpace(answer)
	}
	a.Revision++
	if in.Kind == "submit" {
		finishExam(a, now)
	}
	if err := s.saveExamAttempt(a); err != nil {
		fail(w, 500, "作答保存失败")
		return
	}
	respond(w, 200, publicExamAttempt(a))
}
