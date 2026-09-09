/* dcim-lite clean frontend — no CDN, talks to same-origin /api or API_BASE */
(function () {
  const API_BASE = window.DCIM_API_BASE || "";
  let token = localStorage.getItem("dcim_token") || "";
  let user = null;
  let page = "dashboard";
  let captchaId = "";
  let loading = false;
  let error = "";
  let tree = [];
  let stats = { dc: 0, room: 0, rack: 0, device: 0 };
  let racks = { items: [], total: 0, page: 1, pageSize: 20 };
  let devices = { items: [], total: 0, page: 1, pageSize: 20 };
  const rackQ = { search: "", status: "", sortBy: "code", sortDir: "asc", pageSize: 20 };
  const devQ = { search: "", lifecycleStatus: "", sortBy: "code", sortDir: "asc" };
  const login = { username: "admin", password: "", captcha: "" };

  async function api(path, opts = {}) {
    const headers = Object.assign({ "Content-Type": "application/json" }, opts.headers || {});
    if (token) headers.Authorization = "Bearer " + token;
    const res = await fetch(API_BASE + path, {
      method: opts.method || "GET",
      headers,
      body: opts.body ? JSON.stringify(opts.body) : undefined,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok || (data.code && data.code !== "SUCCESS")) {
      const err = new Error(data.message || res.statusText);
      err.code = data.code;
      err.status = res.status;
      throw err;
    }
    return data;
  }

  async function loadCaptcha() {
    try {
      const b = await api("/api/v1/auth/captcha");
      captchaId = b.data.id;
      document.getElementById("cap-img").src = b.data.image;
    } catch (e) {
      error = "验证码加载失败";
      render();
    }
  }

  async function doLogin() {
    loading = true;
    error = "";
    render();
    try {
      const b = await api("/api/v1/auth/login", {
        method: "POST",
        body: {
          username: login.username,
          password: login.password,
          captchaId,
          captcha: login.captcha,
        },
      });
      token = b.data.token;
      user = b.data.user;
      localStorage.setItem("dcim_token", token);
      login.password = "";
      login.captcha = "";
      await bootApp();
    } catch (e) {
      error = e.message || "登录失败";
      login.captcha = "";
      await loadCaptcha();
    } finally {
      loading = false;
      render();
    }
  }

  function doLogout() {
    token = "";
    user = null;
    localStorage.removeItem("dcim_token");
    page = "login";
    render();
    loadCaptcha();
  }

  async function bootApp() {
    try {
      const me = await api("/api/v1/auth/me");
      user = me.data;
      page = "dashboard";
      await Promise.all([loadTree(), loadDevices(1), loadRacks(1)]);
    } catch (e) {
      doLogout();
    }
  }

  async function loadTree() {
    const b = await api("/api/v1/resource-tree");
    tree = b.data.items || [];
    let dc = tree.length, room = 0, rack = 0;
    tree.forEach((d) => {
      (d.rooms || []).forEach((r) => {
        room++;
        rack += (r.racks || []).length;
      });
    });
    stats.dc = dc;
    stats.room = room;
    stats.rack = rack;
    render();
  }

  async function loadRacks(pageNo) {
    const p = pageNo || 1;
    const qs = new URLSearchParams({
      page: String(p),
      pageSize: String(rackQ.pageSize),
      sortBy: rackQ.sortBy,
      sortDir: rackQ.sortDir,
    });
    if (rackQ.search) qs.set("search", rackQ.search);
    if (rackQ.status) qs.set("status", rackQ.status);
    const b = await api("/api/v1/racks-page?" + qs.toString());
    racks = {
      items: b.data.items || [],
      total: b.data.total || 0,
      page: b.data.page || p,
      pageSize: b.data.pageSize || rackQ.pageSize,
    };
    render();
  }

  async function loadDevices(pageNo) {
    const p = pageNo || 1;
    const qs = new URLSearchParams({
      page: String(p),
      pageSize: "20",
      sortBy: devQ.sortBy,
      sortDir: devQ.sortDir,
    });
    if (devQ.search) qs.set("search", devQ.search);
    if (devQ.lifecycleStatus) qs.set("lifecycleStatus", devQ.lifecycleStatus);
    const b = await api("/api/v1/devices?" + qs.toString());
    devices = {
      items: b.data.items || [],
      total: b.data.total || 0,
      page: b.data.page || p,
      pageSize: b.data.pageSize || 20,
    };
    if (!stats.device) stats.device = devices.total;
    else stats.device = devices.total;
    render();
  }

  function toggleRackSort(col) {
    if (rackQ.sortBy === col) rackQ.sortDir = rackQ.sortDir === "asc" ? "desc" : "asc";
    else {
      rackQ.sortBy = col;
      rackQ.sortDir = "asc";
    }
    loadRacks(1);
  }
  function toggleDevSort(col) {
    if (devQ.sortBy === col) devQ.sortDir = devQ.sortDir === "asc" ? "desc" : "asc";
    else {
      devQ.sortBy = col;
      devQ.sortDir = "asc";
    }
    loadDevices(1);
  }
  function openRacks() {
    page = "racks";
    loadRacks(1);
  }
  function openDevices() {
    page = "devices";
    loadDevices(1);
  }

  function esc(s) {
    return String(s == null ? "" : s).replace(/[&<>"']/g, (c) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    })[c]);
  }

  const titles = { dashboard: "总览", resources: "资源层级", racks: "机柜台账", devices: "设备台账" };

  function render() {
    const root = document.getElementById("app");
    if (!token) {
      root.innerHTML = `
      <div class="login-wrap">
        <form class="login-card" id="login-form">
          <h1>dcim-lite</h1>
          <p class="sub">轻量 DCIM · 机房资源台账</p>
          <label>用户名<input id="u" value="${esc(login.username)}" autocomplete="username" required></label>
          <label>密码<input id="p" type="password" autocomplete="current-password" required></label>
          <div class="captcha-row">
            <input id="c" maxlength="4" inputmode="numeric" placeholder="4 位数字验证码" required>
            <img id="cap-img" alt="captcha" title="点击刷新">
          </div>
          <button type="submit" ${loading ? "disabled" : ""}>${loading ? "登录中…" : "登录"}</button>
          ${error ? `<p class="error">${esc(error)}</p>` : ""}
        </form>
      </div>`;
      document.getElementById("login-form").onsubmit = (e) => {
        e.preventDefault();
        login.username = document.getElementById("u").value;
        login.password = document.getElementById("p").value;
        login.captcha = document.getElementById("c").value;
        doLogin();
      };
      document.getElementById("cap-img").onclick = loadCaptcha;
      if (!document.getElementById("cap-img").src) loadCaptcha();
      return;
    }

    let body = "";
    if (page === "dashboard") {
      body = `<section class="cards">
        <div class="card"><div class="k">数据中心</div><div class="v">${stats.dc}</div></div>
        <div class="card"><div class="k">机房</div><div class="v">${stats.room}</div></div>
        <div class="card"><div class="k">机柜</div><div class="v">${stats.rack}</div></div>
        <div class="card"><div class="k">设备</div><div class="v">${stats.device}</div></div>
      </section>`;
    } else if (page === "resources") {
      const rows = tree
        .map((dc) => {
          const rooms = (dc.rooms || [])
            .map((room) => {
              const racks = (room.racks || [])
                .map((r) => `<div class="node rack">机柜 ${esc(r.code)} · ${esc(r.name)} · ${r.uHeight}U · ${esc(r.status)}</div>`)
                .join("");
              return `<div class="node room">机房 ${esc(room.code)} · ${esc(room.name)}</div>${racks}`;
            })
            .join("");
          return `<div class="node dc">DC ${esc(dc.code)} · ${esc(dc.name)} <em>${esc(dc.status)}</em></div>${rooms}`;
        })
        .join("");
      body = `<section class="panel"><div class="toolbar"><button id="btn-tree">刷新</button></div>
        <div class="tree">${rows || '<p class="muted">暂无数据</p>'}</div></section>`;
    } else if (page === "racks") {
      const tr = racks.items
        .map(
          (r) => `<tr><td>${esc(r.code)}</td><td>${esc(r.name)}</td>
          <td>${r.uHeight}U · ${r.widthMm}×${r.depthMm}×${r.heightMm}</td>
          <td><span class="tag">${esc(r.status)}</span></td><td>${esc(r.manager || "—")}</td></tr>`
        )
        .join("");
      const pages = Math.max(1, Math.ceil(racks.total / racks.pageSize));
      body = `<section class="panel">
        <div class="toolbar">
          <input id="rq" value="${esc(rackQ.search)}" placeholder="搜索编码/名称/位置/负责人">
          <select id="rs">
            <option value="">全部状态</option>
            ${["AVAILABLE", "PARTIAL", "FULL", "MAINTENANCE", "DISABLED", "PLANNING"]
              .map((s) => `<option ${rackQ.status === s ? "selected" : ""}>${s}</option>`)
              .join("")}
          </select>
          <button id="btn-rack-q">查询</button>
        </div>
        <table><thead><tr>
          <th data-s="code">编码</th><th data-s="name">名称</th><th>规格</th>
          <th data-s="status">状态</th><th>负责人</th>
        </tr></thead><tbody>${tr || '<tr><td colspan="5" class="muted">无数据</td></tr>'}</tbody></table>
        <div class="pager">
          <span>共 ${racks.total} 条</span>
          <button id="rp" ${racks.page <= 1 ? "disabled" : ""}>上一页</button>
          <span>${racks.page} / ${pages}</span>
          <button id="rn" ${racks.page >= pages ? "disabled" : ""}>下一页</button>
          <select id="rps">${[10, 20, 30]
            .map((n) => `<option value="${n}" ${rackQ.pageSize === n ? "selected" : ""}>${n}</option>`)
            .join("")}</select>
        </div></section>`;
    } else {
      const tr = devices.items
        .map(
          (d) => `<tr><td>${esc(d.code)}</td><td>${esc(d.name)}</td>
          <td>${esc((d.type && d.type.name) || "")}</td>
          <td><span class="tag">${esc(d.lifecycleStatus)}</span></td><td>${d.heightU}</td></tr>`
        )
        .join("");
      const pages = Math.max(1, Math.ceil(devices.total / devices.pageSize));
      body = `<section class="panel">
        <div class="toolbar">
          <input id="dq" value="${esc(devQ.search)}" placeholder="搜索编码/名称/序列号/IP">
          <select id="ds">
            <option value="">全部生命周期</option>
            ${["WAITING_RACK", "RUNNING", "MAINTENANCE", "PENDING_REMOVAL", "OFF_RACK", "SCRAPPED"]
              .map((s) => `<option ${devQ.lifecycleStatus === s ? "selected" : ""}>${s}</option>`)
              .join("")}
          </select>
          <button id="btn-dev-q">查询</button>
        </div>
        <table><thead><tr>
          <th data-d="code">编码</th><th data-d="name">名称</th><th>类型</th>
          <th data-d="lifecycleStatus">状态</th><th data-d="heightU">U</th>
        </tr></thead><tbody>${tr || '<tr><td colspan="5" class="muted">无数据</td></tr>'}</tbody></table>
        <div class="pager">
          <span>共 ${devices.total} 条</span>
          <button id="dp" ${devices.page <= 1 ? "disabled" : ""}>上一页</button>
          <span>${devices.page} / ${pages}</span>
          <button id="dn" ${devices.page >= pages ? "disabled" : ""}>下一页</button>
        </div></section>`;
    }

    root.innerHTML = `
      <div class="shell">
        <aside class="side">
          <div class="brand">dcim-lite</div>
          <nav>
            <button class="${page === "dashboard" ? "on" : ""}" data-p="dashboard">总览</button>
            <button class="${page === "resources" ? "on" : ""}" data-p="resources">资源层级</button>
            <button class="${page === "racks" ? "on" : ""}" data-p="racks">机柜台账</button>
            <button class="${page === "devices" ? "on" : ""}" data-p="devices">设备台账</button>
          </nav>
          <button class="logout" id="btn-logout">退出</button>
        </aside>
        <main class="main">
          <header class="top"><strong>${titles[page] || ""}</strong>
            <span class="muted">${esc((user && (user.displayName || user.username)) || "")}</span></header>
          ${body}
        </main>
      </div>`;

    root.querySelectorAll("nav button").forEach((b) => {
      b.onclick = () => {
        const p = b.getAttribute("data-p");
        if (p === "racks") openRacks();
        else if (p === "devices") openDevices();
        else {
          page = p;
          if (p === "resources") loadTree();
          else render();
        }
      };
    });
    document.getElementById("btn-logout").onclick = doLogout;
    const bt = document.getElementById("btn-tree");
    if (bt) bt.onclick = loadTree;
    const bq = document.getElementById("btn-rack-q");
    if (bq)
      bq.onclick = () => {
        rackQ.search = document.getElementById("rq").value;
        rackQ.status = document.getElementById("rs").value;
        loadRacks(1);
      };
    root.querySelectorAll("th[data-s]").forEach((th) => {
      th.onclick = () => toggleRackSort(th.getAttribute("data-s"));
    });
    const rp = document.getElementById("rp");
    if (rp) rp.onclick = () => loadRacks(racks.page - 1);
    const rn = document.getElementById("rn");
    if (rn) rn.onclick = () => loadRacks(racks.page + 1);
    const rps = document.getElementById("rps");
    if (rps)
      rps.onchange = () => {
        rackQ.pageSize = Number(rps.value);
        loadRacks(1);
      };
    const bd = document.getElementById("btn-dev-q");
    if (bd)
      bd.onclick = () => {
        devQ.search = document.getElementById("dq").value;
        devQ.lifecycleStatus = document.getElementById("ds").value;
        loadDevices(1);
      };
    root.querySelectorAll("th[data-d]").forEach((th) => {
      th.onclick = () => toggleDevSort(th.getAttribute("data-d"));
    });
    const dp = document.getElementById("dp");
    if (dp) dp.onclick = () => loadDevices(devices.page - 1);
    const dn = document.getElementById("dn");
    if (dn) dn.onclick = () => loadDevices(devices.page + 1);
  }

  // boot
  if (token) {
    bootApp().then(render).catch(() => {
      token = "";
      localStorage.removeItem("dcim_token");
      render();
      loadCaptcha();
    });
  } else {
    render();
    loadCaptcha();
  }
})();
