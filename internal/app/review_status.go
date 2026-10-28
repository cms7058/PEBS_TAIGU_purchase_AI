package app

import "encoding/json"

type ReviewButtonState struct {
	Code    string `json:"code"`
	Label   string `json:"label"`
	Action  string `json:"action"`
	Replies int    `json:"replies"`
	Pending int    `json:"pending"`
}

func (s *Server) reviewButtonState(a *TrainingAttempt) (ReviewButtonState, error) {
	state := ReviewButtonState{Code: "not-submitted", Label: "待整理提交", Action: "进入复盘并整理问题"}
	if a.Status != "submitted" {
		state.Code = "draft"
		state.Label = "练习尚未提交"
		state.Action = "提交练习后开启复盘"
		return state, nil
	}
	rows, err := s.db.Query("SELECT payload FROM consultations WHERE attempt=? AND learner=?", a.ID, a.Learner)
	if err != nil {
		return state, err
	}
	defer rows.Close()
	total, waiting, processing, followups := 0, 0, 0, 0
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return state, err
		}
		var v Consultation
		if err := json.Unmarshal(raw, &v); err != nil {
			return state, err
		}
		total++
		state.Replies += len(v.Replies)
		n := pendingQuestions(&v)
		followups += n
		if len(v.Replies) == 0 {
			state.Pending++
			if v.Status == "processing" {
				processing++
			} else {
				waiting++
			}
		}
	}
	if rows.Err() != nil {
		return state, rows.Err()
	}
	state.Pending += followups
	switch {
	case total == 0:
		return state, nil
	case followups > 0:
		state.Code = "followup-waiting"
		state.Label = "追问待回复"
		state.Action = "查看已有回复与待答追问"
	case waiting > 0:
		state.Code = "waiting"
		state.Label = "待管理员回复"
		state.Action = "查看提交与处理进度"
	case processing > 0:
		state.Code = "processing"
		state.Label = "管理员处理中"
		state.Action = "查看处理进度"
	default:
		state.Code = "replied"
		state.Label = "管理员已回复"
		state.Action = "查看管理员回复"
	}
	return state, nil
}
