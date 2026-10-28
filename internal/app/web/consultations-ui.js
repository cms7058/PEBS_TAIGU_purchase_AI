"use strict";
window.ConsultationUI = (() => {
  let data = null,
    ticket = null,
    grouping = "module",
    moduleFilter = "",
    learnerFilter = "",
    statusFilter = "",
    selectedQuestion = "";
  const editorDrafts = new Map();
  const labels = {
    submitted: "待处理",
    processing: "处理中",
    returned: "已回复",
  };
  const summaryLabels = {
    systems: "现有系统",
    functions: "已有功能",
    data: "数据来源与字段",
    process: "岗位与流程",
    problems: "实际问题",
    goals: "目标与约束",
    unknowns: "待确认事项",
  };
  const arr = (x) => (Array.isArray(x) ? x : []);
  const time = (x) => (x ? new Date(x).toLocaleString("zh-CN") : "—");
  const url = (id) => "/api/admin/consultations/" + encodeURIComponent(id);
  function message(text, error = false) {
    const n = document.querySelector("#consultation-message");
    if (n) {
      n.textContent = text;
      n.classList.toggle("error-text", error);
    }
  }
  function rows(items) {
    return `<div class="table-scroll"><table><thead><tr><th>学员／模块</th><th>实际问题</th><th>状态与提交时间</th><th>处理</th></tr></thead><tbody>${items.map((v) => `<tr><th>${e(v.learner)}<small>${e(v.module)} · 第${v.number}次练习</small></th><td class="consultation-problem">${e(v.summary?.problems || "待确认")}</td><td><span class="tag">${e(labels[v.status] || v.status)}</span><small>讨论追问 ${arr(v.questions).length} 条 · 待回复 ${arr(v.questions).filter((q) => !arr(v.replies).some((r) => r.questionId === q.id)).length} 条</small><small>${e(time(v.submittedAt))}</small></td><td><button data-consultation-open="${e(v.id)}">查看与回复</button></td></tr>`).join("")}</tbody></table></div>`;
  }
  function renderList() {
    const items = arr(data.items).filter(
      (v) =>
        (!moduleFilter || v.module === moduleFilter) &&
        (!learnerFilter || v.learner === learnerFilter) &&
        (!statusFilter || v.status === statusFilter),
    );
    const keys =
      grouping === "module"
        ? arr(data.modules).map((m) => m.id)
        : [
            ...new Set([
              ...arr(data.learners).map((l) => l.id),
              ...items.map((v) => v.learner),
            ]),
          ].sort();
    const groups = keys
      .map((key) => {
        const values = items.filter((v) => v[grouping] === key);
        values.sort(
          (a, b) =>
            a.learner.localeCompare(b.learner) ||
            a.module.localeCompare(b.module) ||
            a.submittedAt.localeCompare(b.submittedAt),
        );
        return `<section class="panel"><h2>${e(key)}${grouping === "module" ? " · " + e(data.modules.find((m) => m.id === key)?.title || "") : " · " + e(data.learners.find((l) => l.id === key)?.name || "学员")} <small>${values.length}项</small></h2>${values.length ? rows(values) : '<p class="muted">暂无符合条件的问题。</p>'}</section>`;
      })
      .join("");
    window.Features.shell(
      "问题与复盘",
      `<section class="panel"><h2>管理员集中处理学员问题</h2><p class="muted">学员提交 → 分类整理 → 咨询ChatGPT → 核对草稿 → 返回对应学员。草稿和模型结果仅管理员可见，点击返回后学员才可查看。</p><div class="row"><button data-consultation-group="module" class="${grouping === "module" ? "selected" : ""}">按六大模块分类</button><button data-consultation-group="learner" class="${grouping === "learner" ? "selected" : ""}">按学员编号分类</button><button data-consultation-action="refresh-list">刷新问题</button></div><div class="form-grid consultation-filters"><label>模块<select data-consultation-filter="module"><option value="">全部六模块</option>${arr(
        data.modules,
      )
        .map(
          (m) =>
            `<option value="${e(m.id)}" ${moduleFilter === m.id ? "selected" : ""}>${e(m.id + " · " + m.title)}</option>`,
        )
        .join(
          "",
        )}</select></label><label>学员<select data-consultation-filter="learner"><option value="">全部学员</option>${arr(
        data.learners,
      )
        .map(
          (l) =>
            `<option value="${e(l.id)}" ${learnerFilter === l.id ? "selected" : ""}>${e(l.id + " · " + l.name)}</option>`,
        )
        .join(
          "",
        )}</select></label><label>处理状态<select data-consultation-filter="status"><option value="">全部状态</option>${Object.entries(
        labels,
      )
        .map(
          ([k, v]) =>
            `<option value="${k}" ${statusFilter === k ? "selected" : ""}>${v}</option>`,
        )
        .join(
          "",
        )}</select></label></div><p>当前筛选 ${items.length} 项 · 待处理 ${items.filter((v) => v.status === "submitted").length} 项</p></section>${groups}<p id="consultation-message" role="status"></p>`,
    );
  }
  async function show() {
    ticket = null;
    data = await api("/api/admin/consultations");
    data.items = arr(data.items);
    data.modules = arr(data.modules);
    data.learners = arr(data.learners);
    renderList();
  }
  function questionSelect() {
    return `<label>当前回复对象<select id="consultation-question"><option value="">最初提交的问题</option>${arr(
      ticket.questions,
    )
      .map(
        (q, i) =>
          `<option value="${e(q.id)}" ${selectedQuestion === q.id ? "selected" : ""}>追问 ${i + 1} · ${arr(ticket.replies).some((r) => r.questionId === q.id) ? "已回复" : "待回复"} · ${e(q.text.slice(0, 45))}</option>`,
      )
      .join(
        "",
      )}</select></label><p class="notice" id="consultation-question-context"></p>`;
  }
  function editorKey(q) {
    return ticket.id + ":" + (q || "initial");
  }
  function rememberEditor() {
    const box = document.querySelector("#consultation-draft"),
      note = document.querySelector("#consultation-note");
    if (box && note)
      editorDrafts.set(editorKey(selectedQuestion), {
        text: box.value,
        note: note.value,
      });
  }
  function chooseQuestion(q, remember = true) {
    if (remember) rememberEditor();
    selectedQuestion = q;
    const saved =
      editorDrafts.get(editorKey(q)) ||
      ticket.drafts?.[q || "initial"] ||
      (ticket.draftQuestionId === q
        ? { text: ticket.draft, note: ticket.promptNote }
        : { text: "", note: "" });
    document.querySelector("#consultation-question").value = q;
    document.querySelector("#consultation-draft").value = saved.text || "";
    document.querySelector("#consultation-note").value = saved.note || "";
    const question = arr(ticket.questions).find((x) => x.id === q);
    document.querySelector("#consultation-question-context").textContent =
      question
        ? "正在回复：" +
          question.text +
          "；关联内容：" +
          (question.context || "整个答复")
        : "正在回复最初提交的问题。针对后续讨论时，请先选择对应追问。";
    document.querySelector("#consultation-report-preview").innerHTML = "";
  }
  function discussionHistory() {
    const replies = arr(ticket.replies),
      questions = arr(ticket.questions);
    const showReply = (r) =>
      `<section class="review-step"><p class="muted">${e(time(r.at))} · ${e(r.actor)} · ${e(r.source)}</p>${r.report ? window.ReportUI.render(r.report, { ticketId: ticket.id, replyId: r.id }) : `<p class="pre-wrap">${e(r.text)}</p>`}</section>`;
    return `<section class="panel"><h2>报告与多轮讨论记录</h2>${
      replies
        .filter((r) => !r.questionId)
        .map(showReply)
        .join("") || '<p class="muted">尚未返回初次报告。</p>'
    }${questions
      .map(
        (q, i) =>
          `<section class="discussion-round"><div class="discussion-question"><h3>学员追问 ${i + 1}</h3><p class="muted">${e(time(q.at))} · 关联：${e(q.context || "整个答复")}</p><p class="pre-wrap">${e(q.text)}</p><button data-consultation-question="${e(q.id)}">回复此追问</button></div>${
            replies
              .filter((r) => r.questionId === q.id)
              .map(showReply)
              .join("") || '<p class="notice">待管理员回复</p>'
          }</section>`,
      )
      .join("")}</section>`;
  }
  function renderEditor(report) {
    const preview = document.querySelector("#consultation-report-preview");
    preview.innerHTML =
      `<div class="report-editor-toolbar"><h3>与学员一致的图文预览</h3><p class="muted">各章节可展开编辑标题、栏目和正文。图表与流程根据正文生成；修改对应的指标或阶段内容后，保存编辑稿即可重新生成。保存草稿不会改变学员已收到的版本。</p><button type="button" data-consultation-action="save-report-edits">保存图文编辑为草稿</button><p data-report-editor-status role="status"></p></div>` +
      window.ReportUI.render(report, { replyId: "preview", editable: true });
  }
  function renderDetail(result) {
    ticket = result.ticket;
    selectedQuestion = ticket.draftQuestionId || "";
    window.Features.shell(
      "问题处理 · " + ticket.module + " / " + ticket.learner,
      `<div class="row"><button data-consultation-action="refresh-list">返回问题列表</button><span class="tag">${e(labels[ticket.status])}</span><span>${e(ticket.learner)} · ${e(ticket.module)} · 第${ticket.number}次练习 · ${e(time(ticket.submittedAt))}</span></div><section class="panel"><h2>学员提交快照</h2><p class="muted">本记录只关联上述学员和练习；企业情况来自学员陈述，未经独立核实。学员后续修改不会改变这份提交。</p><details><summary>口头／文字陈述</summary><p class="pre-wrap">${e(ticket.transcript)}</p></details>${Object.entries(
        summaryLabels,
      )
        .map(
          ([key, label]) =>
            `<section class="consultation-summary"><h3>${label}</h3><p class="pre-wrap">${e(ticket.summary?.[key] || "待确认")}</p></section>`,
        )
        .join(
          "",
        )}</section><section class="panel form-stack"><h2>咨询ChatGPT并核对结果</h2>${questionSelect()}<label>管理员补充问题／关注点<textarea id="consultation-note" rows="3" maxlength="1800" placeholder="例如：重点分析现有RPA的边界、可先试点的改进方案，以及还需追问的信息。">${e(ticket.promptNote)}</textarea></label><div class="row"><button data-consultation-action="export-prompt">生成咨询资料</button><a href="https://chatgpt.com/" target="_blank" rel="noopener noreferrer">打开ChatGPT ↗</a><button data-consultation-action="consult" ${result.apiAvailable ? "" : "disabled"}>后台模型咨询（生成草稿）</button></div><p class="notice">${result.apiAvailable ? "后台接口已配置。模型结果只保存为管理员草稿，核对后才能返回。" : "后台接口未配置。点击生成咨询资料，复制到ChatGPT；取得结果后粘贴到下方草稿，核对后返回。"}</p><label>咨询资料（全选复制到ChatGPT）<textarea id="consultation-prompt" rows="5" readonly></textarea></label><label>回复草稿（仅管理员可见，可直接粘贴中文结果或analysis JSON）<textarea id="consultation-draft" rows="12" maxlength="12000" placeholder="请核对依据、利弊、适配建议、实施前提与验证指标，再返回给学员。">${e(ticket.draft)}</textarea></label><p class="muted">来源：${e(ticket.draftSource || "管理员待填写")}。可保存草稿稍后处理；返回只发送给 ${e(ticket.learner)}。</p><div class="row"><button data-consultation-action="preview-report">预览学员图文报告</button><button data-consultation-action="save-draft">保存草稿</button><button class="primary" data-consultation-action="publish">确认返回给 ${e(ticket.learner)}</button></div><p id="consultation-message" role="status" aria-live="polite"></p><section id="consultation-report-preview"></section></section>${discussionHistory()}<section class="panel"><h2>处理记录</h2><details><summary>处理记录</summary><ul>${arr(
        ticket.events,
      )
        .map(
          (ev) =>
            `<li>${e(time(ev.at))} · ${e(ev.actor)} · ${e({ submit: "学员提交", consult: "模型咨询并保存草稿", "save-draft": "管理员保存草稿", publish: "返回学员" }[ev.action] || ev.action)}</li>`,
        )
        .join("")}</ul></details></section>`,
    );
    chooseQuestion(selectedQuestion, false);
    if (result.draftReport && ticket.draft) renderEditor(result.draftReport);
  }
  async function open(id) {
    renderDetail(await api(url(id)));
  }
  async function act(kind) {
    const changed = document.querySelector(
      "#consultation-report-preview [data-report-editor-status]",
    );
    if (kind === "save-draft" && changed?.dataset.dirty === "true")
      kind = "save-report-edits";
    if (
      ["publish", "preview-report", "consult"].includes(kind) &&
      changed?.dataset.dirty === "true"
    ) {
      message(
        "请先点击“保存图文编辑为草稿”，核对更新后的预览再返回学员。",
        true,
      );
      return;
    }
    const id = ticket.id,
      revision = ticket.revision,
      note = document.querySelector("#consultation-note").value;
    if (kind === "save-report-edits") {
      const editor = document.querySelector(
        "#consultation-report-preview [data-report-editor]",
      );
      if (!editor) {
        message("请先预览图文报告。", true);
        return;
      }
      document.querySelector("#consultation-draft").value =
        window.ReportUI.editorText(editor);
      kind = "save-draft";
    }
    let draft = document.querySelector("#consultation-draft").value;

    const controls = [
        ...document.querySelectorAll(
          "#workspace button,#workspace textarea,#workspace select",
        ),
      ],
      before = controls.map((x) => x.disabled);
    controls.forEach((x) => (x.disabled = true));
    message(
      kind === "consult" ? "模型正在处理，结果将只存为草稿……" : "正在处理……",
    );
    try {
      const result = await api(url(id), {
        action: kind,
        revision,
        note,
        draft,
        questionId: selectedQuestion,
      });
      if (window.Features.active !== "consultations" || ticket?.id !== id)
        return;
      if (result.report) {
        const preview = document.querySelector("#consultation-report-preview");
        renderEditor(result.report);
        preview.scrollIntoView({ behavior: "smooth", block: "start" });
        message(
          "这是学员将看到的结构化预览，尚未返回。请核对图表来源及目标标签。",
        );
      } else if (result.prompt) {
        const box = document.querySelector("#consultation-prompt");
        box.value = result.prompt;
        box.select();
        message(
          "咨询资料已生成，请复制到ChatGPT；模型不会自动向学员发送结果。",
        );
      } else {
        editorDrafts.delete(editorKey(selectedQuestion));
        renderDetail(result);
        message(
          kind === "publish"
            ? "已返回给 " + ticket.learner + "，学员刷新处理状态即可查看。"
            : "已保存为管理员草稿，尚未返回学员。",
        );
      }
    } catch (err) {
      if (ticket?.id === id) message(err.message, true);
    } finally {
      controls.forEach((x, i) => {
        if (x.isConnected) x.disabled = before[i];
      });
    }
  }
  document.addEventListener("report-editor-changed", () => {
    const n = document.querySelector("[data-report-editor-status]");
    if (n) {
      n.dataset.dirty = "true";
      n.textContent =
        "正文已修改，尚未保存；保存图文编辑后会同步更新图表和流程。";
    }
  });
  document.addEventListener("input", (ev) => {
    if (ev.target.closest("[data-report-editor]")) {
      const n = document.querySelector("[data-report-editor-status]");
      if (n) {
        n.dataset.dirty = "true";
        n.textContent = "图文内容已修改，请保存编辑稿后再发送。";
      }
    }

    if (
      ticket &&
      ["consultation-draft", "consultation-note"].includes(ev.target.id)
    )
      rememberEditor();
  });
  document.addEventListener("change", (ev) => {
    if (ev.target.id === "consultation-question") {
      chooseQuestion(ev.target.value);
      return;
    }
    const key = ev.target.dataset.consultationFilter;
    if (!key) return;
    if (key === "module") moduleFilter = ev.target.value;
    if (key === "learner") learnerFilter = ev.target.value;
    if (key === "status") statusFilter = ev.target.value;
    renderList();
  });
  document.addEventListener("click", async (ev) => {
    const b = ev.target.closest("button");
    if (!b || b.disabled || user?.role !== "admin") return;
    try {
      if (b.dataset.consultationGroup) {
        grouping = b.dataset.consultationGroup;
        renderList();
      }
      if (b.dataset.consultationQuestion) {
        chooseQuestion(b.dataset.consultationQuestion);
        document
          .querySelector("#consultation-question")
          .scrollIntoView({ behavior: "smooth", block: "center" });
      }
      if (b.dataset.consultationOpen) await open(b.dataset.consultationOpen);
      const kind = b.dataset.consultationAction;
      if (kind === "refresh-list") await show();
      else if (kind) await act(kind);
    } catch (err) {
      message(err.message, true);
    }
  });
  return { show };
})();
