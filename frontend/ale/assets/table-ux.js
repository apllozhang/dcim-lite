/* 表格 UX：排序 / 调宽 / 换行保证 / 操作列折叠
   约束：无 MutationObserver；history 钩子 + 2s 兜底；全程幂等 */
(function () {
  if (window.__cmtTableUX) return;
  window.__cmtTableUX = true;

  var OPS_MENU_ID = "cmt-ops-menu";

  function qs(sel, root) {
    return (root || document).querySelector(sel);
  }
  function qsa(sel, root) {
    return Array.prototype.slice.call((root || document).querySelectorAll(sel));
  }

  function cellText(el) {
    return (el && (el.innerText || el.textContent) || "").replace(/\s+/g, " ").trim();
  }

  function headerCells(table) {
    var ths = qsa("thead th", table);
    if (ths.length >= 2) return ths;
    ths = qsa(".el-table__header th", table);
    if (ths.length >= 2) return ths;
    ths = qsa(".el-table__header-wrapper th", table);
    if (ths.length >= 2) return ths;
    // EP 有时用 div.el-table__cell 做表头
    ths = qsa(".el-table__header-wrapper .el-table__cell", table);
    return ths;
  }

  function bodyRows(table) {
    return qsa(".el-table__body tbody tr, tbody.el-table__body tr, .el-table__body tr", table);
  }

  function parseNum(s) {
    var m = String(s).replace(/[,¥$%\sU台个]/g, "").match(/-?\d+(\.\d+)?/);
    return m ? parseFloat(m[0]) : null;
  }

  function sortTable(table, colIdx, dir) {
    var bodies = qsa(".el-table__body tbody, tbody", table);
    var tbody = null;
    for (var i = 0; i < bodies.length; i++) {
      if (bodies[i].querySelector("tr")) { tbody = bodies[i]; break; }
    }
    if (!tbody) return;
    var rows = qsa("tr", tbody);
    rows.sort(function (a, b) {
      var ac = a.cells[colIdx] || a.children[colIdx];
      var bc = b.cells[colIdx] || b.children[colIdx];
      var av = cellText(ac), bv = cellText(bc);
      var an = parseNum(av), bn = parseNum(bv);
      var cmp = an !== null && bn !== null ? an - bn : av.localeCompare(bv, "zh");
      return dir === "asc" ? cmp : -cmp;
    });
    var frag = document.createDocumentFragment();
    rows.forEach(function (r) { frag.appendChild(r); });
    tbody.appendChild(frag);
  }

  function closeOpsMenu() {
    var m = document.getElementById(OPS_MENU_ID);
    if (m) m.remove();
    document.removeEventListener("click", onDocClick, true);
  }
  function onDocClick(e) {
    var m = document.getElementById(OPS_MENU_ID);
    if (m && !m.contains(e.target) && !e.target.closest(".cmt-ops-btn")) closeOpsMenu();
  }

  function openOpsMenu(btn, cell) {
    closeOpsMenu();
    var menu = document.createElement("div");
    menu.id = OPS_MENU_ID;
    menu.className = "cmt-ops-menu";
    qsa("button, a", cell).forEach(function (src) {
      var b = document.createElement("button");
      b.type = "button";
      b.className = "cmt-ops-item";
      b.textContent = cellText(src) || "操作";
      if (src.classList && src.classList.contains("el-button--danger")) b.classList.add("danger");
      b.addEventListener("click", function (ev) {
        ev.preventDefault();
        ev.stopPropagation();
        closeOpsMenu();
        src.click();
      });
      menu.appendChild(b);
    });
    document.body.appendChild(menu);
    var r = btn.getBoundingClientRect();
    var top = r.bottom + window.scrollY + 4;
    var left = Math.min(r.left + window.scrollX - 80, window.innerWidth - 140);
    menu.style.top = top + "px";
    menu.style.left = Math.max(8, left) + "px";
    setTimeout(function () {
      document.addEventListener("click", onDocClick, true);
    }, 0);
  }

  function collapseOps(table) {
    var ths = headerCells(table);
    if (!ths.length) return;
    var opsIdx = -1;
    for (var i = 0; i < ths.length; i++) {
      if (/操作/.test(cellText(ths[i]))) { opsIdx = i; break; }
    }
    if (opsIdx < 0) return;
    // 表头收窄
    ths[opsIdx].classList.add("cmt-ops-th");
    var headCell = ths[opsIdx].querySelector(".cell") || ths[opsIdx];
    if (!qs(".cmt-ops-head-label", ths[opsIdx])) {
      var span = document.createElement("span");
      span.className = "cmt-ops-head-label";
      span.textContent = "操作";
      if (headCell.firstChild) headCell.insertBefore(span, headCell.firstChild);
    }

    bodyRows(table).forEach(function (tr) {
      var cell = tr.cells[opsIdx] || tr.children[opsIdx];
      if (!cell || cell.getAttribute("data-cmt-ops") === "1") return;
      var btns = qsa("button, a", cell);
      if (!btns.length) return;
      cell.setAttribute("data-cmt-ops", "1");
      cell.classList.add("cmt-ops-cell");
      btns.forEach(function (b) { b.classList.add("cmt-ops-hidden"); });
      var more = document.createElement("button");
      more.type = "button";
      more.className = "cmt-ops-btn";
      more.title = "更多操作";
      more.setAttribute("aria-label", "更多操作");
      more.innerHTML = '<svg viewBox="0 0 16 16" width="16" height="16" fill="currentColor"><circle cx="3" cy="8" r="1.5"/><circle cx="8" cy="8" r="1.5"/><circle cx="13" cy="8" r="1.5"/></svg>';
      more.addEventListener("click", function (ev) {
        ev.preventDefault();
        ev.stopPropagation();
        openOpsMenu(more, cell);
      });
      cell.insertBefore(more, cell.firstChild);
    });
  }

  function bindHeader(table) {
    if (table.getAttribute("data-cmt-ux") === "1") return;
    var ths = headerCells(table);
    if (ths.length < 2) return;
    table.setAttribute("data-cmt-ux", "1");

    ths.forEach(function (th, idx) {
      if (/选择|操作/.test(cellText(th))) return;
      th.classList.add("cmt-sortable", "cmt-resizable");

      th.addEventListener("click", function (ev) {
        if (th.classList.contains("cmt-resizing")) return;
        if (ev.target && ev.target.closest && ev.target.closest(".cmt-ops-btn")) return;
        var dir = th.classList.contains("cmt-sort-asc") ? "desc" : "asc";
        ths.forEach(function (t) { t.classList.remove("cmt-sort-asc", "cmt-sort-desc"); });
        th.classList.add(dir === "asc" ? "cmt-sort-asc" : "cmt-sort-desc");
        sortTable(table, idx, dir);
        // 排序后重新挂操作按钮（行被重排）
        qsa("tr", table).forEach(function () {});
        collapseOps(table);
      });

      var startX = 0, startW = 0;
      function onMove(e) {
        var w = Math.max(72, startW + (e.clientX - startX));
        th.style.width = w + "px";
        th.style.minWidth = w + "px";
      }
      function onUp() {
        th.classList.remove("cmt-resizing");
        document.removeEventListener("mousemove", onMove);
        document.removeEventListener("mouseup", onUp);
      }
      th.addEventListener("mousedown", function (e) {
        var rect = th.getBoundingClientRect();
        if (e.clientX - rect.left < rect.width - 12) return;
        e.preventDefault();
        e.stopPropagation();
        startX = e.clientX;
        startW = rect.width;
        th.classList.add("cmt-resizing");
        document.addEventListener("mousemove", onMove);
        document.addEventListener("mouseup", onUp);
      });
    });
  }

  function ensureTrack(wrap, table, pos) {
    var attr = "data-cmt-hs-" + pos;
    var id = table.getAttribute(attr) || "";
    var track = id ? document.getElementById(id) : null;
    if (track) return track;
    id = "cmt-hs-" + pos + "-" + Math.random().toString(36).slice(2, 7);
    table.setAttribute(attr, id);
    track = document.createElement("div");
    track.id = id;
    track.className = "cmt-hscroll cmt-hscroll--" + pos;
    track.innerHTML = '<div class="cmt-hscroll-inner"></div>';
    if (pos === "top") wrap.insertBefore(track, table);
    else wrap.appendChild(track);
    return track;
  }

  function addHScroll(table) {
    var wrap = table.closest(".el-card") || table.parentElement;
    if (!wrap) return;
    var top = ensureTrack(wrap, table, "top");
    var bot = ensureTrack(wrap, table, "bottom");
    var body = qs(".el-table__body-wrapper", table);
    if (!body) return;

    if (!table.getAttribute("data-cmt-hs-bind")) {
      table.setAttribute("data-cmt-hs-bind", "1");
      var syncing = false;
      function syncFrom(el) {
        return function () {
          if (syncing) return;
          syncing = true;
          var L = el.scrollLeft;
          body.scrollLeft = L;
          top.scrollLeft = L;
          bot.scrollLeft = L;
          syncing = false;
        };
      }
      top.addEventListener("scroll", syncFrom(top));
      bot.addEventListener("scroll", syncFrom(bot));
      body.addEventListener("scroll", syncFrom(body));
    }

    var w = Math.max(
      body.scrollWidth,
      table.scrollWidth,
      (qs(".el-table__header-wrapper", table) || {}).scrollWidth || 0
    );
    [top, bot].forEach(function (track) {
      var inner = qs(".cmt-hscroll-inner", track);
      if (inner) inner.style.width = w + "px";
      var overflow = w > (wrap.clientWidth || 0) + 2;
      track.style.display = overflow ? "block" : "none";
    });
  }

  function scan() {
    qsa(".el-table").forEach(function (table) {
      bindHeader(table);
      collapseOps(table);
      addHScroll(table);
    });
  }

  function onRoute() {
    setTimeout(scan, 150);
    setTimeout(scan, 500);
    setTimeout(scan, 1200);
    setTimeout(scan, 2500);
  }

  var ps = history.pushState;
  history.pushState = function () {
    var r = ps.apply(this, arguments);
    onRoute();
    return r;
  };
  var rs = history.replaceState;
  history.replaceState = function () {
    var r = rs.apply(this, arguments);
    onRoute();
    return r;
  };
  window.addEventListener("popstate", onRoute);
  window.addEventListener("hashchange", onRoute);
  setInterval(scan, 2000);

  if (document.readyState === "complete") onRoute();
  else window.addEventListener("load", onRoute);
})();
