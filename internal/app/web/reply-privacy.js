"use strict";
window.ReplyPrivacy = (() => {
  const selector = ".protected-reply";
  const visible = () =>
    [...document.querySelectorAll(selector)].filter(
      (n) => n.getClientRects().length > 0,
    );
  function wrap(content, { reference = "管理员回复" } = {}) {
    if (user?.role !== "learner") return content;
    const stamp = new Date().toLocaleString("zh-CN", {
      timeZone: "Asia/Shanghai",
      hour12: false,
    });
    const mark = (user?.id || "学员") + " · " + stamp;
    return `<section class="protected-reply" data-protected-reply><p class="reply-privacy-notice">仅供本人在线学习，请勿截图、录屏或传播。页面已加学员水印；网页无法保证阻止系统截图。</p><div class="reply-protected-content"><div class="reply-watermark" aria-hidden="true">${Array.from({ length: 36 }, () => `<span>${e(mark)}</span>`).join("")}</div><div class="reply-readable">${content}</div><p class="reply-trace-label">${e(user.id)} · ${e(reference)} · ${e(stamp)}</p></div><div class="reply-privacy-mask" role="status"><div><strong>回复内容已暂时隐藏</strong><p>切换窗口或离开页面时自动隐藏。返回后点击继续查看。</p><button type="button" data-reply-resume>继续查看回复</button></div></div><p class="reply-privacy-feedback" role="status" aria-live="polite"></p></section>`;
  }
  function sync() {
    document.body.classList.toggle(
      "learner-reply-reading",
      user?.role === "learner" && visible().length > 0,
    );
  }
  function conceal() {
    if (user?.role !== "learner") return;
    visible().forEach((n) => n.classList.add("reply-concealed"));
    sync();
  }
  function protectedTarget(ev) {
    const node =
      ev.target instanceof Element ? ev.target : ev.target?.parentElement;
    return node?.closest(selector);
  }
  function selectionProtected() {
    const sel = window.getSelection();
    if (!sel) return false;
    for (let i = 0; i < sel.rangeCount; i++) {
      const range = sel.getRangeAt(i);
      if (visible().some((n) => range.intersectsNode(n))) return true;
    }
    return false;
  }

  function notice(node, text) {
    const status = node?.querySelector(".reply-privacy-feedback");
    if (status) status.textContent = text;
  }
  document.addEventListener("click", (ev) => {
    const b = ev.target.closest?.("[data-reply-resume]");
    if (b) {
      b.closest(selector).classList.remove("reply-concealed");
      sync();
    }
  });
  ["copy", "cut", "contextmenu", "dragstart", "selectstart"].forEach((type) =>
    document.addEventListener(
      type,
      (ev) => {
        if (user?.role !== "learner") return;
        const node = protectedTarget(ev);
        if (
          !node &&
          ev.target instanceof Element &&
          ev.target.closest("input,textarea,[contenteditable=true]")
        )
          return;
        if (
          node ||
          ((type === "copy" || type === "cut") && selectionProtected())
        ) {
          ev.preventDefault();
          notice(node || visible()[0], "回复仅供在线学习，已限制复制和导出。");
        }
      },
      true,
    ),
  );
  window.addEventListener("blur", conceal);
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) conceal();
  });
  document.addEventListener(
    "keydown",
    (ev) => {
      if (user?.role !== "learner" || !visible().length) return;
      const key = ev.key.toLowerCase();
      if ((ev.ctrlKey || ev.metaKey) && ["p", "s"].includes(key)) {
        ev.preventDefault();
        notice(visible()[0], "回复仅供在线学习，已限制打印及页面保存。");
      }
      if (
        key === "printscreen" ||
        (ev.metaKey && ev.shiftKey && ["3", "4", "5", "s"].includes(key))
      ) {
        ev.preventDefault();
        conceal();
      }
    },
    true,
  );
  window.addEventListener("beforeprint", sync);
  new MutationObserver(sync).observe(document.body, {
    childList: true,
    subtree: true,
  });
  return { wrap, conceal };
})();
