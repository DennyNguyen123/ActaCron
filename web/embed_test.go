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

