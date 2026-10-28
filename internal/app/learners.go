package app

import (
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const learnerCapacity = 10

var accountIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{1,31}$`)

func validAccounts(st State) bool {
	if len(st.Accounts) != len(st.Learners)+1 {
		return false
	}
	accounts := map[string]string{}
	admins := 0
	for _, a := range st.Accounts {
		key := strings.ToLower(a.ID)
		if accounts[key] != "" || (a.Role != "admin" && a.Role != "learner") {
			return false
		}
		accounts[key] = a.Role
		if a.Role == "admin" {
			admins++
		}
	}
	seen := map[string]bool{}
	active := 0
	for _, l := range st.Learners {
		key := strings.ToLower(l.ID)
		if seen[key] || accounts[key] != "learner" {
			return false
		}
		seen[key] = true
		if !l.Disabled {
			active++
		}
	}
	return admins == 1 && active <= learnerCapacity
}
func (s *Server) learnerEnabled(id string) bool {
	for _, l := range s.state.Learners {
		if l.ID == id {
			return !l.Disabled
		}
	}
	return false
}
func (s *Server) revokeLearnerSessions(ids []string) {
	for token, ss := range s.sessions {
		if slices.Contains(ids, ss.ID) {
			delete(s.sessions, token)
		}
	}
	for _, id := range ids {
		delete(s.attempts, id)
	}
}
func (s *Server) learnersView(w http.ResponseWriter) {
	respond(w, 200, map[string]any{"revision": s.state.Revision, "learners": s.state.Learners, "online": s.online(), "capacity": learnerCapacity})
}
func (s *Server) learnersAPI(w http.ResponseWriter, r *http.Request, ss session) {
	if r.Method == "GET" {
		s.learnersView(w)
		return
	}
	var in struct {
		Revision   int      `json:"revision"`
		Kind       string   `json:"kind"`
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Department string   `json:"department"`
		Enabled    bool     `json:"enabled"`
		IDs        []string `json:"ids"`
		Password   string   `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Revision != s.state.Revision {
		fail(w, 409, "名单或课堂已更新，请刷新学员管理后重试")
		return
	}
	in.ID = strings.TrimSpace(in.ID)
	in.Name = strings.TrimSpace(in.Name)
	in.Department = strings.TrimSpace(in.Department)
	if utf8.RuneCountInString(in.Name) > 60 || utf8.RuneCountInString(in.Department) > 100 {
		fail(w, 422, "姓名最多60字，部门最多100字")
		return
	}
	if in.Password != "" && (utf8.RuneCountInString(in.Password) < 6 || utf8.RuneCountInString(in.Password) > 200) {
		fail(w, 422, "密码应为6–200字符，留空则自动生成")
		return
	}
	next := s.state
	next.Accounts = slices.Clone(s.state.Accounts)
	next.Learners = slices.Clone(s.state.Learners)
	next.Audit = slices.Clone(s.state.Audit)
	target := slices.IndexFunc(next.Learners, func(l Learner) bool { return l.ID == in.ID })
	active := 0
	for _, l := range next.Learners {
		if !l.Disabled {
			active++
		}
	}
	credentials := []map[string]string{}
	revoked := []string{}
	targets := []string{in.ID}
	setPassword := func(id, password string) {
		for i, a := range next.Accounts {
			if a.ID == id {
				salt := randomToken()
				next.Accounts[i].Salt = salt
				next.Accounts[i].Hash = passwordHash(password, salt)
				credentials = append(credentials, map[string]string{"id": id, "password": password})
			}
		}
	}
	switch in.Kind {
	case "create":
		if active >= learnerCapacity {
			fail(w, 422, "最多启用10名学员，请先停用离班账号再新增")
			return
		}
		if !accountIDPattern.MatchString(in.ID) {
			fail(w, 422, "账号为2–32位字母、数字、下划线或短横线，并以字母开头")
			return
		}
		if slices.ContainsFunc(next.Accounts, func(a Account) bool { return strings.EqualFold(a.ID, in.ID) }) {
			fail(w, 409, "账号已存在")
			return
		}
		next.Accounts = append(next.Accounts, Account{ID: in.ID, Role: "learner"})
		next.Learners = append(next.Learners, Learner{ID: in.ID, Name: in.Name, Department: in.Department, Pages: []string{}, Modules: []string{}})
		password := in.Password
		if password == "" {
			password = randomToken()[:20]
		}
		setPassword(in.ID, password)
	case "update":
		if target < 0 {
			fail(w, 404, "学员不存在")
			return
		}
		next.Learners[target].Name = in.Name
		next.Learners[target].Department = in.Department
	case "set-enabled":
		if target < 0 {
			fail(w, 404, "学员不存在")
			return
		}
		if in.Enabled && next.Learners[target].Disabled && active >= learnerCapacity {
			fail(w, 422, "最多同时启用10名学员")
			return
		}
		next.Learners[target].Disabled = !in.Enabled
		if !in.Enabled {
			next.Learners[target].Pages = []string{}
			next.Learners[target].Modules = []string{}
			next.Learners[target].Current = ""
			revoked = append(revoked, in.ID)
		}
	case "reset-passwords":
		if len(in.IDs) == 0 || len(in.IDs) > len(next.Learners) {
			fail(w, 422, "请选择要重置密码的学员")
			return
		}
		if in.Password != "" && len(in.IDs) != 1 {
			fail(w, 422, "批量重置使用每人独立的随机密码")
			return
		}
		seen := map[string]bool{}
		for _, id := range in.IDs {
			if seen[id] || !slices.ContainsFunc(next.Learners, func(l Learner) bool { return l.ID == id }) {
				fail(w, 422, "学员名单无效")
				return
			}
			seen[id] = true
		}
		for _, id := range in.IDs {
			password := in.Password
			if password == "" {
				password = randomToken()[:20]
			}
			setPassword(id, password)
		}
		revoked = in.IDs
		targets = in.IDs
	default:
		fail(w, 400, "未知学员管理操作")
		return
	}
	next.Revision++
	next.Audit = append(next.Audit, Event{At: time.Now().UTC(), Actor: ss.ID, Action: Action{Kind: "learner-" + in.Kind, Learners: targets}})
	if err := s.persist(next); err != nil {
		fail(w, 500, "学员信息保存失败，未改变账号或密码")
		return
	}
	s.state = next
	s.revokeLearnerSessions(revoked)
	respond(w, 200, map[string]any{"ok": true, "revision": next.Revision, "credentials": credentials})
}
