package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

func pptxSlides(raw []byte) ([]Slide, error) {
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("PPTX文件无效")
	}
	files := map[string]*zip.File{}
	var total uint64
	for _, f := range z.File {
		total += f.UncompressedSize64
		if total > 100<<20 || f.UncompressedSize64 > 10<<20 {
			return nil, fmt.Errorf("课件解压体积过大")
		}
		files[f.Name] = f
	}
	read := func(name string) ([]byte, error) {
		f := files[name]
		if f == nil {
			return nil, fmt.Errorf("课件缺少 %s", name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(io.LimitReader(rc, 10<<20))
	}
	relData, err := read("ppt/_rels/presentation.xml.rels")
	if err != nil {
		return nil, err
	}
	var rels struct {
		Items []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
			Mode   string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	if xml.Unmarshal(relData, &rels) != nil {
		return nil, fmt.Errorf("课件关联信息错误")
	}
	mapping := map[string]string{}
	for _, rel := range rels.Items {
		if rel.Mode != "External" {
			name := path.Clean(path.Join("ppt", rel.Target))
			if strings.HasPrefix(rel.Target, "/") {
				name = strings.TrimPrefix(path.Clean(rel.Target), "/")
			}
			mapping[rel.ID] = name
		}
	}
	pres, err := read("ppt/presentation.xml")
	if err != nil {
		return nil, err
	}
	dec := xml.NewDecoder(bytes.NewReader(pres))
	order := []string{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if el, ok := tok.(xml.StartElement); ok && el.Name.Local == "sldId" {
			for _, a := range el.Attr {
				if a.Name.Local == "id" && a.Name.Space != "" {
					order = append(order, mapping[a.Value])
				}
			}
		}
	}
	if len(order) == 0 || len(order) > 200 {
		return nil, fmt.Errorf("课件需要1–200页")
	}
	out := []Slide{}
	for i, name := range order {
		data, err := read(name)
		if err != nil {
			return nil, err
		}
		d := xml.NewDecoder(bytes.NewReader(data))
		texts := []string{}
		for {
			tok, err := d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if el, ok := tok.(xml.StartElement); ok && el.Name.Local == "t" {
				var value string
				if err := d.DecodeElement(&value, &el); err != nil {
					return nil, err
				}
				if strings.TrimSpace(value) != "" {
					texts = append(texts, value)
				}
			}
		}
		title := "第" + strconv.Itoa(i+1) + "页"
		if len(texts) > 0 {
			title = texts[0]
			texts = texts[1:]
		}
		if len([]rune(title)) > 120 {
			title = string([]rune(title)[:120])
		}
		desc := strings.Join(texts, " · ")
		if len([]rune(desc)) > 4000 {
			desc = string([]rune(desc)[:4000])
		}
		out = append(out, Slide{ID: fmt.Sprintf("P%03d", i+1), Title: title, Description: desc, Steps: []string{}, Suggested: "M01"})
	}
	return out, nil
}

func (s *Server) coursewareAPI(w http.ResponseWriter, r *http.Request, ss session) {
	if r.Method == "GET" {
		respond(w, 200, map[string]any{"slides": s.state.Slides, "pdfAvailable": pdfAvailable(), "formats": []string{"pptx-text", "pdf", "json"}, "notice": "保留版式与图片请上传从PPT导出的PDF（需启用PDF渲染）。PPTX直接导入为按原页序排列的文本预览，不保留原版式、图片、动画和视频。"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	var in struct {
		Revision int     `json:"revision"`
		Slides   []Slide `json:"slides"`
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			fail(w, 413, "课件超过10MB或格式错误")
			return
		}
		defer r.MultipartForm.RemoveAll()
		file, header, err := r.FormFile("file")
		if err != nil {
			fail(w, 400, "请选择课件")
			return
		}
		defer file.Close()
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext != ".pptx" && ext != ".pdf" {
			fail(w, 422, "请选择PPTX或PDF课件")
			return
		}
		raw, err := io.ReadAll(file)
		if err != nil {
			fail(w, 400, "读取课件失败")
			return
		}
		in.Revision, _ = strconv.Atoi(r.FormValue("revision"))
		if ext == ".pdf" {
			s.mu.Unlock()
			in.Slides, err = renderPDF(r.Context(), raw, s.cfg.DataDir)
			s.mu.Lock()
		} else {
			in.Slides, err = pptxSlides(raw)
		}
		if err != nil {
			fail(w, 422, err.Error())
			return
		}
	} else {
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		if d.Decode(&in) != nil {
			fail(w, 400, "页面JSON格式错误")
			return
		}
	}
	if in.Revision != s.state.Revision {
		fail(w, 409, "课堂已更新，请重新载入后导入")
		return
	}
	if len(in.Slides) < 1 || len(in.Slides) > 200 {
		fail(w, 422, "需要1–200页课件")
		return
	}
	ids := map[string]bool{}
	for _, p := range in.Slides {
		if p.Asset != "" {
			parts := strings.Split(p.Asset, "/")
			if len(parts) != 2 || !strings.HasPrefix(parts[0], "pdf-") || strings.Contains(p.Asset, "..") || path.Clean(p.Asset) != p.Asset || !strings.HasSuffix(p.Asset, ".png") {
				fail(w, 422, "课件资源路径无效")
				return
			}
			if _, err := os.Stat(filepath.Join(s.cfg.DataDir, "courseware", p.Asset)); err != nil {
				fail(w, 422, "课件图片资源不存在")
				return
			}
		}
		if p.ID == "" || len(p.ID) > 40 || ids[p.ID] || len(p.Title) == 0 || len(p.Title) > 1000 || len(p.Description) > 16000 || !slices.ContainsFunc(modules, func(m Module) bool { return m.ID == p.Suggested }) {
			fail(w, 422, "页码、标题、内容或关联模块无效")
			return
		}
		ids[p.ID] = true
	}
	b, _ := json.Marshal(s.state)
	var next State
	json.Unmarshal(b, &next)
	next.Slides = in.Slides
	next.Projection = ""
	next.Closed = []string{}
	for i := range next.Learners {
		next.Learners[i].Pages = []string{}
		next.Learners[i].Current = ""
	}
	next.Revision++
	next.Audit = append(next.Audit, Event{At: nowUTC(), Actor: ss.ID, Action: Action{Kind: "courseware-replace", Revision: s.state.Revision}})
	if s.persist(next) != nil {
		fail(w, 500, "课件保存失败")
		return
	}
	s.state = next
	s.classroom(w)
}
