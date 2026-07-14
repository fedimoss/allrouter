package console_setting

import "testing"

func TestFilterAnnouncementsForProviderSites(t *testing.T) {
	// 同时覆盖显式开启、显式关闭、历史缺失字段和非法类型四种数据形态。
	visible := map[string]interface{}{"content": "visible", "showToProviders": true}
	hidden := map[string]interface{}{"content": "hidden", "showToProviders": false}
	legacy := map[string]interface{}{"content": "legacy"}
	invalid := map[string]interface{}{"content": "invalid", "showToProviders": "true"}

	got := filterAnnouncementsForProviderSites([]map[string]interface{}{
		visible,
		hidden,
		legacy,
		invalid,
	})

	if len(got) != 1 {
		t.Fatalf("expected one provider-visible announcement, got %d", len(got))
	}
	if got[0]["content"] != "visible" {
		t.Fatalf("expected visible announcement, got %#v", got[0])
	}
}

func TestValidateAnnouncementsShowToProviders(t *testing.T) {
	// 保存入口必须拒绝字符串形式的布尔值，防止过滤阶段产生歧义。
	valid := `[{"content":"notice","publishDate":"2026-07-14T08:00:00Z","showToProviders":false}]`
	if err := validateAnnouncements(valid); err != nil {
		t.Fatalf("expected boolean showToProviders to be valid: %v", err)
	}

	invalid := `[{"content":"notice","publishDate":"2026-07-14T08:00:00Z","showToProviders":"false"}]`
	if err := validateAnnouncements(invalid); err == nil {
		t.Fatal("expected non-boolean showToProviders to be rejected")
	}
}
