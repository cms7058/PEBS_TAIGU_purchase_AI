package app

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type CustomColumn struct {
	Column
	Required bool   `json:"required"`
	Purpose  string `json:"purpose"`
}
type SourceSnapshot struct {
	ID        string              `json:"id"`
	System    string              `json:"system"`
	Object    string              `json:"object"`
	KeyField  string              `json:"keyField"`
	Collector string              `json:"collector"`
	AsOf      string              `json:"asOf"`
	Format    string              `json:"format"`
	Fields    []string            `json:"fields"`
	Rows      []map[string]string `json:"rows"`
}
type AcquisitionPlan struct {
	Mode      string           `json:"mode"`
	System    string           `json:"system"`
	Collector string           `json:"collector"`
	AsOf      string           `json:"asOf"`
	Note      string           `json:"note"`
	Sources   []SourceSnapshot `json:"sources"`
	Bindings  []FieldBinding   `json:"bindings"`
	Assembled bool             `json:"assembled"`
	Warnings  []string         `json:"warnings,omitempty"`
}
type AcquisitionHeader struct {
	Mode      string `json:"mode"`
	System    string `json:"system"`
	Collector string `json:"collector"`
	AsOf      string `json:"asOf"`
	Note      string `json:"note"`
}

var customKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

func attemptColumns(a *TrainingAttempt) []Column {
	cols := slices.Clone(caseSpec(a.Module).Columns)
	for _, c := range a.CustomColumns {
		cols = append(cols, c.Column)
	}
	return cols
}
func validateCustomColumns(module string, cols []CustomColumn) error {
	if len(cols) > 20 {
		return fmt.Errorf("每份练习最多20个自定义字段")
	}
	seen := map[string]bool{"constructor": true, "prototype": true}
	for _, c := range caseSpec(module).Columns {
		seen[c.Key] = true
	}
	for _, c := range cols {
		if !customKeyPattern.MatchString(c.Key) || seen[c.Key] || len([]rune(c.Label)) < 1 || len([]rune(c.Label)) > 40 || len(c.Unit) > 60 || len([]rune(c.Purpose)) > 200 {
			return fmt.Errorf("自定义字段键须唯一且为2–32位小写字母/数字/下划线，以字母开头，不能覆盖标准字段")
		}
		if !slices.Contains([]string{"text", "number", "percent", "bool", "date"}, c.Kind) {
			return fmt.Errorf("字段类型无效")
		}
		seen[c.Key] = true
	}
	return nil
}
func validateParameters(module string, values map[string]float64) error {
	allowed := map[string]ParameterDefinition{}
	for _, p := range enterpriseBlueprint(module).Parameters {
		allowed[p.Key] = p
	}
	for k, v := range values {
		d, ok := allowed[k]
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < d.Min || v > d.Max {
			return fmt.Errorf("场景参数%s无效或超出范围", k)
		}
	}
	if module == "M03" {
		sum := parameter(module, "onTimeWeight", values) + parameter(module, "qualityWeight", values) + parameter(module, "shareWeight", values)
		if math.Abs(sum-100) > .0001 {
			return fmt.Errorf("供应商评分权重合计必须为100%%")
		}
	}
	return nil
}
func validateAttemptRows(a *TrainingAttempt, rows []map[string]string) []string {
	errors := validateRows(caseSpec(a.Module), rows)
	for i, row := range rows {
		for _, c := range a.CustomColumns {
			value := strings.TrimSpace(row[c.Key])
			prefix := fmt.Sprintf("第%d行 / %s：", i+2, c.Label)
			if value == "" {
				if c.Required {
					errors = append(errors, prefix+"必填")
				}
				continue
			}
			if len([]rune(value)) > 500 {
				errors = append(errors, prefix+"最多500字")
				continue
			}
			if c.Kind == "date" {
				if _, err := time.Parse("2006-01-02", value); err != nil {
					errors = append(errors, prefix+"请用YYYY-MM-DD日期")
				}
			}
			if c.Kind != "text" && c.Kind != "date" {
				n, err := strconv.ParseFloat(value, 64)
				if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 1e9 || (c.Kind == "percent" && n > 100) || (c.Kind == "bool" && n != 0 && n != 1) {
					errors = append(errors, prefix+"类型或范围不符")
				}
			}
		}
		if len(errors) >= 30 {
			return errors[:30]
		}
	}
	return errors
}
func invalidateData(a *TrainingAttempt) {
	a.Result = nil
	a.Steps = slices.DeleteFunc(a.Steps, func(s string) bool { return s == "data" || s == "calculate" || s == "review" })
	a.Active = false
}
func scenarioRows(module string) []map[string]string {
	base := caseSpec(module).Rows
	rows := []map[string]string{}
	for i := 0; i < 24; i++ {
		row := map[string]string{}
		for k, v := range base[i%len(base)] {
			row[k] = v
		}
		row["id"] = fmt.Sprintf("%s-%03d", row["id"], i+1)
		switch module {
		case "M01":
			row["planned"] = strconv.Itoa(8 + i%17)
		case "M02":
			row["price"] = strconv.Itoa(800 + i*37)
			row["lead"] = strconv.Itoa(2 + i%14)
		case "M03":
			row["share"] = formatFloat(100.0 / 24)
			row["ontime"] = strconv.Itoa(80 + i%20)
		case "M04":
			row["confirmed"] = strconv.Itoa(2 + i%13)
		case "M05":
			row["daily"] = formatFloat(.2 + float64(i%11)*.3)
		case "M06":
			row["eta"] = strconv.Itoa(2 + i%11)
			row["cost"] = strconv.Itoa(9000 + i*1300)
		}
		rows = append(rows, row)
	}
	return rows
}
func sourceSample(module, id string) SourceSnapshot {
	b := enterpriseBlueprint(module)
	idx := slices.IndexFunc(b.Sources, func(s SourceDefinition) bool { return s.ID == id })
	if idx < 0 {
		return SourceSnapshot{}
	}
	def := b.Sources[idx]
	snapshot := SourceSnapshot{ID: id, System: def.Name + "（仿真）", Object: def.Object, KeyField: def.Key, AsOf: nowUTC().Format(timeFormat), Format: "sample", Fields: []string{def.Key, "work_package", "site", "ownership"}, Rows: []map[string]string{}}
	for _, key := range def.Keys {
		snapshot.Fields = append(snapshot.Fields, sourceAliases[key])
	}
	for i, canonical := range scenarioRows(module) {
		row := map[string]string{def.Key: canonical["id"], "work_package": fmt.Sprintf("WP-%03d", i/3+1), "site": "XM", "ownership": []string{"OWN", "CUSTOMER"}[i%2]}
		for _, key := range def.Keys {
			row[sourceAliases[key]] = canonical[key]
		}
		snapshot.Rows = append(snapshot.Rows, row)
	}
	return snapshot
}
func parseSource(content, format string) ([]string, []map[string]string, error) {
	fields := []string{}
	rows := []map[string]string{}
	if format == "csv" {
		r := csv.NewReader(strings.NewReader(strings.TrimPrefix(content, "\ufeff")))
		r.FieldsPerRecord = -1
		records, err := r.ReadAll()
		if err != nil || len(records) < 2 {
			return nil, nil, fmt.Errorf("CSV需要表头和数据")
		}
		fields = records[0]
		for i, record := range records[1:] {
			if len(record) != len(fields) {
				return nil, nil, fmt.Errorf("第%d行列数错误", i+2)
			}
			row := map[string]string{}
			for j, f := range fields {
				row[f] = strings.TrimSpace(record[j])
			}
			rows = append(rows, row)
		}
	} else if format == "json" {
		var raw any
		decoder := json.NewDecoder(strings.NewReader(content))
		decoder.UseNumber()
		if err := decoder.Decode(&raw); err != nil {
			return nil, nil, fmt.Errorf("JSON格式无效")
		}
		if decoder.Decode(new(any)) != io.EOF {
			return nil, nil, fmt.Errorf("JSON包含多份文档或尾部无效内容")
		}
		if wrapper, ok := raw.(map[string]any); ok {
			for _, key := range []string{"rows", "data", "records"} {
				if value, found := wrapper[key]; found {
					raw = value
					break
				}
			}
		}
		array, ok := raw.([]any)
		if !ok {
			return nil, nil, fmt.Errorf("JSON应为对象数组，或包含rows/data/records数组")
		}
		all := map[string]bool{}
		for _, item := range array {
			obj, ok := item.(map[string]any)
			if !ok {
				return nil, nil, fmt.Errorf("每行必须是扁平JSON对象")
			}
			row := map[string]string{}
			for key, value := range obj {
				all[key] = true
				switch v := value.(type) {
				case string:
					row[key] = v
				case json.Number:
					row[key] = string(v)
				case bool:
					if v {
						row[key] = "1"
					} else {
						row[key] = "0"
					}
				case nil:
					row[key] = ""
				default:
					return nil, nil, fmt.Errorf("字段%s为嵌套对象，请先导出扁平记录", key)
				}
			}
			rows = append(rows, row)
		}
		for key := range all {
			fields = append(fields, key)
		}
		slices.Sort(fields)
	} else {
		return nil, nil, fmt.Errorf("支持CSV或JSON导出快照")
	}
	if len(rows) < 1 || len(rows) > 5000 || len(fields) < 1 || len(fields) > 60 {
		return nil, nil, fmt.Errorf("每来源需要1–5000行、1–60列")
	}
	seen := map[string]bool{}
	for _, field := range fields {
		if field == "" || len(field) > 100 || seen[field] || slices.Contains([]string{"__proto__", "constructor", "prototype"}, field) {
			return nil, nil, fmt.Errorf("来源表头为空、重复或无效")
		}
		seen[field] = true
	}
	for _, row := range rows {
		for _, value := range row {
			if len([]rune(value)) > 500 {
				return nil, nil, fmt.Errorf("来源单元格最多500字")
			}
		}
	}
	return fields, rows, nil
}
func validateSnapshot(source SourceSnapshot) error {
	if len(source.System) == 0 || len(source.System) > 160 || len(source.Object) == 0 || len(source.Object) > 200 || len(source.Collector) == 0 || len(source.Collector) > 80 {
		return fmt.Errorf("请填写来源系统、采集人和有效业务对象")
	}
	if _, err := time.Parse(time.RFC3339, source.AsOf); err != nil {
		return fmt.Errorf("采集时间请使用包含时区的ISO时间")
	}
	if !slices.Contains(source.Fields, source.KeyField) {
		return fmt.Errorf("业务主键字段不存在")
	}
	seen := map[string]bool{}
	for i, row := range source.Rows {
		key := strings.TrimSpace(row[source.KeyField])
		if key == "" || seen[key] {
			return fmt.Errorf("第%d行业务主键为空或重复；请按约定粒度先聚合，不能静默合并多行", i+2)
		}
		seen[key] = true
	}
	return nil
}
func assembleSources(a *TrainingAttempt, bindings []FieldBinding) ([]map[string]string, error) {
	if a.Acquisition == nil {
		return nil, fmt.Errorf("尚未采集来源")
	}
	maps := map[string]map[string]map[string]string{}
	sources := map[string]SourceSnapshot{}
	for _, s := range a.Acquisition.Sources {
		index := map[string]map[string]string{}
		for _, row := range s.Rows {
			index[strings.TrimSpace(row[s.KeyField])] = row
		}
		maps[s.ID] = index
		sources[s.ID] = s
	}
	targets := map[string]FieldBinding{}
	validTargets := map[string]bool{}
	for _, c := range attemptColumns(a) {
		validTargets[c.Key] = true
	}
	for _, b := range bindings {
		source, ok := sources[b.Source]
		if !validTargets[b.Target] || targets[b.Target].Target != "" || !ok || !slices.Contains(source.Fields, b.Field) {
			return nil, fmt.Errorf("字段%s的映射无效、重复或来源尚未采集", b.Target)
		}
		targets[b.Target] = b
	}
	for _, c := range caseSpec(a.Module).Columns {
		if targets[c.Key].Target == "" {
			return nil, fmt.Errorf("标准字段%s尚未映射", c.Label)
		}
	}
	primary := targets["id"]
	src := sources[primary.Source]
	a.Acquisition.Warnings = []string{}
	for _, s := range a.Acquisition.Sources {
		extra := 0
		for key := range maps[s.ID] {
			if _, ok := maps[src.ID][key]; !ok {
				extra++
			}
		}
		if extra > 0 {
			a.Acquisition.Warnings = append(a.Acquisition.Warnings, fmt.Sprintf("来源%s有%d条额外主键未纳入主来源范围，请确认统计窗口及过滤规则", s.System, extra))
		}
	}
	if primary.Field != src.KeyField {
		return nil, fmt.Errorf("标准编号须映射到主来源的业务主键字段")
	}
	rows := []map[string]string{}
	for _, raw := range src.Rows {
		key := strings.TrimSpace(raw[src.KeyField])
		row := map[string]string{}
		for _, c := range attemptColumns(a) {
			b, ok := targets[c.Key]
			if !ok {
				row[c.Key] = ""
				continue
			}
			sourceRow, found := maps[b.Source][key]
			if !found {
				return nil, fmt.Errorf("主键%s在来源%s没有匹配记录，请核对粒度和采集窗口", key, b.Source)
			}
			row[c.Key] = strings.TrimSpace(sourceRow[b.Field])
		}
		rows = append(rows, row)
	}
	if errors := validateAttemptRows(a, rows); len(errors) > 0 {
		return nil, fmt.Errorf("关联后数据无效：%s", strings.Join(errors[:min(3, len(errors))], "；"))
	}
	return rows, nil
}
