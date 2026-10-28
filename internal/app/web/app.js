"use strict";
const $ = (s) => document.querySelector(s),
  app = $("#app");
let user = null,
  classroom = null,
  selected = "P01",
  draft = null,
  selectedStudentPage = "",
  lastRevision = -1,
  polling = false;
const projector = location.pathname === "/projector";
const portal = /^\/(admin|learner)\/?$/.exec(location.pathname)?.[1] || "";
const portalURL = (path) => (portal ? "/" + portal + path : path);
const escapeHTML = (s) =>
  String(s ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );
const e = escapeHTML;
function renderLogin(expired = false) {
  const admin = portal === "admin";
  document.title = (admin ? "管理员" : "学员") + " · 航材采购 AI 实训中心";
  app.hidden = false;
  app.innerHTML = `<section class="login ${admin ? "admin-login" : "learner-login"}"><a class="back-link" href="/">← 返回入口选择</a><p class="eyebrow">${admin ? "TEACHING CONSOLE · 授课管理" : "LEARNING SPACE · 学员课堂"}</p><h1>${admin ? "管理员登录" : "学员登录"}</h1><p class="muted">${admin ? "管理课件、开放实训，掌握学员学习进度。" : "使用讲师分配的学员账号，开始课堂学习与实训。"}</p>${expired ? '<p class="notice" role="status">登录已失效，请重新登录。</p>' : ""}<form id="login-form" method="post" action="${portalURL("/api/login")}"><label>${admin ? "管理员账号" : "学员账号"}<input name="id" autocomplete="username" required maxlength="80" placeholder="${admin ? "admin" : "例如 TG01"}"></label><label>密码<input name="password" type="password" autocomplete="current-password" required maxlength="200"></label><button class="primary" type="submit">${admin ? "登录授课管理" : "进入学员课堂"}</button><p id="login-error" role="alert"></p></form><p class="login-switch">${admin ? '我是学员，<a href="/learner">前往学员入口</a>' : '我是讲师，<a href="/admin">前往管理员入口</a>'}</p></section>`;
  $("#identity").textContent = admin ? "管理员入口" : "学员入口";
}
let projectionToken = "";
let projectionImageURL = "";
if (projector) {
  projectionToken =
    location.hash.slice(1) || sessionStorage.getItem("projectionToken") || "";
  if (projectionToken) {
    sessionStorage.setItem("projectionToken", projectionToken);
    history.replaceState(null, "", "/projector");
  }
  document.body.classList.add("projection-mode");
}
async function api(path, body) {
  const options = {
    method: body === undefined ? "GET" : "POST",
    headers: {},
    cache: "no-store",
  };
  if (body !== undefined) {
    options.headers["Content-Type"] = "application/json";
    options.body = JSON.stringify(body);
  }
  if (projector) {
    options.headers.Authorization = "Bearer " + projectionToken;
    options.credentials = "omit";
  }
  const res = await fetch(portalURL(path), options);
  const result = await res.json();
  if (!res.ok) {
    const err = new Error(result.error || "请求失败");
    err.status = res.status;
    throw err;
  }
  return result;
}
function status(text) {
  $("#status").textContent = text;
}
function slideView(slide) {
  if (!slide)
    return '<div class="waiting"><h1>等待讲师激活课件</h1><p class="muted">页面权限更新后会自动显示。</p></div>';
  if (slide.asset)
    return `<article class="slide-image"><img ${projector ? "" : `src="${portalURL("/api/slide-images/" + encodeURIComponent(slide.id))}"`} data-slide-image="${e(slide.id)}" alt="${e(slide.title)}"></article>`;
  return `<article class="slide"><div class="eyebrow">TAIGU · PROCUREMENT AI WORKSHOP</div><h1>${e(slide.title)}</h1><p>${e(slide.description)}</p><div class="steps">${(Array.isArray(slide.steps) ? slide.steps : []).map((s, i) => `<div class="step"><small>STEP 0${i + 1}</small>${e(s)}</div>`).join("")}</div><footer><span>航材采购 AI 实战工作坊 · 课堂课件</span><span>${e(slide.id)}</span></footer></article>`;
}
const pageUsers = (id) =>
  classroom.learners.filter((l) => l.pages.includes(id)).map((l) => l.id);
