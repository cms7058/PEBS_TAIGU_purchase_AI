"use strict";
window.ReviewUI = (() => {
  let current = null,
    id = "",
    recognizer = null,
    transcriptDirty = false,
    summaryDirty = false,
    tickets = [];
  const fields = [
    ["systems", "现有系统及版本"],
    ["functions", "已有功能"],
    ["data", "数据来源与字段"],
    ["process", "岗位与业务流程"],
    ["problems", "实际问题与具体事例"],
    ["goals", "目标与约束"],
    ["unknowns", "待确认事项／需要追问的问题"],
  ];
  const worksheetDefinitions = {
    data: [
      ["需要哪些信息", "例如：件号、需求数量、库存数量、需要到货的日期"],
      [
        "这些信息在哪里",
        "例如：采购系统、库存系统、Excel、邮件；不知道系统名称可写待确认",
      ],
      ["谁提供或整理", "例如：计划员提供需求，仓库提供库存，采购员汇总"],
      ["多久更新一次", "例如：每天上午导出；有变化发邮件；不清楚写待确认"],
      [
        "目前哪里容易出错",
        "例如：同一件号写法不一致、数量过期、缺少日期；没有可写暂无",
      ],
    ],
    process: [
      ["谁提出需求", "例如：维修计划员根据工作包提出航材需求"],
      ["谁录入或整理", "例如：采购员把需求整理到系统或Excel"],
      [
        "谁核对和批准",
        "例如：仓库核对库存，质量人员核对文件，主管审批；以实际为准",
      ],
      ["谁执行和跟进", "例如：采购员询价下单并催交，物流跟进运输"],
      ["平时怎样交接", "例如：系统流转、邮件、表格、电话；先后顺序可直接描述"],
      ["最容易卡在哪一步", "例如：等待回复、重复录入、审批慢；不确定写待确认"],
    ],
    goals: [
      [
        "最想先解决什么",
        "例如：减少手工录入、避免漏催交；先选一个最重要的问题",
      ],
      ["现在是什么情况", "例如：每天花约2小时核对；没有统计可写尚未统计"],
      [
        "希望改善到什么程度",
        "例如：希望减少一半核对时间；没有目标可写先试点后确定",
      ],
      ["希望何时看到效果", "例如：培训后先试用两周；尚未安排可写待确认"],
      ["有哪些现实限制", "例如：暂不更换原系统、只能两人参与、预算未定"],
      ["哪些要求必须保留", "例如：保留人工审批、不能上传客户资料；按实际填写"],
    ],
  };
  function worksheet(key) {
    const rows = worksheetDefinitions[key];
    if (!rows) return "";
    return `<details class="review-worksheet"><summary>不熟悉这些说法？按问答表填写</summary><p class="muted">只填知道的内容，其余可留空，生成时标为“待确认”。以下提示只是示例，不代表企业实际情况。填完点击下方按钮，再核对生成的文字。</p><div class="table-scroll"><table><thead><tr><th>用自己的话回答</th><th>实际情况</th></tr></thead><tbody>${rows.map(([label, example]) => `<tr><th>${e(label)}</th><td><input type="text" data-review-cell="${key}" data-review-label="${e(label)}" maxlength="220" aria-label="${e(label)}" placeholder="${e(example)}"><small class="muted">${e(example)}</small></td></tr>`).join("")}</tbody></table></div><div class="row"><label>生成后如何放入上方文字<select data-review-template-mode="${key}" aria-label="${e(key)}模板写入方式"><option value="append">保留原文，追加表格内容</option><option value="replace">用表格内容替换原文</option></select></label><button type="button" data-review-template="${key}">生成可编辑文字</button></div><p class="muted">表格本身不单独保存；生成的文字将随“确认以上内容”保存。</p><p data-review-template-status="${key}" role="status" aria-live="polite"></p></details>`;
  }
  function summaryField(key, label, v) {
    const plain = {
      data: "信息从哪里来、具体用到什么",
      process: "谁来做、先做什么、再做什么",
      goals: "希望改善什么、有哪些限制",
    };
    return `<div class="review-summary-field"><label>${e(label)}${plain[key] ? `<small class="muted">${e(plain[key])}</small>` : ""}<textarea name="${key}" rows="3" maxlength="1800" ${["functions", "problems"].includes(key) ? "required" : ""}>${e(v.summary[key])}</textarea></label>${worksheet(key)}</div>`;
  }
  function applyWorksheet(key) {
    const values = [...root().querySelectorAll(`[data-review-cell="${key}"]`)];
    const node = root().querySelector(`[data-review-template-status="${key}"]`);
    if (!values.some((x) => x.value.trim())) {
      node.textContent = "请至少填写一项实际情况，再生成文字。";
      return false;
    }
    const text = values
      .map((x) => x.dataset.reviewLabel + "：" + (x.value.trim() || "待确认"))
      .join("\n");
    const box = root().querySelector(`#review-confirm-form [name="${key}"]`);
    const append =
      root().querySelector(`[data-review-template-mode="${key}"]`).value ===
      "append";
    const sheet = box.closest(".review-summary-field").querySelector("details");
    const previous = sheet.dataset.generated;
    const next =
      append && previous && box.value.includes(previous)
        ? box.value.replace(previous, text)
        : append && box.value.trim()
          ? box.value.trim() + "\n\n" + text
          : text;
    if (next.length > 1800) {
      node.textContent = "内容超过1800字，请先精简原文或表格内容，原文未改变。";
      return false;
    }
    box.value = next;
    sheet.dataset.generated = text;
    dirty("summary");
    node.textContent = "已生成到上方，可继续修改；最后点击“确认以上内容”保存。";
    box
      .closest(".review-summary-field")
      .querySelector("details").dataset.pending = "false";
    root().querySelector(`[data-review-template="${key}"]`).disabled = true;
    box.focus();
    return true;
  }
  const root = () => document.querySelector("#history-review-panel");
  const endpoint = () => "/api/history/" + encodeURIComponent(id) + "/review";
  function stop() {
    if (recognizer) {
      recognizer.abort();
      recognizer = null;
    }
  }
  function message(text, error = false) {
    const node = root()?.querySelector("[data-review-message]");
    if (node) {
      node.textContent = text;
      node.classList.toggle("error-text", error);
    }
  }
  function dirty(kind) {
    if (kind === "transcript") transcriptDirty = true;
    else summaryDirty = true;

    root()
      ?.querySelectorAll("[data-review-action]")
      .forEach((b) => {
        const action = b.dataset.reviewAction;
        if (
          action === "submit" ||
          (transcriptDirty && ["manual-summary", "confirm"].includes(action))
        )
          b.disabled = true;
      });
    message(
      kind === "transcript"
        ? "陈述已修改，请先保存。保存后需重新整理和确认，之前提交及管理员回复会保留。"
        : "整理内容已修改，请重新确认后再提交。",
    );
  }
  function render(data) {
    stop();
    window.EnterpriseUI?.updateReviewButton(data.reviewState);
    current = data.review;
    tickets = Array.isArray(data.tickets) ? data.tickets : [];
    current.summary = current.summary || {};
    transcriptDirty = summaryDirty = false;
    const v = current,
      ready = v.summaryReady,
      confirmed = !!v.confirmedAt,
      supported = !!(
        window.SpeechRecognition || window.webkitSpeechRecognition
      );
    root().innerHTML = `<section class="panel enterprise-interview"><p class="eyebrow">企业情况访谈 · 历史练习复盘</p><h2>整理问题，提交管理员复盘</h2><ol class="review-progress"><li class="current">1 陈述企业情况</li><li class="${ready ? "current" : ""}">2 手工整理</li><li class="${confirmed ? "current" : ""}">3 核对与提交</li><li class="${tickets.length ? "current" : ""}">4 管理员回复</li></ol><p class="notice">学员只需描述实际情况、手工整理并提交。管理员按模块和学员汇总问题，咨询ChatGPT并核对后，将结果返回给本人；此处不直接生成AI建议。</p>${tickets.length ? '<details class="review-intake"><summary>查看或补充原始企业情况（修改后可重新提交）</summary>' : ""}<label>口头陈述转写／文字补充<textarea id="review-transcript" rows="6" maxlength="5000" placeholder="我们目前用什么系统或表格，能做哪些事，遇到什么问题？不清楚的内容写待确认。">${e(v.transcript)}</textarea></label><div class="row"><button data-review-action="voice" ${supported ? "" : "disabled"}>开始口述</button><button data-review-action="stop-voice" disabled>结束口述</button><span data-review-voice role="status">${supported ? "点击后请求麦克风授权，转写可修改" : "此浏览器不支持语音识别，可直接输入或使用系统听写"}</span></div><p class="muted">浏览器可能使用厂商在线服务识别语音，本系统只保存文字；请核对转写中的系统名、件号及数量。</p><div class="row"><button data-review-action="save">保存陈述</button><button data-review-action="manual-summary" ${v.transcript ? "" : "disabled"}>手工整理</button></div><div data-review-message role="status" aria-live="polite"></div>${ready ? `<section class="review-confirmation"><h3>核对整理结果</h3><p class="muted">填写实际已知内容；不确定的部分写待确认。下方问答表可辅助填写。修改后请重新确认，原提交和管理员回复仍会保留。</p><form id="review-confirm-form" class="form-stack">${fields.map(([key, label]) => summaryField(key, label, v)).join("")}<button type="submit" class="primary" data-review-action="confirm">${confirmed ? "重新确认当前内容" : "确认以上内容"}</button></form>${confirmed ? `<p class="notice">内容已确认。点击提交后，管理员将收到这一版内容的快照。</p><button class="primary" data-review-action="submit">提交</button>` : '<p class="muted">先确认内容，再提交管理员。</p>'}</section>` : ""}${tickets.length ? "</details>" : ""}<section class="consultation-receipts"><h3>我的提交与管理员回复</h3><button data-review-action="refresh">刷新处理状态</button>${tickets.length ? tickets.map((t) => window.DiscussionUI.render(t)).join("") : '<p class="muted">尚未提交。整理并确认后，点击“提交”交给管理员。</p>'}</section></section>`;
  }
  async function open(attempt) {
    stop();
    id = attempt;
    const data = await api(endpoint());
    if (!data?.review)
      throw new Error("复盘页面版本不一致，请刷新页面后重试。");
    if (
      !root() ||
      id !== attempt ||
      window.EnterpriseUI.historyContext()?.id !== attempt
    )
      return;
    render(data);
    root().scrollIntoView({ behavior: "smooth", block: "start" });
  }
  async function action(kind, extra = {}) {
    const attempt = id,
      revision = current.revision;
    const buttons = [...root().querySelectorAll("button")];
    const old = buttons.map((b) => b.disabled);
    buttons.forEach((b) => (b.disabled = true));
    const inputs = [...root().querySelectorAll("input, textarea, select")];
    const inputStates = inputs.map((x) => x.disabled);
    inputs.forEach((x) => (x.disabled = true));
    message(kind === "submit" ? "正在提交管理员……" : "正在保存……");
    try {
      const data = await api(endpoint(), { action: kind, revision, ...extra });
      if (
        id !== attempt ||
        !root() ||
        window.EnterpriseUI.historyContext()?.id !== attempt
      )
        return;
      render(data);
      message(
        kind === "submit"
          ? "已提交管理员，请等待处理。可点击刷新查看回复。"
          : kind === "confirm"
            ? "内容已确认，点击提交交给管理员。"
            : "已保存。",
      );
    } catch (err) {
      if (id === attempt) message(err.message, true);
    } finally {
      buttons.forEach((b, i) => {
        if (b.isConnected) b.disabled = old[i];
      });
      inputs.forEach((x, i) => {
        if (x.isConnected) x.disabled = inputStates[i];
      });
    }
  }
  function voice() {
    stop();
    const Speech = window.SpeechRecognition || window.webkitSpeechRecognition;
    if (!Speech) return;
    const box = root().querySelector("#review-transcript"),
      badge = root().querySelector("[data-review-voice]");
    const rec = new Speech();
    recognizer = rec;
    rec.lang = "zh-CN";
    rec.continuous = true;
    rec.interimResults = true;
    rec.onresult = (ev) => {
      if (!box.isConnected) {
        stop();
        return;
      }
      let interim = "";
      for (let i = ev.resultIndex; i < ev.results.length; i++) {
        if (ev.results[i].isFinal) {
          box.value += (box.value ? "\n" : "") + ev.results[i][0].transcript;
          dirty("transcript");
        } else interim += ev.results[i][0].transcript;
      }
      badge.textContent = interim ? "识别中：" + interim : "正在听取，请继续……";
    };
    rec.onerror = (ev) => {
      badge.textContent =
        "语音识别未完成（" +
        ev.error +
        "）。请检查麦克风授权与网络，或使用文字／系统听写。";
    };
    rec.onend = () => {
      if (recognizer === rec) recognizer = null;
      root()
        ?.querySelector("[data-review-action=voice]")
        ?.removeAttribute("disabled");
      root()
        ?.querySelector("[data-review-action=stop-voice]")
        ?.setAttribute("disabled", "");
      if (
        badge.textContent.startsWith("正在") ||
        badge.textContent.startsWith("识别中")
      )
        badge.textContent = "口述已结束，请核对文字并保存。";
    };
    try {
      rec.start();
      badge.textContent = "正在请求麦克风并启动识别……";
      root().querySelector("[data-review-action=voice]").disabled = true;
      root().querySelector("[data-review-action=stop-voice]").disabled = false;
    } catch {
      stop();
      badge.textContent = "无法启动语音识别，请使用文字或系统听写。";
    }
  }
  document.addEventListener("click", (ev) => {
    const b = ev.target.closest("button[data-review-template]");
    if (b && !b.disabled && root()) applyWorksheet(b.dataset.reviewTemplate);
  });
  document.addEventListener("input", (ev) => {
    if (ev.target.dataset.reviewCell) {
      ev.target.closest("details").dataset.pending = "true";
      root().querySelector(
        `[data-review-template="${ev.target.dataset.reviewCell}"]`,
      ).disabled = false;
      dirty("summary");
    }
    if (ev.target.dataset.reviewTemplateMode) {
      root().querySelector(
        `[data-review-template="${ev.target.dataset.reviewTemplateMode}"]`,
      ).disabled = false;
    }
    if (ev.target.id === "review-transcript") dirty("transcript");
    if (ev.target.closest("#review-confirm-form")) dirty("summary");
  });
  document.addEventListener("submit", (ev) => {
    if (ev.target.id !== "review-confirm-form") return;
    ev.preventDefault();
    stop();
    for (const key of Object.keys(worksheetDefinitions)) {
      const sheet = ev.target
        .querySelector(`[data-review-template="${key}"]`)
        .closest("details");
      if (sheet.dataset.pending === "true" && !applyWorksheet(key)) {
        sheet.open = true;
        return;
      }
    }
    action("confirm", { summary: Object.fromEntries(new FormData(ev.target)) });
  });
  document.addEventListener("click", async (ev) => {
    const b = ev.target.closest("button[data-review-action]");
    if (!b || b.disabled || !root()) return;
    const kind = b.dataset.reviewAction;
    if (kind === "confirm") return;
    if (kind === "voice") {
      voice();
      return;
    }
    if (kind === "stop-voice") {
      recognizer?.stop();
      return;
    }
    stop();
    if (kind === "save")
      await action(kind, {
        transcript: root().querySelector("#review-transcript").value,
      });
    else if (kind === "refresh") {
      if (transcriptDirty || summaryDirty) {
        message("有未保存的修改，请先保存或确认后再刷新。", true);
        return;
      }
      await open(id);
    } else await action(kind);
  });
  new MutationObserver(() => {
    if (recognizer && !document.querySelector("#review-transcript")) stop();
  }).observe(document.body, { childList: true, subtree: true });
  return { open, stop };
})();
