/* 登录验证码 — 稳定版：只在 DOMContentLoaded / load 各尝试一次，不用定时器/观察器 */
(function () {
  if (window.__cmtCaptchaInstalled) return;
  window.__cmtCaptchaInstalled = true;

  var captchaId = "";

  function loadCaptcha(img) {
    fetch("/api/v1/auth/captcha")
      .then(function (r) { return r.json(); })
      .then(function (b) {
        var d = (b && b.data) || {};
        captchaId = d.id || "";
        window.__cmtCaptchaId = captchaId;
        if (img && d.image) img.src = d.image;
      })
      .catch(function () {});
  }

  function tryInject() {
    if (document.getElementById("cmt-captcha-input")) return;
    var pwd = document.querySelector("input[type=password]");
    if (!pwd) return;
    var host = pwd.closest(".el-form-item") || pwd.parentElement;
    if (!host || !host.parentElement) return;

    var row = document.createElement("div");
    row.className = "el-form-item";
    row.style.marginTop = "12px";
    row.innerHTML =
      '<div style="display:flex;gap:8px;align-items:center">' +
      '<input id="cmt-captcha-input" maxlength="4" inputmode="numeric" placeholder="4 位数字验证码" ' +
      'style="flex:1;height:40px;padding:0 12px;border:1px solid #dcdfe6;border-radius:4px;font-size:14px" />' +
      '<img id="cmt-captcha-img" alt="验证码" title="点击刷新" ' +
      'style="width:110px;height:40px;border:1px solid #dcdfe6;border-radius:4px;background:#fff;cursor:pointer" />' +
      "</div>";
    host.parentElement.insertBefore(row, host.nextSibling);

    var img = document.getElementById("cmt-captcha-img");
    var input = document.getElementById("cmt-captcha-input");
    if (img) {
      img.onclick = function () {
        loadCaptcha(img);
        if (input) input.value = "";
      };
      loadCaptcha(img);
    }
  }

  function hookXHR() {
    if (window.__cmtCaptchaXHR) return;
    window.__cmtCaptchaXHR = true;
    var open = XMLHttpRequest.prototype.open;
    var send = XMLHttpRequest.prototype.send;
    XMLHttpRequest.prototype.open = function (m, u) {
      this.__u = u;
      return open.apply(this, arguments);
    };
    XMLHttpRequest.prototype.send = function (body) {
      try {
        if (this.__u && String(this.__u).indexOf("/auth/login") >= 0 && body) {
          var p = typeof body === "string" ? JSON.parse(body) : body;
          var ci = document.getElementById("cmt-captcha-input");
          p.captchaId = window.__cmtCaptchaId || "";
          p.captcha = ci ? String(ci.value || "").trim() : "";
          body = JSON.stringify(p);
        }
      } catch (e) {}
      return send.call(this, body);
    };
  }

  function boot() {
    hookXHR();
    tryInject();
    // 仅三次延迟重试，覆盖 Vue 异步挂载；之后不再打扰
    var n = 0;
    var id = setInterval(function () {
      tryInject();
      n += 1;
      if (n >= 3 || document.getElementById("cmt-captcha-input")) {
        clearInterval(id);
      }
    }, 800);
  }

  if (document.readyState === "complete") boot();
  else window.addEventListener("load", boot);
})();
