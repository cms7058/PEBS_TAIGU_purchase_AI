"use strict";
window.Features = (() => {
  const workspace = document.querySelector("#workspace");
  let active = "",
    spec = null,
    attempt = null,
    step = 0,
    busy = false;
  let report = null,
    exam = null,
    examDraft = {},
    examDirty = false;
  let lastActivity = Date.now(),
    lastExamSave = 0,
    pulseRunning = false;
  let deckPages = [];
  let assistantPauseAttempt = "";
  async function assistantTiming(waiting) {
    if (!waiting) {
      const resume = attempt && assistantPauseAttempt === attempt.id;
      assistantPauseAttempt = "";
      if (!resume) return;
    }
    if (active !== "training" || !attempt || attempt.status !== "draft" || busy)
      return;
    busy = true;
    try {
      await saveOperation("heartbeat", { active: !waiting });
      assistantPauseAttempt = waiting ? attempt.id : "";
    } finally {
      busy = false;
    }
  }
  const titles = {
    reports: "管理员统计",
    courseware: "课件管理",
    "exam-admin": "结业考试管理",
    exam: "第二天结业考试",
    resources: "软件体验渠道",
    password: "修改密码",
    learners: "学员管理",
    consultations: "问题与复盘",
    lessons: "课堂课件",
    history: "历史练习",
  };
  const nav = (role, current = active) => {
    const items =
      role === "admin"
        ? [
            ["classroom", "授课控制"],
            ["learners", "学员管理"],
            ["reports", "实训统计"],
            ["consultations", "问题与复盘"],
            ["courseware", "课件管理"],
            ["exam-admin", "结业考试"],
            ["resources", "软件渠道"],
            ["password", "密码"],
          ]
        : [
            ["classroom", "我的课堂"],
            ["exam", "结业考试"],
            ["resources", "软件渠道"],
            ["password", "密码"],
          ];
    return `<nav class="feature-nav" aria-label="功能导航">${items.map(([id, title]) => `<button data-view="${id}" class="${current === id ? "selected" : ""}">${title}</button>`).join("")}</nav>`;
  };
  function shell(title, content) {
    if (user.role === "learner" && window.LearnerUI) {
      workspace.innerHTML = window.LearnerUI.content(title, content);
      window.LearnerUI.updateSidebar();
      window.Assistant?.updateContext();
      return;
    }
    workspace.innerHTML = `<div class="shell">${nav(user.role)}<div class="toolbar"><h1>${e(title)}</h1><button data-view="classroom">返回课堂</button></div>${content}<p id="feature-message" role="status" aria-live="polite"></p></div>`;
  }
  function message(text, error = false) {
    const target = document.querySelector("#feature-message");
    if (target) {
      target.textContent = text;
      target.classList.toggle("error-text", error);
    }
  }
  async function show(view, options = {}) {
    if (!user) return;
    active = view;
    document.querySelector("#app").hidden = view !== "classroom";
    workspace.hidden = view === "classroom";
    if (view === "classroom") {
      active = "";
      if (user.role === "admin") renderAdmin();
      else renderLearner();
      return;
    }
    shell(titles[view] || "引导式实训", '<div class="panel">正在载入…</div>');
    try {
      if (view === "reports") {
        report = await api("/api/admin/reports");
        renderReports();
      }
      if (view === "courseware") await courseware();
      if (view === "consultations") await window.ConsultationUI.show();
      if (view === "learners") await window.Learners.show();
      if (view === "exam-admin") await examAdmin();
      if (view === "exam") await loadExam();
      if (view === "lessons") window.LearnerUI.lessons();
      if (view === "history")
        await window.EnterpriseUI.history(options.module || "");
      if (view === "resources") {
        const items = await api("/api/resources");
        shell(
          titles[view],
          `<p class="notice">公开信息核查于2026-10-27，未注册验证实际账号；请自行核实开通条件及课堂权限。</p><div class="resource-grid">${items.map((x) => `<section class="panel"><h2>${e(x.name)}</h2><span class="tag">${e(x.type)}</span><p>${e(x.period)}</p><p class="muted">关联模块：${e(x.modules)}</p><a href="${e(x.url)}" target="_blank" rel="noopener noreferrer">前往官方渠道 ↗</a></section>`).join("")}</div>`,
        );
      }
      if (view === "password")
        shell(
          titles[view],
          `<form id="password-form" class="panel form-stack"><label>当前密码<input type="password" name="current" autocomplete="current-password" required></label><label>新密码（至少${user.role === "learner" ? 6 : 16}字符）<input type="password" name="next" autocomplete="new-password" minlength="${user.role === "learner" ? 6 : 16}" maxlength="200" required></label><button class="primary">修改并重新登录</button></form>`,
        );
    } catch (err) {
      message(err.message, true);
    }
  }
  async function training(module) {
    if (busy) return;
    busy = true;
    active = "training-entry";
    document.querySelector("#app").hidden = true;
    workspace.hidden = false;
    shell("引导式实训", '<div class="panel">载入案例…</div>');
    try {
      const data = await api("/api/training/" + module);
      spec = data.case;
      attempt = data.attempt;
      renderTrainingEntry();
    } catch (err) {
      message(err.message, true);
    } finally {
      busy = false;
    }
  }
  function renderTrainingEntry() {
    active = "training-entry";
    const submitted = attempt?.status === "submitted";
    shell(
      spec.title,
      `<section class="panel training-entry"><p class="eyebrow">引导式实训 · ${e(spec.module)}</p><h2>实训任务入口</h2><p>${e(spec.goal)}</p><ol>${spec.instructions.map((x) => `<li>${e(x)}</li>`).join("")}</ol><p class="notice">${!attempt ? "尚未开始。开始后将从阅读任务进入五步引导。" : submitted ? `第 ${attempt.number} 次练习已于 ${new Date(attempt.submittedAt).toLocaleString("zh-CN")} 提交。可开始新一轮练习，历史作业和评分会保留。` : `第 ${attempt.number} 次练习尚未提交。继续练习会恢复已保存的数据与进度。`}</p><div class="row"><button class="primary" data-training-entry="${!attempt || submitted ? "begin" : "resume"}">${!attempt ? "开始实训" : submitted ? "开始新一轮练习" : "继续未完成练习"}</button>${submitted ? '<button data-training-entry="receipt">查看上次提交回执</button>' : attempt ? '<button data-training-entry="instructions">查看任务说明</button>' : ""}<button data-view="history" data-history-module="${e(spec.module)}">查看历史练习</button><button data-view="classroom">返回课堂</button></div><p class="muted">流程：阅读任务 → 检查数据 → 计算分析 → 复核与提交 → 提交回执。进入本页不会自动创建或提交作业。</p></section>`,
    );
  }
  async function enterTraining(mode) {
    if (busy) return;
    busy = true;
    try {
      if (mode === "receipt") {
        active = "training-history";
        step = 4;
        renderTraining();
        return;
      }
      attempt = await api("/api/training/" + spec.module + "/start", {
        retry: mode === "begin",
      });
      if (attempt.status === "submitted") {
        renderTrainingEntry();
        message("这份练习已提交，请选择开始新一轮或查看回执。");
        return;
      }
      active = "training";
      step =
        mode === "instructions" || !attempt.steps.includes("inspect")
          ? 0
          : !attempt.steps.includes("data")
            ? 1
            : !attempt.steps.includes("calculate")
              ? 2
              : 3;
      lastActivity = Date.now();
      renderTraining();
    } catch (err) {
      message(err.message, true);
    } finally {
      busy = false;
    }
  }
  function dataTable(rows) {
    const cols = trainingColumns();
    return `<div class="table-scroll"><table><thead><tr>${cols.map((c) => `<th>${e(c.label)}<small>${e(c.key)}${c.unit ? " · " + e(c.unit) : ""}${c.purpose ? " · 自定义" : ""}</small></th>`).join("")}<th>操作</th></tr></thead><tbody>${rows.map((row, i) => `<tr>${cols.map((c) => `<td><input data-row="${i}" data-col="${e(c.key)}" aria-label="第${i + 1}行 ${e(c.label)}" value="${e(row[c.key])}" ${c.kind !== "text" && c.kind !== "date" ? 'inputmode="decimal"' : ""}></td>`).join("")}<td><button data-enterprise-remove-row="${i}" ${rows.length < 2 ? "disabled" : ""}>移除</button></td></tr>`).join("")}</tbody></table></div>`;
  }
  function trainingColumns() {
    return [...spec.columns, ...(attempt?.customColumns || [])];
  }
  function currentRows() {
    if (!document.querySelector('[data-row="0"]')) return attempt.rows;
    return attempt.rows.map((_, i) =>
      Object.fromEntries(
        trainingColumns().map((c) => [
          c.key,
          document.querySelector(`[data-row="${i}"][data-col="${c.key}"]`)
            ?.value || "",
        ]),
      ),
    );
  }
  async function enterpriseOperation(kind, extra = {}) {
    if (busy) throw new Error("请等待当前操作完成");
    busy = true;
    try {
      if (kind === "configure" && !extra.rows) extra.rows = currentRows();
      await saveOperation(kind, extra);
      if (kind !== "configure" || step !== 2) step = 1;
      renderTraining();
      return attempt;
    } finally {
      busy = false;
    }
  }
  function editRows(remove) {
    if (busy) return;
    attempt.rows = currentRows();
    if (remove === undefined) {
      if (attempt.rows.length >= 5000)
        return message("每份练习最多5000行", true);
      attempt.rows.push(
        Object.fromEntries(trainingColumns().map((c) => [c.key, ""])),
      );
    } else if (attempt.rows.length > 1) attempt.rows.splice(remove, 1);
    renderTraining();
  }
  function resultTable(result) {
    if (!result || !result.rows.length) return "";
    const columns = Object.keys(result.rows[0]);
    return `<div class="table-scroll"><table><thead><tr>${columns.map((k) => `<th>${e(k)}</th>`).join("")}</tr></thead><tbody>${result.rows.map((row) => `<tr>${columns.map((k) => `<td>${e(typeof row[k] === "boolean" ? (row[k] ? "是" : "否") : row[k])}</td>`).join("")}</tr>`).join("")}</tbody></table></div><p class="muted">${e(result.explanation)}</p>`;
  }
  function renderTraining() {
    if (!attempt || !spec) return;
    const names = [
      "阅读任务",
      "检查数据",
      "计算分析",
      "复核与提交",
      "提交回执",
    ];
    const a = attempt.answer;
    let body = "";
    if (step === 0)
      body = `<h2>${e(spec.title)}</h2><p>${e(spec.goal)}</p><ol>${spec.instructions.map((x) => `<li>${e(x)}</li>`).join("")}</ol><p class="muted">案例与模板版本：${e(spec.version)} · 第 ${attempt.number} 次练习 · 数据来源按本轮采集快照记录</p><button class="primary" data-training="inspect">我已阅读，开始数据检查</button>`;
    if (step === 1)
      body = `${window.EnterpriseUI.dataPanel(spec, attempt)}${!window.EnterpriseUI.showRows(attempt) ? '<section class="panel"><h3>业务数据尚未就绪</h3><p class="muted">请先采集来源并生成关联表，或选择人工填报。基础样例仅用于熟悉操作，不代表企业数据。</p><button data-training="back">返回任务说明</button></section>' : `<section class="normalized-data"><div class="toolbar"><div><h2>归一化业务表</h2><p class="muted">${attempt.rows.length} 行 · ${trainingColumns().length} 个字段。标准字段必填，自定义字段按各自设置校验。</p></div><button data-enterprise-action="add-row">新增业务行</button></div><div class="row"><button data-download="blank">下载当前字段模板</button><button data-download="example">下载填写示例</button><label class="upload-button">导入归一化CSV<input type="file" id="training-file" accept=".csv,text/csv"></label></div>${dataTable(attempt.rows)}<div class="row"><button class="primary" data-training="validate" ${attempt.acquisition?.mode === "mapped" && !attempt.acquisition.assembled ? "disabled" : ""}>验证并使用表格</button><button data-training="back">上一步</button></div></section>`}`;
    if (step === 2)
      body = `${window.EnterpriseUI.parameterPanel(spec, attempt)}<h2>运行分析并核对结果</h2><p>${e(spec.goal)}</p><button class="primary" data-training="calculate">运行教学模型</button>${resultTable(attempt.result)}${attempt.result ? '<button class="primary" data-training="to-review">填写结论与人工复核</button>' : ""}<button data-training="back">返回检查数据</button>`;
    if (step === 3)
      body = `<h2>分析与决策</h2>${resultTable(attempt.result)}<div class="form-grid">${spec.metrics.map((m, i) => `<label>${e(m)}<input type="number" step="any" id="metric-${i}" value="${e(a.metrics[i])}"></label>`).join("")}</div><label class="block-label">${e(spec.decisionHint)}<input id="training-decision" value="${e(a.decision)}" maxlength="160"></label><label class="block-label">分析说明<textarea id="training-analysis" rows="5" maxlength="12000" placeholder="比较候选方案，写明数据依据、约束与风险，至少20字。">${e(a.analysis)}</textarea></label><div class="review-checks">${[
        ["source", "已核对原始数据和来源"],
        ["units", "已核对单位、币种和计算"],
        ["constraints", "已核对证书、交期或模型适用约束"],
      ]
        .map(
          ([id, text]) =>
            `<label><input type="checkbox" data-check="${id}" ${a.checks?.includes(id) ? "checked" : ""}> ${text}</label>`,
        )
        .join(
          "",
        )}</div><div class="row"><button data-training="save">保存草稿</button><button data-training="ai">获取AI方法提示</button><button class="primary" data-training="submit">提交练习</button><button data-training="back">上一步</button></div><div id="ai-answer" class="notice" hidden></div>`;
    if (step === 4)
      body = `<h2>练习已提交</h2><p>提交时间：${new Date(attempt.submittedAt).toLocaleString("zh-CN", { timeZone: "Asia/Shanghai" })}</p><p>已保留数据、步骤及本次结论，管理员可查看过程评价和评分。</p><p class="muted">第 ${attempt.number} 次练习。重做会创建新记录。</p><button data-training="retry">重新练习</button><button data-view="history" data-history-module="${e(spec.module)}">查看历史练习与复盘</button><button data-view="classroom">返回课堂</button>`;
    shell(
      spec.title,
      `<div class="row"><span class="tag">第 ${attempt.number} 次练习 · ${attempt.status === "submitted" ? "已提交记录" : "进行中"}</span><button data-training="overview">返回任务入口</button></div><div class="step-rail">${names.map((n, i) => `<span class="${i === step ? "current" : i < step ? "done" : ""}">${i + 1} · ${n}</span>`).join("")}</div><section class="panel training-body">${window.EnterpriseUI.stepCard(spec, attempt, ["inspect", "data", "calculate", "review", "submit"][step])}${body}</section>`,
    );
  }
  function readAnswer() {
    if (!document.querySelector("#training-analysis")) return attempt.answer;
    return {
      metrics: [0, 1].map((i) =>
        Number(document.querySelector("#metric-" + i).value),
      ),
      decision: document.querySelector("#training-decision").value,
      analysis: document.querySelector("#training-analysis").value,
      checks: [...document.querySelectorAll("[data-check]:checked")].map(
        (x) => x.dataset.check,
      ),
    };
  }
  async function saveOperation(kind, extra = {}) {
    while (pulseRunning)
      await new Promise((resolve) => setTimeout(resolve, 20));
    const answer = readAnswer();
    const data = await api("/api/training/attempts/" + attempt.id, {
      revision: attempt.revision,
      kind,
      ...extra,
      answer,
    });
    attempt = data;
    return data;
  }
  function tableCSV() {
    const quote = (v) => '"' + String(v ?? "").replaceAll('"', '""') + '"';
    const lines = [
      trainingColumns()
        .map((c) => c.key)
        .join(","),
    ];
    for (let i = 0; i < attempt.rows.length; i++)
      lines.push(
        trainingColumns()
          .map((c) =>
            quote(
              document.querySelector(`[data-row="${i}"][data-col="${c.key}"]`)
                .value,
            ),
          )
          .join(","),
      );
    return lines.join("\r\n");
  }
  async function uploadCSV(text) {
    while (pulseRunning)
      await new Promise((resolve) => setTimeout(resolve, 20));
    const res = await fetch(
      portalURL("/api/training/attempts/" + attempt.id + "/import"),
      {
        method: "POST",
        headers: {
          "Content-Type": "text/csv; charset=utf-8",
          "X-Attempt-Revision": String(attempt.revision),
        },
        body: text,
      },
    );
    const data = await res.json();
    if (!res.ok)
      throw new Error(
        data.error + (data.details ? "\n" + data.details.join("\n") : ""),
      );
    attempt = data;
  }
  async function download(url, filename) {
    const res = await fetch(portalURL(url));
    if (!res.ok) throw new Error("下载失败，请检查权限");
    const objectURL = URL.createObjectURL(await res.blob());
    const link = document.createElement("a");
    link.href = objectURL;
    link.download = filename;
    link.click();
    setTimeout(() => URL.revokeObjectURL(objectURL), 1000);
  }
  function chart(values, label) {
    const width = 720,
      left = 180,
      top = 22,
      rowHeight = 42,
      height = top + values.length * rowHeight + 35;
    return `<svg class="report-chart" viewBox="0 0 ${width} ${height}" role="img" aria-label="${e(label)}"><text x="0" y="14">${e(label)}</text>${values.map((v, i) => `<text x="0" y="${top + i * rowHeight + 20}">${e(v.name)}</text><rect class="chart-track" x="${left}" y="${top + i * rowHeight + 7}" width="450" height="18" rx="3"/><rect class="chart-bar" x="${left}" y="${top + i * rowHeight + 7}" width="${Math.max(0, Math.min(100, v.value)) * 4.5}" height="18" rx="3"/><text x="645" y="${top + i * rowHeight + 21}">${v.count ? v.value.toFixed(1) + " 分" : "待提交"}</text>`).join("")}<text x="${left}" y="${height - 6}">0</text><text x="405" y="${height - 6}">50</text><text x="630" y="${height - 6}">100 分</text></svg>`;
  }
  function numericChart(values, label, unit) {
    const max = Math.max(1, ...values.map((v) => v.value)),
      height = values.length * 40 + 60;
    return `<svg class="report-chart" viewBox="0 0 720 ${height}" role="img" aria-label="${e(label)}"><text x="0" y="16">${e(label)}</text>${values.map((v, i) => `<text x="0" y="${45 + i * 40}">${e(v.name)}</text><rect class="chart-track" x="180" y="${31 + i * 40}" width="430" height="18" rx="3"/><rect class="chart-bar" x="180" y="${31 + i * 40}" width="${(v.value / max) * 430}" height="18" rx="3"/><text x="625" y="${45 + i * 40}">${v.value.toFixed(unit === "次" ? 0 : 1)} ${e(unit)}</text>`).join("")}<text x="180" y="${height - 3}">0</text><text x="590" y="${height - 3}">${max.toFixed(1)} ${e(unit)}</text></svg>`;
  }
  function renderReports() {
    const completed = report.attempts.filter(
      (a) => a.status === "submitted" && a.score,
    );
    const avg = completed.length
      ? completed.reduce((x, a) => x + a.score.total, 0) / completed.length
      : null;
    const averages = report.modules.map((m) => {
      const a = completed.filter((x) => x.module === m.id);
      return {
        name: m.title,
        count: a.length,
        value: a.length
          ? a.reduce((n, x) => n + x.score.total, 0) / a.length
          : 0,
      };
    });
    const cell = (id, mid) => {
      const a = report.attempts.find(
        (x) => x.learner === id && x.module === mid,
      );
      return a
        ? `<button class="score-cell" data-attempt="${e(a.id)}">${a.status === "draft" ? "进行中" : a.score ? a.score.total + " 分" : "待评"}</button>`
        : '<span class="muted">未开始</span>';
    };
    shell(
      "管理员统计",
      `<div class="row"><button data-view="reports">刷新统计</button><button data-export="reports">导出成绩CSV</button></div><div class="stat-grid"><section class="panel"><span>在线学员</span><strong>${report.online.length} / 10</strong></section><section class="panel"><span>已提交模块练习</span><strong>${completed.length} / ${report.learners.length * report.modules.length}</strong></section><section class="panel"><span>已评分练习均分</span><strong>${avg === null ? "—" : avg.toFixed(1)}</strong></section></div><section class="panel">${chart(averages, "各模块最近一次已提交练习平均分")}<p class="muted">${e(report.scorePolicy)}</p></section><section class="panel"><h2>学员与模块</h2><div class="table-scroll"><table><thead><tr><th>学员</th>${report.modules.map((m) => `<th>${e(m.id)}</th>`).join("")}</tr></thead><tbody>${report.learners.map((l) => `<tr><th>${e(l.id)}</th>${report.modules.map((m) => `<td>${cell(l.id, m.id)}</td>`).join("")}</tr>`).join("")}</tbody></table></div></section><section class="panel"><h2>结业考试</h2>${report.exams.length ? `<div class="table-scroll"><table><thead><tr><th>学员</th><th>状态</th><th>成绩</th><th>结果</th><th>复核</th></tr></thead><tbody>${report.exams.map((x) => `<tr><td>${e(x.attempt.learner)}</td><td>${x.attempt.status === "draft" ? "作答中" : x.complete ? "已评分" : "主观题待评"}</td><td>${x.complete ? x.total : "待完成评分"}</td><td>${x.complete ? (x.passed ? "通过" : "未通过") : "—"}</td><td><button data-exam-review="${e(x.attempt.learner)}">逐题查看</button></td></tr>`).join("")}</tbody></table></div>` : '<p class="muted">尚无考试记录。</p>'}</section>`,
    );
    const durations = report.modules.map((m) => {
      const records = completed.filter((a) => a.module === m.id);
      return {
        name: m.title,
        value: records.length
          ? records.reduce((sum, a) => sum + a.activeSeconds, 0) /
            records.length /
            60
          : 0,
      };
    });
    const deductions = new Map();
    completed.forEach((a) =>
      a.score.items.forEach((item) => {
        if (item.score < item.max)
          deductions.set(item.name, (deductions.get(item.name) || 0) + 1);
      }),
    );
    const analytics = document.createElement("section");
    analytics.className = "panel";
    analytics.innerHTML = completed.length
      ? numericChart(durations, "已提交练习平均有效操作时长", "分钟") +
        (deductions.size
          ? numericChart(
              [...deductions].map(([name, value]) => ({ name, value })),
              "扣分维度出现次数",
              "次",
            )
          : '<p class="muted">暂无规则扣分；人工调整的依据见单条复核记录。</p>')
      : '<h2>过程与薄弱环节</h2><p class="muted">提交练习后展示各模块操作时长与扣分维度。</p>';
    workspace.querySelector(".stat-grid").after(analytics);
  }
  async function attemptDetail(id) {
    const data = await api("/api/admin/attempts/" + id),
      a = data.attempt;
    shell(
      `${a.learner} · ${a.module} · 第${a.number}次`,
      `<section class="panel"><p>状态：${e(a.status)} · 有效操作 ${Math.round(a.activeSeconds)} 秒 · 总历时 ${Math.round((new Date(a.submittedAt || a.updatedAt) - new Date(a.startedAt)) / 1000)} 秒</p>${a.score ? `<h2>总分 ${a.score.total}</h2><p class="muted">评分规则 ${e(a.score.version)}</p><table><thead><tr><th>维度</th><th>得分</th><th>依据</th></tr></thead><tbody>${a.score.items.map((x) => `<tr><td>${e(x.name)}</td><td>${x.score} / ${x.max}</td><td>${e(x.reason)}</td></tr>`).join("")}</tbody></table><p>${e(a.score.reviewReason)}</p><form id="score-review" data-id="${e(a.id)}" data-revision="${a.revision}" class="form-stack"><label>复核总分<input type="number" name="total" min="0" max="100" step="0.5" value="${a.score.total}" required></label><label>调整理由<textarea name="reason" minlength="5" required></textarea></label><button class="primary">保存复核（保留原分）</button></form>` : "<p>尚未提交，无评分。</p>"}<h3>学员分析</h3><p class="pre-wrap">${e(a.answer.analysis)}</p><h3>操作时间线</h3><ol>${data.events.map((x) => `<li>${new Date(x.at).toLocaleTimeString("zh-CN")} · ${e(x.kind)}</li>`).join("")}</ol><button data-view="reports">返回统计</button></section>`,
    );
  }
  async function courseware() {
    const data = await api("/api/admin/courseware");
    deckPages = data.slides;
    shell(
      "课件管理",
      `<p class="notice">${e(data.notice)}</p><form id="courseware-form" class="panel form-stack"><h2>导入课件</h2><p class="muted">PDF原版式渲染：${data.pdfAvailable ? "已启用" : "此部署未启用，可选PDF运行镜像"}</p><label>课件文件<input type="file" name="file" accept=".pptx,.pdf" required></label><label><input name="confirm" type="checkbox" required> 我已知悉：替换后清空课件授权与当前投影，保留练习数据。</label><button class="primary">上传并导入课件</button></form><form id="pages-form" class="panel form-stack"><h2>页面内容与建议实训</h2><p class="muted">修改内容后需要重新激活课件。授课时仍可选择其他或多个实训模块。</p>${data.slides.map((p, i) => `<fieldset data-page-editor="${i}"><legend>${e(p.id)}</legend><label>标题<input data-field="title" value="${e(p.title)}" required maxlength="160"></label><label>正文<textarea data-field="description" rows="3">${e(p.description)}</textarea></label><label>建议关联实训<select data-field="suggested">${classroom.modules.map((m) => `<option value="${m.id}" ${p.suggested === m.id ? "selected" : ""}>${e(m.title)}</option>`).join("")}</select></label></fieldset>`).join("")}<button class="primary">保存页面与关联</button></form>`,
    );
  }
  async function examAdmin() {
    const config = await api("/api/admin/exam");
    shell(
      "第二天结业考试管理",
      `<p class="notice">当前为${config.example ? "示例试卷，正式授课前请按课程PPT替换并审核" : "课程试卷"}。只安排结业考试；发布时间窗口应在第二天。已有学员开考后配置会锁定。</p><form id="exam-config-form" class="panel form-stack" data-revision="${config.revision}"><label>试卷标题<input name="title" value="${e(config.title)}" required></label><div class="form-grid"><label>第二天日期<input name="day2" type="date" value="${e(config.day2)}"></label><label>时长（分钟）<input name="minutes" type="number" value="${config.minutes}" min="1" max="180" required></label><label>开放时间（本机时区）<input name="opens" type="datetime-local" value="${localDate(config.opensAt)}"></label><label>关闭时间（本机时区）<input name="closes" type="datetime-local" value="${localDate(config.closesAt)}"></label><label>通过线<input name="pass" type="number" value="${config.passScore}" min="0" max="100" required></label></div><h2>试题编辑</h2><p class="muted">总分须为100分。客观题自动评分，案例题由管理员依据参考标准复核。</p><div id="question-editors" class="form-stack">${config.questions.map(questionEditor).join("")}</div><div class="row"><button type="button" data-add-question="choice">新增选择题</button><button type="button" data-add-question="text">新增案例题</button></div><label><input type="checkbox" name="example" ${config.example ? "checked" : ""}> 此试卷仍为示例</label><label><input type="checkbox" name="published" ${config.published ? "checked" : ""}> 向全班10名学员发布</label><button class="primary">校验并保存考试</button></form>`,
    );
  }
  function questionEditor(q) {
    return `<fieldset data-question="${e(q.id)}" data-kind="${e(q.kind)}"><legend>${e(q.id)} · ${q.kind === "choice" ? "选择题" : "案例题"}</legend><label>题干<textarea data-question-field="prompt" rows="3" required>${e(q.prompt)}</textarea></label><div class="form-grid"><label>分值<input data-question-field="points" type="number" min="0.5" max="100" step="0.5" value="${q.points}" required></label><label>对应课件页<input data-question-field="slide" value="${e(q.slide)}"></label></div>${q.kind === "choice" ? `<label>选项（每行一个）<textarea data-question-field="options" rows="3" required>${e((q.options || []).join("\n"))}</textarea></label><label>正确选项序号（从1开始）<input data-question-field="answer" type="number" min="1" max="10" value="${Number(q.answer || 0) + 1}" required></label>` : `<label>评分参考<textarea data-question-field="rubric" rows="3">${e(q.rubric)}</textarea></label>`}<button type="button" data-remove-question="${e(q.id)}">删除此题</button></fieldset>`;
  }
  function localDate(iso) {
    const d = new Date(iso);
    if (Number.isNaN(+d) || d.getFullYear() < 2000) return "";
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}T${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
  }
  async function loadExam() {
    exam = await api("/api/exam");
    exam._offset = new Date(exam.serverNow) - Date.now();
    examDirty = false;
    examDraft = { ...(exam.attempt?.answers || {}) };
    renderExam();
  }
  function renderExam() {
    const a = exam.attempt;
    if (!a) {
      shell(
        "第二天结业考试",
        `<section class="panel"><h2>${e(exam.title)}</h2><p>${exam.example ? "当前为示例考试。" : ""}考试时长 ${exam.minutes} 分钟。</p><p>${exam.available ? "已分配考试，请在管理员设置的时间内开始。" : "等待管理员发布结业考试。"}</p><button class="primary" data-exam="start" ${exam.available ? "" : "disabled"}>开始考试</button><button data-view="exam">刷新状态</button></section>`,
      );
      return;
    }
    if (a.status === "submitted") {
      shell(
        "结业考试已交卷",
        '<section class="panel"><h2>交卷成功</h2><p>答案已保存。考试评价和成绩由管理员查看及复核。</p><button data-view="classroom">返回课堂</button></section>',
      );
      return;
    }
    shell(
      exam.title,
      `<p class="notice">剩余时间 <strong id="exam-countdown"></strong> · 服务端截止 ${new Date(a.deadline).toLocaleString("zh-CN", { timeZone: "Asia/Shanghai" })}（北京时间）</p><form id="exam-answers" class="panel form-stack">${a.questions.map((q, i) => `<fieldset><legend>${i + 1}. ${e(q.prompt)}（${q.points}分）</legend>${q.kind === "choice" ? q.options.map((v, j) => `<label><input type="radio" name="${e(q.id)}" value="${j}" ${examDraft[q.id] === String(j) ? "checked" : ""}> ${e(v)}</label>`).join("") : `<textarea name="${e(q.id)}" rows="5" maxlength="12000">${e(examDraft[q.id] || "")}</textarea>`}</fieldset>`).join("")}<div class="row"><button type="button" data-exam="save">保存作答</button><button class="primary" type="submit">确认交卷</button></div><p id="exam-save-status" class="muted">作答时自动保存。</p></form>`,
    );
    updateCountdown();
  }
  function updateCountdown() {
    const target = document.querySelector("#exam-countdown");
    if (!target || !exam?.attempt) return;
    const remaining = Math.max(
      0,
      Math.ceil(
        (new Date(exam.attempt.deadline) - Date.now() - (exam._offset || 0)) /
          1000,
      ),
    );
    target.textContent = `${Math.floor(remaining / 60)}:${String(remaining % 60).padStart(2, "0")}`;
    if (!remaining && !busy) saveExam("submit");
  }
  async function saveExam(kind) {
    if (busy) return;
    busy = true;
    try {
      const result = await api("/api/exam", {
        kind,
        revision: exam.attempt?.revision || 0,
        answers: examDraft,
      });
      exam.attempt = result;
      examDirty = false;
      lastExamSave = Date.now();
      if (kind !== "save" || result.status === "submitted") renderExam();
      else {
        const x = document.querySelector("#exam-save-status");
        if (x)
          x.textContent = "作答已保存 · " + new Date().toLocaleTimeString();
      }
    } catch (err) {
      message(err.message + "；请保留当前页面后重试。", true);
    } finally {
      busy = false;
    }
  }
  function renderExamReview(learner) {
    const x = report.exams.find((x) => x.attempt.learner === learner),
      a = x.attempt;
    shell(
      `${learner} · 考试逐题复核`,
      `<form id="exam-review-form" data-learner="${e(learner)}" data-revision="${a.revision}" class="panel form-stack">${a.questions.map((q) => `<fieldset><legend>${e(q.id)} · ${e(q.prompt)}（${q.points}分）</legend><p class="pre-wrap">学员答案：${q.kind === "choice" ? e(q.options[Number(a.answers[q.id])]) : e(a.answers[q.id] || "未作答")}</p>${q.kind === "choice" ? `<p class="muted">标准答案：${e(q.options[Number(q.answer)])}</p>` : `<p class="muted">评分参考：${e(q.rubric)}</p><label>得分<input name="${e(q.id)}" type="number" min="0" max="${q.points}" step="0.5" required value="${a.manualScores[q.id] ?? ""}"></label>`}</fieldset>`).join("")}<label>复核说明<textarea name="reason" minlength="5" required>${e(a.reviewReason)}</textarea></label><button class="primary" ${a.status === "draft" ? "disabled" : ""}>保存主观题评分</button><button type="button" data-view="reports">返回统计</button></form>`,
    );
  }
  document.addEventListener("click", async (ev) => {
    const button = ev.target.closest("button, a");
    if (!button || button.disabled) return;
    let trainingLocked = false;
    try {
      if (button.dataset.addQuestion) {
        const nodes = [...document.querySelectorAll("[data-question]")];
        let n = 1;
        while (nodes.some((el) => el.dataset.question === "Q" + n)) n++;
        document.querySelector("#question-editors").insertAdjacentHTML(
          "beforeend",
          questionEditor({
            id: "Q" + n,
            kind: button.dataset.addQuestion,
            prompt: "",
            points: 10,
            slide: "",
            options: ["选项一", "选项二"],
            answer: "0",
            rubric: "",
          }),
        );
        return;
      }
      if (button.dataset.removeQuestion) {
        button.closest("[data-question]").remove();
        return;
      }
      if (button.dataset.view)
        await show(button.dataset.view, {
          module: button.dataset.historyModule || "",
        });
      if (button.dataset.download)
        await download(
          "/api/training/" +
            spec.module +
            "/template?example=" +
            (button.dataset.download === "example" ? "1" : "0") +
            "&attempt=" +
            encodeURIComponent(attempt.id),
          spec.module + "-" + button.dataset.download + ".csv",
        );
      if (button.dataset.export)
        await download("/api/admin/reports?format=csv", "training-results.csv");
      if (button.dataset.attempt) await attemptDetail(button.dataset.attempt);
      if (button.dataset.examReview)
        renderExamReview(button.dataset.examReview);
      if (button.dataset.exam) await saveExam(button.dataset.exam);
      if (button.dataset.trainingEntry) {
        await enterTraining(button.dataset.trainingEntry);
        return;
      }
      const command = button.dataset.training;
      if (!command || busy) return;
      busy = true;
      trainingLocked = true;
      button.disabled = true;
      if (command === "overview") {
        if (attempt.status === "draft")
          await saveOperation("heartbeat", { active: false });
        renderTrainingEntry();
      }
      if (command === "inspect") {
        await saveOperation("step", { step: "inspect" });
        step = 1;
        renderTraining();
      }
      if (command === "validate") {
        await uploadCSV(tableCSV());
        step = 2;
        renderTraining();
      }
      if (command === "calculate") {
        await saveOperation("calculate");
        renderTraining();
      }
      if (command === "to-review") {
        step = 3;
        renderTraining();
      }
      if (command === "save") {
        await saveOperation("save");
        message("草稿已保存");
      }
      if (command === "submit") {
        await saveOperation("submit");
        step = 4;
        renderTraining();
      }
      if (command === "back") {
        attempt.answer = readAnswer();
        step = Math.max(0, step - 1);
        renderTraining();
      }
      if (command === "retry") {
        attempt = await api("/api/training/" + spec.module + "/start", {
          retry: true,
        });
        step = 0;
        active = "training";
        renderTraining();
      }
      if (command === "ai") {
        await saveOperation("save");
        await saveOperation("heartbeat", { active: false });
        let result;
        try {
          result = await api("/api/ai/assist", {
            attempt: attempt.id,
            prompt:
              "解释当前案例的方法、约束与复核要点，帮助我改进自己的分析。",
          });
        } finally {
          await saveOperation("heartbeat", { active: true });
        }
        const box = document.querySelector("#ai-answer");
        box.hidden = false;
        box.textContent =
          (result.mode === "api" ? "实时AI建议 · " : "") + result.text;
      }
    } catch (err) {
      message(err.message, true);
    } finally {
      if (trainingLocked) busy = false;
      button.disabled = false;
    }
  });
  document.addEventListener("submit", async (ev) => {
    const form = ev.target,
      id = form.getAttribute("id");
    if (
      ![
        "password-form",
        "courseware-form",
        "pages-form",
        "exam-config-form",
        "score-review",
        "exam-review-form",
        "exam-answers",
      ].includes(id)
    )
      return;
    ev.preventDefault();
    const button = form.querySelector("button.primary");
    if (button) button.disabled = true;
    try {
      const f = new FormData(form);
      if (id === "password-form") {
        await api("/api/password", {
          current: f.get("current"),
          next: f.get("next"),
        });
        location.reload();
      }
      if (id === "courseware-form") {
        f.append("revision", String(classroom.revision));
        const res = await fetch(portalURL("/api/admin/courseware"), {
          method: "POST",
          body: f,
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        classroom = data;
        await courseware();
        message("课件已导入，授权已清空，请返回课堂按页激活。");
      }
      if (id === "pages-form") {
        classroom = await api("/api/admin/courseware", {
          revision: classroom.revision,
          slides: [...form.querySelectorAll("[data-page-editor]")].map(
            (node) => {
              const original = deckPages[Number(node.dataset.pageEditor)];
              return {
                ...original,
                title: node.querySelector("[data-field=title]").value,
                description: node.querySelector("[data-field=description]")
                  .value,
                suggested: node.querySelector("[data-field=suggested]").value,
              };
            },
          ),
        });
        await courseware();
        message("课件已保存，授权已清空。");
      }
      if (id === "exam-config-form") {
        await api("/api/admin/exam", {
          revision: Number(form.dataset.revision),
          title: f.get("title"),
          day2: f.get("day2"),
          opensAt: f.get("opens")
            ? new Date(f.get("opens")).toISOString()
            : null,
          closesAt: f.get("closes")
            ? new Date(f.get("closes")).toISOString()
            : null,
          minutes: Number(f.get("minutes")),
          passScore: Number(f.get("pass")),
          published: f.has("published"),
          example: f.has("example"),
          learners: classroom.learners.map((l) => l.id),
          questions: [...form.querySelectorAll("[data-question]")].map(
            (node) => {
              const get = (key) =>
                node.querySelector(`[data-question-field=${key}]`)?.value || "";
              return {
                id: node.dataset.question,
                kind: node.dataset.kind,
                prompt: get("prompt"),
                points: Number(get("points")),
                slide: get("slide"),
                options:
                  node.dataset.kind === "choice"
                    ? get("options")
                        .split("\n")
                        .map((x) => x.trim())
                        .filter(Boolean)
                    : [],
                answer:
                  node.dataset.kind === "choice"
                    ? String(Number(get("answer")) - 1)
                    : "",
                rubric: get("rubric"),
              };
            },
          ),
        });
        await examAdmin();
        message("考试设置已保存。");
      }
      if (id === "score-review") {
        await api("/api/admin/attempts/" + form.dataset.id, {
          revision: Number(form.dataset.revision),
          total: Number(f.get("total")),
          reason: f.get("reason"),
        });
        await attemptDetail(form.dataset.id);
        message("复核已保存，原分和调整依据已保留。");
      }
      if (id === "exam-review-form") {
        const scores = {};
        const item = report.exams.find(
          (x) => x.attempt.learner === form.dataset.learner,
        );
        item.attempt.questions
          .filter((q) => q.kind === "text")
          .forEach((q) => (scores[q.id] = Number(f.get(q.id))));
        await api("/api/admin/exam/review", {
          learner: form.dataset.learner,
          revision: Number(form.dataset.revision),
          scores,
          reason: f.get("reason"),
        });
        await show("reports");
        message("主观题评分已保存。");
      }
      if (id === "exam-answers") await saveExam("submit");
    } catch (err) {
      message(err.message, true);
    } finally {
      if (button) button.disabled = false;
    }
  });
  document.addEventListener("input", (ev) => {
    lastActivity = Date.now();
    if (ev.target.closest("#exam-answers")) {
      examDraft[ev.target.name] = ev.target.value;
      examDirty = true;
    }
  });
  document.addEventListener("pointerdown", () => (lastActivity = Date.now()));
  document.addEventListener("change", async (ev) => {
    if (ev.target.id === "training-file" && ev.target.files[0]) {
      try {
        if (ev.target.files[0].size > 10 * 1024 * 1024)
          throw new Error("文件不能超过10 MB");
        await uploadCSV(await ev.target.files[0].text());
        renderTraining();
        message("CSV通过检查，已保存；确认后进入计算。");
      } catch (err) {
        message(err.message, true);
      }
    }
  });
  setInterval(async () => {
    if (active === "exam") {
      updateCountdown();
      if (examDirty && Date.now() - lastExamSave > 10000) saveExam("save");
    }
    if (
      active !== "training" ||
      !attempt ||
      attempt.status !== "draft" ||
      busy ||
      pulseRunning
    )
      return;
    pulseRunning = true;
    try {
      const answer = readAnswer();
      const data = await api("/api/training/attempts/" + attempt.id, {
        kind: "heartbeat",
        revision: attempt.revision,
        answer,
        active:
          assistantPauseAttempt !== attempt.id &&
          !document.hidden &&
          Date.now() - lastActivity < 45000,
      });
      data.answer = answer;
      attempt = data;
    } catch (err) {
      message(err.message, true);
    } finally {
      pulseRunning = false;
    }
  }, 15000);
  setInterval(() => {
    if (active === "exam") updateCountdown();
  }, 1000);
  function sync(nextUser, nextClassroom) {
    if (!nextUser) {
      active = "";
      workspace.hidden = true;
      window.Learners?.close();
      window.EnterpriseUI?.clear();
      return;
    }
    if (active === "history" && nextUser.examActive) {
      active = "history-paused";
      window.EnterpriseUI?.clear();
      shell(
        "考试期间暂停历史查看",
        '<section class="panel"><p>请先完成结业考试，交卷后可重新进入历史练习。</p><button data-view="exam">返回考试</button></section>',
      );
    }
    if (
      ["training", "training-entry", "training-history"].includes(active) &&
      nextUser.role === "learner" &&
      spec &&
      !nextUser.modules.some((m) => m.id === spec.module)
    ) {
      active = "closed-training";
      shell(
        "实训已结束",
        '<section class="panel"><h2>管理员已关闭该实训入口</h2><p>已经保存的练习数据保留，待重新开放后继续。</p><button data-view="classroom">返回课堂</button></section>',
      );
    }
  }
  return {
    nav,
    show,
    training,
    sync,
    shell,
    enterpriseOperation,
    editRows,
    get trainingState() {
      return { spec, attempt, step };
    },
    assistantTiming,
    get context() {
      return {
        view: active || "classroom",
        historyAttempt:
          active === "history"
            ? window.EnterpriseUI?.historyContext()?.id || ""
            : "",
        module: ["training", "training-entry", "training-history"].includes(
          active,
        )
          ? spec?.module || ""
          : "",
      };
    },
    get active() {
      return active;
    },
  };
})();
