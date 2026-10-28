"use strict";
window.Assistant = (() => {
  const root = document.querySelector("#ai-assistant");
  let identity = "",
    expanded = false,
    busy = false,
    messages = [],
    generation = 0;
  let suppressLauncherClickUntil = 0,
    renderedScope = "";
  const visibleMessages = () =>
    messages.filter(
      (m) =>
        !user?.examActive &&
        (!m.scope || m.scope === (context().historyAttempt || "")),
    );
  function place(element, left, top) {
    const rect = element.getBoundingClientRect();
    element.style.left =
      Math.max(8, Math.min(left, innerWidth - rect.width - 8)) + "px";
    element.style.top =
      Math.max(8, Math.min(top, innerHeight - rect.height - 8)) + "px";
    element.style.right = "auto";
    element.style.bottom = "auto";
  }
  function fitPanel() {
    const panel = root.querySelector("#assistant-panel"),
      launcher = root.querySelector("#assistant-launcher");
    if (!panel || panel.hidden) return;
    const rect = panel.getBoundingClientRect(),
      anchor = launcher.getBoundingClientRect();
    place(
      panel,
      panel.style.left ? rect.left : anchor.right - rect.width,
      panel.style.top ? rect.top : anchor.top - rect.height - 12,
    );
  }
  function resetPosition() {
    const launcher = root.querySelector("#assistant-launcher"),
      panel = root.querySelector("#assistant-panel");
    launcher.style.cssText = "";
    panel.style.cssText = "";
    fitPanel();
  }
  function wireDragging() {
    const bind = (element, handle, isLauncher = false) => {
      let drag = null;
      handle.addEventListener("pointerdown", (ev) => {
        if (
          ev.button !== 0 ||
          (!isLauncher && ev.target.closest("button,input,textarea,a"))
        )
          return;
        const r = element.getBoundingClientRect();
        drag = {
          id: ev.pointerId,
          x: ev.clientX,
          y: ev.clientY,
          left: r.left,
          top: r.top,
          moved: false,
        };
        handle.setPointerCapture(ev.pointerId);
      });
      handle.addEventListener("pointermove", (ev) => {
        if (!drag || drag.id !== ev.pointerId) return;
        const dx = ev.clientX - drag.x,
          dy = ev.clientY - drag.y;
        if (Math.hypot(dx, dy) > 5) drag.moved = true;
        if (drag.moved) {
          ev.preventDefault();
          place(element, drag.left + dx, drag.top + dy);
        }
      });
      const end = () => {
        if (drag?.moved && isLauncher)
          suppressLauncherClickUntil = Date.now() + 350;
        drag = null;
      };
      handle.addEventListener("pointerup", end);
      handle.addEventListener("pointercancel", end);
      handle.addEventListener("keydown", (ev) => {
        if (
          !["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"].includes(
            ev.key,
          ) ||
          ev.target !== handle
        )
          return;
        ev.preventDefault();
        const r = element.getBoundingClientRect(),
          n = ev.shiftKey ? 30 : 10;
        place(
          element,
          r.left +
            (ev.key === "ArrowLeft" ? -n : ev.key === "ArrowRight" ? n : 0),
          r.top + (ev.key === "ArrowUp" ? -n : ev.key === "ArrowDown" ? n : 0),
        );
      });
    };
    const launcher = root.querySelector("#assistant-launcher"),
      panel = root.querySelector("#assistant-panel"),
      head = panel.querySelector(".assistant-head");
    launcher.title = "点击答疑；拖动可移动助手入口";
    head.tabIndex = 0;
    head.setAttribute("aria-label", "拖动对话框标题栏，或使用方向键移动");
    bind(launcher, launcher, true);
    bind(panel, head);
  }
  root.addEventListener(
    "click",
    (ev) => {
      if (
        Date.now() < suppressLauncherClickUntil &&
        ev.target.closest("#assistant-launcher")
      ) {
        ev.preventDefault();
        ev.stopImmediatePropagation();
      }
    },
    true,
  );
  window.addEventListener("resize", () => {
    const launcher = root.querySelector("#assistant-launcher");
    if (launcher) {
      const r = launcher.getBoundingClientRect();
      place(launcher, r.left, r.top);
    }
    fitPanel();
  });
  const context = () => ({
    module: window.Features?.context.module || "",
    historyAttempt: window.Features?.context.historyAttempt || "",
    slide: window.Features?.context.historyAttempt
      ? ""
      : user?.role === "admin"
        ? selected
        : window.LearnerUI?.context.slide || user?.learner?.current || "",
  });
  function label() {
    const c = context();
    if (c.historyAttempt) {
      const h = window.EnterpriseUI.historyContext();
      return "历史复盘 · " + h.module + " 第" + h.number + "次";
    }
    if (c.module)
      return (
        (user.catalog || []).find((m) => m.id === c.module)?.title || c.module
      );
    const pages = user.role === "admin" ? classroom?.slides : user.slides;
    const p = pages?.find((p) => p.id === c.slide);
    return p ? "课件 · " + p.title : "工作台操作与采购术语";
  }
  function init() {
    root.innerHTML = `<button id="assistant-launcher" aria-expanded="false" aria-controls="assistant-panel"><span class="assistant-tool-icon" aria-hidden="true">▤</span><span>采购助手</span></button><section id="assistant-panel" hidden aria-label="采购AI助手"><div class="assistant-head"><div><span class="assistant-avatar" aria-hidden="true">问</span><div><h2>采购助手</h2><span id="assistant-mode"></span></div></div><div class="assistant-window-actions"><button data-assistant="reset-position" title="将助手和对话框移回默认位置">归位</button><button data-assistant="close" aria-label="收起助手">×</button></div></div><div class="assistant-context"><span>当前上下文</span><strong id="assistant-context"></strong></div><div id="assistant-messages" role="log" aria-live="polite" aria-relevant="additions"></div><div class="assistant-quick"><button data-assistant-question="当前页面如何操作？">操作指引</button><button data-assistant-question="ROP是什么意思？">名词解释</button><button data-assistant-question="请讲解当前PPT课件。">讲解课件</button></div><p id="assistant-error" role="status"></p><form id="assistant-form"><label class="sr-only" for="assistant-question">向采购AI助手提问</label><textarea id="assistant-question" rows="2" maxlength="2000" placeholder="询问操作、术语或课件内容…"></textarea><div class="assistant-compose-footer"><button type="button" data-assistant="clear">清空对话</button><button class="primary" type="submit">发送</button></div></form><p class="assistant-footnote">仅参考你有权限的课件；结业考试中暂停答疑。</p></section>`;
    wireDragging();
    drawMessages();
    updateContext();
  }
  function drawMessages() {
    const list = root.querySelector("#assistant-messages");
    if (!list) return;
    const visible = visibleMessages();
    renderedScope =
      (context().historyAttempt || "") + ":" + Boolean(user?.examActive);
    list.innerHTML = visible.length
      ? visible
          .map(
            (m) =>
              `<div class="chat-message ${m.role}"><span class="chat-role">${m.role === "user" ? "你" : "采购助手"}</span><div class="chat-bubble">${m.protectedReply && window.ReplyPrivacy ? window.ReplyPrivacy.wrap(e(m.content), { reference: "助手转述管理员回复" }) : e(m.content)}</div>${m.mode ? `<small class="chat-mode">${m.mode === "api" ? "在线模型" : "本地知识库"}</small>` : ""}${m.sources?.length ? `<div class="chat-sources">${m.sources.map((s) => `<button data-assistant-source="${e(s.id)}" title="${e(s.title)}">课件 ${e(s.id)} ↗</button>`).join("")}</div>` : ""}</div>`,
          )
          .join("")
      : '<div class="assistant-welcome"><span class="assistant-avatar">问</span><h3>让采购操作更清楚</h3><p>我可以说明当前页面怎么操作，解释航材采购术语，或结合已开放的课件帮助你理解课程。</p></div>';
    if (busy)
      list.insertAdjacentHTML(
        "beforeend",
        '<p class="assistant-thinking" role="status">正在整理答复…</p>',
      );
    list.scrollTop = list.scrollHeight;
  }
  function updateContext() {
    if (!user || root.hidden || !root.querySelector("#assistant-context"))
      return;
    root.querySelector("#assistant-context").textContent = label();
    if (
      renderedScope !==
      (context().historyAttempt || "") + ":" + Boolean(user.examActive)
    )
      drawMessages();
    root.querySelector("#assistant-mode").textContent =
      user.aiMode === "api"
        ? "在线模型 · 结合当前页面"
        : "本地知识库 · 未连接在线模型";
    const paused = !!user.examActive;
    root.querySelector("#assistant-question").disabled = paused;
    root.querySelector("#assistant-question").placeholder = paused
      ? "结业考试进行中，交卷后恢复答疑"
      : "询问操作、术语或课件内容…";
    root.querySelector('button[type="submit"]').disabled = busy || paused;
    root
      .querySelectorAll("[data-assistant-question]")
      .forEach((b) => (b.disabled = busy || paused));
    if (paused)
      root.querySelector("#assistant-error").textContent =
        "结业考试进行中，AI助手暂停答疑。";
    else if (
      root.querySelector("#assistant-error").textContent ===
      "结业考试进行中，AI助手暂停答疑。"
    )
      root.querySelector("#assistant-error").textContent = "";
  }
  function toggle(open) {
    expanded = open;
    root.querySelector("#assistant-panel").hidden = !open;
    root
      .querySelector("#assistant-launcher")
      .setAttribute("aria-expanded", String(open));
    if (open) {
      fitPanel();
      updateContext();
      root.querySelector("#assistant-question").focus();
    }
  }
  async function ask(question) {
    if (busy || !user || user.examActive) return;
    const text = question.trim();
    if (!text) return;
    const sequence = generation,
      requestContext = context();
    const history = visibleMessages()
      .slice(-8)
      .map((m) => ({ role: m.role, content: m.content.slice(0, 3000) }));
    messages.push({
      role: "user",
      content: text,
      scope: requestContext.historyAttempt,
    });
    busy = true;
    root.querySelector("#assistant-question").value = "";
    root.querySelector("#assistant-error").textContent = "";
    drawMessages();
    updateContext();
    try {
      await window.Features?.assistantTiming(true);
      const result = await api("/api/ai/chat", {
        message: text,
        ...requestContext,
        history,
      });
      if (sequence !== generation) return;
      messages.push({
        role: "assistant",
        scope: requestContext.historyAttempt,
        content: result.text,
        protectedReply: !!result.protectedReply,
        mode: result.mode,
        sources: result.sources || [],
      });
      if (messages.length > 24) messages = messages.slice(-24);
    } catch (err) {
      if (sequence === generation) {
        messages.pop();
        root.querySelector("#assistant-question").value = text;
        root.querySelector("#assistant-error").textContent = err.message;
      }
    } finally {
      try {
        await window.Features?.assistantTiming(false);
      } catch {
        /* A closed attempt/session is handled by the normal synchronization. */
      }
      if (sequence === generation) {
        busy = false;
        drawMessages();
        updateContext();
      }
    }
  }
  function sync(next) {
    const key = next ? `${next.role}:${next.id}` : "";
    if (!next || next.role === "projector") {
      generation++;
      identity = "";
      expanded = false;
      messages = [];
      busy = false;
      root.hidden = true;
      root.replaceChildren();
      return;
    }
    if (key !== identity) {
      generation++;
      identity = key;
      expanded = false;
      messages = [];
      busy = false;
      root.hidden = false;
      init();
    }
    updateContext();
  }
  document.addEventListener("click", async (ev) => {
    const button = ev.target.closest("button");
    if (!button || button.disabled) return;
    if (button.id === "assistant-launcher") toggle(!expanded);
    if (button.dataset.assistant === "close") toggle(false);
    if (button.dataset.assistant === "reset-position") resetPosition();
    if (button.dataset.assistant === "clear" && !busy) {
      messages = [];
      drawMessages();
      root.querySelector("#assistant-error").textContent = "";
    }
    if (button.dataset.assistantQuestion) ask(button.dataset.assistantQuestion);
    if (button.dataset.assistantSource) {
      if (user.role === "learner") {
        if (!user.slides.some((p) => p.id === button.dataset.assistantSource)) {
          root.querySelector("#assistant-error").textContent =
            "这页课件已关闭，请等待讲师重新开放。";
          return;
        }
        await window.LearnerUI.selectLesson(button.dataset.assistantSource);
      } else {
        selected = button.dataset.assistantSource;
        await window.Features.show("classroom");
      }
      updateContext();
    }
  });
  document.addEventListener("submit", (ev) => {
    if (ev.target.getAttribute("id") === "assistant-form") {
      ev.preventDefault();
      ask(root.querySelector("#assistant-question").value);
    }
  });
  document.addEventListener("keydown", (ev) => {
    if (ev.key === "Escape" && expanded) toggle(false);
    if (
      ev.target.id === "assistant-question" &&
      ev.key === "Enter" &&
      !ev.shiftKey &&
      !ev.isComposing
    ) {
      ev.preventDefault();
      root.querySelector("#assistant-form").requestSubmit();
    }
  });
  return { sync, updateContext };
})();
