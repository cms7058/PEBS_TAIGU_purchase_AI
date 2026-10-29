package app

import (
	"net/http"
	"slices"
)

func (s *Server) projectionAPI(w http.ResponseWriter, r *http.Request, ss session) {
	if ss.Role != "admin" && ss.Role != "projector" {
		fail(w, 403, "仅限投影显示端")
		return
	}
	index := slices.IndexFunc(s.state.Slides, func(p Slide) bool { return p.ID == s.state.Projection })
	canControl := ss.Role == "admin" || ss.ProjectionControl
	if r.Method == "POST" {
		if !canControl {
			fail(w, 403, "此投影凭证仅支持观看，请由管理员重新打开播放窗口")
			return
		}
		var in struct {
			Revision  int    `json:"revision"`
			Direction string `json:"direction"`
		}
		if !decode(w, r, &in) {
			return
		}
		if in.Revision != s.state.Revision {
			fail(w, 409, "投影页面已更新，请重试")
			return
		}
		if index < 0 {
			fail(w, 409, "当前未开始播放，请在管理员端投影页面")
			return
		}
		next := index
		switch in.Direction {
		case "previous":
			next--
		case "next":
			next++
		default:
			fail(w, 400, "翻页方向无效")
			return
		}
		if next < 0 || next >= len(s.state.Slides) {
			fail(w, 422, "已到课件首页或末页")
			return
		}
		s.action(w, ss, Action{Kind: "project", Revision: in.Revision, Page: s.state.Slides[next].ID})
		return
	}
	var slide *Slide
	if index >= 0 {
		value := s.state.Slides[index]
		slide = &value
	}
	respond(w, 200, map[string]any{"revision": s.state.Revision, "slide": slide, "index": index, "total": len(s.state.Slides), "canControl": canControl})
}
