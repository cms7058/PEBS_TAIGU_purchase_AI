package app

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

type SourceImport struct {
	ID        string `json:"id"`
	System    string `json:"system"`
	Object    string `json:"object"`
	KeyField  string `json:"keyField"`
	Collector string `json:"collector"`
	AsOf      string `json:"asOf"`
	Format    string `json:"format"`
	Content   string `json:"content"`
}

func configureAttempt(a *TrainingAttempt, cols *[]CustomColumn, params map[string]float64, header *AcquisitionHeader) error {
	keepData := cols == nil && header == nil && params != nil && slices.Contains(a.Steps, "data")
	if cols != nil {
		if err := validateCustomColumns(a.Module, *cols); err != nil {
			return err
		}
		a.CustomColumns = *cols
		keep := map[string]bool{}
		for _, c := range attemptColumns(a) {
			keep[c.Key] = true
		}
		for _, row := range a.Rows {
			for key := range row {
				if !keep[key] {
					delete(row, key)
				}
			}
		}
	}
	if params != nil {
		if err := validateParameters(a.Module, params); err != nil {
			return err
		}
		a.Parameters = params
	}
	if header != nil {
		if !slices.Contains([]string{"sample", "manual", "mapped"}, header.Mode) || len(header.System) > 160 || len(header.Collector) > 80 || len([]rune(header.Note)) > 500 {
			return fmt.Errorf("来源设置无效")
		}
		if header.Mode == "manual" {
			if strings.TrimSpace(header.System) == "" || strings.TrimSpace(header.Collector) == "" {
				return fmt.Errorf("人工采集请填写数据来源和采集人")
			}
			if _, err := time.Parse(time.RFC3339, header.AsOf); err != nil {
				return fmt.Errorf("采集时间须包含时区")
			}
		}
		if a.Acquisition == nil {
			a.Acquisition = &AcquisitionPlan{Sources: []SourceSnapshot{}, Bindings: enterpriseBlueprint(a.Module).Bindings}
		}
		a.Acquisition.Mode = header.Mode
		a.Acquisition.System = header.System
		a.Acquisition.Collector = header.Collector
		a.Acquisition.AsOf = header.AsOf
		a.Acquisition.Note = header.Note
	}
	if cols != nil && a.Acquisition != nil && a.Acquisition.Mode == "mapped" {
		a.Acquisition.Assembled = false
	}
	invalidateData(a)
	if keepData {
		a.Steps = append(a.Steps, "data")
	}
	return nil
}
func importSource(a *TrainingAttempt, in *SourceImport) error {
	if in == nil {
		return fmt.Errorf("缺少来源数据")
	}
	defs := enterpriseBlueprint(a.Module).Sources
	if !slices.ContainsFunc(defs, func(s SourceDefinition) bool { return s.ID == in.ID }) {
		return fmt.Errorf("来源节点无效")
	}
	fields, rows, err := parseSource(in.Content, in.Format)
	if err != nil {
		return err
	}
	snapshot := SourceSnapshot{ID: in.ID, System: strings.TrimSpace(in.System), Object: in.Object, KeyField: in.KeyField, Collector: strings.TrimSpace(in.Collector), AsOf: in.AsOf, Format: in.Format, Fields: fields, Rows: rows}
	if err := validateSnapshot(snapshot); err != nil {
		return err
	}
	if a.Acquisition == nil {
		a.Acquisition = &AcquisitionPlan{Sources: []SourceSnapshot{}, Bindings: enterpriseBlueprint(a.Module).Bindings}
	}
	index := slices.IndexFunc(a.Acquisition.Sources, func(s SourceSnapshot) bool { return s.ID == in.ID })
	if index < 0 {
		a.Acquisition.Sources = append(a.Acquisition.Sources, snapshot)
	} else {
		a.Acquisition.Sources[index] = snapshot
	}
	a.Acquisition.Mode = "mapped"
	a.Acquisition.Assembled = false
	invalidateData(a)
	return nil
}
