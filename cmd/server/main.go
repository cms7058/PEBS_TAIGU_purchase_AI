package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"taigu.training/internal/app"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		client := http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://127.0.0.1:8080/healthz")
		if err != nil {
			os.Exit(1)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	maxCalls, _ := strconv.Atoi(env("AI_MAX_CALLS", "100"))
	s, err := app.New(app.Config{DataDir: env("DATA_DIR", "data"), AdminPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"), SecureCookie: os.Getenv("COOKIE_SECURE") == "true", AIBaseURL: os.Getenv("AI_BASE_URL"), AIModel: os.Getenv("AI_MODEL"), AIKey: os.Getenv("AI_API_KEY"), AIMaxCalls: maxCalls})
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	server := &http.Server{Addr: env("HTTP_ADDR", ":8080"), Handler: s, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("training server listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
