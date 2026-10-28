"use strict";
window.LearnerUI = (() => {
  const sidebar = document.querySelector("#business-sidebar"),
    frame = document.querySelector("#portal-frame"),
    backdrop = document.querySelector("#sidebar-backdrop");
  let sidebarSignature = "",
    dashboardSignature = "",
    lessonSignature = "",
    currentSlide = "",
    followTeacher = true,
    identity = "";
  const descriptions = {
    M01: "从维修需求识别物料缺口，制定采购计划。",
    M02: "统一报价口径，比较价格、交期与证书。",
    M03: "评估供应商表现与供应集中风险。",
    M04: "跟踪采购订单，识别延期与交付风险。",
    M05: "分析补货参数，平衡库存成本与保障水平。",
    M06: "围绕到达时限，比较紧急保障方案。",
  };
  const icon = (name) => {
    const paths = {
      home: '<path d="m3 10 9-7 9 7v10H3z"/><path d="M9 20v-7h6v7"/>',
      M01: '<path d="M7 3h10v4H7zM5 5H3v16h18V5h-2M7 11h10M7 16h6"/>',
      M02: '<path d="M4 4h16v16H4zM8 8h8M8 12h8M8 16h4"/>',
      M03: '<path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6zM8 12l3 3 5-6"/>',
      M04: '<path d="M3 5h12v12H3zM15 9h4l3 4v4h-7"/><circle cx="7" cy="18" r="2"/><circle cx="18" cy="18" r="2"/>',
      M05: '<path d="m3 7 9-4 9 4v12l-9 3-9-3zM3 7l9 4 9-4M12 11v11"/>',
      M06: '<path d="m13 2-9 12h7l-1 8 10-13h-7z"/>',
      lessons: '<path d="M3 4h18v13H3zM8 22l4-5 4 5M7 8h10M7 12h6"/>',
      history: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
      exam: '<path d="M6 3h12v18H6zM9 7h6M9 11h6M9 15l2 2 4-4"/>',
      resources: '<path d="M10 4H4v16h16v-6M13 3h8v8M10 14 21 3"/>',
      password:
        '<rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3M12 14v3"/>',
    };
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[name] || paths.home}</svg>`;
  };
  function mount() {
    if (!user || user.role !== "learner") return;
    document.body.classList.add("learner-mode");
    frame.classList.add("enterprise-layout");
    sidebar.hidden = false;
    document.title = "采购工作台 · 航材采购管理系统";
    document.querySelector(".brand").innerHTML =
      '<button id="sidebar-toggle" aria-label="打开业务菜单" aria-expanded="false">☰</button><span class="product-mark">T</span><span class="product-name">TAIGU · 航材采购管理系统</span><span class="environment-badge">培训环境</span>';
  }
  function closeMenu() {
    document.body.classList.remove("sidebar-open");
    backdrop.hidden = true;
    document
      .querySelector("#sidebar-toggle")
      ?.setAttribute("aria-expanded", "false");
  }
  // Close before the document navigation handler can replace the clicked node.
  sidebar.addEventListener("click", (ev) => {
    const button = ev.target.closest("button");
    if (button && !button.disabled) closeMenu();
  });
  function updateSidebar() {
    if (!user || user.role !== "learner") return;
    const view = window.Features?.context || { view: "classroom", module: "" };
    const signature = JSON.stringify([
      user.id,
      user.modules,
      user.slides.length,
      user.examActive,
      view,
    ]);
    if (signature === sidebarSignature) return;
    sidebarSignature = signature;
    const catalog = user.catalog || [];
    const utility = (key, label, badge = "") =>
      `<button data-view="${key}" ${key === "history" && user.examActive ? 'disabled title="考试期间暂停历史查看"' : ""} class="business-nav-item ${view.view === key ? "selected" : ""}" ${view.view === key ? 'aria-current="page"' : ""}>${icon(key === "classroom" ? "home" : key)}<span>${label}</span>${badge ? `<small>${badge}</small>` : ""}</button>`;
    sidebar.innerHTML = `<div class="sidebar-heading"><span>PROCUREMENT</span><strong>采购业务中心</strong><p>${user.modules.length} / 6 个业务模块已激活</p></div><nav aria-label="业务模块">${utility("classroom", "采购工作台")}<p class="nav-caption">业务管理</p>${catalog
      .map((m) => {
        const open = user.modules.some((x) => x.id === m.id);
        return `<button data-module="${e(m.id)}" class="business-nav-item module-menu ${view.module === m.id ? "selected" : ""}" ${open ? "" : "disabled"} ${view.module === m.id ? 'aria-current="page"' : ""} title="${open ? "已激活，可进入操作" : "讲师尚未激活此模块"}">${icon(m.id)}<span>${e(m.title)}<small class="module-state ${open ? "is-open" : ""}">${open ? "已激活" : "未激活"}</small></span>${!open ? '<span class="menu-lock" aria-hidden="true">锁</span>' : ""}</button>`;
      })
      .join(
        "",
      )}<p class="nav-caption">课堂与支持</p>${utility("lessons", "课堂课件", String(user.slides.length))}${utility("history", "历史练习")}${utility("exam", "结业考试")}${utility("resources", "软件体验渠道")}${utility("password", "账号与密码")}</nav><div class="sidebar-footer"><span class="connection-dot"></span>讲师授权自动同步<br><small>培训环境 · 数据按来源记录</small></div>`;
  }
  function content(title, body) {
    const context = window.Features?.context || {};
    const selected = (user.catalog || []).find((m) => m.id === context.module);
    return `<div class="shell enterprise-page"><div class="business-breadcrumb">采购管理系统 <span>/</span> ${selected ? e(selected.title) : e(title)}</div><div class="toolbar business-page-heading"><div><h1>${e(title)}</h1><p class="muted">${selected ? e(descriptions[selected.id]) : "统一工作空间 · 课堂授权实时同步"}</p></div>${context.module ? '<span class="status-badge green">业务模块已激活</span>' : ""}</div>${body}<p id="feature-message" role="status" aria-live="polite"></p></div>`;
  }
  function dashboard() {
    if (!user || user.role !== "learner") return;
    if (!document.body.classList.contains("learner-mode")) mount();
    const progress = user.progress || {};
    const open = user.modules.length;
    const done = user.modules.filter(
      (m) => progress[m.id]?.status === "submitted",
    ).length;
    const current =
      user.slides.find((p) => p.id === user.learner.current) || user.slides[0];
    const catalog = user.catalog || [];
    app.innerHTML = `<div class="shell enterprise-page procurement-dashboard"><div class="business-breadcrumb">采购管理系统 <span>/</span> 工作台</div><div class="toolbar business-page-heading"><div><p class="eyebrow">PROCUREMENT WORKSPACE</p><h1>采购工作台</h1><p class="muted">欢迎回来，${e(user.learner.name || user.id)}。从左侧选择已激活的业务模块，开始今天的采购任务。</p></div><span class="status-badge"><span class="connection-dot"></span>课堂已连接</span></div><div class="workspace-kpis">${[
      [open, "已激活业务模块", "共 6 个业务模块"],
      [open - done, "待完成业务实训", "按已激活模块的最近记录"],
      [done, "已提交业务实训", "历史结果由讲师统一评价"],
      [user.slides.length, "可查看课件", "讲师按授课进度开放"],
    ]
      .map(
        (x) =>
          `<section class="kpi-card"><span>${x[1]}</span><strong>${x[0]}<small>${x[1] === "可查看课件" ? "页" : "项"}</small></strong><p>${x[2]}</p></section>`,
      )
      .join(
        "",
      )}</div><div class="workbench-columns"><section class="panel business-tasks"><div class="toolbar"><div><h2>业务任务</h2><p class="muted">按采购流程完成分析、复核与提交</p></div><span class="tag">个人工作空间</span></div><div class="table-scroll"><table><thead><tr><th>业务模块</th><th>授权状态</th><th>最近练习</th><th>操作</th></tr></thead><tbody>${catalog
      .map((m) => {
        const enabled = user.modules.some((x) => x.id === m.id),
          p = progress[m.id];
        return `<tr><th><div class="task-name">${icon(m.id)}<span>${e(m.title)}<small>${m.id}</small></span></div></th><td><span class="status-badge ${enabled ? "green" : "gray"}">${enabled ? "已激活" : "未激活"}</span></td><td>${p ? `<span>${p.status === "submitted" ? "已提交" : "进行中"}</span><small class="cell-detail">第 ${p.number} 次练习</small>` : '<span class="muted">尚未开始</span>'}</td><td><button data-module="${m.id}" ${enabled ? "" : "disabled"}>${enabled ? "进入业务" : "等待激活"}</button></td></tr>`;
      })
      .join(
        "",
      )}</tbody></table></div></section><aside class="workbench-side"><section class="panel course-brief"><div class="section-symbol">${icon("lessons")}</div><span class="eyebrow">课堂同步</span><h2>${current ? e(current.title) : "等待讲师开放课件"}</h2><p class="muted">${current ? `当前可阅 ${user.slides.length} 页。课件独立呈现，随时查阅后返回业务操作。` : "讲师激活页面后会显示在课堂课件中，业务模块按各自权限开放。"}</p><button data-view="lessons">查看课堂课件 →</button></section><section class="panel workflow-brief"><h3>标准作业流程</h3><ol><li>核对任务与业务约束</li><li>按模板准备数据</li><li>运行分析并比较方案</li><li>人工复核后提交</li></ol><p class="muted">有疑问？右下角助手可解释操作与名词术语。</p></section></aside></div></div>`;
    dashboardSignature = JSON.stringify([user.revision, user.progress]);
    updateSidebar();
  }
  function lessons() {
    const pages = user.slides || [];
    if (followTeacher && pages.some((p) => p.id === user.learner.current))
      currentSlide = user.learner.current;
    if (!pages.some((p) => p.id === currentSlide))
      currentSlide = pages[0]?.id || "";
    const index = pages.findIndex((p) => p.id === currentSlide),
      slide = pages[index];
    const module =
        slide && (user.catalog || []).find((m) => m.id === slide.suggested),
      open = module && user.modules.some((m) => m.id === module.id);
    document.querySelector("#workspace").innerHTML = content(
      "课堂课件",
      `<div class="lesson-toolbar panel"><div><span class="status-badge green">${pages.length} 页已授权</span><span class="muted"> 仅展示向本人开放的内容</span></div><button data-lesson-action="follow" aria-pressed="${followTeacher}">${followTeacher ? "正在跟随讲师" : "跟随讲师当前页"}</button></div>${slide ? `<div class="lesson-layout"><nav class="lesson-page-list" aria-label="已激活课件页面">${pages.map((p, i) => `<button data-lesson-page="${e(p.id)}" class="${p.id === currentSlide ? "selected" : ""}" ${p.id === currentSlide ? 'aria-current="page"' : ""}><small>${String(i + 1).padStart(2, "0")} · ${e(p.id)}</small><strong>${e(p.title)}</strong>${p.id === user.learner.current ? '<span class="lesson-live">讲师当前页</span>' : ""}</button>`).join("")}</nav><section><div class="lesson-stage" id="lesson-stage">${slideView(slide)}</div><div class="lesson-controls"><div class="row"><button data-lesson-action="previous" ${index > 0 ? "" : "disabled"}>上一页</button><span class="muted">${index + 1} / ${pages.length}</span><button data-lesson-action="next" ${index < pages.length - 1 ? "" : "disabled"}>下一页</button></div><button data-lesson-action="fullscreen">全屏阅读</button></div><section class="panel lesson-linked"><div><span class="eyebrow">关联业务</span><h3>${module ? e(module.title) : "暂无关联业务模块"}</h3><p class="muted">课件浏览和业务操作分别保存进度。</p></div>${module ? `<button data-module="${module.id}" ${open ? "" : "disabled"}>${open ? "进入业务操作" : "关联模块未激活"}</button>` : ""}</section></section></div>` : '<section class="panel lesson-empty"><div class="section-symbol">' + icon("lessons") + '</div><h2>暂无已激活课件</h2><p class="muted">讲师开放页面后会自动显示。你可以继续操作左侧已激活的业务模块。</p></section>'}`,
    );
    lessonSignature = JSON.stringify([user.slides, user.learner.current]);
    updateSidebar();
    window.Assistant?.updateContext();
  }
  async function selectLesson(id) {
    if (!user?.slides.some((p) => p.id === id)) return;
    followTeacher = false;
    currentSlide = id;
    await window.Features.show("lessons");
  }
  function sync(next) {
    if (!next || next.role !== "learner") {
      document.body.classList.remove("learner-mode");
      frame.classList.remove("enterprise-layout");
      sidebar.hidden = true;
      closeMenu();
      identity = "";
      sidebarSignature = "";
      document.querySelector(".brand").innerHTML =
        "TAIGU <span>航材采购 AI 实训中心</span>";
      return;
    }
    if (identity !== next.id) {
      identity = next.id;
      currentSlide = "";
      followTeacher = true;
      mount();
    }
    updateSidebar();
    if (
      !window.Features.active &&
      dashboardSignature !== JSON.stringify([next.revision, next.progress])
    )
      dashboard();
    if (
      window.Features.active === "lessons" &&
      lessonSignature !== JSON.stringify([next.slides, next.learner.current])
    )
      lessons();
  }
  document.addEventListener("click", async (ev) => {
    const button = ev.target.closest("button");
    if (!button || button.disabled) return;
    if (button.id === "sidebar-toggle") {
      const open = !document.body.classList.contains("sidebar-open");
      document.body.classList.toggle("sidebar-open", open);
      backdrop.hidden = !open;
      button.setAttribute("aria-expanded", String(open));
    }
    if (button === backdrop || button.closest("#business-sidebar")) closeMenu();
    if (button.dataset.lessonPage) {
      followTeacher = false;
      currentSlide = button.dataset.lessonPage;
      lessons();
    }
    const action = button.dataset.lessonAction;
    if (action === "follow") {
      followTeacher = !followTeacher;
      lessons();
    }
    if (action === "previous" || action === "next") {
      const index = user.slides.findIndex((p) => p.id === currentSlide);
      currentSlide =
        user.slides[index + (action === "next" ? 1 : -1)]?.id || currentSlide;
      followTeacher = false;
      lessons();
    }
    if (action === "fullscreen")
      try {
        await document.querySelector("#lesson-stage").requestFullscreen();
      } catch {
        status("浏览器暂不支持全屏，可使用浏览器的全屏显示。");
      }
  });
  document.addEventListener("keydown", (ev) => {
    if (ev.key === "Escape") closeMenu();
  });
  return {
    dashboard,
    lessons,
    content,
    updateSidebar,
    sync,
    selectLesson,
    get context() {
      return {
        slide: window.Features?.active === "lessons" ? currentSlide : "",
      };
    },
  };
})();
