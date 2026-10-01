package web_test

import (
	"strings"
	"testing"

	"actacron/web"
)

func TestEmbeddedFiles(t *testing.T) {
	data, err := web.Assets.ReadFile("index.html")
	if err != nil || len(data) == 0 {
		t.Fatalf("expected index.html to be embedded, got err: %v", err)
	}
	i18nData, err := web.Assets.ReadFile("js/i18n.js")
	if err != nil || len(i18nData) == 0 {
		t.Fatalf("expected js/i18n.js to be embedded, got err: %v", err)
	}
	i18nStr := string(i18nData)
	for _, key := range []string{"workspace_env_btn", "workspace_info_label", "configure_env", "ws_config_tooltip"} {
		if !strings.Contains(i18nStr, key) {
			t.Fatalf("expected i18n.js to contain key %q", key)
		}
	}
}

func TestWorkspaceUIElements(t *testing.T) {
	data, err := web.Assets.ReadFile("index.html")
	if err != nil {
		t.Fatalf("failed reading index.html: %v", err)
	}
	htmlStr := string(data)
	requiredElements := []string{
		`id="btnActiveWsConfig"`,
		`id="inspectorWsSection"`,
		`id="inspWsName"`,
		`id="btnInspWsConfig"`,
	}
	for _, el := range requiredElements {
		if !strings.Contains(htmlStr, el) {
			t.Fatalf("expected index.html to contain element %s", el)
		}
	}
}

func TestAppJSEventWiring(t *testing.T) {
	data, err := web.Assets.ReadFile("js/app.js")
	if err != nil {
		t.Fatalf("failed reading js/app.js: %v", err)
	}
	appStr := string(data)
	for _, token := range []string{"btnActiveWsConfig", "inspectorWsSection", "inspWsName", "btnInspWsConfig"} {
		if !strings.Contains(appStr, token) {
			t.Fatalf("expected js/app.js to reference %s", token)
		}
	}
}

func TestI18nTitleSupport(t *testing.T) {
	i18nData, err := web.Assets.ReadFile("js/i18n.js")
	if err != nil {
		t.Fatalf("failed reading js/i18n.js: %v", err)
	}
	if !strings.Contains(string(i18nData), "data-i18n-title") {
		t.Fatalf("expected js/i18n.js to support data-i18n-title attribute")
	}

	htmlData, err := web.Assets.ReadFile("index.html")
	if err != nil {
		t.Fatalf("failed reading index.html: %v", err)
	}
	if !strings.Contains(string(htmlData), `data-i18n-title="ws_config_tooltip"`) {
		t.Fatalf("expected index.html to contain data-i18n-title=\"ws_config_tooltip\"")
	}
}

func TestResizableLayoutAndExampleEnvElements(t *testing.T) {
	htmlData, err := web.Assets.ReadFile("index.html")
	if err != nil {
		t.Fatalf("failed reading index.html: %v", err)
	}
	htmlStr := string(htmlData)

	htmlElements := []string{
		`id="resizerLeft"`,
		`id="resizerRight"`,
		`id="btnLoadWsEnvExample"`,
		`id="wsEnvExampleHint"`,
	}
	for _, el := range htmlElements {
		if !strings.Contains(htmlStr, el) {
			t.Fatalf("expected index.html to contain element %s", el)
		}
	}

	i18nData, err := web.Assets.ReadFile("js/i18n.js")
	if err != nil {
		t.Fatalf("failed reading js/i18n.js: %v", err)
	}
	i18nStr := string(i18nData)

	i18nKeys := []string{
		`load_from_example:`,
		`env_example_hint:`,
	}
	for _, key := range i18nKeys {
		if !strings.Contains(i18nStr, key) {
			t.Fatalf("expected js/i18n.js to contain key %s", key)
		}
	}
}

func TestAppJSResizerAndExampleEnvWiring(t *testing.T) {
	data, err := web.Assets.ReadFile("js/app.js")
	if err != nil {
		t.Fatalf("failed reading js/app.js: %v", err)
	}
	appStr := string(data)
	required := []string{
		"initResizers",
		"btnLoadWsEnvExample",
		"wsEnvExampleHint",
		"actacron_tree_width",
	}
	for _, req := range required {
		if !strings.Contains(appStr, req) {
			t.Fatalf("expected js/app.js to reference %s", req)
		}
	}
}

