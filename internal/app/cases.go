package app

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const timeFormat = "2006-01-02T15:04:05.999999999Z07:00"

type Column struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
	Unit  string `json:"unit"`
}
type CaseSpec struct {
	Module       string              `json:"module"`
	Version      string              `json:"version"`
	Title        string              `json:"title"`
	Goal         string              `json:"goal"`
	Columns      []Column            `json:"columns"`
	Rows         []map[string]string `json:"rows"`
	Metrics      []string            `json:"metrics"`
	Instructions []string            `json:"instructions"`
	DecisionHint string              `json:"decisionHint"`
	Enterprise   EnterpriseBlueprint `json:"enterprise"`
}
type Calculation struct {
	Rows        []map[string]any `json:"rows"`
	Metrics     [2]float64       `json:"metrics"`
	Decision    string           `json:"decision"`
	Explanation string           `json:"explanation"`
}

func col(key, label, kind, unit string) Column { return Column{key, label, kind, unit} }
func sample(columns []Column, values ...[]string) []map[string]string {
	rows := []map[string]string{}
	for _, v := range values {
		r := map[string]string{}
		for i, c := range columns {
			r[c.Key] = v[i]
		}
		rows = append(rows, r)
	}
	return rows
}
func caseSpec(id string) CaseSpec {
	c := CaseSpec{Module: id, Version: "2026.10-v1", Instructions: []string{"阅读任务，确认目标与业务约束。", "下载示例或空白模板，核对字段后导入，或使用模拟数据。", "运行计算，比较各候选结果并核对单位。", "填写结论，勾选人工复核项后提交。"}}
	switch id {
	case "M01":
		c.Title = "定期维修需求与采购缺口"
		c.Goal = "结合计划需求与交期内消耗，扣除可用库存和在途，编制预防性采购方案。所有数据为模拟。"
		c.Columns = []Column{col("id", "件号", "text", ""), col("planned", "计划需求", "number", "件"), col("daily", "日均非计划需求", "number", "件/天"), col("lead", "采购交期", "number", "天"), col("stock", "可用库存", "number", "件"), col("inbound", "到期前可用在途", "number", "件")}
		c.Rows = sample(c.Columns, []string{"PN-1001", "20", "0.5", "30", "18", "6"}, []string{"PN-1002", "8", "0.2", "45", "10", "0"}, []string{"PN-1003", "12", "0.1", "20", "16", "0"})
		c.Metrics = []string{"总采购缺口（件）", "最长采购交期（天）"}
		c.DecisionHint = "填写应最早启动采购的件号"
	case "M02":
		c.Title = "三家航材报价比选"
		c.Goal = "同一件号、相同件况、数量 2 件，需求期限 10 天。先检查证书和交期，再比较人民币总成本；报价为模拟。"
		c.Columns = []Column{col("id", "供应商", "text", ""), col("price", "单价", "number", "原币/件"), col("qty", "数量", "number", "件"), col("fx", "折人民币汇率", "number", "CNY/原币"), col("freight", "总运费", "number", "CNY"), col("lead", "交期", "number", "天"), col("certificate", "证书齐全", "bool", "1=是,0=否")}
		c.Rows = sample(c.Columns, []string{"SUP-A", "900", "2", "7.2", "500", "5", "0"}, []string{"SUP-B", "1100", "2", "7.2", "300", "8", "1"}, []string{"SUP-C", "1250", "2", "7.2", "200", "3", "1"})
		c.Metrics = []string{"推荐方案总成本（CNY）", "推荐方案交期（天）"}
		c.DecisionHint = "填写通过证书与交期检查且总成本最低的供应商编号"
	case "M03":
		c.Title = "供应商集中风险评估"
		c.Goal = "评价准时率、质量和采购集中度。教学评分 = 准时率×0.4 + (100−缺陷率)×0.35 + (100−份额)×0.25；证书到期不足30天或份额>60%标记风险。"
		c.Columns = []Column{col("id", "供应商", "text", ""), col("ontime", "准时率", "percent", "%"), col("defect", "缺陷率", "percent", "%"), col("share", "采购份额", "percent", "%"), col("certdays", "资质剩余有效期", "number", "天")}
		c.Rows = sample(c.Columns, []string{"SUP-A", "98", "1", "75", "90"}, []string{"SUP-B", "92", "2", "20", "180"}, []string{"SUP-C", "85", "5", "5", "15"})
		c.Metrics = []string{"最高综合评分（分）", "存在风险的供应商数（家）"}
		c.DecisionHint = "填写证书有效期≥30天且综合评分最高的供应商"
	case "M04":
		c.Title = "订单延期与维修影响"
		c.Goal = "日期均为相对今天的天数。比较供应商最新确认到货日与维修需求日，识别延期，优先处理缺口天数最大的订单。"
		c.Columns = []Column{col("id", "采购订单", "text", ""), col("promised", "原承诺到货", "number", "第N天"), col("confirmed", "最新确认到货", "number", "第N天"), col("required", "维修需求", "number", "第N天"), col("qty", "数量", "number", "件")}
		c.Rows = sample(c.Columns, []string{"PO-101", "3", "8", "6", "2"}, []string{"PO-102", "5", "12", "7", "1"}, []string{"PO-103", "4", "4", "8", "5"})
		c.Metrics = []string{"存在维修延期风险的订单数（单）", "最大缺口（天）"}
		c.DecisionHint = "填写应优先催交的采购订单号"
	case "M05":
		c.Title = "再订货点与安全库存"
		c.Goal = "假定日需求相互独立且交期固定，服务系数 z=1.65。安全库存=z×日需求标准差×√交期；再订货点向上取整。年持有率20%。本模型用于教学，不直接用于间歇性关键件决策。"
		c.Columns = []Column{col("id", "件号", "text", ""), col("daily", "日均需求", "number", "件/天"), col("std", "日需求标准差", "number", "件"), col("lead", "交期", "number", "天"), col("price", "单位库存价值", "number", "CNY/件")}
		c.Rows = sample(c.Columns, []string{"PN-2001", "1.5", "0.8", "30", "500"}, []string{"PN-2002", "0.4", "0.5", "45", "2000"}, []string{"PN-2003", "2", "1.2", "15", "300"})
		c.Metrics = []string{"各件号再订货点之和（件）", "安全库存年持有成本（CNY，未取整安全库存）"}
		c.DecisionHint = "填写服务水平策略 service；分析中说明本模型的适用假设"
	case "M06":
		c.Title = "AOG 紧急保障方案"
		c.Goal = "必须12小时内到达。证书齐全、适用性通过后，按运输+清关总时长最短优先，同等时长比较总成本。模拟方案覆盖购买、借件、交换、调拨。"
		c.Columns = []Column{col("id", "方案编号", "text", ""), col("eta", "运输时间", "number", "小时"), col("customs", "清关缓冲", "number", "小时"), col("cost", "方案成本", "number", "CNY"), col("freight", "运输费用", "number", "CNY"), col("certificate", "证书齐全", "bool", "1=是,0=否"), col("compatible", "适用性通过", "bool", "1=是,0=否")}
		c.Rows = sample(c.Columns, []string{"BUY", "6", "4", "42000", "3000", "1", "1"}, []string{"BORROW", "4", "1", "15000", "2000", "1", "1"}, []string{"EXCHANGE", "3", "1", "18000", "2500", "0", "1"}, []string{"TRANSFER", "5", "0", "12000", "1000", "1", "1"})
		c.Metrics = []string{"推荐方案总成本（CNY）", "推荐方案总到达时间（小时）"}
		c.DecisionHint = "填写按上述优先级选择的方案编号"
	}
	c.Enterprise = enterpriseBlueprint(id)
	return c
}

