// ActaCron Client Application
(function () {
  let activeScript = null; // { package, name, file_path, cron_expr, is_mcp, ... }
  let allFunctions = [];

  document.addEventListener("DOMContentLoaded", () => {
    initI18nAndTheme();
    initNav();
    initSettingsTabs();
    initWorkspaceEvents();
    initLogsEvents();
    initSettingsEvents();
    initModals();
    initResizers();

    // Start background health polling
    pollHealth();
    setInterval(pollHealth, 5000);

    // Initial load
    loadFunctions();
    loadLogs();
  });

  // --- Theme & i18n Initialization ---
  function initI18nAndTheme() {
    // Language
    const savedLang = localStorage.getItem("actacron_lang") || "en";
    window.I18n.setLanguage(savedLang);

    // Theme & Density
    const savedTheme = localStorage.getItem("actacron_theme") || "emerald";
    const savedDensity = localStorage.getItem("actacron_density") || "normal";
    const savedFontSize = localStorage.getItem("actacron_fontsize") || "13.5px";

    applyTheme(savedTheme);
    applyDensity(savedDensity);
    applyFontSize(savedFontSize);

    // Header buttons
    document.getElementById("topLangToggle").addEventListener("click", () => {
      const next = window.I18n.getLanguage() === "en" ? "vi" : "en";
      window.I18n.setLanguage(next);
      updateInspectorCronHuman();
    });

    document.getElementById("btnSyncAll").addEventListener("click", syncAllGit);
  }

  function applyTheme(theme) {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem("actacron_theme", theme);
    const select = document.getElementById("uiThemeSelect");
    if (select) select.value = theme;
  }

  function applyDensity(density) {
    document.documentElement.setAttribute("data-density", density);
    localStorage.setItem("actacron_density", density);
    const select = document.getElementById("uiDensitySelect");
    if (select) select.value = density;
  }

  function applyFontSize(size) {
    document.documentElement.style.setProperty("--editor-font-size", size);
    localStorage.setItem("actacron_fontsize", size);
    const select = document.getElementById("uiFontSizeSelect");
    if (select) select.value = size;
  }

  // --- Navigation Router ---
  function initNav() {
    document.querySelectorAll(".nav-item").forEach(item => {
      item.addEventListener("click", () => {
        document.querySelectorAll(".nav-item").forEach(n => n.classList.remove("active"));
        document.querySelectorAll(".view-page").forEach(p => p.classList.remove("active"));

        item.classList.add("active");
        const viewId = item.getAttribute("data-view");
        const page = document.getElementById("view-" + viewId);
        if (page) page.classList.add("active");

        if (viewId === "overview") refreshOverview();
        if (viewId === "workspace") loadFunctions();
        if (viewId === "logs") loadLogs();
        if (viewId === "mcp") loadMcpHub();
        if (viewId === "settings") loadAllSettings();
      });
    });
  }

  // --- Settings Tabs ---
  function initSettingsTabs() {
    document.querySelectorAll(".settings-tab-btn").forEach(btn => {
      btn.addEventListener("click", () => {
        document.querySelectorAll(".settings-tab-btn").forEach(b => b.classList.remove("active"));
        document.querySelectorAll(".settings-panel").forEach(p => p.classList.remove("active"));

        btn.classList.add("active");
        const tab = btn.getAttribute("data-tab");
        const panel = document.getElementById("panel-" + tab);
        if (panel) panel.classList.add("active");
      });
    });
  }

  // --- Health Polling ---
  async function pollHealth() {
    try {
      const res = await fetch("/api/health");
      if (!res.ok) return;
      const data = await res.json();

      let ram = data.memory_mb || data.memory_alloc_mb || "--";
      if (!String(ram).includes("MB")) ram = ram + " MB";

      document.getElementById("headerRam").textContent = ram;
      document.getElementById("headerCrons").textContent = data.active_jobs ?? data.active_cron_jobs ?? 0;
      document.getElementById("statRam").textContent = ram;
    } catch (_) {}
  }

  // --- Overview Refresh ---
  async function refreshOverview() {
    await loadFunctions();
    await loadLogs();

    document.getElementById("statTotalFuncs").textContent = allFunctions.length;

    const cronFuncs = allFunctions.filter(f => f.cron_expr && f.cron_expr.trim() !== "");
    document.getElementById("statActiveCrons").textContent = cronFuncs.filter(f => f.is_enabled !== false).length;

    const mcpFuncs = allFunctions.filter(f => f.is_mcp);
    document.getElementById("statMcpTools").textContent = mcpFuncs.length;

    // Cron table
    const cronTbody = document.getElementById("overviewCronTable");
    cronTbody.innerHTML = "";

    let cronJobs = [];
    try {
      const res = await fetch("/api/cron");
      if (res.ok) cronJobs = await res.json();
    } catch (_) {}

    const jobMap = {};
    if (Array.isArray(cronJobs)) {
      cronJobs.forEach(j => { jobMap[`${j.package_name}/${j.function_name}`] = j; });
    }

    if (cronFuncs.length === 0) {
      cronTbody.innerHTML = '<tr><td colspan="5" style="text-align:center; color:var(--color-muted-foreground);">No active crons configured</td></tr>';
    } else {
      cronFuncs.forEach(fn => {
        const tr = document.createElement("tr");
        const fullKey = `${fn.package}/${fn.name}`;
        const job = jobMap[fullKey] || {};
        const status = job.status || (fn.is_enabled !== false ? "active" : "paused");
        const isEnabled = fn.is_enabled !== false && status !== "paused";

        let badgeClass = "badge-cron-active";
        if (status === "paused") badgeClass = "badge-cron-paused";
        else if (status === "pending") badgeClass = "badge-cron-pending";
        else if (status === "expired") badgeClass = "badge-cron-expired";
        else if (status === "running") badgeClass = "badge-cron-running";
        else if (status === "completed") badgeClass = "badge-cron-paused";

        const human = window.I18n.cronToString(fn.cron_expr);
        const tzText = fn.timezone ? `<span style="font-size:10px; color:var(--color-muted-foreground); margin-left:4px;">[${fn.timezone}]</span>` : '';

        let nextRunText = "-";
        if ((status === "active" || status === "pending" || status === "running") && job.next_run && !job.next_run.startsWith("0001-01-01")) {
          const nextDate = new Date(job.next_run);
          const diffMs = nextDate.getTime() - Date.now();
          if (diffMs > 0) {
            const diffMin = Math.round(diffMs / 60000);
            nextRunText = diffMin < 60 ? `in ${diffMin}m` : nextDate.toLocaleTimeString();
          } else {
            nextRunText = nextDate.toLocaleTimeString();
          }
        }

        const statusLabel = (window.I18n && window.I18n.t("status_" + status)) || status;

        tr.innerHTML = `
          <td><strong>${fn.package}/${fn.name}</strong></td>
          <td><code>${fn.cron_expr}</code> ${tzText}<div style="font-size:11px; color:var(--color-accent); margin-top:2px;">${human}</div></td>
          <td><span class="${badgeClass}">${statusLabel}</span></td>
          <td style="font-size:12px;">${nextRunText}</td>
          <td>
            <div style="display:flex; align-items:center; gap:8px;">
              <button class="btn btn-secondary btn-sm" onclick="runFunction('${fn.package}', '${fn.name}')">Run Now</button>
              <label class="switch-label" title="Toggle Cron Active" style="margin:0;">
                <input type="checkbox" class="switch-input cron-table-toggle" data-pkg="${fn.package}" data-name="${fn.name}" ${isEnabled ? "checked" : ""}>
              </label>
            </div>
          </td>
        `;
        cronTbody.appendChild(tr);
      });

      cronTbody.querySelectorAll(".cron-table-toggle").forEach(toggle => {
        toggle.addEventListener("change", async (e) => {
          const pkg = e.target.getAttribute("data-pkg");
          const name = e.target.getAttribute("data-name");
          const enabled = e.target.checked;
          await toggleCronJob(pkg, name, enabled);
        });
      });
    }

    // Recent logs
    try {
      const res = await fetch("/api/logs?limit=5");
      if (res.ok) {
        const data = await res.json();
        const logs = data.logs || [];
        const tbody = document.getElementById("overviewRecentLogs");
        tbody.innerHTML = "";
        if (logs.length === 0) {
          tbody.innerHTML = '<tr><td colspan="4" style="text-align:center; color:var(--color-muted-foreground);">No executions yet</td></tr>';
        } else {
          logs.forEach(l => {
            const tr = document.createElement("tr");
            const badgeClass = l.status === "success" ? "badge-status-success" : "badge-status-error";
            tr.innerHTML = `
              <td style="font-size:11px; color:var(--color-muted-foreground);">${new Date(l.created_at).toLocaleTimeString()}</td>
              <td>${l.function_name}</td>
              <td><span class="${badgeClass}">${l.status}</span></td>
              <td>${l.duration_ms}ms</td>
            `;
            tbody.appendChild(tr);
          });
        }
      }
    } catch (_) {}
  }

  async function toggleCronJob(pkg, name, enabled) {
    const fullKey = `${pkg}/${name}`;
    try {
      const res = await fetch("/api/cron", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: "toggle", target: fullKey, enable: enabled })
      });
      if (res.ok) {
        const fn = allFunctions.find(f => f.package === pkg && f.name === name);
        if (fn) fn.is_enabled = enabled;
        if (activeScript && activeScript.package === pkg && activeScript.name === name) {
          activeScript.is_enabled = enabled;
          const inspToggle = document.getElementById("inspCronToggle");
          if (inspToggle) inspToggle.checked = enabled;
        }
        renderTree(allFunctions);
        loadOverview();
      }
    } catch (err) {
      alert("Failed to toggle cron: " + err.message);
    }
  }

  // --- Functions / Workspace ---
  async function loadFunctions() {
    try {
      const res = await fetch("/api/functions");
      if (!res.ok) return;
      const data = await res.json();
      allFunctions = Array.isArray(data) ? data : (data.functions || []);

      renderTree(allFunctions);
    } catch (err) {
      console.error("Failed to load functions", err);
    }
  }

  function renderTree(funcs) {
    const tree = document.getElementById("scriptsTree");
    tree.innerHTML = "";

    // Load collapsed package states from localStorage
    let collapsedPackages = new Set();
    try {
      const saved = localStorage.getItem("actacron_collapsed_pkgs");
      if (saved) collapsedPackages = new Set(JSON.parse(saved));
    } catch (e) {}

    // Group by package
    const grouped = {};
    funcs.forEach(f => {
      if (!grouped[f.package]) grouped[f.package] = [];
      grouped[f.package].push(f);
    });

    const pkgNames = Object.keys(grouped).sort((a, b) => {
      if (a === "_shared") return -1;
      if (b === "_shared") return 1;
      return a.localeCompare(b);
    });

    pkgNames.forEach(pkgName => {
      const isShared = pkgName === "_shared";
      const isCollapsed = collapsedPackages.has(pkgName);

      const groupLi = document.createElement("li");
      groupLi.className = "tree-package-group" + (isCollapsed ? " collapsed" : "");

      const headerDiv = document.createElement("div");
      headerDiv.className = "tree-package-header" + (isShared ? " shared-package" : "");

      const headerTitle = isShared 
        ? `<div style="display:flex; align-items:center; gap:6px; min-width:0; flex:1; overflow:hidden;"><span class="tree-chevron" style="flex-shrink:0;">▾</span><span>🔗</span> <strong style="overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">_shared</strong> <span class="badge-shared-library" style="flex-shrink:0;">LIB</span></div>`
        : `<div style="display:flex; align-items:center; gap:6px; min-width:0; flex:1; overflow:hidden;"><span class="tree-chevron" style="flex-shrink:0;">▾</span><svg style="flex-shrink:0;" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg><span style="overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title="${pkgName}">${pkgName}</span></div>`;

      const cfgTooltip = (window.I18n && window.I18n.t("ws_config_tooltip")) || "Workspace Settings & .env";
      const actionButtons = `
        <div style="display:flex; align-items:center; gap:4px; flex-shrink:0;">
          ${!isShared ? `<button class="btn-pkg-action btn-pkg-config" data-i18n-title="ws_config_tooltip" title="${cfgTooltip}" data-pkg="${pkgName}">⚙️</button>` : ''}
          <button class="btn-pkg-action btn-pkg-folder" title="Open '${pkgName}' in Explorer" data-pkg="${pkgName}">📁</button>
        </div>
      `;

      headerDiv.innerHTML = headerTitle + actionButtons;

      // Click header toggles collapse/expand (ignore if clicked on action buttons)
      headerDiv.addEventListener("click", (e) => {
        if (e.target.closest(".btn-pkg-action")) return;
        groupLi.classList.toggle("collapsed");
        if (groupLi.classList.contains("collapsed")) {
          collapsedPackages.add(pkgName);
        } else {
          collapsedPackages.delete(pkgName);
        }
        try {
          localStorage.setItem("actacron_collapsed_pkgs", JSON.stringify([...collapsedPackages]));
        } catch (err) {}
      });

      const folderBtn = headerDiv.querySelector(".btn-pkg-folder");
      if (folderBtn) {
        folderBtn.addEventListener("click", (e) => {
          e.stopPropagation();
          openWorkspaceFolder(pkgName);
        });
      }

      const configBtn = headerDiv.querySelector(".btn-pkg-config");
      if (configBtn) {
        configBtn.addEventListener("click", (e) => {
          e.stopPropagation();
          openWorkspaceConfigModal(pkgName);
        });
      }

      groupLi.appendChild(headerDiv);

      const fileUl = document.createElement("ul");
      fileUl.className = "tree-file-list";

      grouped[pkgName].forEach(fn => {
        const itemLi = document.createElement("li");
        itemLi.className = "tree-file-item";
        if (activeScript && activeScript.package === fn.package && (activeScript.file_path === fn.file_path || activeScript.name === fn.name)) {
          itemLi.classList.add("active");
        }

        const badges = [];
        if (fn.is_mcp) badges.push('<span class="badge-mcp-mini">MCP</span>');
        if (fn.cron_expr) {
          let badgeType = "badge-cron-mini";
          let badgeText = "CRON";

          const now = new Date();
          if (fn.cron_end && new Date(fn.cron_end) < now) {
            badgeType = "badge-cron-expired";
            badgeText = "EXPIRED";
          } else if (fn.max_runs > 0 && fn.run_count >= fn.max_runs) {
            badgeType = "badge-cron-paused";
            badgeText = "PAUSED";
          } else if (fn.is_enabled === false) {
            badgeType = "badge-cron-paused";
            badgeText = "PAUSED";
          } else if (fn.cron_start && new Date(fn.cron_start) > now) {
            badgeType = "badge-cron-pending";
            badgeText = "PENDING";
          }

          badges.push(`<span class="${badgeType}" style="font-size:9px; padding:1px 4px;">${badgeText}</span>`);
        }

        itemLi.innerHTML = `
          <span style="overflow:hidden; text-overflow:ellipsis; white-space:nowrap; min-width:0; flex:1; margin-right:6px;" title="${fn.name}">${fn.name}</span>
          <div style="display:flex; gap:4px; align-items:center; flex-shrink:0;">
            ${badges.join("")}
            ${!isShared ? `<button class="btn-tree-delete" title="Delete script" data-pkg="${fn.package}" data-name="${fn.name}">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
            </button>` : ''}
          </div>
        `;

        itemLi.addEventListener("click", () => openScript(fn));
        const delBtn = itemLi.querySelector(".btn-tree-delete");
        if (delBtn) {
          delBtn.addEventListener("click", (e) => {
            e.stopPropagation();
            deleteScript(fn.package, fn.file_path || (fn.name + ".js"), fn.name);
          });
        }
        fileUl.appendChild(itemLi);
      });

      groupLi.appendChild(fileUl);
      tree.appendChild(groupLi);
    });
  }

  async function openScript(fn) {
    activeScript = fn;
    const btnDel = document.getElementById("btnDeleteScript");
    if (btnDel) btnDel.style.display = "inline-flex";

    document.getElementById("currentFileTitle").innerHTML = `
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path></svg>
      <span>${fn.package}/${fn.file_path || (fn.name + ".js")}</span>
    `;

    // Fetch code content via /api/functions/code?key=pkg/name
    const key = fn.package + "/" + fn.name;
    try {
      const res = await fetch(`/api/functions/code?key=${encodeURIComponent(key)}`);
      if (res.ok) {
        const data = await res.json();
        document.getElementById("scriptEditor").value = data.code || "";
      }
    } catch (_) {}

    // Update inspector
    const cronToggle = document.getElementById("inspCronToggle");
    if (cronToggle) cronToggle.checked = (fn.is_enabled !== false && !!fn.cron_expr);
    document.getElementById("inspCronExpr").value = fn.cron_expr || "";
    updateInspectorCronHuman();

    const tzInput = document.getElementById("inspTimezone");
    if (tzInput) tzInput.value = fn.timezone || "";

    const startInput = document.getElementById("inspCronStart");
    if (startInput) startInput.value = fn.cron_start ? fn.cron_start.replace(" ", "T").substring(0, 16) : "";

    const endInput = document.getElementById("inspCronEnd");
    if (endInput) endInput.value = fn.cron_end ? fn.cron_end.replace(" ", "T").substring(0, 16) : "";

    const maxRunsInput = document.getElementById("inspMaxRuns");
    if (maxRunsInput) maxRunsInput.value = fn.max_runs ? fn.max_runs : "";

    const retryInput = document.getElementById("inspRetryCount");
    if (retryInput) retryInput.value = fn.retry_count ? fn.retry_count : "";

    const overlapInput = document.getElementById("inspNoOverlap");
    if (overlapInput) overlapInput.checked = fn.no_overlap !== false;

    const mcpToggle = document.getElementById("inspMcpToggle");
    if (mcpToggle) mcpToggle.checked = !!fn.is_mcp;

    // Update active workspace buttons
    const btnActiveWs = document.getElementById("btnActiveWsConfig");
    const inspWsSection = document.getElementById("inspectorWsSection");
    const inspWsName = document.getElementById("inspWsName");

    if (fn.package && fn.package !== "_shared") {
      if (btnActiveWs) btnActiveWs.style.display = "inline-flex";
      if (inspWsSection) inspWsSection.style.display = "block";
      if (inspWsName) inspWsName.textContent = fn.package;
    } else {
      if (btnActiveWs) btnActiveWs.style.display = "none";
      if (inspWsSection) inspWsSection.style.display = "none";
    }

    // Highlight in tree
    document.querySelectorAll(".tree-file-item").forEach(el => el.classList.remove("active"));
    renderTree(allFunctions);
  }

  function clearActiveScript() {
    activeScript = null;
    const titleEl = document.getElementById("currentFileTitle");
    if (titleEl) titleEl.innerHTML = "<span>Select a script to edit</span>";
    const editorEl = document.getElementById("scriptEditor");
    if (editorEl) editorEl.value = "";
    const btnDel = document.getElementById("btnDeleteScript");
    if (btnDel) btnDel.style.display = "none";
    const btnActiveWs = document.getElementById("btnActiveWsConfig");
    if (btnActiveWs) btnActiveWs.style.display = "none";
    const inspWsSection = document.getElementById("inspectorWsSection");
    if (inspWsSection) inspWsSection.style.display = "none";

    const cronToggle = document.getElementById("inspCronToggle");
    if (cronToggle) cronToggle.checked = false;
    const inspCron = document.getElementById("inspCronExpr");
    if (inspCron) inspCron.value = "";
    updateInspectorCronHuman();
    const tzInput = document.getElementById("inspTimezone");
    if (tzInput) tzInput.value = "";
    const startInput = document.getElementById("inspCronStart");
    if (startInput) startInput.value = "";
    const endInput = document.getElementById("inspCronEnd");
    if (endInput) endInput.value = "";
    const maxRunsInput = document.getElementById("inspMaxRuns");
    if (maxRunsInput) maxRunsInput.value = "";
    const retryInput = document.getElementById("inspRetryCount");
    if (retryInput) retryInput.value = "";
    const overlapInput = document.getElementById("inspNoOverlap");
    if (overlapInput) overlapInput.checked = true;
    const mcpToggle = document.getElementById("inspMcpToggle");
    if (mcpToggle) mcpToggle.checked = false;
  }

  async function deleteScript(pkg, file, name) {
    const t = window.I18n ? window.I18n.t : (k => k);
    let confirmMsg = (window.I18n && window.I18n.t("delete_confirm")) || `Are you sure you want to delete script '{name}' from package '{pkg}'? This cannot be undone.`;
    confirmMsg = confirmMsg.replace("{name}", name).replace("{pkg}", pkg);

    if (!confirm(confirmMsg)) return;

    try {
      const res = await fetch("/api/functions/delete", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ package: pkg, filename: file })
      });
      if (res.ok) {
        if (activeScript && activeScript.package === pkg && (activeScript.file_path === file || activeScript.name === name)) {
          clearActiveScript();
        }
        await loadFunctions();
        loadOverview();
      } else {
        const err = await res.json().catch(() => ({}));
        alert((t("delete_failed") || "Failed to delete: ") + (err.error || res.statusText));
      }
    } catch (e) {
      alert((t("delete_failed") || "Failed to delete: ") + e.message);
    }
  }

  async function openWorkspaceFolder(pkg = "") {
    try {
      const res = await fetch("/api/workspace/open", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ package: pkg })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        const t = window.I18n ? window.I18n.t : (k => k);
        alert((t("open_folder_failed") || "Failed to open directory: ") + (err.error || res.statusText));
      }
    } catch (e) {
      const t = window.I18n ? window.I18n.t : (k => k);
      alert((t("open_folder_failed") || "Failed to open directory: ") + e.message);
    }
  }

  function updateInspectorCronHuman() {
    const expr = document.getElementById("inspCronExpr").value;
    const humanEl = document.getElementById("inspCronHuman");
    if (expr && expr.trim() !== "") {
      humanEl.textContent = "(" + window.I18n.cronToString(expr) + ")";
    } else {
      humanEl.textContent = "";
    }
  }

  function updateJSDocTag(tag, value) {
    const editor = document.getElementById("scriptEditor");
    if (!editor) return;
    let code = editor.value;
    const tagRegex = new RegExp(`([ \\t]*\\*[ \\t]*@${tag}\\b)[ \\t]+([^\\r\\n]*)`);
    if (tagRegex.test(code)) {
      if (value !== null && value !== "" && value !== undefined) {
        code = code.replace(tagRegex, `$1 ${value}`);
      } else {
        code = code.replace(new RegExp(`([ \\t]*\\*[ \\t]*@${tag}\\b[^\\r\\n]*\\r?\\n?)`), "");
      }
    } else if (value !== null && value !== "" && value !== undefined) {
      if (code.includes("/**")) {
        code = code.replace(/(\/\*\*[\r\n]+)/, `$1 * @${tag} ${value}\n`);
      } else {
        code = `/**\n * @${tag} ${value}\n */\n` + code;
      }
    }
    editor.value = code;
  }

  function onInspectorCronChange() {
    const newCron = document.getElementById("inspCronExpr").value.trim();
    updateInspectorCronHuman();
    updateJSDocTag("cron", newCron);
    if (activeScript) activeScript.cron_expr = newCron;
  }

  function onInspectorTimezoneChange() {
    const tz = document.getElementById("inspTimezone").value.trim();
    updateJSDocTag("timezone", tz);
    if (activeScript) activeScript.timezone = tz;
  }

  function onInspectorCronStartChange() {
    const startVal = document.getElementById("inspCronStart").value.trim();
    const formatted = startVal ? startVal.replace("T", " ") : "";
    updateJSDocTag("cron_start", formatted);
    if (activeScript) activeScript.cron_start = formatted;
  }

  function onInspectorCronEndChange() {
    const endVal = document.getElementById("inspCronEnd").value.trim();
    const formatted = endVal ? endVal.replace("T", " ") : "";
    updateJSDocTag("cron_end", formatted);
    if (activeScript) activeScript.cron_end = formatted;
  }

  function onInspectorMaxRunsChange() {
    const runs = document.getElementById("inspMaxRuns").value.trim();
    updateJSDocTag("max_runs", runs && runs !== "0" ? runs : "");
    if (activeScript) activeScript.max_runs = parseInt(runs, 10) || 0;
  }

  function onInspectorRetryCountChange() {
    const retry = document.getElementById("inspRetryCount").value.trim();
    updateJSDocTag("retry", retry && retry !== "0" ? `${retry} 5s` : "");
    if (activeScript) activeScript.retry_count = parseInt(retry, 10) || 0;
  }

  function onInspectorNoOverlapChange() {
    const checked = document.getElementById("inspNoOverlap").checked;
    updateJSDocTag("no_overlap", checked ? "true" : "false");
    if (activeScript) activeScript.no_overlap = checked;
  }

  async function onInspectorCronToggle() {
    if (!activeScript) return;
    const isChecked = document.getElementById("inspCronToggle").checked;
    await toggleCronJob(activeScript.package, activeScript.name, isChecked);
  }

  function onInspectorMcpToggle() {
    const isMcp = document.getElementById("inspMcpToggle").checked;
    updateJSDocTag("mcp", isMcp ? "true" : "false");
    if (activeScript) activeScript.is_mcp = isMcp;
  }

  function syncInspectorFromCode() {
    const code = document.getElementById("scriptEditor").value;
    const cronMatch = code.match(/@cron[ \t]+([^\r\n*]+)/);
    const cronInput = document.getElementById("inspCronExpr");
    if (cronInput) {
      cronInput.value = cronMatch ? cronMatch[1].trim() : "";
      updateInspectorCronHuman();
    }

    const tzMatch = code.match(/@timezone[ \t]+([^\r\n*]+)/);
    const tzInput = document.getElementById("inspTimezone");
    if (tzInput && document.activeElement !== tzInput) {
      tzInput.value = tzMatch ? tzMatch[1].trim() : "";
    }

    const startMatch = code.match(/@cron_start[ \t]+([^\r\n*]+)/);
    const startInput = document.getElementById("inspCronStart");
    if (startInput && document.activeElement !== startInput) {
      startInput.value = startMatch ? startMatch[1].trim().replace(" ", "T").substring(0, 16) : "";
    }

    const endMatch = code.match(/@cron_end[ \t]+([^\r\n*]+)/);
    const endInput = document.getElementById("inspCronEnd");
    if (endInput && document.activeElement !== endInput) {
      endInput.value = endMatch ? endMatch[1].trim().replace(" ", "T").substring(0, 16) : "";
    }

    const maxRunsMatch = code.match(/@max_runs[ \t]+([^\r\n*]+)/);
    const maxRunsInput = document.getElementById("inspMaxRuns");
    if (maxRunsInput && document.activeElement !== maxRunsInput) {
      maxRunsInput.value = maxRunsMatch ? maxRunsMatch[1].trim() : "";
    }

    const retryMatch = code.match(/@retry[ \t]+([0-9]+)/);
    const retryInput = document.getElementById("inspRetryCount");
    if (retryInput && document.activeElement !== retryInput) {
      retryInput.value = retryMatch ? retryMatch[1].trim() : "";
    }

    const overlapMatch = code.match(/@no_overlap[ \t]+(true|false)/);
    const overlapCheckbox = document.getElementById("inspNoOverlap");
    if (overlapCheckbox && document.activeElement !== overlapCheckbox) {
      overlapCheckbox.checked = overlapMatch ? overlapMatch[1] === "true" : true;
    }

    const mcpMatch = code.match(/@mcp[ \t]+(true|false)/);
    const mcpToggle = document.getElementById("inspMcpToggle");
    if (mcpToggle && document.activeElement !== mcpToggle) {
      mcpToggle.checked = mcpMatch ? mcpMatch[1] === "true" : false;
    }
  }

  function initWorkspaceEvents() {
    const editor = document.getElementById("scriptEditor");

    // Hotkeys: Ctrl+S to save, Ctrl+Enter to run, Tab for indent
    editor.addEventListener("keydown", (e) => {
      if (e.ctrlKey && e.key === "s") {
        e.preventDefault();
        saveCurrentScript();
      } else if (e.ctrlKey && e.key === "Enter") {
        e.preventDefault();
        runCurrentScript();
      } else if (e.key === "Tab") {
        e.preventDefault();
        const start = editor.selectionStart;
        const end = editor.selectionEnd;
        editor.value = editor.value.substring(0, start) + "  " + editor.value.substring(end);
        editor.selectionStart = editor.selectionEnd = start + 2;
      }
    });

    editor.addEventListener("input", syncInspectorFromCode);
    const cronToggle = document.getElementById("inspCronToggle");
    if (cronToggle) cronToggle.addEventListener("change", onInspectorCronToggle);
    const cronExpr = document.getElementById("inspCronExpr");
    if (cronExpr) cronExpr.addEventListener("input", onInspectorCronChange);
    const tzInput = document.getElementById("inspTimezone");
    if (tzInput) tzInput.addEventListener("input", onInspectorTimezoneChange);
    const cronStart = document.getElementById("inspCronStart");
    if (cronStart) cronStart.addEventListener("change", onInspectorCronStartChange);
    const cronEnd = document.getElementById("inspCronEnd");
    if (cronEnd) cronEnd.addEventListener("change", onInspectorCronEndChange);
    const maxRuns = document.getElementById("inspMaxRuns");
    if (maxRuns) maxRuns.addEventListener("input", onInspectorMaxRunsChange);
    const retryCount = document.getElementById("inspRetryCount");
    if (retryCount) retryCount.addEventListener("input", onInspectorRetryCountChange);
    const noOverlap = document.getElementById("inspNoOverlap");
    if (noOverlap) noOverlap.addEventListener("change", onInspectorNoOverlapChange);
    const mcpToggle = document.getElementById("inspMcpToggle");
    if (mcpToggle) mcpToggle.addEventListener("change", onInspectorMcpToggle);

    document.getElementById("btnSaveScript").addEventListener("click", saveCurrentScript);
    document.getElementById("btnRunScript").addEventListener("click", runCurrentScript);
    document.getElementById("btnRunInspector").addEventListener("click", runCurrentScript);

    const btnDelScript = document.getElementById("btnDeleteScript");
    if (btnDelScript) {
      btnDelScript.addEventListener("click", () => {
        if (activeScript) {
          deleteScript(activeScript.package, activeScript.file_path || (activeScript.name + ".js"), activeScript.name);
        }
      });
    }

    const btnOpenWs = document.getElementById("btnOpenWorkspaceFolder");
    if (btnOpenWs) {
      btnOpenWs.addEventListener("click", () => openWorkspaceFolder(""));
    }
  }

  async function saveCurrentScript() {
    if (!activeScript) return;
    const content = document.getElementById("scriptEditor").value;
    const filename = activeScript.file_path || (activeScript.name.endsWith(".js") ? activeScript.name : activeScript.name + ".js");

    try {
      const res = await fetch("/api/functions/save", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          package: activeScript.package,
          filename: filename,
          code: content
        })
      });
      if (res.ok) {
        showConsole("Script saved successfully.", 0);
        await loadFunctions();
      } else {
        const err = await res.json();
        showConsole("Failed to save: " + (err.error || "Unknown error"), 0);
      }
    } catch (err) {
      showConsole("Error saving script: " + err.message, 0);
    }
  }

  async function runCurrentScript() {
    if (!activeScript) return;
    const funcKey = activeScript.package + "/" + activeScript.name;
    let payload = {};
    try {
      const raw = document.getElementById("inspTestPayload").value;
      if (raw.trim() !== "") payload = JSON.parse(raw);
    } catch (err) {
      showConsole("Invalid JSON input payload: " + err.message, 0);
      return;
    }

    showConsole(window.I18n.t("running_func"), 0);
    const start = performance.now();

    try {
      const res = await fetch("/api/run", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          key: funcKey,
          params: payload
        })
      });

      const data = await res.json();
      const elapsed = Math.round(performance.now() - start);

      let out = "";
      if (data.console_logs && data.console_logs.trim() !== "") {
        out += "--- Logs ---\n" + data.console_logs + "\n\n";
      }
      out += "--- Return Value ---\n" + (typeof data.output === "object" ? JSON.stringify(data.output, null, 2) : data.output);
      if (data.error) {
        out += "\n\n--- Error ---\n" + data.error;
      }

      showConsole(out, data.duration_ms || elapsed);
    } catch (err) {
      showConsole("Network or server error: " + err.message, 0);
    }
  }

  window.runFunction = async function (pkg, name) {
    const fn = allFunctions.find(f => f.package === pkg && f.name === name);
    if (fn) {
      document.querySelector('[data-view="workspace"]').click();
      await openScript(fn);
      runCurrentScript();
    }
  };

  function showConsole(text, durationMs) {
    document.getElementById("outputConsole").textContent = text;
    document.getElementById("execDuration").textContent = durationMs + "ms";
  }

  // --- Logs Viewer ---
  function initLogsEvents() {
    document.getElementById("btnRefreshLogs").addEventListener("click", loadLogs);
    document.getElementById("logFilterStatus").addEventListener("change", loadLogs);
    document.getElementById("logFilterFunc").addEventListener("keyup", (e) => {
      if (e.key === "Enter") loadLogs();
    });
  }

  async function loadLogs() {
    const func = document.getElementById("logFilterFunc").value;
    const status = document.getElementById("logFilterStatus").value;
    const url = `/api/logs?func=${encodeURIComponent(func)}&status=${encodeURIComponent(status)}&limit=50`;

    try {
      const res = await fetch(url);
      if (!res.ok) return;
      const data = await res.json();
      const tbody = document.getElementById("logsTableBody");
      tbody.innerHTML = "";

      const logs = data.logs || [];
      if (logs.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;">No logs found</td></tr>';
        return;
      }

      logs.forEach(l => {
        const tr = document.createElement("tr");
        const badgeClass = l.status === "success" ? "badge-status-success" : "badge-status-error";
        tr.innerHTML = `
          <td style="font-size:12px; color:var(--color-muted-foreground);">${new Date(l.created_at).toLocaleString()}</td>
          <td><strong>${l.function_name}</strong></td>
          <td><span class="${badgeClass}">${(l.status || "").toUpperCase()}</span></td>
          <td>${l.duration_ms}ms</td>
          <td>${l.trigger_type || "manual"}</td>
          <td><button class="btn btn-outline btn-sm" onclick="viewLogDetail(${l.id})">View</button></td>
        `;
        tr._logData = l;
        tbody.appendChild(tr);
      });
    } catch (_) {}
  }

  window.viewLogDetail = function (id) {
    const rows = document.querySelectorAll("#logsTableBody tr");
    for (const r of rows) {
      if (r._logData && r._logData.id === id) {
        const l = r._logData;
        document.getElementById("modalLogTitle").textContent = `Log: ${l.function_name} (${l.status})`;
        let details = `Timestamp: ${l.created_at}\nDuration: ${l.duration_ms}ms\nTrigger: ${l.trigger_type}\n\n`;
        if (l.error_message) details += `ERROR:\n${l.error_message}\n\n`;
        if (l.output_data) details += `OUTPUT:\n${l.output_data}\n\n`;
        if (l.console_logs) details += `CONSOLE LOGS:\n${l.console_logs}`;
        document.getElementById("modalLogOutput").textContent = details;
        document.getElementById("modalLog").classList.add("open");
        break;
      }
    }
  };

  // --- MCP Hub ---
  async function loadMcpHub() {
    try {
      const res = await fetch("/api/mcp/tools");
      if (!res.ok) return;
      const data = await res.json();
      const tools = data.tools || [];

      const tbody = document.getElementById("mcpToolsTable");
      tbody.innerHTML = "";
      if (tools.length === 0) {
        tbody.innerHTML = '<tr><td colspan="3" style="text-align:center;">No MCP tools exported yet</td></tr>';
        return;
      }

      tools.forEach(t => {
        const tr = document.createElement("tr");
        tr.innerHTML = `
          <td><strong>${t.name}</strong></td>
          <td>${t.description || "—"}</td>
          <td><code>${JSON.stringify(t.inputSchema || {})}</code></td>
        `;
        tbody.appendChild(tr);
      });
    } catch (_) {}
  }

  window.copyText = function (elementId) {
    const el = document.getElementById(elementId);
    if (!el) return;
    navigator.clipboard.writeText(el.innerText || el.textContent).then(() => {
      alert(window.I18n.t("copied"));
    });
  };

  // --- Settings ---
  function initSettingsEvents() {
    document.getElementById("btnSaveGeneral").addEventListener("click", saveGeneralSettings);
    document.getElementById("btnSaveGit").addEventListener("click", saveGitSettings);
    document.getElementById("btnSaveEnv").addEventListener("click", saveEnvSettings);
    document.getElementById("btnSaveUI").addEventListener("click", saveUISettings);
    document.getElementById("btnAddEnvRow").addEventListener("click", () => addEnvRow("", ""));
  }

  async function loadAllSettings() {
    try {
      const res = await fetch("/api/settings");
      if (res.ok) {
        const data = await res.json();
        document.getElementById("settingPort").value = data.port || 8080;
        document.getElementById("settingTimeout").value = data.timeout_seconds || 30;
        document.getElementById("settingRetention").value = data.retention_days || 7;
        document.getElementById("settingAutostart").checked = !!data.run_on_startup;
        document.getElementById("settingAllowShell").checked = !!data.allow_shell;

        document.getElementById("settingGitAuthor").value = data.git_author_name || "";
        document.getElementById("settingGitEmail").value = data.git_author_email || "";
        document.getElementById("settingGitToken").value = data.git_token || "";

        if (data.language) window.I18n.setLanguage(data.language);
        if (data.theme_accent) {
          const themeMap = { "#22C55E": "emerald", "#38BDF8": "blue", "#A855F7": "purple", "#C084FC": "purple" };
          applyTheme(themeMap[data.theme_accent] || "emerald");
        }
        if (data.density) applyDensity(data.density);
        if (data.editor_font_size) applyFontSize(data.editor_font_size);
      }

      // Load env
      const envRes = await fetch("/api/env");
      if (envRes.ok) {
        const envData = await envRes.json();
        const container = document.getElementById("envTableRows");
        container.innerHTML = "";
        const envMap = envData.env || {};
        Object.keys(envMap).forEach(k => addEnvRow(k, envMap[k]));
      }

      await loadGitPackages();
    } catch (err) {
      console.error("Failed to load settings", err);
    }
  }

  function addEnvRow(key, val) {
    const container = document.getElementById("envTableRows");
    const div = document.createElement("div");
    div.className = "env-row";
    div.innerHTML = `
      <input type="text" class="env-key" value="${key}" placeholder="VARIABLE_NAME" style="width:200px; font-family:var(--font-heading);">
      <input type="text" class="env-val" value="${val}" placeholder="Value" style="flex:1;">
      <button class="btn btn-outline btn-sm btn-del-env">&times;</button>
    `;
    div.querySelector(".btn-del-env").addEventListener("click", () => div.remove());
    container.appendChild(div);
  }

  async function saveGeneralSettings() {
    const payload = {
      port: parseInt(document.getElementById("settingPort").value, 10),
      timeout_seconds: parseInt(document.getElementById("settingTimeout").value, 10),
      retention_days: parseInt(document.getElementById("settingRetention").value, 10),
      run_on_startup: document.getElementById("settingAutostart").checked,
      allow_shell: document.getElementById("settingAllowShell").checked
    };
    await postSettings(payload);
  }

  async function saveGitSettings() {
    const payload = {
      git_author_name: document.getElementById("settingGitAuthor").value,
      git_author_email: document.getElementById("settingGitEmail").value,
      git_token: document.getElementById("settingGitToken").value
    };
    await postSettings(payload);
  }

  async function saveUISettings() {
    const lang = document.getElementById("uiLangSelect").value;
    const theme = document.getElementById("uiThemeSelect").value;
    const density = document.getElementById("uiDensitySelect").value;
    const fontSize = document.getElementById("uiFontSizeSelect").value;

    window.I18n.setLanguage(lang);
    applyTheme(theme);
    applyDensity(density);
    applyFontSize(fontSize);

    const themeAccentMap = { emerald: "#22C55E", blue: "#38BDF8", purple: "#A855F7" };

    const payload = {
      language: lang,
      theme_accent: themeAccentMap[theme] || "#22C55E",
      density: density,
      editor_font_size: fontSize
    };
    await postSettings(payload);
  }

  async function postSettings(payload) {
    try {
      const res = await fetch("/api/settings", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
      if (res.ok) {
        alert(window.I18n.t("saved_successfully"));
      } else {
        const err = await res.json();
        alert(window.I18n.t("save_failed") + (err.error || ""));
      }
    } catch (err) {
      alert(window.I18n.t("save_failed") + err.message);
    }
  }

  async function saveEnvSettings() {
    const envObj = {};
    document.querySelectorAll(".env-row").forEach(row => {
      const k = row.querySelector(".env-key").value.trim();
      const v = row.querySelector(".env-val").value;
      if (k) envObj[k] = v;
    });

    try {
      const res = await fetch("/api/env", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ env: envObj })
      });
      if (res.ok) {
        alert(window.I18n.t("saved_successfully"));
      } else {
        const err = await res.json();
        alert(window.I18n.t("save_failed") + (err.error || ""));
      }
    } catch (err) {
      alert(window.I18n.t("save_failed") + err.message);
    }
  }

  async function loadGitPackages() {
    const tbody = document.getElementById("gitPackagesTableBody");
    if (!tbody) return;
    try {
      const res = await fetch("/api/packages");
      if (!res.ok) return;
      const pkgs = await res.json();
      tbody.innerHTML = "";

      if (!pkgs || pkgs.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" style="text-align:center; color:var(--color-muted-foreground);">No packages found</td></tr>';
        return;
      }

      pkgs.forEach(p => {
        const tr = document.createElement("tr");
        const typeBadge = p.is_git 
          ? '<span class="badge-status-success">Git Repo</span>' 
          : '<span style="color:var(--color-muted-foreground); font-size:11px;">Local Folder</span>';
        
        let actionBtns = "";
        if (p.is_git) {
          actionBtns = `
            <button class="btn btn-secondary btn-sm" onclick="pullPackage('${p.name}')">Pull</button>
            <button class="btn btn-outline btn-sm" onclick="commitPackagePrompt('${p.name}')">Commit</button>
          `;
        } else {
          actionBtns = `<span style="font-size:11px; color:var(--color-muted-foreground);">Manual edit</span>`;
        }

        tr.innerHTML = `
          <td><strong>${p.name}</strong></td>
          <td>${typeBadge}</td>
          <td><code>${p.status || "clean"}</code></td>
          <td>${actionBtns}</td>
        `;
        tbody.appendChild(tr);
      });
    } catch (err) {
      console.error("Failed to load git packages", err);
    }
  }

  window.pullPackage = async function (pkgName) {
    try {
      const res = await fetch("/api/packages/pull", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ package_name: pkgName })
      });
      const data = await res.json();
      if (res.ok) {
        alert(`Package '${pkgName}': ${data.status}`);
        await loadFunctions();
        await loadGitPackages();
      } else {
        alert(`Pull error: ${data.error || "Unknown"}`);
      }
    } catch (err) {
      alert("Network error: " + err.message);
    }
  };

  window.commitPackagePrompt = async function (pkgName) {
    const msg = prompt(`Enter commit message for '${pkgName}':`, "Update scripts via ActaCron");
    if (!msg) return;

    try {
      const res = await fetch("/api/packages/commit", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          package_name: pkgName,
          message: msg
        })
      });
      const data = await res.json();
      if (res.ok) {
        alert(`Package '${pkgName}' committed and pushed successfully.`);
        await loadGitPackages();
      } else {
        alert(`Commit error: ${data.error || "Unknown"}`);
      }
    } catch (err) {
      alert("Network error: " + err.message);
    }
  };

  async function syncAllGit() {
    try {
      const btn = document.getElementById("btnSyncAll");
      btn.textContent = "Syncing...";
      btn.disabled = true;
      const res = await fetch("/api/packages/sync", {
        method: "POST",
        headers: { "Content-Type": "application/json" }
      });
      if (res.ok) {
        const data = await res.json();
        if (data.synced > 0) {
          alert(`Synced ${data.synced} Git repository(s):\n` + (data.results || []).join("\n"));
        } else {
          alert("No Git repositories configured to sync.\nTo link a repository, go to Settings > Git Credentials > Clone Repository.");
        }
        await loadFunctions();
        await loadGitPackages();
      } else {
        const err = await res.json();
        alert("Git sync error: " + (err.error || "Unknown"));
      }
    } catch (err) {
      alert("Network error: " + err.message);
    } finally {
      const btn = document.getElementById("btnSyncAll");
      btn.textContent = window.I18n.t("sync_all");
      btn.disabled = false;
    }
  }

  // --- Modals ---
  function initModals() {
    // Log Modal
    const modalLog = document.getElementById("modalLog");
    document.getElementById("btnCloseLogModal").addEventListener("click", () => modalLog.classList.remove("open"));
    document.getElementById("btnCloseLogModalBtn").addEventListener("click", () => modalLog.classList.remove("open"));

    // New Script Modal
    const modalNew = document.getElementById("modalNewScript");
    document.getElementById("btnNewScript").addEventListener("click", () => modalNew.classList.add("open"));
    document.getElementById("btnCloseNewScript").addEventListener("click", () => modalNew.classList.remove("open"));
    document.getElementById("btnCancelNewScript").addEventListener("click", () => modalNew.classList.remove("open"));

    document.getElementById("btnConfirmNewScript").addEventListener("click", async () => {
      const pkg = document.getElementById("newScriptPkg").value.trim() || "default";
      let file = document.getElementById("newScriptFile").value.trim();
      if (!file) return;
      if (!file.endsWith(".js")) file += ".js";

      const template = `/**
 * @name ${file.replace(".js", "")}
 * @cron * * * * *
 * @mcp true
 * @description Describe your function here
 * @param {string} message - Greeting input
 */
function main(params) {
  console.log("Running task:", params.message);
  return { status: "ok", reply: "Hello " + (params.message || "World") };
}
`;
      try {
        const res = await fetch("/api/functions/save", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            package: pkg,
            filename: file,
            code: template
          })
        });
        if (res.ok) {
          modalNew.classList.remove("open");
          await loadFunctions();
          const created = allFunctions.find(f => f.package === pkg && f.name === file.replace(".js", ""));
          if (created) openScript(created);
        } else {
          const err = await res.json();
          alert("Failed to create script: " + (err.error || ""));
        }
      } catch (err) {
        alert("Error creating script: " + err.message);
      }
    });

    // Git Clone Modal
    const modalClone = document.getElementById("modalGitClone");
    const openCloneHandler = () => modalClone.classList.add("open");
    const btnOpenClone = document.getElementById("btnOpenCloneModal");
    if (btnOpenClone) btnOpenClone.addEventListener("click", openCloneHandler);
    const btnOpenCloneSettings = document.getElementById("btnOpenCloneFromSettings");
    if (btnOpenCloneSettings) btnOpenCloneSettings.addEventListener("click", openCloneHandler);

    document.getElementById("btnCloseGitClone").addEventListener("click", () => modalClone.classList.remove("open"));
    document.getElementById("btnCancelGitClone").addEventListener("click", () => modalClone.classList.remove("open"));

    document.getElementById("btnConfirmGitClone").addEventListener("click", async () => {
      const url = document.getElementById("cloneRepoUrl").value.trim();
      const target = document.getElementById("clonePkgName").value.trim();
      const branch = document.getElementById("cloneBranch").value.trim();
      const token = document.getElementById("cloneToken").value.trim();

      if (!url || !target) {
        alert("Please provide Git Repository URL and Package Name.");
        return;
      }

      const btn = document.getElementById("btnConfirmGitClone");
      btn.textContent = "Cloning...";
      btn.disabled = true;

      try {
        const res = await fetch("/api/packages/clone", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            url: url,
            target_name: target,
            branch: branch,
            token: token
          })
        });
        if (res.ok) {
          alert(`Successfully cloned repository '${target}'!`);
          modalClone.classList.remove("open");
          await loadFunctions();
          await loadGitPackages();
        } else {
          const err = await res.json();
          alert("Failed to clone: " + (err.error || "Unknown error"));
        }
      } catch (err) {
        alert("Clone error: " + err.message);
      } finally {
        btn.textContent = "Clone Repository";
        btn.disabled = false;
      }
    });

    // Workspace Config Modal
    const modalWs = document.getElementById("modalWorkspaceConfig");
    const closeWsModal = () => modalWs.classList.remove("open");
    document.getElementById("btnCloseWsConfig").addEventListener("click", closeWsModal);
    document.getElementById("btnCancelWsConfig").addEventListener("click", closeWsModal);
    document.getElementById("btnSaveWsConfig").addEventListener("click", saveWorkspaceConfig);
    document.getElementById("btnAddWsEnvRow").addEventListener("click", () => addWsEnvRow("", ""));

    const btnActiveWs = document.getElementById("btnActiveWsConfig");
    if (btnActiveWs) {
      btnActiveWs.addEventListener("click", () => {
        if (activeScript && activeScript.package && activeScript.package !== "_shared") {
          openWorkspaceConfigModal(activeScript.package);
        }
      });
    }

    const btnInspWs = document.getElementById("btnInspWsConfig");
    if (btnInspWs) {
      btnInspWs.addEventListener("click", () => {
        if (activeScript && activeScript.package && activeScript.package !== "_shared") {
          openWorkspaceConfigModal(activeScript.package);
        }
      });
    }
  }

  // --- Workspace Config Helpers ---
  async function openWorkspaceConfigModal(pkgName) {
    const modal = document.getElementById("modalWorkspaceConfig");
    document.getElementById("modalWsConfigTitle").textContent = "Workspace Configuration: " + pkgName;
    document.getElementById("wsConfigPkgName").value = pkgName;

    const timeoutInput = document.getElementById("wsConfigTimeout");
    const descInput = document.getElementById("wsConfigDesc");
    const envContainer = document.getElementById("wsEnvTableRows");

    timeoutInput.value = "30";
    descInput.value = "";
    envContainer.innerHTML = "<div style='color:var(--color-muted-foreground); font-size:12px; padding:6px;'>Loading configuration...</div>";
    modal.classList.add("open");

    try {
      const res = await fetch("/api/workspace/config?package=" + encodeURIComponent(pkgName));
      if (res.ok) {
        const data = await res.json();
        timeoutInput.value = data.timeout_seconds || 30;
        descInput.value = data.description || "";
        renderWsEnvRows(data.env || {});

        if (data.is_from_example) {
          document.getElementById("wsEnvExampleHint").style.display = "block";
        } else {
          document.getElementById("wsEnvExampleHint").style.display = "none";
        }

        const btnLoadEx = document.getElementById("btnLoadWsEnvExample");
        if (btnLoadEx) {
          if (data.example_env && Object.keys(data.example_env).length > 0) {
            btnLoadEx.style.display = "inline-flex";
            btnLoadEx.onclick = () => {
              renderWsEnvRows(data.example_env);
              document.getElementById("wsEnvExampleHint").style.display = "block";
            };
          } else {
            btnLoadEx.style.display = "none";
          }
        }
      } else {
        renderWsEnvRows({});
      }
    } catch (err) {
      console.error("Failed to load workspace config", err);
      renderWsEnvRows({});
    }
  }

  function renderWsEnvRows(envMap) {
    const container = document.getElementById("wsEnvTableRows");
    container.innerHTML = "";
    const keys = Object.keys(envMap || {});
    if (keys.length === 0) {
      addWsEnvRow("", "");
      return;
    }
    keys.sort().forEach(k => addWsEnvRow(k, envMap[k]));
  }

  function addWsEnvRow(key, val) {
    const container = document.getElementById("wsEnvTableRows");
    const row = document.createElement("div");
    row.className = "ws-env-row";
    row.style.cssText = "display:flex; gap:8px; margin-bottom:8px; align-items:center;";
    row.innerHTML = `
      <input type="text" class="ws-env-key" placeholder="KEY" value="${key || ''}" style="flex:1; font-family:var(--font-heading); font-size:12px;">
      <input type="text" class="ws-env-val" placeholder="VALUE" value="${val || ''}" style="flex:2; font-family:var(--font-heading); font-size:12px;">
      <button class="btn btn-outline btn-sm btn-del-ws-env" style="color:var(--color-danger); border-color:rgba(239,68,68,0.35); padding:2px 8px;">&times;</button>
    `;
    row.querySelector(".btn-del-ws-env").addEventListener("click", () => row.remove());
    container.appendChild(row);
  }

  async function saveWorkspaceConfig() {
    const pkgName = document.getElementById("wsConfigPkgName").value.trim();
    if (!pkgName) return;

    const timeout = parseInt(document.getElementById("wsConfigTimeout").value, 10) || 30;
    const desc = document.getElementById("wsConfigDesc").value.trim();

    const envMap = {};
    document.querySelectorAll(".ws-env-row").forEach(r => {
      const k = r.querySelector(".ws-env-key").value.trim();
      const v = r.querySelector(".ws-env-val").value;
      if (k) envMap[k] = v;
    });

    const payload = {
      package: pkgName,
      timeout_seconds: timeout,
      description: desc,
      env: envMap
    };

    try {
      const res = await fetch("/api/workspace/config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
      if (res.ok) {
        document.getElementById("modalWorkspaceConfig").classList.remove("open");
        alert(window.I18n.t("saved_successfully") || "Workspace configuration saved.");
        await loadFunctions();
      } else {
        const err = await res.json();
        alert("Failed saving config: " + (err.error || "Unknown error"));
      }
    } catch (err) {
      alert("Network error: " + err.message);
    }
  }
  function initResizers() {
    const leftResizer = document.getElementById("resizerLeft");
    const rightResizer = document.getElementById("resizerRight");
    const treePane = document.getElementById("treePane");
    const inspectorPane = document.getElementById("inspectorPane");
    
    const savedTreeWidth = localStorage.getItem("actacron_tree_width");
    if (savedTreeWidth && treePane) treePane.style.width = savedTreeWidth + "px";
    
    const savedInspWidth = localStorage.getItem("actacron_inspector_width");
    if (savedInspWidth && inspectorPane) inspectorPane.style.width = savedInspWidth + "px";

    if (leftResizer && treePane) {
      leftResizer.addEventListener("mousedown", (e) => {
        e.preventDefault();
        leftResizer.classList.add("dragging");
        document.body.style.cursor = "col-resize";
        document.body.style.userSelect = "none";

        const startX = e.clientX;
        const startWidth = treePane.offsetWidth;
        
        const onMouseMove = (e) => {
          let newWidth = startWidth + (e.clientX - startX);
          if (newWidth < 200) newWidth = 200;
          if (newWidth > 600) newWidth = 600;
          treePane.style.width = newWidth + "px";
          localStorage.setItem("actacron_tree_width", newWidth);
        };
        
        const onMouseUp = () => {
          leftResizer.classList.remove("dragging");
          document.body.style.cursor = "";
          document.body.style.userSelect = "";
          document.removeEventListener("mousemove", onMouseMove);
          document.removeEventListener("mouseup", onMouseUp);
        };
        
        document.addEventListener("mousemove", onMouseMove);
        document.addEventListener("mouseup", onMouseUp);
      });
    }

    if (rightResizer && inspectorPane) {
      rightResizer.addEventListener("mousedown", (e) => {
        e.preventDefault();
        rightResizer.classList.add("dragging");
        document.body.style.cursor = "col-resize";
        document.body.style.userSelect = "none";

        const startX = e.clientX;
        const startWidth = inspectorPane.offsetWidth;
        
        const onMouseMove = (e) => {
          let newWidth = startWidth - (e.clientX - startX);
          if (newWidth < 200) newWidth = 200;
          if (newWidth > 600) newWidth = 600;
          inspectorPane.style.width = newWidth + "px";
          localStorage.setItem("actacron_inspector_width", newWidth);
        };
        
        const onMouseUp = () => {
          rightResizer.classList.remove("dragging");
          document.body.style.cursor = "";
          document.body.style.userSelect = "";
          document.removeEventListener("mousemove", onMouseMove);
          document.removeEventListener("mouseup", onMouseUp);
        };
        
        document.addEventListener("mousemove", onMouseMove);
        document.addEventListener("mouseup", onMouseUp);
      });
    }
  }
})();
