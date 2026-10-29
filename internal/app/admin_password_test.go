package app

import (
	"testing"
)

func TestAdminPasswordSixCharacterMinimum(t *testing.T) {
	s, cfg := newTestServer(t)
	for _, next := range []string{"12345", "一二三四五"} {
		mustStatus(t, request(s, "POST", "/admin/api/password", "admin-test", map[string]string{"current": cfg.AdminPassword, "next": next}), 422)
	}
	mustStatus(t, request(s, "POST", "/admin/api/password", "admin-test", map[string]string{"current": "incorrect", "next": "abcdef"}), 403)
	mustStatus(t, request(s, "POST", "/admin/api/password", "admin-test", map[string]string{"current": cfg.AdminPassword, "next": "abcdef"}), 200)
	mustStatus(t, request(s, "GET", "/admin/api/me", "admin-test", nil), 401)
	mustStatus(t, request(s, "POST", "/admin/api/login", "", map[string]string{"id": "admin", "password": cfg.AdminPassword}), 401)
	login := request(s, "POST", "/admin/api/login", "", map[string]string{"id": "admin", "password": "abcdef"})
	mustStatus(t, login, 200)
	mustStatus(t, request(s, "POST", "/admin/api/password", login.Result().Cookies()[0].Value, map[string]string{"current": "abcdef", "next": "一二三四五六"}), 200)
	reopened, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	mustStatus(t, request(reopened, "POST", "/admin/api/login", "", map[string]string{"id": "admin", "password": "一二三四五六"}), 200)
}

func TestBootstrapAdminPasswordMinimum(t *testing.T) {
	for _, password := range []string{"12345", "一二三四五", "123456"} {
		t.Run(password, func(t *testing.T) {
			s, err := New(Config{DataDir: t.TempDir(), AdminPassword: password})
			if password == "123456" {
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				mustStatus(t, request(s, "POST", "/admin/api/login", "", map[string]string{"id": "admin", "password": password}), 200)
			} else if err == nil {
				s.Close()
				t.Fatal("accepted a password shorter than six characters")
			}
		})
	}
}
