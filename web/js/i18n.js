// ActaCron i18n Dictionary & Cron Humanizer (EN default, VI secondary)
(function () {
  const translations = {
    en: {
      app_title: "ActaCron",
      nav_overview: "Overview",
      nav_workspace: "Workspace",
      nav_logs: "Logs",
      nav_mcp: "MCP Hub",
      nav_settings: "Settings",

      sys_healthy: "System Operational",
      sys_running: "Active Jobs",
      ram_usage: "RAM",
      uptime: "Uptime",
      sync_all: "Sync All Git",
      cron_active: "Active Crons",

      scripts_tree: "Packages & Scripts",
      new_script: "New Script",
      open_folder: "Open Dir",
      delete_script: "Delete",
      delete_confirm: "Are you sure you want to delete script '{name}' from package '{pkg}'? This cannot be undone.",
      script_deleted: "Script deleted successfully.",
      delete_failed: "Failed to delete script: ",
      open_folder_failed: "Failed to open directory: ",
      save_script: "Save (Ctrl+S)",
      run_script: "Run (Ctrl+Enter)",
      test_payload: "JSON Test Input",
      output_console: "Execution Output & Logs",
      execution_time: "Execution Time",
      copy_snippet: "Copy MCP Config",
      workspace_env_btn: "Workspace .env",
      workspace_info_label: "Workspace & Environment",
      configure_env: "⚙️ Config & .env",
      ws_config_tooltip: "Workspace Settings & .env",
      load_from_example: "📄 Load .env.example",
      env_example_hint: "💡 Pre-filled with template values from .env.example. Save configuration to persist to .env.",
      add_folder: "+ Folder",
      add_external_folder_title: "Add Folder as Workspace",
      folder_path: "Folder Path",
      folder_path_help: "Select an existing folder on your computer to use directly.",
      browse_folder: "📂 Browse...",
      workspace_name: "Workspace Name",
      workspace_name_help: "Identifier for this workspace (letters, numbers, underscores).",
      external_folder_note: "💡 Files will remain in their original folder and will NOT be copied into packages.",
      add_workspace: "Add Workspace",
      add_workspace_menu: "+ Workspace ▾",
      add_local_folder: "Add Local Folder",
      clone_git_repo: "Clone Git Repo",
      unlink_workspace: "Unlink Workspace",
      confirm_unlink_workspace: "Are you sure you want to unlink workspace '{name}'? Files on disk will NOT be deleted.",
      delete_workspace: "Delete Workspace",
      confirm_delete_workspace: "Are you sure you want to permanently delete workspace '{name}' from disk? This action cannot be undone.",

      logs_title: "Execution Logs",
      filter_func: "Filter by Function",
      filter_status: "Status",
      all_status: "All Statuses",
      refresh: "Refresh",
      col_time: "Timestamp",
      col_func: "Function",
      col_status: "Status",
      col_duration: "Duration",
      col_trigger: "Trigger",
      col_details: "Action",
      view_log: "View Output",

      mcp_title: "Model Context Protocol (MCP) Hub",
      mcp_desc: "Export dynamic JavaScript functions as local or remote AI agent tools.",
      mcp_tools_count: "Exported MCP Tools",
      mcp_stdio_cmd: "StdIO Run Command",
      mcp_claude_cfg: "Claude Desktop Config (claude_desktop_config.json)",
      mcp_cursor_cfg: "Cursor IDE MCP Configuration",
      copy_config: "Copy Config",
      copied: "Copied!",

      settings_title: "Application Settings",
      settings_general: "General",
      settings_git: "Git Credentials",
      settings_env: "Environment (.env)",
      settings_ui: "UI & Appearance",

      port_label: "HTTP Server Port",
      timeout_label: "Script Timeout (seconds)",
      retention_label: "Log Retention (days)",
      autostart_label: "Run on Windows Startup",
      autostart_desc: "Add ActaCron background tray process to Windows Registry Run key.",
      allow_shell_label: "Allow Shell Commands",
      allow_shell_desc: "Grant JS runtime access to invoke system CLI tools via exec().",
      save_general: "Save General Settings",

      git_author_name: "Git Author Name",
      git_author_email: "Git Author Email",
      git_token: "Personal Access Token (PAT)",
      git_token_desc: "Used for HTTPS authenticated Git fetch & push.",
      save_git: "Save Git Settings",

      env_desc: "Configure shared environment variables accessible to all scripts via env(\"KEY\").",
      add_env_var: "+ Add Variable",
      key_placeholder: "VARIABLE_NAME",
      val_placeholder: "Value",
      save_env: "Save .env File",
      mask_secrets: "Mask Values",

      ui_theme_label: "Accent Color Theme",
      ui_theme_emerald: "Emerald Green (Run)",
      ui_theme_blue: "Cyber Blue",
      ui_theme_purple: "Neon Purple",
      ui_density_label: "Interface Density",
      ui_density_normal: "Balanced (Default)",
      ui_density_compact: "Compact (Dense)",
      ui_density_spacious: "Spacious",
      ui_font_size_label: "Code Editor Font Size",
      ui_lang_label: "Display Language",
      save_ui: "Save Appearance Settings",

      "cron_toggle_label": "Enable Cron Schedule",
      "timezone_label": "Timezone",
      "adv_schedule_label": "Advanced Scheduling",
      "cron_start_label": "Start Datetime",
      "cron_end_label": "End Datetime",
      "max_runs_label": "Max Runs",
      "retry_label": "Retry Count",
      "no_overlap_label": "Prevent Overlap (Skip if running)",
      "status_active": "Active",
      "status_paused": "Paused",
      "status_pending": "Pending",
      "status_expired": "Expired",
      "status_running": "Running",
      "status_completed": "Completed",

      cron_modal_title: "Cron Schedule Details",
      saved_successfully: "Settings saved successfully.",
      save_failed: "Failed to save settings: ",
      running_func: "Running function...",
      success: "SUCCESS",
      error: "ERROR",
      close: "Close"
    },
    vi: {
      app_title: "ActaCron",
      nav_overview: "Tổng quan",
      nav_workspace: "Không gian làm việc",
      nav_logs: "Nhật ký thực thi",
      nav_mcp: "Trung tâm MCP",
      nav_settings: "Cài đặt",

      sys_healthy: "Hệ thống hoạt động tốt",
      sys_running: "Tác vụ đang chạy",
      ram_usage: "Bộ nhớ RAM",
      uptime: "Thời gian hoạt động",
      sync_all: "Đồng bộ tất cả Git",
      cron_active: "Cron đang kích hoạt",

      scripts_tree: "Gói & Kịch bản JS",
      new_script: "Tạo kịch bản mới",
      open_folder: "Mở thư mục",
      delete_script: "Xóa",
      delete_confirm: "Bạn có chắc chắn muốn xóa kịch bản '{name}' khỏi gói '{pkg}'? Thao tác này không thể hoàn tác.",
      script_deleted: "Đã xóa kịch bản thành công.",
      delete_failed: "Xóa kịch bản thất bại: ",
      open_folder_failed: "Mở thư mục thất bại: ",
      save_script: "Lưu (Ctrl+S)",
      run_script: "Chạy thử (Ctrl+Enter)",
      test_payload: "Dữ liệu đầu vào JSON",
      output_console: "Kết quả thực thi & Nhật ký",
      execution_time: "Thời gian thực thi",
      copy_snippet: "Sao chép cấu hình MCP",
      workspace_env_btn: "Biến môi trường (.env)",
      workspace_info_label: "Workspace & Môi trường",
      configure_env: "⚙️ Cấu hình & .env",
      ws_config_tooltip: "Cấu hình Workspace & .env",
      load_from_example: "📄 Tải từ .env.example",
      env_example_hint: "💡 Đã tự động điền giá trị mẫu từ .env.example. Bấm Lưu cấu hình để tạo tệp .env.",
      add_folder: "+ Thư mục",
      add_external_folder_title: "Thêm thư mục làm Workspace",
      folder_path: "Đường dẫn thư mục",
      folder_path_help: "Chọn một thư mục có sẵn trên máy tính để sử dụng trực tiếp.",
      browse_folder: "📂 Chọn...",
      workspace_name: "Tên Workspace",
      workspace_name_help: "Tên định danh cho workspace này (chữ cái, số, gạch dưới).",
      external_folder_note: "💡 Các file sẽ được giữ nguyên tại thư mục gốc, KHÔNG copy vào packages.",
      add_workspace: "Thêm Workspace",
      add_workspace_menu: "+ Workspace ▾",
      add_local_folder: "Thêm thư mục máy",
      clone_git_repo: "Clone từ Git",
      unlink_workspace: "Hủy liên kết",
      confirm_unlink_workspace: "Bạn có chắc chắn muốn hủy liên kết Workspace '{name}' khỏi ActaCron? Các file trên ổ đĩa sẽ KHÔNG bị xóa.",
      delete_workspace: "Xóa Workspace",
      confirm_delete_workspace: "Bạn có chắc chắn muốn xóa vĩnh viễn workspace '{name}' khỏi đĩa? Thao tác này không thể hoàn tác.",

      logs_title: "Nhật ký thực thi",
      filter_func: "Lọc theo hàm",
      filter_status: "Trạng thái",
      all_status: "Tất cả trạng thái",
      refresh: "Làm mới",
      col_time: "Thời gian",
      col_func: "Hàm kịch bản",
      col_status: "Trạng thái",
      col_duration: "Thời lượng",
      col_trigger: "Nguồn kích hoạt",
      col_details: "Hành động",
      view_log: "Xem chi tiết",

      mcp_title: "Giao thức Model Context Protocol (MCP)",
      mcp_desc: "Xuất bản các hàm JavaScript động thành công cụ AI Agent trên máy trạm hoặc từ xa.",
      mcp_tools_count: "Công cụ MCP được xuất",
      mcp_stdio_cmd: "Lệnh chạy StdIO",
      mcp_claude_cfg: "Cấu hình Claude Desktop (claude_desktop_config.json)",
      mcp_cursor_cfg: "Cấu hình Cursor IDE MCP",
      copy_config: "Sao chép cấu hình",
      copied: "Đã sao chép!",

      settings_title: "Cài đặt ứng dụng",
      settings_general: "Chung",
      settings_git: "Thông tin Git",
      settings_env: "Biến môi trường (.env)",
      settings_ui: "Giao diện & Ngôn ngữ",

      port_label: "Cổng máy chủ HTTP",
      timeout_label: "Thời gian chờ kịch bản (giây)",
      retention_label: "Thời gian lưu nhật ký (ngày)",
      autostart_label: "Khởi động cùng Windows",
      autostart_desc: "Tự động kích hoạt khay hệ thống ActaCron khi mở máy tính qua Windows Registry.",
      allow_shell_label: "Cho phép lệnh Shell",
      allow_shell_desc: "Cấp quyền cho JavaScript thực thi lệnh CLI hệ thống thông qua exec().",
      save_general: "Lưu cấu hình chung",

      git_author_name: "Tên tác giả Git",
      git_author_email: "Email tác giả Git",
      git_token: "Mã truy cập cá nhân (PAT)",
      git_token_desc: "Sử dụng cho xác thực kéo/đẩy qua HTTPS.",
      save_git: "Lưu cấu hình Git",

      env_desc: "Quản lý biến môi trường chung, có thể truy xuất trong mã qua hàm env(\"KEY\").",
      add_env_var: "+ Thêm biến mới",
      key_placeholder: "TÊN_BIẾN",
      val_placeholder: "Giá trị",
      save_env: "Lưu tệp .env",
      mask_secrets: "Ẩn giá trị nhạy cảm",

      ui_theme_label: "Màu chủ đạo (Accent Theme)",
      ui_theme_emerald: "Xanh ngọc lục bảo (Emerald)",
      ui_theme_blue: "Xanh công nghệ (Cyber Blue)",
      ui_theme_purple: "Tím ánh xạ (Neon Purple)",
      ui_density_label: "Mật độ bố cục giao diện",
      ui_density_normal: "Cân bằng (Mặc định)",
      ui_density_compact: "Thu gọn (Dày đặc)",
      ui_density_spacious: "Thoáng đãng",
      ui_font_size_label: "Cỡ chữ trình biên tập mã",
      ui_lang_label: "Ngôn ngữ hiển thị",
      save_ui: "Lưu cấu hình giao diện",

      "cron_toggle_label": "Bật lịch chạy Cron",
      "timezone_label": "Múi giờ",
      "adv_schedule_label": "Cài đặt lịch nâng cao",
      "cron_start_label": "Thời gian bắt đầu",
      "cron_end_label": "Thời gian kết thúc",
      "max_runs_label": "Số lần chạy tối đa",
      "retry_label": "Số lần thử lại",
      "no_overlap_label": "Chống chạy đè (Bỏ qua nếu đang chạy)",
      "status_active": "Đang hoạt động",
      "status_paused": "Đã tạm dừng",
      "status_pending": "Chờ bắt đầu",
      "status_expired": "Hết hạn",
      "status_running": "Đang thực thi",
      "status_completed": "Hoàn thành",

      cron_modal_title: "Chi tiết lịch định kỳ Cron",
      saved_successfully: "Cài đặt đã được lưu thành công.",
      save_failed: "Lưu cài đặt thất bại: ",
      running_func: "Đang chạy kịch bản...",
      success: "THÀNH CÔNG",
      error: "LỖI",
      close: "Đóng"
    }
  };

  let currentLang = localStorage.getItem("actacron_lang") || "en";

  function t(key) {
    if (translations[currentLang] && translations[currentLang][key]) {
      return translations[currentLang][key];
    }
    if (translations["en"] && translations["en"][key]) {
      return translations["en"][key];
    }
    return key;
  }

  function setLanguage(lang) {
    if (!translations[lang]) return;
    currentLang = lang;
    localStorage.setItem("actacron_lang", lang);
    applyI18n();
    document.documentElement.lang = lang;
    const langSelect = document.getElementById("uiLangSelect");
    if (langSelect) langSelect.value = lang;
    const topLangBtn = document.getElementById("topLangToggle");
    if (topLangBtn) topLangBtn.textContent = lang === "en" ? "🌐 EN" : "🌐 VI";
  }

  function applyI18n() {
    document.querySelectorAll("[data-i18n]").forEach(el => {
      const key = el.getAttribute("data-i18n");
      const val = t(key);
      if (el.tagName === "INPUT" && el.hasAttribute("placeholder")) {
        el.setAttribute("placeholder", val);
      } else {
        el.textContent = val;
      }
    });
    document.querySelectorAll("[data-i18n-title]").forEach(el => {
      const key = el.getAttribute("data-i18n-title");
      el.setAttribute("title", t(key));
    });
  }

  function cronToString(expr, lang) {
    if (!expr) return "";
    lang = lang || currentLang;
    const parts = expr.trim().split(/\s+/);
    if (parts.length < 5) return expr;

    const [min, hour, dom, mon, dow] = parts;

    if (lang === "vi") {
      if (expr === "* * * * *") return "Mỗi phút";
      if (min.startsWith("*/")) return `Mỗi ${min.slice(2)} phút`;
      if (min === "0" && hour === "*") return "Mỗi giờ (tại phút 0)";
      if (min === "0" && hour.startsWith("*/")) return `Mỗi ${hour.slice(2)} giờ`;
      if (dom === "*" && mon === "*" && dow === "*") return `Hàng ngày lúc ${hour.padStart(2, "0")}:${min.padStart(2, "0")}`;
      return expr;
    } else {
      if (expr === "* * * * *") return "Every minute";
      if (min.startsWith("*/")) return `Every ${min.slice(2)} minutes`;
      if (min === "0" && hour === "*") return "Every hour at minute 0";
      if (min === "0" && hour.startsWith("*/")) return `Every ${hour.slice(2)} hours`;
      if (dom === "*" && mon === "*" && dow === "*") return `Daily at ${hour.padStart(2, "0")}:${min.padStart(2, "0")}`;
      return expr;
    }
  }

  window.I18n = {
    t,
    setLanguage,
    getLanguage: () => currentLang,
    applyI18n,
    cronToString
  };
})();
