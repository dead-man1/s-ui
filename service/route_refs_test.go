package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/core"
	"github.com/alireza0/s-ui/database/model"
)

// sing-box refuses to build a box whose route names a missing outbound, so the
// panel must not let a referenced outbound or endpoint disappear.
func TestRouteReferencesBlockRemoval(t *testing.T) {
	db := clientTestDB(t)
	prev := corePtr
	corePtr = core.NewCore()
	t.Cleanup(func() { corePtr = prev })

	settings := SettingService{}
	err := settings.SetConfig(`{"route":{"final":"wg","rules":[
		{"domain":["a.example"],"outbound":"proxy"},
		{"type":"logical","mode":"or","rules":[{"domain":["b.example"]}],"action":"route","outbound":"proxy"},
		{"domain":["c.example"],"action":"reject"}
	]}}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"proxy", "free"} {
		if err := db.Create(&model.Outbound{Type: "direct", Tag: tag, Options: json.RawMessage(`{}`)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.Endpoint{Type: "wireguard", Tag: "wg", Options: json.RawMessage(`{}`)}).Error; err != nil {
		t.Fatal(err)
	}
	var proxy model.Outbound
	if err := db.Where("tag = ?", "proxy").First(&proxy).Error; err != nil {
		t.Fatal(err)
	}

	outbounds := OutboundService{}
	endpoints := EndpointService{}
	tagJSON := func(tag string) json.RawMessage {
		data, _ := json.Marshal(tag)
		return data
	}
	expectBlocked := func(t *testing.T, err error, want string) {
		t.Helper()
		if err == nil {
			t.Fatal("a referenced tag was dropped")
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in %v", want, err)
		}
	}

	t.Run("delete referenced outbound", func(t *testing.T) {
		err := outbounds.Save(db, "del", tagJSON("proxy"))
		expectBlocked(t, err, "route.rules[0], route.rules[1]")
	})
	t.Run("rename referenced outbound", func(t *testing.T) {
		data, _ := json.Marshal(map[string]any{"id": proxy.Id, "type": "direct", "tag": "proxy2"})
		err := outbounds.Save(db, "edit", data)
		expectBlocked(t, err, "outbound [proxy]")
	})
	t.Run("edit referenced outbound without rename", func(t *testing.T) {
		data, _ := json.Marshal(map[string]any{"id": proxy.Id, "type": "direct", "tag": "proxy"})
		if err := outbounds.Save(db, "edit", data); err != nil {
			t.Fatalf("an edit keeping the tag was refused: %v", err)
		}
	})
	t.Run("delete referenced endpoint", func(t *testing.T) {
		err := endpoints.Save(db, "del", tagJSON("wg"))
		expectBlocked(t, err, "route.final")
	})
	t.Run("delete unreferenced outbound", func(t *testing.T) {
		if err := outbounds.Save(db, "del", tagJSON("free")); err != nil {
			t.Fatalf("an unreferenced outbound could not be deleted: %v", err)
		}
	})
}
