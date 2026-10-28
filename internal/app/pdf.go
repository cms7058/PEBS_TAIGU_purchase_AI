package app

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

func (s *Server) slideImage(w http.ResponseWriter, r *http.Request, ss session, id string) {
	if r.Method != "GET" {
		fail(w, 405, "不支持此方法")
		return
	}
	allowed := ss.Role == "admin" || ss.Role == "projector" && s.state.Projection == id
	for _, l := range s.state.Learners {
		if l.ID == ss.ID && ss.Role == "learner" && slices.Contains(l.Pages, id) {
			allowed = true
		}
	}
	if !allowed {
		fail(w, 403, "无该页访问权限")
		return
	}
	for _, p := range s.state.Slides {
		if p.ID == id && p.Asset != "" {
			http.ServeFile(w, r, filepath.Join(s.cfg.DataDir, "courseware", p.Asset))
			return
		}
	}
	fail(w, 404, "页面图片不存在")
}
func pdfAvailable() bool {
	_, a := exec.LookPath("pdftoppm")
	_, b := exec.LookPath("pdfinfo")
	return a == nil && b == nil
}
func renderPDF(parent context.Context, raw []byte, dir string) (pages []Slide, err error) {
	if !pdfAvailable() {
		return nil, fmt.Errorf("此部署未启用PDF渲染，请选择PDF运行镜像或使用PPTX文本预览")
	}
	id := "pdf-" + randomToken()[:16]
	folder := filepath.Join(dir, "courseware", id)
	if err = os.MkdirAll(folder, 0700); err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			os.RemoveAll(folder)
		}
	}()
	input := filepath.Join(folder, "source.pdf")
	if err = os.WriteFile(input, raw, 0600); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	info := exec.CommandContext(ctx, "pdfinfo", input)
	info.Env = append(os.Environ(), "LC_ALL=C")
	data, err := info.Output()
	if err != nil {
		return nil, fmt.Errorf("无法读取PDF，请检查是否损坏或加密")
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			count, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Pages:")))
		}
	}
	if count < 1 || count > 200 {
		return nil, fmt.Errorf("PDF需要1–200页")
	}
	command := exec.CommandContext(ctx, "pdftoppm", "-scale-to", "1600", "-png", input, filepath.Join(folder, "page"))
	command.Stderr = &bytes.Buffer{}
	if err = command.Run(); err != nil {
		return nil, fmt.Errorf("PDF渲染失败或超过60秒，请拆分课件后重试")
	}
	names, err := filepath.Glob(filepath.Join(folder, "page-*.png"))
	if err != nil || len(names) != count {
		return nil, fmt.Errorf("PDF页数与渲染结果不符")
	}
	slices.Sort(names)
	// Extract authorized page text once at import time for course Q&A. Image-only
	// pages retain an explicit missing-text marker instead of fabricated content.
	texts := make([]string, count)
	textFile := filepath.Join(folder, "pages.txt")
	if textErr := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", input, textFile).Run(); textErr == nil {
		os.Chmod(textFile, 0600)
		if stat, statErr := os.Stat(textFile); statErr == nil && stat.Size() <= 4<<20 {
			if rawText, readErr := os.ReadFile(textFile); readErr == nil {
				for i, text := range strings.Split(string(rawText), "\f") {
					if i < count {
						texts[i] = shortText(strings.TrimSpace(text), 8000)
					}
				}
			}
		}
	}
	var total int64
	for i, name := range names {
		f, err := os.Stat(name)
		if err != nil {
			return nil, err
		}
		total += f.Size()
		if total > 200<<20 {
			return nil, fmt.Errorf("课件渲染结果超过200MB，请压缩后上传")
		}
		os.Chmod(name, 0600)
		description := texts[i]
		if description == "" {
			description = "该页未提取到文字，请讲师补充课件文字说明。"
		}
		pages = append(pages, Slide{ID: fmt.Sprintf("P%03d", i+1), Title: fmt.Sprintf("课件第%d页", i+1), Description: description, Steps: []string{}, Suggested: "M01", Asset: id + "/" + filepath.Base(name)})
	}
	success = true
	return pages, nil
}