function renderAdmin() {
  const slide =
    classroom.slides.find((s) => s.id === selected) || classroom.slides[0];
  selected = slide.id;
  const ids = pageUsers(selected),
    online = classroom.online;
  app.innerHTML = `<div class="shell">${window.Features?.nav("admin", "classroom") || ""}<div class="toolbar"><div><h2>授课控制</h2><span class="muted">在线 ${online.length} / 10 人 · 课堂课件</span></div><div class="row"><span class="tag">实战课堂</span><button data-action="projector-window">打开独立投影窗口</button></div></div><div class="layout"><aside class="slide-list">${classroom.slides
    .map((s) => {
      const n = pageUsers(s.id).length;
      return `<button class="thumb ${s.id === selected ? "selected" : ""}" data-page="${e(s.id)}"><span>${e(s.id)}</span><strong>${e(s.title)}</strong><span class="${n ? "active" : ""}">${n ? "已激活 · " + n + " 人" : classroom.closed.includes(s.id) ? "已关闭" : "未激活"}</span></button>`;
    })
    .join(
      "",
    )}</aside><section>${slideView(slide)}<p class="footnote">选择页面只更新管理员预览，确认操作后才改变学员权限。</p><section class="panel"><div class="toolbar"><h3>${e(selected)} · 页面操作</h3><span class="tag">${ids.length ? ids.length + " 人已激活" : classroom.closed.includes(selected) ? "已关闭" : "未激活"}</span></div><div class="row"><button class="primary" data-action="audience">${ids.length ? "调整激活学员" : classroom.closed.includes(selected) ? "重新激活页面" : "激活页面"}</button><button data-action="modules">开放关联实训</button>${ids.length || classroom.projection === selected ? '<button class="danger" data-action="close-page">关闭此页</button>' : ""}</div><p class="muted">${ids.length ? "当前可查看：" + ids.map(e).join("、") : "尚未向学员开放。默认对确认时的所有在线学员激活。"}</p></section><div class="columns"><section class="panel"><h3>课堂投影</h3><p class="muted">${classroom.projection ? "正在投影 " + e(classroom.projection) : "已暂停 · 等待画面"}</p><div class="row"><button data-action="project">投影当前页</button><button data-action="pause" ${classroom.projection ? "" : "disabled"}>暂停投影</button></div></section><section class="panel"><h3>实训控制</h3><p class="muted">${classroom.learners.filter((l) => l.modules.length).length} 名学员有已开放的模块入口</p><button data-action="end-modules">选择学员并结束实训</button></section></div><p class="footnote">模拟案例供课堂练习。提交后由管理员查看统计和评价；正式课件与试题可在管理页面导入。</p></section></div></div>`;
}
function renderLearner() {
  window.LearnerUI?.dashboard();
}