func TestComprehensiveCronUIElementsAndI18n(t *testing.T) {
	htmlData, err := web.Assets.ReadFile("index.html")
	if err != nil {
		t.Fatalf("failed reading index.html: %v", err)
	}
	htmlStr := string(htmlData)

	elements := []string{
		`id="inspCronToggle"`,
		`id="inspTimezone"`,
		`id="inspAdvScheduleContent"`,
		`id="inspCronStart"`,
		`id="inspCronEnd"`,
		`id="inspMaxRuns"`,
		`id="inspRetryCount"`,
		`id="inspNoOverlap"`,
		`id="overviewCronTable"`,
	}
	for _, el := range elements {
		if !strings.Contains(htmlStr, el) {
			t.Fatalf("expected index.html to contain %s", el)
		}
	}

	tableHeaders := []string{
		"Function",
		"Schedule & Timezone",
		"Status",
		"Next Run",
		"Actions",
	}
	for _, th := range tableHeaders {
		if !strings.Contains(htmlStr, th) {
			t.Fatalf("expected index.html overview table header to contain %s", th)
		}
	}

	cssData, err := web.Assets.ReadFile("css/style.css")
	if err != nil {
		t.Fatalf("failed reading css/style.css: %v", err)
	}
	cssStr := string(cssData)
	cssClasses := []string{
		".adv-schedule-accordion",
		".badge-cron-active",
		".badge-cron-paused",
		".badge-cron-pending",
		".badge-cron-expired",
		".badge-cron-running",
	}
	for _, cls := range cssClasses {
		if !strings.Contains(cssStr, cls) {
			t.Fatalf("expected css/style.css to contain %s", cls)
		}
	}

	i18nData, err := web.Assets.ReadFile("js/i18n.js")
	if err != nil {
		t.Fatalf("failed reading js/i18n.js: %v", err)
	}
	i18nStr := string(i18nData)
	keys := []string{
		"cron_toggle_label",
		"timezone_label",
		"adv_schedule_label",
		"cron_start_label",
		"cron_end_label",
		"max_runs_label",
		"retry_label",
		"no_overlap_label",
		"status_active",
		"status_paused",
		"status_pending",
		"status_expired",
		"status_running",
	}
	for _, k := range keys {
		if !strings.Contains(i18nStr, `"`+k+`"`) {
			t.Fatalf("expected js/i18n.js to contain translation key %s", k)
		}
	}
}

func TestAppJSComprehensiveCronWiring(t *testing.T) {
	data, err := web.Assets.ReadFile("js/app.js")
	if err != nil {
		t.Fatalf("failed reading js/app.js: %v", err)
	}
	appStr := string(data)
	required := []string{
		"inspCronToggle",
		"inspCronStart",
		"inspCronEnd",
		"inspTimezone",
		"inspNoOverlap",
		"badge-cron-",
	}
	for _, req := range required {
		if !strings.Contains(appStr, req) {
			t.Fatalf("expected js/app.js to reference %s", req)
		}
	}
}

func TestUpdateUIElements(t *testing.T) {
	data, err := web.Assets.ReadFile("index.html")
	if err != nil {
		t.Fatalf("failed reading index.html: %v", err)
	}
	htmlStr := string(data)
	requiredElements := []string{
		`id="app-version-badge"`,
		`id="modalUpdate"`,
		`id="btnApplyUpdate"`,
		`id="btnCheckUpdateSettings"`,
	}
	for _, el := range requiredElements {
		if !strings.Contains(htmlStr, el) {
			t.Fatalf("expected index.html to contain element %s", el)
		}
	}
}

func TestAppJSUpdateWiring(t *testing.T) {
	data, err := web.Assets.ReadFile("js/app.js")
	if err != nil {
		t.Fatalf("failed reading js/app.js: %v", err)
	}
	appStr := string(data)
	for _, token := range []string{"btnCheckUpdateSettings", "modalUpdate", "btnApplyUpdate", "/api/version", "/api/update/check"} {
		if !strings.Contains(appStr, token) {
			t.Fatalf("expected js/app.js to reference %s", token)
		}
	}
}