func validateRows(spec CaseSpec, rows []map[string]string) []string {
	errs := []string{}
	if len(rows) == 0 || len(rows) > 5000 {
		return []string{"需要 1–5000 行数据"}
	}
	ids := map[string]bool{}
	for i, r := range rows {
		for _, c := range spec.Columns {
			v := strings.TrimSpace(r[c.Key])
			if v == "" {
				errs = append(errs, fmt.Sprintf("第%d行 / %s：必填", i+2, c.Label))
				continue
			}
			if len(v) > 160 {
				errs = append(errs, fmt.Sprintf("第%d行 / %s：内容过长", i+2, c.Label))
				continue
			}
			if c.Kind != "text" {
				n, err := strconv.ParseFloat(v, 64)
				if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 1e9 {
					errs = append(errs, fmt.Sprintf("第%d行 / %s：应为0–1e9的数字", i+2, c.Label))
				} else if c.Kind == "bool" && n != 0 && n != 1 {
					errs = append(errs, fmt.Sprintf("第%d行 / %s：只能填0或1", i+2, c.Label))
				} else if c.Kind == "percent" && n > 100 {
					errs = append(errs, fmt.Sprintf("第%d行 / %s：百分比应在0–100", i+2, c.Label))
				} else if (c.Key == "fx" || c.Key == "qty" || c.Key == "lead") && n == 0 {
					errs = append(errs, fmt.Sprintf("第%d行 / %s：必须大于0", i+2, c.Label))
				}
			}
		}
		if ids[r["id"]] {
			errs = append(errs, fmt.Sprintf("第%d行 / 编号重复", i+2))
		}
		ids[r["id"]] = true
		if len(errs) >= 30 {
			break
		}
	}
	return errs
}
func num(r map[string]string, k string) float64 { n, _ := strconv.ParseFloat(r[k], 64); return n }
func round(n float64) float64                   { return math.Round(n*100) / 100 }
func calculate(id string, rows []map[string]string) Calculation {
	return calculateConfigured(id, rows, nil)
}
func calculateConfigured(id string, rows []map[string]string, parameters map[string]float64) Calculation {
	p := defaultParameters(id)
	for k, v := range parameters {
		p[k] = v
	}
	out := Calculation{Rows: []map[string]any{}, Metrics: [2]float64{0, 0}}
	best := math.Inf(1)
	high := -1.0
	for _, r := range rows {
		v := map[string]any{"id": r["id"]}
		switch id {
		case "M01":
			gap := math.Max(0, math.Ceil((num(r, "planned")+num(r, "daily")*num(r, "lead"))*p["demandFactor"]-num(r, "stock")-num(r, "inbound")))
			v["采购缺口"] = gap
			v["交期天数"] = num(r, "lead")
			out.Metrics[0] += gap
			out.Metrics[1] = math.Max(out.Metrics[1], num(r, "lead"))
			if gap > 0 && num(r, "lead") > high {
				high = num(r, "lead")
				out.Decision = r["id"]
			}
		case "M02":
			cost := num(r, "price")*num(r, "qty")*num(r, "fx") + num(r, "freight")
			valid := num(r, "certificate") == 1 && num(r, "lead") <= p["deadlineDays"]
			v["总成本CNY"] = round(cost)
			v["交期天数"] = num(r, "lead")
			v["满足约束"] = valid
			if valid && cost < best {
				best = cost
				out.Decision = r["id"]
				out.Metrics = [2]float64{round(cost), num(r, "lead")}
			}
		case "M03":
			score := (num(r, "ontime")*p["onTimeWeight"] + (100-num(r, "defect"))*p["qualityWeight"] + (100-num(r, "share"))*p["shareWeight"]) / 100
			risk := num(r, "certdays") < p["certificateDays"] || num(r, "share") > 60
			v["综合评分"] = round(score)
			v["风险标记"] = risk
			if risk {
				out.Metrics[1]++
			}
			if num(r, "certdays") >= p["certificateDays"] && score > high {
				high = score
				out.Decision = r["id"]
				out.Metrics[0] = round(score)
			}
		case "M04":
			delay := math.Max(0, num(r, "confirmed")-num(r, "required")+p["bufferDays"])
			v["维修缺口天数"] = delay
			v["相对原承诺变化"] = num(r, "confirmed") - num(r, "promised")
			if delay > 0 {
				out.Metrics[0]++
			}
			if delay > high {
				high = delay
				out.Metrics[1] = delay
				out.Decision = r["id"]
			}
		case "M05":
			ss := p["serviceZ"] * num(r, "std") * math.Sqrt(num(r, "lead"))
			rop := math.Ceil(num(r, "daily")*num(r, "lead") + ss)
			v["安全库存"] = round(ss)
			v["再订货点"] = rop
			v["年持有成本CNY"] = round(ss * num(r, "price") * p["holdingRate"] / 100)
			out.Metrics[0] += rop
			out.Metrics[1] += ss * num(r, "price") * p["holdingRate"] / 100
			out.Decision = "service"
		case "M06":
			eta := num(r, "eta") + num(r, "customs")
			cost := num(r, "cost") + num(r, "freight")
			valid := num(r, "certificate") == 1 && num(r, "compatible") == 1 && eta <= p["arrivalHours"]
			v["总到达小时"] = eta
			v["总成本CNY"] = cost
			v["满足约束"] = valid
			if valid && (eta < best || eta == best && cost < out.Metrics[0]) {
				best = eta
				out.Decision = r["id"]
				out.Metrics = [2]float64{cost, eta}
			}
		}
		out.Rows = append(out.Rows, v)
	}
	out.Metrics[0] = round(out.Metrics[0])
	out.Metrics[1] = round(out.Metrics[1])
	if out.Decision == "" {
		out.Decision = "NONE"
	}
	out.Explanation = "计算依据为本案例明确的教学规则；请逐行核对输入、单位、业务约束与适用假设，并在分析中比较候选方案。"
	return out
}