async function refresh() {
  if (polling) return;
  polling = true;
  try {
    if (projector) {
      const data = await api("/api/projection");
      if (data.revision !== lastRevision) {
        app.innerHTML = `<div class="shell projector">${data.slide ? slideView(data.slide) : '<div class="waiting"><h1>等待讲师开始授课</h1></div>'}</div>`;
        if (projectionImageURL) {
          URL.revokeObjectURL(projectionImageURL);
          projectionImageURL = "";
        }
        if (data.slide?.asset) {
          const image = await fetch(
            "/api/slide-images/" + encodeURIComponent(data.slide.id),
            {
              headers: { Authorization: "Bearer " + projectionToken },
              credentials: "omit",
              cache: "no-store",
            },
          );
          if (!image.ok) throw new Error("投影图片暂不可用");
          projectionImageURL = URL.createObjectURL(await image.blob());
          app.querySelector("[data-slide-image]").src = projectionImageURL;
        }
        lastRevision = data.revision;
      }
      status("");
      return;
    }
    const oldCurrent = user?._lastCurrent;
    user = await api("/api/me");
    if (oldCurrent !== undefined) user._lastCurrent = oldCurrent;
    $("#identity").innerHTML =
      `<span>${e(user.id)} · ${user.role === "admin" ? "管理员" : "学员"}</span><button data-action="logout">退出</button>`;
    if (user.role === "admin") {
      classroom = await api("/api/admin/classroom");
      if (
        !window.Features?.active &&
        !$("#audience-dialog").open &&
        !$("#close-dialog").open
      )
        renderAdmin();
    } else if (
      !window.Features?.active &&
      (user.revision !== lastRevision || !$(".procurement-dashboard"))
    )
      renderLearner();
    window.Features?.sync(user, classroom);
    window.LearnerUI?.sync(user);
    window.Assistant?.sync(user);
    lastRevision = user.revision;
    status("");
  } catch (err) {
    if (err.status === 401) {
      if (user || projector) {
        user = null;
        window.Features?.sync(null, null);
        window.LearnerUI?.sync(null);
        window.Assistant?.sync(null);
        if (portal) renderLogin(true);
        else {
          app.hidden = false;
          app.innerHTML =
            '<div class="shell waiting"><h1>登录已失效</h1><p>请由管理员重新打开投影。</p><a href="/admin">管理员入口</a></div>';
          $("#identity").textContent = "";
        }
      }
    } else status("连接异常：" + err.message + "。恢复后将自动同步。");
  } finally {
    polling = false;
  }
}
function openAudience(mode) {
  const current = pageUsers(selected);
  draft = {
    mode,
    page: selected,
    revision: classroom.revision,
    audience: mode === "page" && current.length ? "custom" : "online",
    ids:
      mode === "page" && current.length ? [...current] : [...classroom.online],
    modules:
      mode === "modules"
        ? [classroom.slides.find((s) => s.id === selected).suggested]
        : [],
    original: current,
  };
  $("#dialog-title").textContent =
    mode === "page"
      ? current.length
        ? "调整激活学员"
        : "激活页面"
      : mode === "modules"
        ? "开放关联实训"
        : "结束学员实训";
  $("#dialog-page").textContent =
    selected + " · " + classroom.slides.find((s) => s.id === selected).title;
  $("#dialog-description").textContent =
    mode === "page"
      ? "保存后按完整勾选名单更新权限，取消勾选会撤销访问。"
      : mode === "modules"
        ? "为所选学员开放模块入口，不改变课件权限。"
        : "结束所选学员的全部已开放实训，保留已保存数据。";
  $("#module-details").hidden = mode === "end";
  $("#module-details").open = mode === "modules";
  $("#dialog-error").textContent = "";
  renderDraft();
  $("#audience-dialog").showModal();
}
function renderDraft() {
  document
    .querySelectorAll('[name="audience"]')
    .forEach((x) => (x.checked = x.value === draft.audience));
  $("#learner-grid").innerHTML = classroom.learners
    .map(
      (l) =>
        `<label class="learner"><input type="checkbox" data-learner="${e(l.id)}" ${draft.ids.includes(l.id) ? "checked" : ""}><strong>${e(l.id)}${l.name ? " · " + e(l.name) : ""}</strong><span class="${classroom.online.includes(l.id) ? "online" : ""}">${classroom.online.includes(l.id) ? "在线" : "离线"}</span></label>`,
    )
    .join("");
  $("#module-grid").innerHTML = classroom.modules
    .map(
      (m) =>
        `<label class="module-choice"><input type="checkbox" data-choice="${e(m.id)}" ${draft.modules.includes(m.id) ? "checked" : ""}> ${e(m.title)}</label>`,
    )
    .join("");
  const added = draft.ids.filter((x) => !draft.original.includes(x)).length,
    removed = draft.original.filter((x) => !draft.ids.includes(x)).length;
  $("#selection-count").textContent = `已选 ${draft.ids.length} 人`;
  $("#change-summary").textContent =
    draft.mode === "page"
      ? `当前 ${draft.original.length} 人 → 保存后 ${draft.ids.length} 人；新增 ${added} 人、移除 ${removed} 人。${!draft.ids.length ? "保存后所有学员均不可查看此页。" : ""}`
      : `对 ${draft.ids.length} 人${draft.mode === "end" ? "结束实训" : "开放 " + draft.modules.length + " 个模块"}。`;
  $("#confirm-audience").textContent =
    draft.mode === "page"
      ? draft.original.length
        ? "保存学员权限"
        : "确认激活"
      : draft.mode === "end"
        ? "结束实训"
        : "确认开放";
  $("#confirm-audience").disabled =
    draft.mode === "page"
      ? !draft.ids.length && !draft.original.length
      : !draft.ids.length ||
        (draft.mode === "modules" && !draft.modules.length);
}
function cancelDraft() {
  $("#audience-dialog").close();
  draft = null;
  renderAdmin();
}
async function action(kind, extra = {}) {
  const data = await api("/api/admin/actions", {
    kind,
    revision: classroom.revision,
    ...extra,
  });
  classroom = data;
  renderAdmin();
  status("操作已保存");
}
document.addEventListener("submit", async (ev) => {
  if (ev.target.getAttribute("id") === "login-form") {
    ev.preventDefault();
    const form = new FormData(ev.target);
    const button = ev.target.querySelector("button");
    button.disabled = true;
    try {
      await api("/api/login", {
        id: form.get("id"),
        password: form.get("password"),
      });
      lastRevision = -1;
      await refresh();
    } catch (err) {
      $("#login-error").textContent = err.message;
    } finally {
      button.disabled = false;
    }
    return;
  }
  if (ev.target.getAttribute("id") === "audience-form") {
    ev.preventDefault();
    const button = $("#confirm-audience");
    button.disabled = true;
    try {
      const data = await api("/api/admin/actions", {
        kind:
          draft.mode === "page"
            ? "page-grants"
            : draft.mode === "end"
              ? "modules-close"
              : "modules-open",
        revision: draft.revision,
        page: draft.page,
        learners: draft.ids,
        modules: draft.modules,
        onlineOnly: draft.audience === "online",
      });
      classroom = data;
      $("#audience-dialog").close();
      draft = null;
      renderAdmin();
      status("学员权限已保存");
    } catch (err) {
      $("#dialog-error").textContent = err.message;
    } finally {
      button.disabled = false;
    }
  }
});
document.addEventListener("click", async (ev) => {
  const button = ev.target.closest("button");
  if (!button || button.disabled) return;
  if (button.dataset.page) {
    selected = button.dataset.page;
    renderAdmin();
    return;
  }
  if (button.dataset.module) {
    window.Features?.training(button.dataset.module);
    return;
  }
  const command = button.dataset.action;
  try {
    if (command === "logout") {
      await api("/api/logout", {});
      location.reload();
    }
    if (command === "audience") openAudience("page");
    if (command === "modules") openAudience("modules");
    if (command === "end-modules") openAudience("end");
    if (command === "cancel-dialog") cancelDraft();
    if (command === "select-all") {
      draft.audience = "custom";
      draft.ids = classroom.learners.map((l) => l.id);
      renderDraft();
    }
    if (command === "select-none") {
      draft.audience = "custom";
      draft.ids = [];
      renderDraft();
    }
    if (command === "close-page") {
      $("#close-description").textContent =
        selected +
        " · " +
        classroom.slides.find((s) => s.id === selected).title;
      $("#close-dialog").dataset.page = selected;
      $("#close-dialog").dataset.revision = classroom.revision;
      $("#close-error").textContent = "";
      $("#close-dialog").showModal();
    }
    if (command === "cancel-close") $("#close-dialog").close();
    if (command === "confirm-close") {
      try {
        classroom = await api("/api/admin/actions", {
          kind: "page-close",
          revision: Number($("#close-dialog").dataset.revision),
          page: $("#close-dialog").dataset.page,
        });
        $("#close-dialog").close();
        renderAdmin();
      } catch (err) {
        $("#close-error").textContent = err.message;
      }
    }
    if (command === "project") await action("project", { page: selected });
    if (command === "pause") await action("pause");
    if (command === "projector-window") {
      const popup = window.open("about:blank", "taigu-projector");
      if (!popup) {
        status("请允许打开投影窗口");
        return;
      }
      try {
        const result = await api("/api/admin/projection-token", {});
        popup.location.href = "/projector#" + result.token;
      } catch (err) {
        popup.close();
        throw err;
      }
    }
  } catch (err) {
    status(err.message);
  }
});
document.addEventListener("change", (ev) => {
  const t = ev.target;
  if (t.id === "student-page") {
    selectedStudentPage = t.value;
    renderLearner();
  }
  if (!draft) return;
  if (t.name === "audience") {
    draft.audience = t.value;
    if (t.value === "online") draft.ids = [...classroom.online];
    renderDraft();
  }
  if (t.dataset.learner) {
    draft.audience = "custom";
    draft.ids = t.checked
      ? [...new Set([...draft.ids, t.dataset.learner])]
      : draft.ids.filter((x) => x !== t.dataset.learner);
    renderDraft();
  }
  if (t.dataset.choice) {
    draft.modules = t.checked
      ? [...draft.modules, t.dataset.choice]
      : draft.modules.filter((x) => x !== t.dataset.choice);
    renderDraft();
  }
});
$("#audience-dialog").addEventListener("cancel", (ev) => {
  ev.preventDefault();
  cancelDraft();
});
if (portal || projector) {
  if (portal) renderLogin();
  refresh();
  setInterval(refresh, 1000);
}
