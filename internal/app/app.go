package app

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

//go:embed web/*
var web embed.FS

type Config struct {
	DataDir, AdminPassword    string
	SecureCookie              bool
	AIBaseURL, AIModel, AIKey string
	AIMaxCalls                int
	AIClient                  *http.Client
}
type Account struct {
	ID   string `json:"id"`
	Role string `json:"role"`
	Salt string `json:"salt"`
	Hash string `json:"hash"`
}
type Slide struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
	Suggested   string   `json:"suggested"`
	Asset       string   `json:"asset,omitempty"`
}
type Module struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}
type Learner struct {
	ID         string   `json:"id"`
	Name       string   `json:"name,omitempty"`
	Department string   `json:"department,omitempty"`
	Disabled   bool     `json:"disabled,omitempty"`
	Pages      []string `json:"pages"`
	Modules    []string `json:"modules"`
	Current    string   `json:"current"`
}
type Event struct {
	At     time.Time `json:"at"`
	Actor  string    `json:"actor"`
	Action Action    `json:"action"`
}
type State struct {
	Revision   int       `json:"revision"`
	Accounts   []Account `json:"accounts"`
	Learners   []Learner `json:"learners"`
	Projection string    `json:"projection"`
	Closed     []string  `json:"closed"`
	Audit      []Event   `json:"audit"`
	Slides     []Slide   `json:"slides"`
}
type Action struct {
	Revision   int      `json:"revision"`
	Kind       string   `json:"kind"`
	Page       string   `json:"page"`
	Pages      []string `json:"pages,omitempty"`
	Learners   []string `json:"learners"`
	Modules    []string `json:"modules"`
	OnlineOnly bool     `json:"onlineOnly"`
}
type session struct {
	ID, Role          string
	ProjectionControl bool
	Expires, LastSeen time.Time
}
type attempt struct {
	Count   int
	Started time.Time
}
type Server struct {
	mu       sync.Mutex
	cfg      Config
	state    State
	sessions map[string]session
	attempts map[string]attempt
	web      http.Handler
	db       *sql.DB
	aiSlots  chan struct{}
}

var slides = []Slide{
	{"P01", "需求预测与采购计划", "从维修计划出发，识别需求缺口与采购时点。", []string{"维修计划", "需求预测", "采购建议"}, "M01", ""},
	{"P02", "三家报价，如何选择？", "结合价格、交期、件况和证书，建立统一比较口径。", []string{"提取报价", "统一口径", "综合比选"}, "M02", ""},
	{"P03", "供应商风险与保障策略", "识别单一来源风险，制定关键航材保障方案。", []string{"资质复核", "风险评分", "保障策略"}, "M03", ""},
	{"P04", "订单执行与交付预警", "识别交期变化，核实对维修任务的影响。", []string{"更新交期", "识别影响", "生成催交"}, "M04", ""},
	{"P05", "高库存，为什么仍缺料？", "调整补货参数，比较库存成本和缺货风险。", []string{"清洗数据", "策略计算", "情景比较"}, "M05", ""},
	{"P06", "AOG：与时间赛跑", "在有限时间内比较可行的航材保障方案。", []string{"确认需求", "方案比较", "决策复核"}, "M06", ""},
}
var modules = []Module{{"M01", "需求预测与采购计划"}, {"M02", "询价解析与比价"}, {"M03", "供应商风险评估"}, {"M04", "订单跟踪与交付"}, {"M05", "库存策略优化"}, {"M06", "AOG 紧急采购"}}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func passwordHash(password, salt string) string {
	key, err := pbkdf2.Key(sha256.New, password, []byte(salt), 120000, 32)
	if err != nil {
		panic(err)
	}
	return base64.RawStdEncoding.EncodeToString(key)
}

