package app

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (s *Server) reportAPI(w http.ResponseWriter, r *http.Request, ss session, p []string) {
	if len(p) >= 4 && p[2] == "attempts" {
		a, err := s.getAttempt(p[3])
		if err != nil {
			fail(w, 404, "作业不存在")
			return
		}
		if r.Method == "GET" {
			rows, err := s.db.Query("SELECT at,kind,payload FROM training_events WHERE attempt=? ORDER BY id", a.ID)
			if err != nil {
				fail(w, 500, "读取步骤记录失败")
				return
			}
			events := []map[string]any{}
			for rows.Next() {
				var at, kind string
				var b []byte
				if rows.Scan(&at, &kind, &b) != nil {
					continue
				}
				var detail any
				json.Unmarshal(b, &detail)
				events = append(events, map[string]any{"at": at, "kind": kind, "detail": detail})
			}
			rows.Close()
			respond(w, 200, map[string]any{"attempt": a, "events": events})
			return
		}
		var in struct {
			Revision int     `json:"revision"`
			Total    float64 `json:"total"`
			Reason   string  `json:"reason"`
		}
		if !decode(w, r, &in) {
			return
		}
		if a.Score == nil || a.Revision != in.Revision {
			fail(w, 409, "评分已更新或作业未提交")
			return
		}
		if in.Total < 0 || in.Total > 100 || len([]rune(in.Reason)) < 5 || len(in.Reason) > 2000 {
			fail(w, 422, "分数应为0–100，并填写至少5字调整原因")
			return
		}
		old := a.Score.Total
		if a.Score.OriginalTotal == nil {
			a.Score.OriginalTotal = &old
		}
		a.Score.Total = in.Total
		a.Score.Reviewer = ss.ID
		a.Score.ReviewReason = in.Reason
		a.Revision++
		a.UpdatedAt = time.Now().UTC()
		if err := s.saveAttempt(a, "manual-review", map[string]any{"before": old, "after": in.Total, "reason": in.Reason, "actor": ss.ID}); err != nil {
			fail(w, 500, "保存复核失败")
			return
		}
		respond(w, 200, a)
		return
	}
	if r.Method != "GET" {
		fail(w, 405, "不支持此方法")
		return
	}
	rows, err := s.db.Query("SELECT payload FROM training_attempts ORDER BY learner,module,number DESC")
	if err != nil {
		fail(w, 500, "读取统计失败")
		return
	}
	attempts := []TrainingAttempt{}
	latest := map[string]bool{}
	for rows.Next() {
		var b []byte
		if rows.Scan(&b) != nil {
			continue
		}
		var a TrainingAttempt
		if json.Unmarshal(b, &a) == nil {
			key := a.Learner + ":" + a.Module
			if !latest[key] {
				attempts = append(attempts, a)
				latest[key] = true
			}
		}
	}
	rows.Close()
	exams := []map[string]any{}
	erows, err := s.db.Query("SELECT payload FROM exam_attempts")
	if err != nil {
		fail(w, 500, "读取考试统计失败")
		return
	}
	expired := []*ExamAttempt{}
	for erows.Next() {
		var b []byte
		erows.Scan(&b)
		var a ExamAttempt
		if json.Unmarshal(b, &a) != nil {
			continue
		}
		if a.Status == "draft" && time.Now().After(a.Deadline) {
			finishExam(&a, a.Deadline)
			copy := a
			expired = append(expired, &copy)
		}
		total, complete := examTotal(&a)
		exams = append(exams, map[string]any{"attempt": a, "total": total, "complete": complete, "passed": complete && total >= a.PassScore})
	}
	erows.Close()
	for _, a := range expired {
		if s.saveExamAttempt(a) != nil {
			fail(w, 500, "考试到时交卷保存失败")
			return
		}
	}
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="training-results.csv"`)
		w.Write([]byte{0xef, 0xbb, 0xbf})
		cw := csv.NewWriter(w)
		cw.Write([]string{"学员", "模块", "尝试", "状态", "得分", "有效操作秒", "总历时秒", "复核说明"})
		for _, a := range attempts {
			score := ""
			reason := ""
			if a.Score != nil {
				score = formatFloat(a.Score.Total)
				reason = csvSafe(a.Score.ReviewReason)
			}
			end := a.UpdatedAt
			if a.SubmittedAt != nil {
				end = *a.SubmittedAt
			}
			cw.Write([]string{a.Learner, a.Module, formatFloat(float64(a.Number)), a.Status, score, formatFloat(a.ActiveSeconds), formatFloat(end.Sub(a.StartedAt).Seconds()), reason})
		}
		cw.Flush()
		return
	}
	respond(w, 200, map[string]any{"learners": s.state.Learners, "online": s.online(), "modules": modules, "attempts": attempts, "exams": exams, "scorePolicy": "每个学员每模块最近一次尝试；未提交不计入均分", "serverNow": time.Now().UTC()})
}
func csvSafe(v string) string {
	if strings.HasPrefix(v, "=") || strings.HasPrefix(v, "+") || strings.HasPrefix(v, "-") || strings.HasPrefix(v, "@") || strings.HasPrefix(v, "\t") {
		return "'" + v
	}
	return v
}

var softwareResources = []map[string]string{
	{"name": "SAP Ariba", "url": "https://www.sap.com/products/spend-management/direct-procurement-software/trial.html", "type": "基础试用", "period": "公开页面14天；以实际开通为准", "modules": "M01/M02/M03/M04"},
	{"name": "IBM Maximo", "url": "https://www.ibm.com/account/reg/us-en/signup?formid=urx-52271", "type": "注册试用", "period": "公开页面14天；库存优化权限需另确认", "modules": "M01/M05"},
	{"name": "IATA MRO SmartHub", "url": "https://www.iata.org/mro-smarthub", "type": "受限免费账户", "period": "未公开固定期限；一人一号", "modules": "M02/M06"},
	{"name": "金蝶星空", "url": "https://www.kingdee.com/products/galaxy.html", "type": "体验申请", "period": "公开页面30天；模块与人数待确认", "modules": "M01/M04/M05"},
	{"name": "甄云", "url": "https://www.going-link.com/chat.html?free=true", "type": "预约演示", "period": "账号与期限待确认", "modules": "M02/M03/M04"},
	{"name": "商越", "url": "https://www.sunyur.com/", "type": "体验申请", "period": "账号与期限待确认", "modules": "M02/M03/M04"},
	{"name": "用友", "url": "https://yonyou.com/", "type": "体验入口", "period": "具体产品、账号与期限待确认", "modules": "M01/M04/M05"},
	{"name": "IFS", "url": "https://www.ifs.com/en/book-a-demo", "type": "预约演示", "period": "未确认自助试用期限", "modules": "M01/M05/M06"},
	{"name": "JAGGAER", "url": "https://www.jaggaer.com/book-a-demo", "type": "预约演示", "period": "未确认完整沙箱与期限", "modules": "M02/M03"},
	{"name": "Coupa", "url": "https://get.coupa.com/On-Demand-Demo_Procurement-Invoicing-Payments.html", "type": "产品演示", "period": "非完整采购方试用账户", "modules": "M02/M03/M04"},
}
