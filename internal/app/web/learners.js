"use strict";
window.Learners = (() => {
  const dialog = document.querySelector("#account-dialog");
  let data = null;
  const selected = new Set();
  const notice = (text, error = false) => {
    const node = document.querySelector("#feature-message");
    if (node) {
      node.textContent = text;
      node.classList.toggle("error-text", error);
    }
  };
  async function show() {
    data = await api("/api/admin/learners");
    for (const id of selected)
      if (!data.learners.some((l) => l.id === id)) selected.delete(id);
    const active = data.learners.filter((l) => !l.disabled).length;
    document.querySelector("#workspace").innerHTML =
      `<div class="shell">${window.Features.nav("admin", "learners")}<div class="toolbar"><div><h1>学员管理</h1><p class="muted">已启用 ${active} / ${data.capacity} 人 · 在线 ${data.online.length} 人 · 共 ${data.learners.length} 个学员账号</p></div><button class="primary" data-account-action="create">新增学员</button></div><p class="notice">停用会退出该账号并关闭课件与实训权限，历史作业和考试记录保留。最多同时启用10人；换班时可先停用原账号再新增。</p><section class="panel"><div class="row"><button data-account-action="reset-selected">重置所选密码</button><button data-account-action="refresh">刷新名单</button><span id="account-selection" class="muted">已选 ${selected.size} 人</span></div><div class="table-scroll"><table class="account-table"><thead><tr><th><input type="checkbox" id="account-select-all" aria-label="选择全部学员" ${selected.size === data.learners.length && selected.size ? "checked" : ""}></th><th>账号</th><th>姓名</th><th>部门 / 小组</th><th>状态</th><th>操作</th></tr></thead><tbody>${data.learners.map((l) => `<tr><td><input type="checkbox" data-account-select="${e(l.id)}" aria-label="选择 ${e(l.id)}" ${selected.has(l.id) ? "checked" : ""}></td><th>${e(l.id)}</th><td>${e(l.name || "未填写")}</td><td>${e(l.department || "—")}</td><td><span class="tag ${l.disabled ? "account-disabled" : ""}">${l.disabled ? "已停用" : data.online.includes(l.id) ? "在线" : "离线"}</span></td><td><div class="row"><button data-account-edit="${e(l.id)}">编辑</button><button data-account-reset="${e(l.id)}">重置密码</button><button ${l.disabled ? "" : 'class="danger"'} data-account-toggle="${e(l.id)}">${l.disabled ? "启用" : "停用"}</button></div></td></tr>`).join("")}</tbody></table></div></section><p class="muted">系统不提供旧密码查询。新增或重置后显示新密码，管理员可及时分发；管理员自身密码在“密码”页面修改。</p><p id="feature-message" role="status" aria-live="polite"></p></div>`;
  }
  function openForm(kind, id = "", ids = []) {
    const learner = data.learners.find((l) => l.id === id);
    const title = {
      create: "新增学员",
      update: "编辑学员信息",
      "set-enabled": learner?.disabled ? "启用学员账号" : "停用学员账号",
      "reset-passwords": ids.length > 1 ? "批量重置学员密码" : "重置学员密码",
    }[kind];
    const passwordField = `<label>新密码（至少6位，留空自动生成）<input name="password" type="password" autocomplete="new-password" minlength="6" maxlength="200"></label><label>再次输入新密码<input name="confirmation" type="password" autocomplete="new-password" maxlength="200"></label>`;
    dialog.innerHTML = `<form id="account-form" data-kind="${kind}" data-id="${e(id)}" data-revision="${data.revision}"><div class="dialog-head"><h2>${title}</h2><button type="button" data-account-action="close" aria-label="关闭账号管理">×</button></div>${kind === "create" || kind === "update" ? `<div class="form-stack"><label>学员账号<input name="id" required minlength="2" maxlength="32" value="${e(id)}" ${kind === "update" ? "readonly" : ""} placeholder="例如 TG11"></label><label>姓名<input name="name" maxlength="60" value="${e(learner?.name || "")}"></label><label>部门 / 小组<input name="department" maxlength="100" value="${e(learner?.department || "")}"></label>${kind === "create" ? passwordField : ""}</div>` : ""}${kind === "reset-passwords" ? `<p>将重置：${ids.map(e).join("、")}</p><p class="notice">保存后这些学员的旧密码和登录状态立即失效，练习与考试数据保留。${ids.length > 1 ? "每人生成独立的随机密码。" : ""}</p>${ids.length === 1 ? `<div class="form-stack">${passwordField}</div>` : ""}<input type="hidden" name="ids" value="${e(JSON.stringify(ids))}">` : ""}${kind === "set-enabled" ? `<p>${e(id)}${learner.name ? " · " + e(learner.name) : ""}</p><p class="notice">${learner.disabled ? "启用后可使用原密码登录。课件与实训须由讲师重新开放。" : "停用后立即退出账号并关闭课件与实训权限。历史作业、评分和考试保留。"}</p><input type="hidden" name="enabled" value="${learner.disabled ? "true" : "false"}">` : ""}<p id="account-error" role="alert"></p><footer class="dialog-footer"><button type="button" data-account-action="close">取消</button><button type="submit" class="primary">${kind === "reset-passwords" ? "确认重置密码" : "保存"}</button></footer></form>`;
    dialog.showModal();
  }
  function credentialsView(items) {
    dialog.innerHTML = `<div class="dialog-head"><h2>账号密码已更新</h2><button type="button" data-account-action="close" aria-label="关闭账号管理">×</button></div><p class="notice">新密码仅在这里显示，请及时复制并分发。关闭后不提供密码查询，可再次重置。</p><div class="credential-list">${items.map((item) => `<label>${e(item.id)}<input class="credential-value" readonly value="${e(item.password)}" aria-label="${e(item.id)} 新密码" autocomplete="off"></label>`).join("")}</div><p id="account-copy-status" role="status"></p><footer class="dialog-footer"><button data-account-action="copy">复制账号与密码</button><button class="primary" data-account-action="close">完成并关闭</button></footer>`;
    if (!dialog.open) dialog.showModal();
  }
  function closeDialog() {
    dialog.close();
    dialog.replaceChildren();
  }
  dialog.addEventListener("cancel", (ev) => {
    ev.preventDefault();
    closeDialog();
  });
  document.addEventListener("click", async (ev) => {
    const button = ev.target.closest("button");
    if (!button || button.disabled) return;
    try {
      const action = button.dataset.accountAction;
      if (action === "close") closeDialog();
      if (action === "refresh") await show();
      if (action === "create") openForm("create");
      if (action === "reset-selected") {
        if (!selected.size) return notice("请先勾选需要重置密码的学员。", true);
        openForm("reset-passwords", "", [...selected]);
      }
      if (action === "copy") {
        const values = [...dialog.querySelectorAll(".credential-value")]
          .map(
            (x) => x.closest("label").firstChild.textContent + "\t" + x.value,
          )
          .join("\n");
        await navigator.clipboard.writeText(values);
        dialog.querySelector("#account-copy-status").textContent = "已复制。";
      }
      if (button.dataset.accountEdit)
        openForm("update", button.dataset.accountEdit);
      if (button.dataset.accountToggle)
        openForm("set-enabled", button.dataset.accountToggle);
      if (button.dataset.accountReset)
        openForm("reset-passwords", "", [button.dataset.accountReset]);
    } catch (err) {
      const target =
        dialog.open &&
        dialog.querySelector("#account-error, #account-copy-status");
      if (target) target.textContent = err.message;
      else notice(err.message, true);
    }
  });
  document.addEventListener("change", (ev) => {
    const target = ev.target;
    if (target.id === "account-select-all") {
      selected.clear();
      if (target.checked) data.learners.forEach((l) => selected.add(l.id));
      document
        .querySelectorAll("[data-account-select]")
        .forEach((x) => (x.checked = target.checked));
    } else if (target.dataset.accountSelect) {
      if (target.checked) selected.add(target.dataset.accountSelect);
      else selected.delete(target.dataset.accountSelect);
      document.querySelector("#account-select-all").checked =
        selected.size === data.learners.length;
    } else return;
    document.querySelector("#account-selection").textContent =
      `已选 ${selected.size} 人`;
  });
  document.addEventListener("submit", async (ev) => {
    const form = ev.target;
    if (form.getAttribute("id") !== "account-form") return;
    ev.preventDefault();
    const button = form.querySelector("button[type=submit]");
    button.disabled = true;
    try {
      const f = new FormData(form),
        kind = form.dataset.kind;
      if (
        f.get("password") !== null &&
        f.get("password") !== f.get("confirmation")
      )
        throw new Error("两次输入的密码不一致。");
      const body = { kind, revision: Number(form.dataset.revision) };
      if (kind === "create" || kind === "update")
        Object.assign(body, {
          id: f.get("id"),
          name: f.get("name"),
          department: f.get("department"),
        });
      if (kind === "create" || kind === "reset-passwords")
        body.password = f.get("password") || "";
      if (kind === "reset-passwords") body.ids = JSON.parse(f.get("ids"));
      if (kind === "set-enabled")
        Object.assign(body, {
          id: form.dataset.id,
          enabled: f.get("enabled") === "true",
        });
      const result = await api("/api/admin/learners", body);
      await show();
      if (result.credentials.length) credentialsView(result.credentials);
      else {
        closeDialog();
        notice("学员信息已保存。");
      }
    } catch (err) {
      const target = dialog.querySelector("#account-error");
      if (target) target.textContent = err.message;
    } finally {
      button.disabled = false;
    }
  });
  return { show, close: closeDialog };
})();