func New(cfg Config) (*Server, error) {
	if cfg.DataDir == "" {
		return nil, errors.New("DATA_DIR is required")
	}
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return nil, err
	}
	db, err := openStorage(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			db.Close()
		}
	}()
	s := &Server{cfg: cfg, db: db, aiSlots: make(chan struct{}, 2), sessions: map[string]session{}, attempts: map[string]attempt{}}
	b, err := s.loadState()
	if err == nil {
		if err = json.Unmarshal(b, &s.state); err != nil {
			return nil, fmt.Errorf("read classroom: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	} else {
		if n := utf8.RuneCountInString(cfg.AdminPassword); n < 6 || n > 200 {
			return nil, errors.New("first start requires BOOTSTRAP_ADMIN_PASSWORD with 6–200 characters")
		}
		s.state = State{Revision: 1, Learners: []Learner{}, Closed: []string{}, Audit: []Event{}}
		add := func(id, role, password string) {
			salt := randomToken()
			s.state.Accounts = append(s.state.Accounts, Account{id, role, salt, passwordHash(password, salt)})
		}
		add("admin", "admin", cfg.AdminPassword)
		file, err := os.OpenFile(filepath.Join(cfg.DataDir, "bootstrap-credentials.csv"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return nil, fmt.Errorf("create credentials (check incomplete initialization): %w", err)
		}
		w := csv.NewWriter(file)
		_ = w.Write([]string{"account", "password"})
		for i := 1; i <= 10; i++ {
			id := fmt.Sprintf("TG%02d", i)
			password := randomToken()[:20]
			add(id, "learner", password)
			s.state.Learners = append(s.state.Learners, Learner{ID: id, Pages: []string{}, Modules: []string{}})
			_ = w.Write([]string{id, password})
		}
		w.Flush()
		writeErr := w.Error()
		syncErr := file.Sync()
		closeErr := file.Close()
		if writeErr != nil {
			return nil, writeErr
		}
		if syncErr != nil {
			return nil, syncErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if err := s.persist(s.state); err != nil {
			return nil, err
		}
	}
	if !validAccounts(s.state) {
		return nil, errors.New("invalid classroom state")
	}
	assets, err := fs.Sub(web, "web")
	if err != nil {
		return nil, err
	}
	s.web = http.FileServer(http.FS(assets))
	if len(s.state.Slides) == 0 {
		s.state.Slides = slices.Clone(slides)
	}
	if err := s.persist(s.state); err != nil {
		return nil, err
	}
	if err := s.initExam(); err != nil {
		return nil, err
	}
	success = true
	return s, nil
}

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeLimit(w, r, v, 64<<10)
}
func decodeLimit(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		fail(w, 415, "需要 JSON 请求")
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fail(w, 400, "请求格式错误")
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		fail(w, 400, "请求内容不完整或过多")
		return false
	}
	return true
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Each portal owns its session cookie and API namespace, so a lecturer can
	// keep the learner portal open in another tab without replacing either login.
	portal, cookieName, cookiePath := "", "taigu_session", "/"
	for _, role := range []string{"admin", "learner"} {
		if strings.HasPrefix(r.URL.Path, "/"+role+"/api/") {
			portal, cookieName, cookiePath = role, "taigu_"+role+"_session", "/"+role
			r = r.Clone(r.Context())
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/"+role)
			break
		}
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob:; script-src 'self'; style-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Cache-Control", "no-store")
	}
	if r.Method != "GET" && r.Method != "POST" {
		fail(w, 405, "不支持此方法")
		return
	}
	if r.Method == "POST" {
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			fail(w, 403, "请求来源无效")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host {
				fail(w, 403, "请求来源无效")
				return
			}
		}
	}
	if r.URL.Path == "/healthz" {
		respond(w, 200, map[string]string{"status": "ok", "stage": "training-preview"})
		return
	}
	if r.URL.Path == "/api/login" && r.Method == "POST" {
		s.login(w, r, portal, cookieName, cookiePath)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		if r.Method != "GET" {
			fail(w, 405, "不支持此方法")
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/projector" || r.URL.Path == "/admin" || r.URL.Path == "/learner" || r.URL.Path == "/admin/" || r.URL.Path == "/learner/" {
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFileFS(w, r, web, "web/index.html")
		} else {
			w.Header().Set("Cache-Control", "no-store")
			s.web.ServeHTTP(w, r)
		}
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		if c, err := r.Cookie(cookieName); err == nil {
			token = c.Value
		}
	}
	ss, ok := s.sessions[token]
	now := time.Now()
	if !ok || now.After(ss.Expires) || (ss.Role == "learner" && !s.learnerEnabled(ss.ID)) {
		delete(s.sessions, token)
		fail(w, 401, "请先登录")
		return
	}
	if portal != "" && ss.Role != portal {
		fail(w, 403, "账号身份与当前入口不匹配")
		return
	}
	ss.LastSeen = now
	s.sessions[token] = ss
	if r.URL.Path == "/api/logout" && r.Method == "POST" {
		delete(s.sessions, token)
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: cookiePath, HttpOnly: true, Secure: s.cfg.SecureCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	if r.URL.Path == "/api/me" && r.Method == "GET" && ss.Role != "projector" {
		s.me(w, ss)
		return
	}
	if r.URL.Path == "/api/projection" {
		s.projectionAPI(w, r, ss)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/api/consultations/") {
		s.mu.Unlock()
		s.learnerDiscussion(w, r, ss)
		s.mu.Lock()
		return
	}
	if r.URL.Path == "/api/admin/consultations" || strings.HasPrefix(r.URL.Path, "/api/admin/consultations/") {
		s.mu.Unlock()
		s.adminConsultations(w, r, ss)
		s.mu.Lock()
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/history/") && strings.HasSuffix(r.URL.Path, "/review") {
		s.mu.Unlock()
		s.enterpriseReviewAPI(w, r, ss)
		s.mu.Lock()
		return
	}
	if (r.URL.Path == "/api/ai/assist" || r.URL.Path == "/api/ai/chat") && r.Method == "POST" {
		if ss.Role == "projector" {
			fail(w, 403, "投影端无AI答疑权限")
			return
		}
		s.mu.Unlock()
		if r.URL.Path == "/api/ai/chat" {
			s.aiChat(w, r, ss)
		} else {
			s.aiAssist(w, r, ss)
		}
		s.mu.Lock()
		return
	}
	if s.featureAPI(w, r, ss) {
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/admin/") {
		if ss.Role != "admin" {
			fail(w, 403, "仅管理员可操作")
			return
		}
		switch {
		case r.URL.Path == "/api/admin/classroom" && r.Method == "GET":
			s.classroom(w)
		case r.URL.Path == "/api/admin/actions" && r.Method == "POST":
			var a Action
			if decode(w, r, &a) {
				s.action(w, ss, a)
			}
		case r.URL.Path == "/api/admin/projection-token" && r.Method == "POST":
			t := randomToken()
			s.sessions[t] = session{ID: "projection", Role: "projector", ProjectionControl: true, Expires: now.Add(8 * time.Hour), LastSeen: now}
			respond(w, 200, map[string]string{"token": t})
		default:
			fail(w, 404, "接口不存在")
		}
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/slides/") && r.Method == "GET" {
		id := strings.TrimPrefix(r.URL.Path, "/api/slides/")
		allowed := ss.Role == "admin"
		for _, l := range s.state.Learners {
			if l.ID == ss.ID && slices.Contains(l.Pages, id) {
				allowed = true
			}
		}
		if !allowed {
			fail(w, 403, "该页未对你开放")
			return
		}
		for _, p := range s.state.Slides {
			if p.ID == id {
				respond(w, 200, p)
				return
			}
		}
		fail(w, 404, "页面不存在")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/modules/") && r.Method == "GET" {
		id := strings.TrimPrefix(r.URL.Path, "/api/modules/")
		allowed := ss.Role == "admin"
		for _, l := range s.state.Learners {
			if l.ID == ss.ID && slices.Contains(l.Modules, id) {
				allowed = true
			}
		}
		if !allowed {
			fail(w, 403, "该模块未对你开放")
			return
		}
		for _, m := range modules {
			if m.ID == id {
				respond(w, 200, map[string]any{"module": m, "implementation": "guided", "message": "引导式实训已开放"})
				return
			}
		}
		fail(w, 404, "模块不存在")
		return
	}
	fail(w, 404, "接口不存在")
}

func (s *Server) login(w http.ResponseWriter, r *http.Request, portal, cookieName, cookiePath string) {
	var input struct {
		ID       string `json:"id"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.ID = strings.TrimSpace(input.ID)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	key := input.ID
	at := s.attempts[key]
	if now.Sub(at.Started) > time.Minute {
		at = attempt{Started: now}
	}
	if at.Count >= 5 {
		fail(w, 429, "尝试次数过多，请一分钟后重试")
		return
	}
	at.Count++
	s.attempts[key] = at
	var found *Account
	for i := range s.state.Accounts {
		if s.state.Accounts[i].ID == input.ID {
			found = &s.state.Accounts[i]
			break
		}
	}
	valid := false
	if found != nil {
		valid = subtle.ConstantTimeCompare([]byte(passwordHash(input.Password, found.Salt)), []byte(found.Hash)) == 1
	}
	if !valid {
		fail(w, 401, "账号或密码错误")
		return
	}
	if portal != "" && found.Role != portal {
		fail(w, 403, "账号身份与当前入口不匹配，请使用对应的登录入口")
		return
	}
	if found.Role == "learner" && !s.learnerEnabled(found.ID) {
		fail(w, 403, "该学员账号已停用，请联系管理员")
		return
	}
	delete(s.attempts, key)
	for t, v := range s.sessions {
		if now.After(v.Expires) {
			delete(s.sessions, t)
		}
	}
	token := randomToken()
	s.sessions[token] = session{ID: found.ID, Role: found.Role, Expires: now.Add(8 * time.Hour), LastSeen: now}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: cookiePath, HttpOnly: true, Secure: s.cfg.SecureCookie, SameSite: http.SameSiteStrictMode, MaxAge: 8 * 3600})
	respond(w, 200, map[string]string{"id": found.ID, "role": found.Role})
}
func (s *Server) online() []string {
	ids := []string{}
	now := time.Now()
	for _, ss := range s.sessions {
		if ss.Role == "learner" && s.learnerEnabled(ss.ID) && now.Before(ss.Expires) && now.Sub(ss.LastSeen) < 45*time.Second && !slices.Contains(ids, ss.ID) {
			ids = append(ids, ss.ID)
		}
	}
	slices.Sort(ids)
	return ids
}
func (s *Server) classroom(w http.ResponseWriter) {
	learners := []Learner{}
	for _, l := range s.state.Learners {
		if !l.Disabled {
			learners = append(learners, l)
		}
	}
	respond(w, 200, map[string]any{"revision": s.state.Revision, "slides": s.state.Slides, "modules": modules, "learners": learners, "online": s.online(), "projection": s.state.Projection, "closed": s.state.Closed})
}
func (s *Server) me(w http.ResponseWriter, ss session) {
	var learner *Learner
	visible := []Slide{}
	for _, l := range s.state.Learners {
		if l.ID == ss.ID {
			v := l
			learner = &v
			for _, p := range s.state.Slides {
				if slices.Contains(l.Pages, p.ID) {
					visible = append(visible, p)
				}
			}
		}
	}
	available := []Module{}
	if learner != nil {
		for _, m := range modules {
			if slices.Contains(learner.Modules, m.ID) {
				available = append(available, m)
			}
		}
	}
	progress := map[string]any{}
	examActive := false
	if ss.Role == "learner" {
		rows, err := s.db.Query("SELECT module,number,status FROM training_attempts WHERE learner=? ORDER BY module,number DESC", ss.ID)
		if err != nil {
			fail(w, 500, "读取任务状态失败")
			return
		}
		for rows.Next() {
			var module, status string
			var number int
			if rows.Scan(&module, &number, &status) == nil {
				if _, found := progress[module]; !found {
					progress[module] = map[string]any{"status": status, "number": number}
				}
			}
		}
		rows.Close()
		if a, err := s.examAttempt(ss.ID); err == nil {
			examActive = a.Status == "draft" && time.Now().Before(a.Deadline)
		}
	}
	aiMode := "local"
	if s.cfg.AIKey != "" && s.cfg.AIBaseURL != "" && s.cfg.AIModel != "" {
		aiMode = "api"
	}
	respond(w, 200, map[string]any{"id": ss.ID, "role": ss.Role, "revision": s.state.Revision, "learner": learner, "slides": visible, "modules": available, "catalog": modules, "progress": progress, "examActive": examActive, "aiMode": aiMode})
}

func (s *Server) action(w http.ResponseWriter, ss session, a Action) {
	if a.Revision != s.state.Revision {
		fail(w, 409, "课堂已更新，请重新打开操作弹窗")
		return
	}
	if a.OnlineOnly {
		a.Learners = s.online()
	}
	if !slices.Contains([]string{"page-grants", "pages-activate", "page-close", "modules-open", "modules-close", "project", "pause"}, a.Kind) {
		fail(w, 400, "未知操作")
		return
	}
	if slices.Contains([]string{"page-grants", "page-close", "project"}, a.Kind) && !slices.ContainsFunc(s.state.Slides, func(p Slide) bool { return p.ID == a.Page }) {
		fail(w, 400, "页面不存在")
		return
	}
	if a.Kind == "pages-activate" {
		if len(a.Pages) == 0 || len(a.Pages) > len(s.state.Slides) || len(a.Learners) == 0 {
			fail(w, 400, "请选择页面和学员")
			return
		}
		seenPages := map[string]bool{}
		for _, id := range a.Pages {
			if seenPages[id] || !slices.ContainsFunc(s.state.Slides, func(p Slide) bool { return p.ID == id }) {
				fail(w, 400, "页面名单无效")
				return
			}
			seenPages[id] = true
		}
	}
	seen := map[string]bool{}
	for _, id := range a.Learners {
		if seen[id] || !s.learnerEnabled(id) {
			fail(w, 400, "学员名单无效")
			return
		}
		seen[id] = true
	}
	for _, id := range a.Modules {
		if !slices.ContainsFunc(modules, func(m Module) bool { return m.ID == id }) {
			fail(w, 400, "实训模块无效")
			return
		}
	}
	if (a.Kind == "modules-open" || a.Kind == "modules-close") && len(a.Learners) == 0 {
		fail(w, 400, "请选择学员")
		return
	}
	if a.Kind == "modules-open" && len(a.Modules) == 0 {
		fail(w, 400, "请选择实训")
		return
	}
	b, _ := json.Marshal(s.state)
	var next State
	_ = json.Unmarshal(b, &next)
	for i := range next.Learners {
		l := &next.Learners[i]
		selected := slices.Contains(a.Learners, l.ID)
		switch a.Kind {
		case "pages-activate":
			if selected {
				for _, page := range a.Pages {
					if !slices.Contains(l.Pages, page) {
						l.Pages = append(l.Pages, page)
					}
				}
				l.Current = a.Pages[0]
			}
		case "page-grants":
			if selected {
				if !slices.Contains(l.Pages, a.Page) {
					l.Pages = append(l.Pages, a.Page)
					l.Current = a.Page
				}
			} else {
				l.Pages = slices.DeleteFunc(l.Pages, func(p string) bool { return p == a.Page })
				if l.Current == a.Page {
					l.Current = ""
				}
			}
		case "page-close":
			l.Pages = slices.DeleteFunc(l.Pages, func(p string) bool { return p == a.Page })
			if l.Current == a.Page {
				l.Current = ""
			}
		case "modules-close":
			if selected {
				l.Modules = []string{}
			}
		}
		if selected && (a.Kind == "modules-open" || a.Kind == "page-grants") {
			for _, m := range a.Modules {
				if !slices.Contains(l.Modules, m) {
					l.Modules = append(l.Modules, m)
				}
			}
		}
	}
	if a.Kind == "pages-activate" {
		next.Closed = slices.DeleteFunc(next.Closed, func(p string) bool { return slices.Contains(a.Pages, p) })
	}
	if a.Kind == "page-grants" || a.Kind == "page-close" {
		next.Closed = slices.DeleteFunc(next.Closed, func(p string) bool { return p == a.Page })
		if a.Kind == "page-close" || len(a.Learners) == 0 {
			next.Closed = append(next.Closed, a.Page)
			if next.Projection == a.Page {
				next.Projection = ""
			}
		}
	}
	if a.Kind == "project" {
		next.Projection = a.Page
	}
	if a.Kind == "pause" {
		next.Projection = ""
	}
	next.Revision++
	next.Audit = append(next.Audit, Event{At: time.Now().UTC(), Actor: ss.ID, Action: a})
	if err := s.persist(next); err != nil {
		fail(w, 500, "保存失败，权限未改变")
		return
	}
	s.state = next
	if ss.Role == "projector" {
		respond(w, 200, map[string]any{"ok": true, "revision": next.Revision})
		return
	}
	s.classroom(w)
}
