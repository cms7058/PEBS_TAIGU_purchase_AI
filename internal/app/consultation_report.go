package app

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ReportBlock struct {
	Label string `json:"label"`
	Text  string `json:"text"`
	Kind  string `json:"kind"`
}
type ReportSection struct {
	ID     string        `json:"id"`
	Title  string        `json:"title"`
	Blocks []ReportBlock `json:"blocks"`
}
type ReportPoint struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}
type ReportChart struct {
	Title  string        `json:"title"`
	Unit   string        `json:"unit"`
	Note   string        `json:"note"`
	Basis  string        `json:"basis"`
	Points []ReportPoint `json:"points"`
}
type ReportFlow struct {
	Title string   `json:"title"`
	Note  string   `json:"note"`
	Nodes []string `json:"nodes"`
}
type ConsultationReport struct {
	Version  string          `json:"version"`
	Title    string          `json:"title"`
	Raw      string          `json:"raw"`
	Sections []ReportSection `json:"sections"`
	Charts   []ReportChart   `json:"charts"`
	Flows    []ReportFlow    `json:"flows"`
	Warnings []string        `json:"warnings"`
}

var reportHeading = regexp.MustCompile(`^(?:[一二三四五六七八九十百]+[、．]|#{1,4}\s+|[0-9]+[、．]\s*)`)
var reportLabel = regexp.MustCompile(`^【([^】]{1,80})】\s*`)
var baselineHours = regexp.MustCompile(`(?:每日|每天|学员陈述的|学员所说的)[^。；\n]{0,16}?([0-9]+(?:\.[0-9]+)?)\s*小时`)
var targetHours = regexp.MustCompile(`目标(?:约|为约|为)?\s*([0-9]+(?:\.[0-9]+)?)\s*小时`)

func normalizeAnalysisText(text string) (string, []string) {
	text = strings.TrimSpace(text)
	warnings := []string{}
	candidate := text
	if strings.HasPrefix(candidate, "```") {
		candidate = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(candidate, "```json"), "```"), "```"))
	}
	var wrapped struct {
		Analysis string `json:"analysis"`
	}
	if json.Unmarshal([]byte(candidate), &wrapped) == nil && wrapped.Analysis != "" {
		text = wrapped.Analysis
	} else if strings.HasPrefix(candidate, "{") && strings.Contains(candidate, `"analysis"`) && json.Unmarshal([]byte(candidate+"}"), &wrapped) == nil && wrapped.Analysis != "" {
		text = wrapped.Analysis
		warnings = append(warnings, "输入的analysis JSON末尾缺少右括号，已提取正文；请对照原文核对。")
	}
	if !strings.Contains(text, "\n") && strings.Contains(text, `\n`) {
		text = strings.ReplaceAll(text, `\n`, "\n")
		warnings = append(warnings, "已将转义换行转换为正文段落。")
	}
	return strings.ReplaceAll(text, "\r\n", "\n"), warnings
}
func blockKind(label string) string {
	switch {
	case strings.Contains(label, "待确认") || strings.Contains(label, "待核实"):
		return "unknown"
	case strings.Contains(label, "局限") || strings.Contains(label, "问题") || strings.Contains(label, "风险"):
		return "risk"
	case strings.Contains(label, "验证") || strings.Contains(label, "指标") || strings.Contains(label, "判据"):
		return "measure"
	case strings.Contains(label, "成本") || strings.Contains(label, "前置"):
		return "condition"
	case strings.Contains(label, "建议") || strings.Contains(label, "阶段"):
		return "action"
	case strings.Contains(label, "优势"):
		return "advantage"
	case strings.Contains(label, "依据") || strings.Contains(label, "支持的判断"):
		return "evidence"
	}
	return "text"
}

