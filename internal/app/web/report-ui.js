"use strict";
window.ReportUI = (() => {
  const arr = (x) => (Array.isArray(x) ? x : []);
  const safeID = (x) => String(x || "").replace(/[^a-zA-Z0-9_-]/g, "-");
  function chart(c) {
    const points = arr(c.points).filter(
        (p) => Number.isFinite(p.value) && p.value >= 0,
      ),
      max = Math.max(1, ...points.map((p) => p.value)),
      height = points.length * 46 + 28;
    return `<figure class="report-chart"><figcaption><h4>${e(c.title)}</h4><p class="muted">${e(c.note)}</p></figcaption><svg viewBox="0 0 700 ${height}" role="img" aria-label="${e(c.title)}"><title>${e(c.title)}</title>${points.map((p, i) => `<text x="5" y="${i * 46 + 29}" class="report-chart-label">${e(p.label)}</text><rect x="235" y="${i * 46 + 10}" width="${(p.value / max) * 350}" height="24" rx="4" class="${i % 2 ? "report-bar-goal" : "report-bar"}"></rect><text x="${242 + (p.value / max) * 350}" y="${i * 46 + 28}" class="report-chart-value">${e(p.value)}</text>`).join("")}</svg><div class="table-scroll"><table><thead><tr><th>项目</th><th>数值（${e(c.unit)}）</th></tr></thead><tbody>${points.map((p) => `<tr><td>${e(p.label)}</td><td>${e(p.value)}</td></tr>`).join("")}</tbody></table></div><details><summary>查看图表依据</summary><p class="pre-wrap muted">${e(c.basis)}</p></details></figure>`;
  }
  function render(
    report,
    {
      ticketId = "",
      replyId = "preview",
      interactive = false,
      editable = false,
    } = {},
  ) {
    if (!report) return "";
    const prefix = "report-" + safeID(ticketId) + "-" + safeID(replyId);
    const sections = arr(report.sections);
    return `<article class="structured-report" ${editable ? 'data-report-editor="true"' : ""}><header class="report-heading"><span class="eyebrow">结构化复盘报告</span><h3>${e(report.title)}</h3><p class="muted">依据原答复的标题和栏目整理；图表均标明数据性质，未补造企业绩效或结论。</p></header>${arr(
      report.warnings,
    )
      .map((w) => `<p class="notice">${e(w)}</p>`)
      .join(
        "",
      )}<nav class="report-toc" aria-label="报告目录">${sections.map((s) => `<a href="#${prefix}-${safeID(s.id)}" data-report-jump="${prefix}-${safeID(s.id)}">${e(s.title)}</a>`).join("")}</nav>${arr(
      report.flows,
    )
      .map(
        (f) =>
          `<figure class="report-flow"><figcaption><h4>${e(f.title)}</h4><p class="muted">${e(f.note)}</p></figcaption><ol>${arr(
            f.nodes,
          )
            .map((n, i) => `<li><span>${i + 1}</span><div>${e(n)}</div></li>`)
            .join("")}</ol></figure>`,
      )
      .join(
        "",
      )}<div class="report-chart-grid">${arr(report.charts).map(chart).join("")}</div>${sections
      .map(
        (s, i) =>
          `<details class="report-section" id="${prefix}-${safeID(s.id)}" ${i === 0 || i === sections.length - 1 ? "open" : ""}><summary>${e(s.title)}</summary>${
            editable
              ? `<button type="button" data-report-edit-toggle>编辑本部分</button><div class="report-edit-area form-stack" hidden><label>本部分标题<input data-report-edit-title value="${e(s.title)}" maxlength="100"></label>${arr(
                  s.blocks,
                )
                  .map(
                    (b) =>
                      `<div class="report-edit-block"><label>栏目名称<input data-report-edit-label value="${e(b.label)}" maxlength="80"></label><label>内容<textarea data-report-edit-text rows="6">${e(b.text)}</textarea></label></div>`,
                  )
                  .join(
                    "",
                  )}<div class="row"><button type="button" data-report-edit-done>完成本部分编辑</button><button type="button" data-report-edit-cancel>取消本部分修改</button></div></div>`
              : ""
          }<div class="report-block-grid">${arr(s.blocks)
            .map(
              (b) =>
                `<section class="report-block report-kind-${e(b.kind)}"><h4>${e(b.label)}</h4><div class="pre-wrap">${e(b.text)}</div></section>`,
            )
            .join(
              "",
            )}</div>${interactive ? `<button type="button" data-discussion-topic="${e(ticketId)}" data-discussion-context="${e(s.title)}">就这一部分追问管理员</button>` : ""}</details>`,
      )
      .join(
        "",
      )}<details class="report-original"><summary>对照完整原文</summary><p class="pre-wrap">${e(report.raw)}</p></details></article>`;
  }
  function editorText(root) {
    return [...root.querySelectorAll(".report-section")]
      .map((section) => {
        const title = section
          .querySelector("[data-report-edit-title]")
          .value.trim();
        const blocks = [...section.querySelectorAll(".report-edit-block")].map(
          (b) => {
            const label = b
                .querySelector("[data-report-edit-label]")
                .value.trim(),
              text = b.querySelector("[data-report-edit-text]").value.trim();
            return (
              (label && label !== "说明" ? "【" + label + "】" : "") + text
            );
          },
        );
        // Ensure even a user-rewritten heading remains recognizable on reparse.
        return (
          (/^[一二三四五六七八九十百]+[、．]|^#/.test(title)
            ? title
            : "## " + title) +
          "\n\n" +
          blocks.join("\n\n")
        );
      })
      .join("\n\n");
  }
  document.addEventListener("click", (ev) => {
    const b = ev.target.closest(
      "[data-report-edit-toggle],[data-report-edit-done],[data-report-edit-cancel]",
    );
    if (!b) return;
    const section = b.closest(".report-section"),
      area = section.querySelector(".report-edit-area"),
      grid = section.querySelector(".report-block-grid");
    if (b.hasAttribute("data-report-edit-toggle")) {
      area.hidden = false;
      grid.hidden = true;
      area.querySelector("input").focus();
      return;
    }
    if (b.hasAttribute("data-report-edit-cancel")) {
      area
        .querySelectorAll("input,textarea")
        .forEach((n) => (n.value = n.defaultValue));
    } else {
      section.querySelector("summary").textContent = area.querySelector(
        "[data-report-edit-title]",
      ).value;
      const blocks = [...area.querySelectorAll(".report-edit-block")];
      grid.innerHTML = blocks
        .map(
          (n) =>
            `<section class="report-block"><h4>${e(n.querySelector("[data-report-edit-label]").value)}</h4><div class="pre-wrap">${e(n.querySelector("textarea").value)}</div></section>`,
        )
        .join("");
      area
        .querySelectorAll("input,textarea")
        .forEach((n) => (n.defaultValue = n.value));
      section
        .closest("[data-report-editor]")
        .dispatchEvent(
          new CustomEvent("report-editor-changed", { bubbles: true }),
        );
    }
    area.hidden = true;
    grid.hidden = false;
  });
  document.addEventListener("click", (ev) => {
    const a = ev.target.closest("[data-report-jump]");
    if (!a) return;
    const n = document.getElementById(a.dataset.reportJump);
    if (n) {
      ev.preventDefault();
      n.open = true;
      n.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  });
  return { render, editorText };
})();
window.DiscussionUI = (() => {
  const arr = (x) => (Array.isArray(x) ? x : []);
  const date = (x) => new Date(x).toLocaleString("zh-CN");
  const stored = new Map();
  function render(t) {
    const replies = arr(t.replies),
      questions = arr(t.questions),
      saved = stored.get(t.id) || { text: "", context: "" };
    const answer = (r) => {
      const content = `<p class="muted">管理员回复 · ${e(date(r.at))} · ${e(r.source)}</p>${r.report ? window.ReportUI.render(r.report, { ticketId: t.id, replyId: r.id, interactive: true }) : `<p class="pre-wrap">${e(r.text)}</p>`}`;
      return `<section class="discussion-answer" data-review-analysis>${window.ReplyPrivacy.wrap(content, { reference: t.module + " / 回复 " + (r.id || "历史记录") })}</section>`;
    };
    return `<section class="consultation-thread" data-consultation-receipt="${e(t.id)}"><p><span class="tag">${e({ submitted: "待管理员处理", processing: "管理员处理中", returned: "已回复" }[t.status] || t.status)}</span> · ${e(date(t.submittedAt))}</p><div class="discussion-question"><h4>最初提交的问题</h4><p class="pre-wrap">${e(t.summary?.problems || "待确认")}</p></div>${replies
      .filter((r) => !r.questionId)
      .map(answer)
      .join("")}${questions
      .map((q, i) => {
        const answers = replies.filter((r) => r.questionId === q.id);
        return `<section class="discussion-round"><div class="discussion-question"><span class="tag">追问 ${i + 1}</span><small>${e(date(q.at))}</small><p class="muted">关联内容：${e(q.context || "整个答复")}</p><p class="pre-wrap">${e(q.text)}</p>${answers.length ? "" : '<p class="notice">已发送，等待管理员回复；可以继续补充问题。</p>'}</div>${answers.map(answer).join("")}</section>`;
      })
      .join(
        "",
      )}${replies.length ? `<form class="discussion-form form-stack" data-discussion-form="${e(t.id)}"><h4>继续向管理员提问</h4><p class="muted">可以连续追问、补充事实或讨论实施细节。每次追问保留记录，不限制问答轮次；管理员处理后在这里返回。</p><label>关联内容（可修改）<input name="context" maxlength="200" value="${e(saved.context)}" placeholder="例如：数据校验、试点指标或整个答复"></label><label>想进一步了解的问题<textarea name="text" rows="4" minlength="2" maxlength="2000" required placeholder="请说清想澄清什么、实际情况有什么补充。">${e(saved.text)}</textarea></label><button class="primary" type="submit">发送追问给管理员</button><p data-discussion-status role="status" aria-live="polite"></p></form>` : '<p class="muted">管理员首次回复后，可以在此继续多轮讨论。</p>'}</section>`;
  }
  document.addEventListener("input", (ev) => {
    const form = ev.target.closest("[data-discussion-form]");
    if (form) {
      const old = stored.get(form.dataset.discussionForm) || {};
      stored.set(form.dataset.discussionForm, {
        ...old,
        text: form.elements.text.value,
        context: form.elements.context.value,
      });
    }
  });
  document.addEventListener("click", (ev) => {
    const b = ev.target.closest("[data-discussion-topic]");
    if (!b) return;
    const form = [...document.querySelectorAll("[data-discussion-form]")].find(
      (f) => f.dataset.discussionForm === b.dataset.discussionTopic,
    );
    if (form) {
      form.elements.context.value = b.dataset.discussionContext;
      const saved = stored.get(b.dataset.discussionTopic) || { text: "" };
      stored.set(b.dataset.discussionTopic, {
        ...saved,
        context: b.dataset.discussionContext,
      });
      form.scrollIntoView({ behavior: "smooth", block: "center" });
      form.elements.text.focus();
    }
  });
  document.addEventListener("submit", async (ev) => {
    const form = ev.target.closest("[data-discussion-form]");
    if (!form) return;
    ev.preventDefault();
    const id = form.dataset.discussionForm,
      button = form.querySelector("button[type=submit]"),
      status = form.querySelector("[data-discussion-status]");
    const saved = stored.get(id) || {};
    const text = form.elements.text.value,
      context = form.elements.context.value;
    if (
      saved.sentText !== text ||
      saved.sentContext !== context ||
      !saved.clientId
    ) {
      saved.clientId = crypto.randomUUID();
      saved.sentText = text;
      saved.sentContext = context;
    }
    stored.set(id, { ...saved, text, context });
    button.disabled = true;
    status.textContent = "正在发送追问……";
    try {
      const ticket = await api(
        "/api/consultations/" + encodeURIComponent(id) + "/questions",
        { text, context, clientId: saved.clientId },
      );
      stored.delete(id);
      window.EnterpriseUI?.refreshReviewStatus().catch(() => {});
      const thread = form.closest(".consultation-thread");
      if (thread?.isConnected) {
        thread.outerHTML = render(ticket);
        const node = [
          ...document.querySelectorAll("[data-discussion-form]"),
        ].find((f) => f.dataset.discussionForm === id);
        if (node)
          node.querySelector("[data-discussion-status]").textContent =
            "追问已发送，等待管理员回复；可以继续补充。";
      }
    } catch (err) {
      status.textContent = err.message;
      button.disabled = false;
    }
  });
  return {
    render,
    clear() {
      stored.clear();
    },
  };
})();
