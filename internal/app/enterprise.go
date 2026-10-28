package app

import (
	"encoding/json"
	"net/http"
)

type ReferenceSoftware struct {
	Name          string `json:"name"`
	Module        string `json:"module"`
	Documentation string `json:"documentation"`
	Channel       string `json:"channel"`
	Access        string `json:"access"`
}
type EnterpriseStep struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Action   string            `json:"action"`
	Software ReferenceSoftware `json:"software"`
	Fields   []string          `json:"fields"`
}
type SourceDefinition struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Object string   `json:"object"`
	Key    string   `json:"key"`
	Keys   []string `json:"keys"`
}
type FieldBinding struct {
	Target string `json:"target"`
	Source string `json:"source"`
	Field  string `json:"field"`
}
type ParameterDefinition struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Default float64 `json:"default"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Unit    string  `json:"unit"`
}
type EnterpriseBlueprint struct {
	Version    string                `json:"version"`
	Boundary   string                `json:"boundary"`
	Sources    []SourceDefinition    `json:"sources"`
	Bindings   []FieldBinding        `json:"bindings"`
	Steps      []EnterpriseStep      `json:"steps"`
	Parameters []ParameterDefinition `json:"parameters"`
}

var sourceAliases = map[string]string{"id": "business_key", "planned": "planned_requirement_qty", "daily": "average_daily_demand", "lead": "procurement_lead_days", "stock": "serviceable_available_qty", "inbound": "inbound_before_need_qty", "price": "quoted_unit_price", "qty": "required_quantity", "fx": "fx_to_cny", "freight": "freight_total_cny", "certificate": "certificate_verified", "ontime": "on_time_delivery_pct", "defect": "defect_pct", "share": "spend_share_pct", "certdays": "qualification_remaining_days", "promised": "original_arrival_day", "confirmed": "latest_arrival_day", "required": "maintenance_need_day", "std": "daily_demand_std", "eta": "transport_hours", "customs": "customs_buffer_hours", "cost": "option_cost_cny", "compatible": "applicability_verified"}

func referenceSoftware(id string) ReferenceSoftware {
	switch id {
	case "M02":
		return ReferenceSoftware{"SAP Ariba", "Sourcing / RFQ与报价评审", "https://help.sap.com/docs/ARIBA_SOURCING", "https://www.sap.com/products/spend-management/direct-procurement-software/trial.html", "官方体验渠道；具体Sourcing功能及账号许可需核实"}
	case "M03":
		return ReferenceSoftware{"SAP Ariba", "Supplier Lifecycle and Performance / Supplier Risk", "https://www.sap.com/products/spend-management/supplier-management.html", "https://www.sap.com/products/spend-management/direct-procurement-software/trial.html", "官方体验渠道；供应商风险模块并非已确认包含在免费体验内"}
	case "M06":
		return ReferenceSoftware{"IFS Aviation Maintenance", "Supply Chain / Maintenance Material Requisitions", "https://docs.ifs.com/documents/Supply_Chain_ICAM_fixed.pdf", "https://www.ifs.com/en/book-a-demo", "预约演示；未确认可自助开通的免费沙箱"}
	case "M05":
		return ReferenceSoftware{"IBM Maximo Manage", "Inventory / Reorder Items", "https://www.ibm.com/docs/en/masv-and-l/maximo-manage/cd?topic=replenishment-reordering-inventory-items", "https://www.ibm.com/account/reg/us-en/signup?formid=urx-52271", "官方试用渠道；航空扩展与库存权限需核实"}
	default:
		return ReferenceSoftware{"IBM Maximo for Aviation", "Purchasing / Purchase Requisitions、Purchase Orders、Shipments", "https://www.ibm.com/docs/en/maximo-for-aviation/7.6.8?topic=applications-purchasing-module", "https://www.ibm.com/account/reg/us-en/signup?formid=urx-52271", "官方试用渠道；具体产品版本及航空功能需核实"}
	}
}
func enterpriseBlueprint(id string) EnterpriseBlueprint {
	b := EnterpriseBlueprint{Version: "enterprise-v1", Boundary: "按通用MRO／ERP／WMS／SRM／QMS分层设计。厦门太古实际系统名称、私有表名、API及审批规则待确认。软件对应为教学映射，导出字段为示例别名，不代表原厂数据库字段；不宣称已连接企业生产系统。"}
	switch id {
	case "M01":
		b.Sources = []SourceDefinition{{"planning", "MRO维修计划", "维修工作包需求汇总", "business_key", []string{"planned"}}, {"inventory", "WMS库存与在途", "件号可用量及需求日前在途", "business_key", []string{"stock", "inbound"}}, {"supply", "ERP消耗及交期", "需求历史与供应周期汇总", "business_key", []string{"daily", "lead"}}}
		b.Parameters = []ParameterDefinition{{"demandFactor", "需求情景倍率", 1, .1, 5, "倍"}}
	case "M02":
		b.Sources = []SourceDefinition{{"quote", "SRM询报价", "同一需求的供应商报价明细", "business_key", []string{"price", "qty", "lead"}}, {"finance", "ERP财务与物流", "报价币种汇率和运输成本", "business_key", []string{"fx", "freight"}}, {"quality", "QMS文件核验", "候选报价证书核对结果", "business_key", []string{"certificate"}}}
		b.Parameters = []ParameterDefinition{{"deadlineDays", "最迟交付期限", 10, 1, 365, "天"}}
	case "M03":
		b.Sources = []SourceDefinition{{"performance", "SRM履约绩效", "供应商交期和质量汇总", "business_key", []string{"ontime", "defect"}}, {"spend", "ERP采购支出", "同一统计窗口采购份额", "business_key", []string{"share"}}, {"quality", "QMS准入资质", "资质到期与文件复核", "business_key", []string{"certdays"}}}
		b.Parameters = []ParameterDefinition{{"onTimeWeight", "准时率权重", 40, 0, 100, "%"}, {"qualityWeight", "质量权重", 35, 0, 100, "%"}, {"shareWeight", "集中度权重", 25, 0, 100, "%"}, {"certificateDays", "资质有效期下限", 30, 1, 365, "天"}}
	case "M04":
		b.Sources = []SourceDefinition{{"orders", "ERP采购订单", "采购订单行及原承诺", "business_key", []string{"promised", "qty"}}, {"logistics", "物流协同", "最新确认到货日期", "business_key", []string{"confirmed"}}, {"planning", "MRO维修计划", "订单关联的维修需求时点", "business_key", []string{"required"}}}
		b.Parameters = []ParameterDefinition{{"bufferDays", "要求提前到货缓冲", 0, 0, 30, "天"}}
	case "M05":
		b.Sources = []SourceDefinition{{"usage", "MRO领退料", "同件号同库位消耗统计", "business_key", []string{"daily", "std"}}, {"supply", "ERP供应履约", "件号实际采购交期汇总", "business_key", []string{"lead"}}, {"finance", "ERP库存价值", "同口径单位库存价值", "business_key", []string{"price"}}}
		b.Parameters = []ParameterDefinition{{"serviceZ", "服务水平系数z", 1.65, .1, 4, "系数"}, {"holdingRate", "年持有成本率", 20, 0, 100, "%"}}
	case "M06":
		b.Sources = []SourceDefinition{{"options", "MRO紧急保障", "采购／借件／交换／调拨候选方案", "business_key", []string{"eta", "cost"}}, {"logistics", "物流及通关", "运输报价与清关时长", "business_key", []string{"customs", "freight"}}, {"quality", "QMS工程复核", "证书与适用性核验结果", "business_key", []string{"certificate", "compatible"}}}
		b.Parameters = []ParameterDefinition{{"arrivalHours", "最迟到达时限", 12, 1, 168, "小时"}}
	}
	if len(b.Sources) == 0 {
		return b
	}
	b.Bindings = append(b.Bindings, FieldBinding{"id", b.Sources[0].ID, "business_key"})
	for _, s := range b.Sources {
		for _, key := range s.Keys {
			b.Bindings = append(b.Bindings, FieldBinding{key, s.ID, sourceAliases[key]})
		}
	}
	ref := referenceSoftware(id)
	for i, pair := range [][2]string{{"inspect", "识别业务需求与系统边界"}, {"data", "采集、关联与校验企业数据"}, {"calculate", "配置场景并运行分析"}, {"review", "业务复核与形成决策"}, {"submit", "提交并保留数据快照"}} {
		action := []string{"确认需求对象、业务主键、统计窗口和数据负责人。", "分别获取业务、支撑、质量数据；按业务主键关联，核对单位和缺失。", "选择业务参数，比较场景结果；新增字段必须说明业务用途。", "检查计算依据、资质、交期和例外项，填写本人的判断。", "保存采集来源、映射、字段和参数快照；软件利弊与企业适配在历史复盘中开放。"}[i]
		fields := []string{}
		for _, binding := range b.Bindings {
			fields = append(fields, binding.Target)
		}
		b.Steps = append(b.Steps, EnterpriseStep{pair[0], pair[1], action, ref, fields})
	}
	return b
}
func defaultParameters(id string) map[string]float64 {
	p := map[string]float64{}
	for _, d := range enterpriseBlueprint(id).Parameters {
		p[d.Key] = d.Default
	}
	return p
}
func parameter(id, key string, values map[string]float64) float64 {
	if n, ok := values[key]; ok {
		return n
	}
	return defaultParameters(id)[key]
}
func stepFor(b EnterpriseBlueprint, step string) EnterpriseStep {
	for _, s := range b.Steps {
		if s.ID == step {
			return s
		}
	}
	return EnterpriseStep{}
}
func (s *Server) historyAPI(w http.ResponseWriter, r *http.Request, ss session, p []string) {
	if ss.Role != "learner" {
		fail(w, 403, "仅本人可查看学员历史练习")
		return
	}
	if r.Method != "GET" {
		fail(w, 405, "历史记录仅支持查看")
		return
	}
	if a, err := s.examAttempt(ss.ID); err == nil && a.Status == "draft" && nowUTC().Before(a.Deadline) {
		fail(w, 403, "考试期间暂停查看练习历史")
		return
	}
	if len(p) == 2 {
		module := r.URL.Query().Get("module")
		if module != "" {
			valid := false
			for _, m := range modules {
				if m.ID == module {
					valid = true
					break
				}
			}
			if !valid {
				fail(w, 422, "业务模块不存在")
				return
			}
		}
		query := "SELECT payload FROM training_attempts WHERE learner=?"
		args := []any{ss.ID}
		if module != "" {
			query += " AND module=?"
			args = append(args, module)
		}
		query += " ORDER BY module,number DESC"
		rows, err := s.db.Query(query, args...)
		if err != nil {
			fail(w, 500, "读取历史练习失败")
			return
		}
		items := []map[string]any{}
		for rows.Next() {
			var raw []byte
			if rows.Scan(&raw) != nil {
				continue
			}
			var a TrainingAttempt
			if json.Unmarshal(raw, &a) != nil {
				continue
			}
			items = append(items, map[string]any{"id": a.ID, "module": a.Module, "number": a.Number, "status": a.Status, "startedAt": a.StartedAt, "submittedAt": a.SubmittedAt, "rows": len(a.Rows), "customFields": len(a.CustomColumns)})
		}
		rows.Close()
		respond(w, 200, map[string]any{"items": items})
		return
	}
	if len(p) < 3 || len(p) > 4 {
		fail(w, 404, "历史记录不存在")
		return
	}
	a, err := s.getAttempt(p[2])
	if err != nil || a.Learner != ss.ID {
		fail(w, 404, "历史记录不存在")
		return
	}
	if len(p) == 4 && p[3] == "review-status" {
		state, err := s.reviewButtonState(a)
		if err != nil {
			fail(w, 500, "读取回复状态失败")
			return
		}
		respond(w, 200, state)
		return
	}
	if len(p) == 4 {
		if p[3] != "review" {
			fail(w, 404, "历史功能不存在")
			return
		}
		if a.Status != "submitted" {
			fail(w, 403, "提交后从历史记录进入，才能查看软件利弊及企业适配复盘")
			return
		}
		fail(w, 404, "请使用企业情况访谈复盘入口")
		return
	}
	b := enterpriseBlueprint(a.Module)
	if a.Blueprint != nil {
		b = *a.Blueprint
	}
	state, err := s.reviewButtonState(a)
	if err != nil {
		fail(w, 500, "读取回复状态失败")
		return
	}
	respond(w, 200, map[string]any{"attempt": learnerAttempt(a), "case": caseSpec(a.Module), "blueprint": b, "reviewState": state})
}