// This adapter reorganizes existing text; it does not create business conclusions.
// Numeric charts are deliberately limited to an explicitly labelled baseline/goal.
func buildConsultationReport(text, title string) *ConsultationReport {
	raw, warnings := normalizeAnalysisText(text)
	report := &ConsultationReport{Version: "report-v1", Title: title, Raw: raw, Sections: []ReportSection{}, Charts: []ReportChart{}, Flows: []ReportFlow{}, Warnings: warnings}
	current := ReportSection{ID: "section-1", Title: "答复与建议", Blocks: []ReportBlock{}}
	flush := func() {
		if len(current.Blocks) > 0 {
			report.Sections = append(report.Sections, current)
		}
	}
	// Headings are recognized on their own lines, preserving all remaining text.
	for _, paragraph := range strings.Split(raw, "\n\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		lines := strings.Split(paragraph, "\n")
		if len([]rune(lines[0])) <= 100 && reportHeading.MatchString(lines[0]) {
			flush()
			current = ReportSection{ID: fmt.Sprintf("section-%d", len(report.Sections)+1), Title: strings.TrimLeft(lines[0], "# "), Blocks: []ReportBlock{}}
			paragraph = strings.TrimSpace(strings.Join(lines[1:], "\n"))
			if paragraph == "" {
				continue
			}
		}
		label := "说明"
		if match := reportLabel.FindStringSubmatch(paragraph); len(match) > 1 {
			label = match[1]
			paragraph = reportLabel.ReplaceAllString(paragraph, "")
		}
		current.Blocks = append(current.Blocks, ReportBlock{label, paragraph, blockKind(label)})
	}
	flush()
	if len(report.Sections) == 0 {
		report.Sections = []ReportSection{{ID: "section-1", Title: "答复与建议", Blocks: []ReportBlock{{Label: "说明", Text: raw, Kind: "text"}}}}
	}
	steps, phases := []string{}, []string{}
	counts := map[string]float64{"advantage": 0, "risk": 0, "action": 0, "measure": 0, "unknown": 0}
	for _, section := range report.Sections {
		if strings.Contains(section.Title, "步骤") {
			steps = append(steps, section.Title)
		}
		for _, block := range section.Blocks {
			if _, ok := counts[block.Kind]; ok {
				counts[block.Kind]++
			}
			if strings.HasPrefix(block.Label, "阶段") {
				phases = append(phases, block.Label)
			}
			if len(report.Charts) == 0 && strings.Contains(block.Label, "效率指标") && (strings.Contains(block.Text, "学员陈述") || strings.Contains(block.Text, "学员所说")) {
				baseline, target := baselineHours.FindStringSubmatch(block.Text), targetHours.FindStringSubmatch(block.Text)
				if len(baseline) > 1 && len(target) > 1 {
					b, e1 := strconv.ParseFloat(baseline[1], 64)
					t, e2 := strconv.ParseFloat(target[1], 64)
					if e1 == nil && e2 == nil && b > 0 && t >= 0 && b < 10000 && t < 10000 {
						report.Charts = append(report.Charts, ReportChart{Title: "核对耗时：陈述基线与计划目标", Unit: "小时／日", Note: "基线来自学员陈述，尚未实测；目标为计划值，不代表已经实现改善。", Basis: block.Text, Points: []ReportPoint{{"陈述基线（待核实）", b}, {"计划目标（未验证）", t}}})
					}
				}
			}
		}
	}
	if len(steps) > 1 {
		report.Flows = append(report.Flows, ReportFlow{"五步复盘路径", "按报告章节梳理，不代表企业实际审批顺序。", steps})
	}
	if len(phases) > 1 {
		report.Flows = append(report.Flows, ReportFlow{"建议实施路线", "依据报告中的阶段建议，实际时间和审批仍需确认。", phases})
	}
	points := []ReportPoint{}
	for _, pair := range [][2]string{{"advantage", "优势"}, {"risk", "问题与局限"}, {"action", "建议与阶段"}, {"measure", "验证指标"}, {"unknown", "待确认"}} {
		if counts[pair[0]] > 0 {
			points = append(points, ReportPoint{pair[1], counts[pair[0]]})
		}
	}
	if len(points) > 1 {
		report.Charts = append(report.Charts, ReportChart{"报告分析要点分布", "栏目数", "仅统计本报告对应栏目的数量，不是企业绩效、问题严重度或学员评分。", "依据报告中【栏目标题】分类计数。", points})
	}
	if len(report.Flows) == 0 {
		report.Flows = append(report.Flows, ReportFlow{"讨论与落实路径", "系统讨论流程示意。", []string{"阅读管理员答复", "选择需要澄清的问题", "向管理员继续追问", "核对补充回复", "记录待验证事项"}})
	}
	return report
}
