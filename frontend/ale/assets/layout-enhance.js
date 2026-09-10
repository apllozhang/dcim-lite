/* 布局增强：侧栏折叠 / 亮暗色 / 中英切换
   注意：MutationObserver 必须防抖 + 幂等，禁止在回调里无节制改 DOM */
(function () {
  if (window.__cmtLayoutEnhance) return;
  window.__cmtLayoutEnhance = true;

  var LS_COLLAPSE = "cmt_sidebar_collapsed";
  var LS_THEME = "cmt_theme";
  var LS_LANG = "cmt_lang";

  var MENU = [
    { path: "/", zh: "运行概览", en: "Dashboard", icon: "M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z" },
    { path: "/data-centers", zh: "资源层级", en: "Resources", icon: "M4 6h16v2H4zm0 5h16v2H4zm0 5h10v2H4z" },
    { path: "/room-screen", zh: "机房大屏", en: "Room Screen", icon: "M3 5h18v12H3zm4 15h10v2H7z" },
    { path: "/racks", zh: "机柜管理", en: "Racks", icon: "M4 3h16v18H4zm2 2v3h12V5zm0 5v3h12v-3zm0 5v3h12v-3z" },
    { path: "/rack-templates", zh: "机柜模板", en: "Templates", icon: "M4 4h7v7H4zm9 0h7v7h-7zM4 13h7v7H4zm9 0h7v7h-7z" },
    { path: "/devices", zh: "设备台账", en: "Devices", icon: "M4 5h16v10H4zm2 12h12v2H6z" },
    { path: "/admin", zh: "系统管理", en: "Admin", icon: "M12 8a4 4 0 100 8 4 4 0 000-8zm8.94 5l1.5 1.3-1.5 2.6-1.8-.5a7 7 0 01-1.7 1l-.3 1.9h-3l-.3-1.9a7 7 0 01-1.7-1l-1.8.5-1.5-2.6L5.06 13l-1.5-1.3 1.5-2.6 1.8.5a7 7 0 011.7-1L8.8 6.7h3l.3 1.9a7 7 0 011.7 1l1.8-.5 1.5 2.6L20.94 13z" }
  ];

  var DICT = {
    "基础资源管理平台": "Infrastructure Platform",
    "退出登录": "Sign out",
    "机柜管理工具": "Cabinet Tool"
  };

  function getLang() { return localStorage.getItem(LS_LANG) || "zh"; }
  function getCollapsed() { return localStorage.getItem(LS_COLLAPSE) === "1"; }
  function getTheme() { return localStorage.getItem(LS_THEME) || "light"; }

  function applyTheme() {
    var t = getTheme();
    document.documentElement.setAttribute("data-theme", t);
    if (document.body) document.body.setAttribute("data-theme", t);
  }

  function applyCollapse() {
    var shell = document.querySelector(".app-shell");
    var aside = document.querySelector(".el-aside.sidebar, .sidebar");
    if (!shell || !aside) return;
    var on = getCollapsed();
    aside.classList.toggle("cmt-collapsed", on);
    shell.classList.toggle("cmt-shell-collapsed", on);
  }

  function applyLang() {
    var lang = getLang();
    document.querySelectorAll(".el-menu-item").forEach(function (el) {
      var path = el.getAttribute("index") || "";
      var hit = null;
      for (var i = 0; i < MENU.length; i++) {
        if (MENU[i].path === path) { hit = MENU[i]; break; }
      }
      if (!hit) return;
      var label = lang === "en" ? hit.en : hit.zh;
      // 只改文本节点，不动结构
      for (var n = 0; n < el.childNodes.length; n++) {
        var node = el.childNodes[n];
        if (node.nodeType === 3 && node.textContent.trim()) {
          node.textContent = label;
          return;
        }
      }
    });
    document.querySelectorAll(".topbar span").forEach(function (el) {
      if (el.closest(".cmt-top-tools")) return;
      var zh = el.getAttribute("data-cmt-zh");
      if (!zh) {
        zh = el.textContent.trim();
        if (!DICT[zh]) return;
        el.setAttribute("data-cmt-zh", zh);
      }
      el.textContent = lang === "en" ? DICT[zh] : zh;
    });
  }

  function injectIcons() {
    document.querySelectorAll(".el-menu-item").forEach(function (el) {
      if (el.querySelector(".cmt-menu-icon")) return;
      var path = el.getAttribute("index") || "";
      var hit = null;
      for (var i = 0; i < MENU.length; i++) {
        if (MENU[i].path === path) { hit = MENU[i]; break; }
      }
      if (!hit) return;
      var span = document.createElement("span");
      span.className = "cmt-menu-icon";
      span.innerHTML =
        '<svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor" aria-hidden="true"><path d="' +
        hit.icon +
        '"/></svg>';
      el.insertBefore(span, el.firstChild);
    });
  }

  function injectToolbar() {
    var topbar = document.querySelector(".topbar");
    if (!topbar || topbar.getAttribute("data-cmt-tools") === "1") return;
    topbar.setAttribute("data-cmt-tools", "1");
    var box = document.createElement("div");
    box.className = "cmt-top-tools";
    box.innerHTML =
      '<button type="button" class="cmt-tool-btn" id="cmt-lang-btn" title="Language">中/EN</button>' +
      '<button type="button" class="cmt-tool-btn" id="cmt-theme-btn" title="Theme">◐</button>';
    var user = topbar.querySelector(".user-menu");
    if (user && user.parentElement === topbar) topbar.insertBefore(box, user);
    else topbar.appendChild(box);

    document.getElementById("cmt-theme-btn").addEventListener("click", function () {
      localStorage.setItem(LS_THEME, getTheme() === "dark" ? "light" : "dark");
      applyTheme();
    });
    document.getElementById("cmt-lang-btn").addEventListener("click", function () {
      localStorage.setItem(LS_LANG, getLang() === "zh" ? "en" : "zh");
      applyLang();
    });
  }

  function injectCollapseBtn() {
    var aside = document.querySelector(".el-aside.sidebar, .sidebar");
    if (!aside || aside.querySelector(".cmt-collapse-btn")) return;
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "cmt-collapse-btn";
    btn.title = "折叠/展开侧栏";
    btn.innerHTML = '<svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M3 6h18v2H3zm0 5h12v2H3zm0 5h18v2H3z"/></svg>';
    btn.addEventListener("click", function () {
      localStorage.setItem(LS_COLLAPSE, getCollapsed() ? "0" : "1");
      applyCollapse();
    });
    aside.style.position = aside.style.position || "relative";
    aside.insertBefore(btn, aside.firstChild);
  }

  var enhancing = false;
  var pending = false;

  function enhance() {
    if (enhancing) { pending = true; return; }
    enhancing = true;
    try {
      if (!document.querySelector(".el-aside.sidebar, .sidebar")) return;
      injectIcons();
      injectCollapseBtn();
      injectToolbar();
      applyTheme();
      applyCollapse();
      applyLang();
    } finally {
      enhancing = false;
      if (pending) {
        pending = false;
        // 下一帧再补一次，避免同步重入
        setTimeout(function () { enhance(); }, 50);
      }
    }
  }

  var timer = null;
  function schedule() {
    if (timer) return;
    timer = setTimeout(function () {
      timer = null;
      enhance();
    }, 200);
  }

  function boot() {
    enhance();
    var obs = new MutationObserver(function (muts) {
      var need = false;
      for (var i = 0; i < muts.length; i++) {
        if (muts[i].addedNodes && muts[i].addedNodes.length) { need = true; break; }
      }
      if (need) schedule();
    });
    obs.observe(document.body, { childList: true });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
})();
