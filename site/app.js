// Gitpane 官网：语言切换、安装命令、最新版本号、可点击的终端演示。
(() => {
  "use strict";
  const LANGS = ["zh-CN", "zh-TW", "en", "ja", "ko"];
  const $ = (s) => document.querySelector(s);

  // ------------------------------------------------------------ 语言

  function pickLang() {
    const q = new URLSearchParams(location.search).get("lang");
    if (LANGS.includes(q)) return q;
    try {
      const saved = localStorage.getItem("gitpane-lang");
      if (LANGS.includes(saved)) return saved;
    } catch (e) { /* 隐私模式等情况下读不到，忽略 */ }
    for (const l of navigator.languages || [navigator.language || "en"]) {
      const s = l.toLowerCase();
      if (/^zh-(tw|hk|mo)|hant/.test(s)) return "zh-TW";
      if (s.startsWith("zh")) return "zh-CN";
      if (s.startsWith("ja")) return "ja";
      if (s.startsWith("ko")) return "ko";
      if (s.startsWith("en")) return "en";
    }
    return "en";
  }

  let lang = pickLang();
  const t = () => window.I18N[lang];

  function applyLang() {
    const s = t();
    document.documentElement.lang = lang;
    document.title = s.title;
    $("#lang").value = lang;
    document.querySelectorAll("[data-t]").forEach((el) => {
      const v = s[el.dataset.t];
      if (typeof v === "string") el.textContent = v;
    });
    document.querySelectorAll("[data-t-label]").forEach((el) => {
      el.setAttribute("aria-label", s[el.dataset.tLabel]);
    });
    // 亮点：每条是紫色分支上的一个提交
    $("#hl-list").innerHTML = s.hl.map((h) => `<li><strong>${h.t}</strong><p>${h.d}</p></li>`).join("");
    renderUsage(s.usage);
    renderLatest();
    syncThemeButton();
    loadScreens();
    if (typeof placeRoot === "function") requestAnimationFrame(placeRoot);
  }

  $("#lang").addEventListener("change", (e) => {
    lang = e.target.value;
    try { localStorage.setItem("gitpane-lang", lang); } catch (err) { /* 忽略 */ }
    const u = new URL(location.href);
    u.searchParams.set("lang", lang);
    history.replaceState(null, "", u);
    applyLang();
  });

  const esc = (s) => s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));

  function renderUsage(text) {
    $("#usage").innerHTML = text.split("\n").map((line) => {
      const i = line.indexOf("#");
      const cmd = i >= 0 ? line.slice(0, i) : line;
      const note = i >= 0 ? `<span class="c">${esc(line.slice(i))}</span>` : "";
      return esc(cmd).replace(/(--[a-z]+)/g, '<span class="f">$1</span>') + note;
    }).join("\n");
  }

  // ------------------------------------------------------------ 安装命令

  const CMDS = {
    mac: ["brew tap 0xBB2B/gitpane https://github.com/0xBB2B/gitpane", "brew install 0xBB2B/gitpane/gitpane"],
    win: ["scoop bucket add gitpane https://github.com/0xBB2B/gitpane", "scoop install gitpane"],
    linux: ["go install github.com/0xBB2B/gitpane@latest"],
  };
  const ua = navigator.userAgent;
  let os = /Windows/.test(ua) ? "win" : /Mac/.test(ua) ? "mac" : /Linux|X11/.test(ua) ? "linux" : "mac";

  function setOS(next, focus) {
    os = next;
    document.querySelectorAll(".os button").forEach((b) => {
      const on = b.dataset.os === os;
      b.setAttribute("aria-selected", on);
      b.tabIndex = on ? 0 : -1;
      if (on && focus) b.focus();
    });
    $("#cmd").setAttribute("aria-labelledby", "tab-" + os);
    $("#cmd-text").innerHTML = CMDS[os].map((c) => `<span class="ln">${esc(c)}</span>`).join("");
  }
  document.querySelectorAll(".os button").forEach((b) => {
    b.addEventListener("click", () => setOS(b.dataset.os));
    b.addEventListener("keydown", (e) => {
      const order = ["mac", "win", "linux"];
      const i = order.indexOf(os);
      if (e.key === "ArrowRight") setOS(order[(i + 1) % 3], true);
      else if (e.key === "ArrowLeft") setOS(order[(i + 2) % 3], true);
      else return;
      e.preventDefault();
    });
  });
  setOS(os);

  const copyBtn = $("#copy");
  let copyTimer;
  copyBtn.addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText(CMDS[os].join("\n"));
    } catch (e) {
      // 不支持剪贴板时，选中文字让用户自己复制
      const r = document.createRange();
      r.selectNodeContents($("#cmd-text"));
      const sel = getSelection();
      sel.removeAllRanges();
      sel.addRange(r);
      return;
    }
    copyBtn.textContent = t().copied;
    copyBtn.classList.add("done");
    clearTimeout(copyTimer);
    copyTimer = setTimeout(() => {
      copyBtn.textContent = t().copy;
      copyBtn.classList.remove("done");
    }, 1800);
  });

  // ------------------------------------------------------------ 最新版本（从 GitHub Releases 读取，发新版不用改网页）

  let latest = null;
  function renderLatest() {
    const a = $("#latest");
    if (!latest) return;
    a.textContent = t().latest.replace("{v}", latest.tag);
    a.href = latest.url;
    a.hidden = false;
  }
  fetch("https://api.github.com/repos/0xBB2B/gitpane/releases/latest", { headers: { Accept: "application/vnd.github+json" } })
    .then((r) => (r.ok ? r.json() : null))
    .then((j) => {
      if (j && j.tag_name) {
        latest = { tag: j.tag_name, url: j.html_url };
        renderLatest();
      }
    })
    .catch(() => { /* 拿不到就不显示版本号 */ });

  // ------------------------------------------------------------ 主题：默认跟随系统，手动切换后记住

  const root = document.documentElement;
  const sysLight = matchMedia("(prefers-color-scheme: light)");
  const theme = () => root.dataset.theme || (sysLight.matches ? "light" : "dark");
  function syncThemeButton() {
    const b = $("#theme");
    const label = theme() === "light" ? t().toDark : t().toLight;
    b.setAttribute("aria-label", label);
    b.title = label;
  }
  $("#theme").addEventListener("click", () => {
    const next = theme() === "light" ? "dark" : "light";
    root.dataset.theme = next;
    try { localStorage.setItem("gitpane-theme", next); } catch (e) { /* 忽略 */ }
    syncThemeButton();
    loadScreens();
  });
  sysLight.addEventListener("change", () => {
    if (root.dataset.theme) return; // 手动选过就不跟随系统
    syncThemeButton();
    loadScreens();
  });

  // ------------------------------------------------------------ 终端演示

  const term = $("#term");
  const rowsEl = $("#rows");
  const hotEl = $("#hot");
  const cache = {};
  let data = null;
  let screen = "graph";
  let loadingLang = null;

  function loadScreens() {
    // 终端演示跟着主题走：浅色主题下显示 gitpane 浅色配色的界面
    const want = lang + (theme() === "light" ? ".light" : "");
    if (cache[want]) {
      data = cache[want];
      draw();
      return;
    }
    loadingLang = want;
    fetch(`screens/${want}.json`)
      .then((r) => r.json())
      .then((j) => {
        cache[want] = j;
        if (loadingLang === want) {
          data = j;
          draw();
        }
      })
      .catch(() => { $(".demo").hidden = true; });
  }

  function draw() {
    if (!data) return;
    rowsEl.innerHTML = data.screens[screen].map((r) => `<div class="r">${r}</div>`).join("");
    hotEl.innerHTML = "";
    const names = t().to;
    for (const h of data.hot[screen]) {
      const b = document.createElement("button");
      b.type = "button";
      b.style.cssText = `--r:${h.r};--c:${h.c};--w:${h.w}`;
      b.setAttribute("aria-label", names[h.to]);
      b.title = names[h.to];
      b.addEventListener("click", () => go(h.to, true));
      hotEl.appendChild(b);
    }
  }

  function go(next, keepFocus) {
    if (!data || !data.screens[next]) return;
    screen = next;
    term.classList.remove("intro");
    draw();
    if (keepFocus) term.focus({ preventScroll: true });
  }

  // 键盘：和 gitpane 本身一致
  term.addEventListener("keydown", (e) => {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    const k = e.key;
    let next = null;
    if (screen === "picker") {
      if (k === "Escape" || k === "Enter" || k === "q") next = "graphf";
    } else if (k === "1") next = "changes";
    else if (k === "2") next = screen === "changes" ? "graph" : screen;
    else if (k === "b") next = "picker";
    if (next) {
      e.preventDefault();
      go(next, true);
    }
  });

  // 第一次滚到终端时，可点的地方亮两下
  if ("IntersectionObserver" in window) {
    const io = new IntersectionObserver((es) => {
      if (es.some((x) => x.isIntersecting)) {
        term.classList.add("intro");
        io.disconnect();
      }
    }, { threshold: 0.5 });
    io.observe(term);
  }

  // 主干的下端停在页尾那个提交点上（页尾高度随语言、窗口宽度变化）
  function placeRoot() {
    const g = $("#main"), r = $("#root");
    const y = g.getBoundingClientRect().bottom - r.getBoundingClientRect().top;
    const lh = parseFloat(getComputedStyle(r).lineHeight) || 24;
    g.style.setProperty("--root-y", (y - lh * 0.55) + "px");
  }
  addEventListener("resize", placeRoot);

  applyLang();
  placeRoot();
})();
