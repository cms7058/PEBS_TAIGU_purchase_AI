"use strict";
window.EnterpriseUI = (() => {
  const dialog = document.querySelector("#enterprise-dialog");
  let record = null,
    historyPage = 0,
    historyModule = "";
  const state = () => window.Features.trainingState;
  const say = (text, error = false) => {
    const node = document.querySelector("#feature-message");
    if (node) {
      node.textContent = text;
      node.classList.toggle("error-text", error);
    }
  };
  const date = (value) =>
    value
      ? new Date(value).toLocaleString("zh-CN", { timeZone: "Asia/Shanghai" })
      : "—";
  const localTime = (value) => {
    const d = new Date(value || Date.now());
    return new Date(d.getTime() - d.getTimezoneOffset() * 60000)
      .toISOString()
      .slice(0, 16);
  };
  function stepCard(spec, attempt, step) {
    const b = attempt.blueprint || spec.enterprise,
      s = b?.steps.find((s) => s.id === step);
    if (!s) return "";
    return `<section class="step-reference"><div><span class="eyebrow">流程与软件对应</span><h3>${e(s.title)}</h3><p>${e(s.action)}</p></div><div class="software-reference"><strong>${e(s.software.name)}</strong><span>${e(s.software.module)}</span><div><a href="${e(s.software.documentation)}" target="_blank" rel="noopener noreferrer">官方模块说明 ↗</a><a href="${e(s.software.channel)}" target="_blank" rel="noopener noreferrer">试用／演示渠道 ↗</a></div><small>${e(s.software.access)}</small></div><details><summary>系统边界及本步骤字段</summary><p class="muted">${e(b.boundary)}</p><p class="muted">标准输入：${(s.fields || []).map(e).join("、")}。自定义字段作为补充采集，不自动改变核心公式；如需影响计算，映射到标准输入或调整受支持参数。</p></details></section>`;
  }
  function dataPanel(spec, a) {
    const b = a.blueprint || spec.enterprise,
      p = a.acquisition || {},
      sources = p.sources || [],
      cols = [...spec.columns, ...(a.customColumns || [])],
      bindings = p.bindings || b.bindings;
    return `<section class="acquisition-workspace">${(p.warnings || []).map((x) => `<p class="notice">${e(x)}</p>`).join("")}<div class="toolbar"><div><h2>企业数据采集与字段配置</h2><p class="muted">分源导出 → 记录来源 → 主键关联 → 数据校验。支持CSV、扁平JSON及手工采集。</p></div><div class="row"><button class="primary" data-enterprise-action="sample">加载24条分源仿真数据</button><button data-enterprise-action="manual">人工填报</button><button data-enterprise-action="basic">使用基础样例</button></div></div><p class="notice">企业实际接口尚未提供。这里采集的是用户上传的导出快照或人工记录；不声称已接入厦门太古内网。</p><div class="source-node-grid">${b.sources
      .map((def) => {
        const s = sources.find((x) => x.id === def.id);
        return `<section class="source-node"><span class="eyebrow">${e(def.id)}</span><h3>${e(def.name)}</h3><p>${e(def.object)}</p><small>示例主键：${e(def.key)} · 需按同一业务粒度先汇总</small><p class="source-status">${s ? `${s.rows.length} 行已采集 · ${e(s.system)}` : "尚未采集"}</p><div class="row"><button data-enterprise-source="${e(def.id)}">${s ? "替换来源快照" : "采集CSV／JSON"}</button><button data-enterprise-template="${e(def.id)}">下载来源示例</button></div></section>`;
      })
      .join(
        "",
      )}</div><details class="enterprise-details" ${p.mode === "mapped" ? "open" : ""}><summary>来源字段关联与归一化</summary><p class="muted">标准字段映射到来源原始字段，按来源各自的业务主键关联。重复或缺失主键会报错，不静默合并。生成关联表将替换下方业务表。</p><div class="table-scroll"><table><thead><tr><th>目标业务字段</th><th>采集来源字段</th><th>类型与用途</th></tr></thead><tbody>${cols
      .map((c) => {
        const binding = bindings.find((x) => x.target === c.key),
          value = binding ? binding.source + "::" + binding.field : "";
        return `<tr><th>${e(c.label)}<small>${e(c.key)}</small></th><td><select data-enterprise-binding="${e(c.key)}"><option value="">${spec.columns.some((x) => x.key === c.key) ? "请先采集来源后选择" : "不关联，留待人工填报"}</option>${sources.map((s) => s.fields.map((f) => `<option value="${e(s.id + "::" + f)}" ${value === s.id + "::" + f ? "selected" : ""}>${e(s.system)} / ${e(f)}</option>`).join("")).join("")}</select></td><td>${e(c.kind)}${c.unit ? " · " + e(c.unit) : ""}<small>${e(c.purpose || "标准模型输入")}</small></td></tr>`;
      })
      .join(
        "",
      )}</tbody></table></div><button class="primary" data-enterprise-action="collect" ${sources.length ? "" : "disabled"}>按映射关联并生成业务表</button><p class="muted">关联状态：${p.mode === "mapped" ? (p.assembled ? "已生成，可继续核对" : "来源或字段已变化，等待重新关联") : "当前使用人工表或基础样例"}</p></details><details class="enterprise-details"><summary>人工采集来源与统计窗口</summary><form id="enterprise-provenance" class="form-stack"><div class="form-grid"><label>数据模式<select name="mode"><option value="manual" ${p.mode === "manual" ? "selected" : ""}>人工采集／导出后手工整理</option><option value="sample" ${p.mode === "sample" || !p.mode ? "selected" : ""}>课堂基础样例</option><option value="mapped" ${p.mode === "mapped" ? "selected" : ""}>分来源关联</option></select></label><label>实际来源系统／部门<input name="system" value="${e(p.system || "")}" maxlength="160" placeholder="填写真实名称，未知时标注待确认"></label><label>采集人<input name="collector" value="${e(p.collector || user.id)}" maxlength="80" required></label><label>数据截至时间<input name="asOf" type="datetime-local" value="${localTime(p.asOf)}" required></label></div><label>口径说明<textarea name="note" rows="2" maxlength="500" placeholder="统计窗口、物权、库位、单位、需求日期或导出版本">${e(p.note || "")}</textarea></label><button type="submit">保存来源信息</button></form></details><details class="enterprise-details"><summary>字段字典与自定义字段（${(a.customColumns || []).length} / 20）</summary><div class="row"><button data-enterprise-action="add-field">增加自定义字段</button></div><div class="table-scroll"><table><thead><tr><th>字段</th><th>类型／单位</th><th>采集与业务用途</th><th>操作</th></tr></thead><tbody>${cols
      .map((c) => {
        const custom = (a.customColumns || []).some((x) => x.key === c.key);
        const binding = b.bindings.find((x) => x.target === c.key);
        return `<tr><th>${e(c.label)}<small>${e(c.key)}</small></th><td>${e(c.kind)} · ${e(c.unit || "无单位")}<small>${custom ? (c.required ? "自定义必填" : "自定义可选") : "标准必填"}</small></td><td>${custom ? e(c.purpose) : e((b.sources.find((x) => x.id === binding?.source)?.name || "标准输入") + " / 示例导出字段 " + (binding?.field || c.key))}</td><td>${custom ? `<button data-enterprise-edit-field="${e(c.key)}">编辑</button><button data-enterprise-delete-field="${e(c.key)}">移除</button>` : "固定核心字段"}</td></tr>`;
      })
      .join(
        "",
      )}</tbody></table></div><p class="muted">新增字段仅属于本人的本模块练习，新一轮沿用字段定义；不会修改其他学员数据。自定义补充字段不自动参与核心公式。</p></details></section>`;
  }
  function parameterPanel(spec, a) {
    const b = a.blueprint || spec.enterprise;
    return `<form id="enterprise-parameters" class="panel form-stack"><h3>业务场景参数</h3><p class="muted">修改参数后须重新运行分析；评分按保存的参数计算，不使用固定标准答案。</p><div class="form-grid">${b.parameters.map((p) => `<label>${e(p.label)}（${e(p.unit)}）<input type="number" name="${p.key}" value="${a.parameters?.[p.key] ?? p.default}" min="${p.min}" max="${p.max}" step="any" required></label>`).join("")}</div><button type="submit">保存场景参数并清除旧结果</button></form>`;
  }
  function open(content) {
    dialog.innerHTML = content;
    dialog.showModal();
  }
  function close() {
    dialog.close();
    dialog.replaceChildren();
  }
  const footer = (label = "保存") =>
    `<p id="enterprise-error" role="alert"></p><footer class="dialog-footer"><button type="button" data-enterprise-action="close">取消</button><button class="primary" type="submit">${label}</button></footer>`;
  function fieldForm(key = "") {
    const { attempt: a } = state(),
      c = (a.customColumns || []).find((c) => c.key === key) || {};
    open(
      `<form id="enterprise-field" data-old-key="${e(key)}" class="form-stack"><h2>${key ? "编辑" : "增加"}自定义字段</h2><label>字段键（小写字母开头）<input name="key" value="${e(c.key || "")}" ${key ? "readonly" : ""} minlength="2" maxlength="32" placeholder="例如 work_package" required></label><label>中文名称<input name="label" value="${e(c.label || "")}" maxlength="40" required></label><div class="form-grid"><label>类型<select name="kind">${[
        ["text", "文本"],
        ["number", "非负数字"],
        ["percent", "百分比"],
        ["bool", "布尔0/1"],
        ["date", "日期YYYY-MM-DD"],
      ]
        .map(
          ([v, l]) =>
            `<option value="${v}" ${c.kind === v ? "selected" : ""}>${l}</option>`,
        )
        .join(
          "",
        )}</select></label><label>单位<input name="unit" value="${e(c.unit || "")}" maxlength="60"></label></div><label>字段用途<textarea name="purpose" rows="3" maxlength="200" required placeholder="说明从哪里采集、用于什么业务判断；补充字段不会自动改变模型">${e(c.purpose || "")}</textarea></label><label><input name="required" type="checkbox" ${c.required ? "checked" : ""}> 此字段必填</label>${footer()}</form>`,
    );
  }
  function sourceForm(id) {
    const { spec, attempt: a } = state(),
      b = a.blueprint || spec.enterprise,
      def = b.sources.find((s) => s.id === id),
      s = a.acquisition?.sources.find((s) => s.id === id);
    open(
      `<form id="enterprise-source" data-source="${e(id)}" class="form-stack"><h2>采集 ${e(def.name)}</h2><p class="muted">来自企业导出或API返回的扁平CSV／JSON快照。所有字段均可在关联时重新映射。</p><label>实际系统名称<input name="system" value="${e(s?.system || def.name)}" maxlength="160" required></label><label>业务对象／导出表名<input name="object" value="${e(s?.object || def.object)}" maxlength="200" required></label><div class="form-grid"><label>来源业务主键字段<input name="keyField" value="${e(s?.keyField || def.key)}" required maxlength="100"></label><label>采集人<input name="collector" value="${e(s?.collector || user.id)}" required maxlength="80"></label></div><label>数据截至时间<input type="datetime-local" name="asOf" value="${localTime(s?.asOf)}" required></label><label>快照文件<input type="file" name="file" accept=".csv,.json,text/csv,application/json"></label><label>或粘贴快照内容<textarea name="content" rows="5" placeholder="粘贴CSV表头与数据，或JSON对象数组"></textarea></label><label>粘贴内容格式<select name="format"><option value="csv">CSV</option><option value="json">JSON</option></select></label>${footer("校验并保存来源快照")}</form>`,
    );
  }
  async function downloadSource(id) {
    const { spec } = state();
    const res = await fetch(
      portalURL(
        "/api/training/" +
          spec.module +
          "/source-template?source=" +
          encodeURIComponent(id) +
          "&example=1",
      ),
    );
    if (!res.ok) throw new Error("来源模板下载失败");
    const url = URL.createObjectURL(await res.blob()),
      link = document.createElement("a");
    link.href = url;
    link.download = spec.module + "-" + id + "-source.csv";
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  async function history(module = "") {
    historyModule = module;
    record = null;
    historyPage = 0;
    const data = await api(
      "/api/history" + (module ? "?module=" + encodeURIComponent(module) : ""),
    );
    if (historyModule !== module || window.Features.active !== "history")
      return;
    const moduleName = module
      ? user.catalog.find((m) => m.id === module)?.title || module
      : "";
    window.Features.shell(
      module ? "历史练习 · " + moduleName : "历史练习",
      `<section class="panel"><div class="toolbar"><div><h2>${module ? e(module + " · " + moduleName) + "的历史练习" : "我的历史练习"}</h2>${module ? '<p class="notice">仅显示当前业务模块的操作记录。</p>' : ""}<p class="muted">按模块保留每次练习。只有已提交记录可开启软件利弊与厦门太古适配复盘；不展示管理员评分。</p></div><button data-view="history" data-history-module="${e(historyModule)}">刷新记录</button></div>${data.items.length ? `<div class="table-scroll"><table><thead><tr><th>模块／次数</th><th>状态</th><th>数据规模</th><th>提交时间</th><th>操作</th></tr></thead><tbody>${data.items.map((a) => `<tr><th>${e(user.catalog.find((m) => m.id === a.module)?.title || a.module)}<small>${e(a.module)} · 第 ${a.number} 次</small></th><td>${a.status === "submitted" ? "已提交" : "未提交草稿"}</td><td>${a.rows} 行 · 自定义 ${a.customFields} 字段</td><td>${date(a.submittedAt)}</td><td><button data-history-open="${e(a.id)}">查看记录</button></td></tr>`).join("")}</tbody></table></div>` : '<p class="muted">暂无练习记录。进入已激活模块开始练习后，这里会保存草稿与提交历史。</p>'}</section>`,
    );
  }
  async function historyDetail(id) {
    record = await api("/api/history/" + encodeURIComponent(id));
    const arr = (x) => (Array.isArray(x) ? x : []);
    const a = record.attempt;
    if (!a || !record.case)
      throw new Error("历史记录格式不完整，请刷新后重试。");
    a.rows = arr(a.rows);
    a.steps = arr(a.steps);
    a.customColumns = arr(a.customColumns);
    a.answer = a.answer || {};
    a.answer.metrics = arr(a.answer.metrics);
    record.case.columns = arr(record.case.columns);
    record.blueprint = record.blueprint || {};
    record.blueprint.parameters = arr(record.blueprint.parameters);
    if (a.acquisition) {
      a.acquisition.bindings = arr(a.acquisition.bindings);
      a.acquisition.sources = arr(a.acquisition.sources);
      a.acquisition.sources.forEach((s) => {
        s.rows = arr(s.rows);
        s.fields = arr(s.fields);
      });
    }
    historyPage = 0;
    renderHistory();
  }
  function readTable(rows, cols) {
    return `<div class="table-scroll"><table><thead><tr>${cols.map((c) => `<th>${e(c.label || c.key)}</th>`).join("")}</tr></thead><tbody>${rows.map((r) => `<tr>${cols.map((c) => `<td>${e(r[c.key] ?? "")}</td>`).join("")}</tr>`).join("")}</tbody></table></div>`;
  }
  let reviewStatusRequest = 0;
  function reviewButton(state) {
    const a = record.attempt,
      s = state || {
        code: a.status === "submitted" ? "not-submitted" : "draft",
        label: a.status === "submitted" ? "待整理提交" : "练习尚未提交",
        action: "进入复盘",
      };
    const known = [
        "draft",
        "not-submitted",
        "waiting",
        "processing",
        "replied",
        "followup-waiting",
      ],
      code = known.includes(s.code) ? s.code : "not-submitted";
    const text =
      code === "replied"
        ? "查看回复 · 软件利弊与厦门太古适配复盘"
        : code === "followup-waiting" && s.replies > 0
          ? "查看回复与追问 · 软件利弊与厦门太古适配复盘"
          : "软件利弊与厦门太古适配复盘";
    return `<div class="history-review-control"><button data-history-review="${e(a.id)}" class="review-entry review-entry-${code}" title="${e(s.action)}" aria-describedby="history-review-status" ${a.status === "submitted" ? "" : "disabled"}>${code === "replied" ? '<span aria-hidden="true">✓ </span>' : ""}${text}</button><span id="history-review-status" class="review-status-tag review-status-${code}" role="status" aria-live="polite">${e(s.label)}${s.pending > 0 ? " · " + s.pending + "项待回复" : code === "replied" && s.replies > 0 ? " · " + s.replies + "条回复" : ""}</span></div>`;
  }
  function updateReviewButton(state) {
    if (!record || !state) return;
    reviewStatusRequest++;
    record.reviewState = state;
    const node = document.querySelector(".history-review-control");
    if (node) node.outerHTML = reviewButton(state);
  }
  async function refreshReviewStatus() {
    if (
      !record ||
      window.Features.active !== "history" ||
      user?.role !== "learner" ||
      user.examActive ||
      document.hidden
    )
      return;
    const id = record.attempt.id,
      seq = ++reviewStatusRequest;
    const data = await api(
      "/api/history/" + encodeURIComponent(id) + "/review-status",
    );
    if (
      seq === reviewStatusRequest &&
      record?.attempt.id === id &&
      window.Features.active === "history"
    )
      updateReviewButton(data);
  }
  setInterval(() => refreshReviewStatus().catch(() => {}), 5000);
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) refreshReviewStatus().catch(() => {});
  });
  function renderHistory() {
    const a = record.attempt,
      cols = [...record.case.columns, ...(a.customColumns || [])],
      from = historyPage * 30;
    window.Features.shell(
      "历史练习 · " + record.case.title,
      `<div class="row history-actions"><button data-view="history" data-history-module="${e(historyModule)}">返回历史列表</button><span class="tag">第 ${a.number} 次 · ${a.status === "submitted" ? "已提交只读快照" : "未提交只读草稿"}</span>${reviewButton(record.reviewState)}</div><p class="muted">开始：${date(a.startedAt)} · 提交：${date(a.submittedAt)}。历史数据只读，新的练习不会覆盖本次快照。</p><section class="panel"><h2>数据、字段与来源快照</h2><p class="muted">${a.rows.length} 行 · ${cols.length} 字段 · 来源模式 ${e(a.acquisition?.mode || "旧版基础样例")}。${a.status !== "submitted" ? "提交后才可开启适配复盘。" : ""}</p>${readTable(a.rows.slice(from, from + 30), cols)}<div class="row"><button data-history-page="previous" ${historyPage ? "" : "disabled"}>上一页</button><span>${from + 1}–${Math.min(from + 30, a.rows.length)} / ${a.rows.length}</span><button data-history-page="next" ${from + 30 < a.rows.length ? "" : "disabled"}>下一页</button></div><details class="enterprise-details"><summary>采集来源与字段映射</summary><p>${e(a.acquisition?.system || "未记录实际来源")} · ${e(a.acquisition?.collector || "—")} · ${date(a.acquisition?.asOf)}</p><p class="muted">${e(a.acquisition?.note || "")}</p>${(
        a.acquisition?.sources || []
      )
        .map(
          (s) =>
            `<section><h3>${e(s.system)} · ${e(s.object)}</h3><p class="muted">${s.rows.length} 行 · 业务主键 ${e(s.keyField)} · ${date(s.asOf)} · 采集人 ${e(s.collector)}</p>${readTable(
              s.rows.slice(0, 5),
              s.fields.map((key) => ({ key, label: key })),
            )}</section>`,
        )
        .join(
          "",
        )}<p class="muted">映射：${(a.acquisition?.bindings || []).map((b) => e(b.target + " ← " + b.source + "." + b.field)).join("；")}</p></details><details class="enterprise-details"><summary>场景参数与字段定义</summary><p>${(record.blueprint.parameters || []).map((p) => e(p.label) + "：" + (a.parameters?.[p.key] ?? p.default) + " " + e(p.unit)).join("；")}</p>${(a.customColumns || []).map((c) => `<p>${e(c.label)} / ${e(c.key)} / ${e(c.kind)}：${e(c.purpose)}</p>`).join("")}</details></section><section class="panel"><h2>过程与本人结论</h2><p class="muted">完成步骤：${a.steps.map(e).join(" → ") || "尚未完成任务步骤"} · 有效活动约 ${Math.round(a.activeSeconds)} 秒</p><p class="pre-wrap">${e(a.answer.analysis || "尚未填写分析")}</p><p>方案：${e(a.answer.decision || "—")}；指标：${a.answer.metrics.map(e).join(" / ")}</p></section><section id="history-review-panel"></section>`,
    );
  }
  async function historyReview(id) {
    await window.ReviewUI.open(id);
  }
  document.addEventListener("click", async (ev) => {
    const b = ev.target.closest("button");
    if (!b || b.disabled) return;
    try {
      const action = b.dataset.enterpriseAction;
      if (action === "close") close();
      if (action === "add-field") fieldForm();
      if (action === "add-row") window.Features.editRows();
      if (action === "manual" || action === "basic") {
        const { spec, attempt: a } = state();
        const rows =
          action === "manual"
            ? [
                Object.fromEntries(
                  [...spec.columns, ...(a.customColumns || [])].map((c) => [
                    c.key,
                    "",
                  ]),
                ),
              ]
            : spec.rows;
        await window.Features.enterpriseOperation("configure", {
          rows,
          dataSource: {
            mode: action === "manual" ? "manual" : "sample",
            system:
              action === "manual"
                ? "人工填报（请补充实际来源）"
                : "课堂基础样例",
            collector: user.id,
            asOf: new Date().toISOString(),
            note:
              action === "basic"
                ? "基础样例已选择"
                : "请填写统计口径及实际来源",
          },
        });
        say(
          action === "manual"
            ? "已切换为空白业务表，请填报实际数据与来源。"
            : "已明确选择基础样例；企业实训建议使用分源采集或人工数据。",
        );
      }
      if (action === "sample") {
        await window.Features.enterpriseOperation("enterprise-sample");
        say("已加载3份来源快照，每份24行。请检查字段映射，再生成业务表。");
      }
      if (action === "collect") {
        const bindings = [
          ...document.querySelectorAll("[data-enterprise-binding]"),
        ]
          .filter((x) => x.value)
          .map((x) => {
            const split = x.value.indexOf("::");
            return {
              target: x.dataset.enterpriseBinding,
              source: x.value.slice(0, split),
              field: x.value.slice(split + 2),
            };
          });
        await window.Features.enterpriseOperation("collect", { bindings });
        say("跨来源主键关联与数据校验已通过，请核对业务表。");
      }
      if (b.dataset.enterpriseSource) sourceForm(b.dataset.enterpriseSource);
      if (b.dataset.enterpriseTemplate)
        await downloadSource(b.dataset.enterpriseTemplate);
      if (b.dataset.enterpriseEditField)
        fieldForm(b.dataset.enterpriseEditField);
      if (b.dataset.enterpriseDeleteField) {
        const cols = (state().attempt.customColumns || []).filter(
          (c) => c.key !== b.dataset.enterpriseDeleteField,
        );
        await window.Features.enterpriseOperation("configure", {
          customColumns: cols,
        });
        say("字段已从本轮草稿移除，历史记录不变。");
      }
      if (b.dataset.enterpriseRemoveRow !== undefined)
        window.Features.editRows(Number(b.dataset.enterpriseRemoveRow));
      if (b.dataset.historyOpen) await historyDetail(b.dataset.historyOpen);
      if (b.dataset.historyReview) await historyReview(b.dataset.historyReview);
      if (b.dataset.historyPage) {
        historyPage += b.dataset.historyPage === "next" ? 1 : -1;
        renderHistory();
      }
    } catch (err) {
      say(err.message, true);
    }
  });
  document.addEventListener("submit", async (ev) => {
    const form = ev.target,
      id = form.getAttribute("id");
    if (
      ![
        "enterprise-field",
        "enterprise-source",
        "enterprise-provenance",
        "enterprise-parameters",
      ].includes(id)
    )
      return;
    ev.preventDefault();
    const button = form.querySelector("button[type=submit]");
    button.disabled = true;
    try {
      const f = new FormData(form);
      const { attempt: a } = state();
      if (id === "enterprise-field") {
        const key = String(f.get("key")).trim(),
          cols = (a.customColumns || []).filter(
            (c) => c.key !== form.dataset.oldKey,
          );
        cols.push({
          key,
          label: f.get("label"),
          kind: f.get("kind"),
          unit: f.get("unit"),
          purpose: f.get("purpose"),
          required: f.has("required"),
        });
        await window.Features.enterpriseOperation("configure", {
          customColumns: cols,
        });
        close();
        say("自定义字段已保存，请补充数据并重新校验。");
      }
      if (id === "enterprise-source") {
        let content = String(f.get("content") || ""),
          format = String(f.get("format"));
        const file = f.get("file");
        if (file?.size) {
          if (file.size > 10 * 1024 * 1024) throw new Error("来源文件最多10MB");
          content = await file.text();
          format = file.name.toLowerCase().endsWith(".json") ? "json" : "csv";
        }
        if (!content.trim()) throw new Error("请选择文件或粘贴来源内容");
        await window.Features.enterpriseOperation("source-import", {
          source: {
            id: form.dataset.source,
            system: f.get("system"),
            object: f.get("object"),
            keyField: f.get("keyField"),
            collector: f.get("collector"),
            asOf: new Date(f.get("asOf")).toISOString(),
            format,
            content,
          },
        });
        close();
        say("来源已保存，接下来检查映射并重新关联。");
      }
      if (id === "enterprise-provenance") {
        await window.Features.enterpriseOperation("configure", {
          dataSource: {
            mode: f.get("mode"),
            system: f.get("system"),
            collector: f.get("collector"),
            asOf: new Date(f.get("asOf")).toISOString(),
            note: f.get("note"),
          },
        });
        say("数据来源信息已保存。");
      }
      if (id === "enterprise-parameters") {
        const parameters = Object.fromEntries(
          [...f.entries()].map(([k, v]) => [k, Number(v)]),
        );
        await window.Features.enterpriseOperation("configure", { parameters });
        say("参数已保存，旧结果已清除，请重新运行分析。");
      }
    } catch (err) {
      const node = dialog.open
        ? dialog.querySelector("#enterprise-error")
        : null;
      if (node) node.textContent = err.message;
      else say(err.message, true);
    } finally {
      button.disabled = false;
    }
  });
  dialog.addEventListener("cancel", (ev) => {
    ev.preventDefault();
    close();
  });
  return {
    showRows(a) {
      return (
        a.acquisition?.mode === "manual" ||
        (a.acquisition?.mode === "mapped" && a.acquisition.assembled) ||
        a.acquisition?.note?.includes("基础样例已选择")
      );
    },
    stepCard,
    dataPanel,
    parameterPanel,
    history,
    updateReviewButton,
    refreshReviewStatus,
    clear() {
      window.DiscussionUI?.clear();
      record = null;
      historyModule = "";
      close();
    },
    historyContext() {
      return record?.attempt.status === "submitted"
        ? {
            id: record.attempt.id,
            module: record.attempt.module,
            number: record.attempt.number,
          }
        : null;
    },
  };
})();
