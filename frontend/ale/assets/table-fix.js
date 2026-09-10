/* 宽表固定列：取消 sticky，防抖观察，禁止卡死 */
(function () {
  if (window.__cmtTableFixInstalled) return;
  window.__cmtTableFixInstalled = true;

  var working = false;

  function unstick() {
    if (working) return;
    working = true;
    try {
      var nodes = document.querySelectorAll(
        ".el-table-fixed-column--right, .el-table-fixed-column--left, .el-table__fixed-right"
      );
      for (var i = 0; i < nodes.length; i++) {
        var el = nodes[i];
        el.style.setProperty("position", "static", "important");
        el.style.setProperty("right", "auto", "important");
        el.style.setProperty("left", "auto", "important");
        el.style.setProperty("z-index", "auto", "important");
        if (el.classList.contains("el-table__fixed-right")) {
          el.style.setProperty("display", "none", "important");
        }
      }
      var bodies = document.querySelectorAll(".el-table__body-wrapper");
      for (var j = 0; j < bodies.length; j++) {
        bodies[j].style.setProperty("overflow-x", "auto", "important");
      }
    } finally {
      working = false;
    }
  }

  var timer = null;
  function schedule() {
    if (timer) return;
    timer = setTimeout(function () {
      timer = null;
      unstick();
    }, 250);
  }

  function boot() {
    unstick();
    var obs = new MutationObserver(function (muts) {
      for (var i = 0; i < muts.length; i++) {
        if (muts[i].addedNodes && muts[i].addedNodes.length) {
          schedule();
          return;
        }
      }
    });
    obs.observe(document.body, { childList: true });
    window.addEventListener("resize", schedule);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
})();
