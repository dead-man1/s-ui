package service

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util/common"

	"gorm.io/gorm"
)

// routeReferences lists the places in the stored route config that send
// traffic to tag. sing-box refuses to build a box whose route rules name an
// outbound that does not exist, so dropping such a tag would keep the core
// from starting again.
func routeReferences(tx *gorm.DB, tag string) ([]string, error) {
	var setting model.Setting
	err := tx.Model(model.Setting{}).Where("key = ?", "config").First(&setting).Error
	config := setting.Value
	if database.IsNotFound(err) {
		config = defaultValueMap["config"]
	} else if err != nil {
		return nil, err
	}
	var parsed struct {
		Route *struct {
			Final string            `json:"final"`
			Rules []json.RawMessage `json:"rules"`
		} `json:"route"`
	}
	if err = json.Unmarshal([]byte(config), &parsed); err != nil || parsed.Route == nil {
		// A config the panel cannot read is checked by sing-box itself.
		return nil, nil
	}
	var refs []string
	if parsed.Route.Final == tag {
		refs = append(refs, "route.final")
	}
	for i, rawRule := range parsed.Route.Rules {
		// Only the top-level rule carries the action: the sub-rules of a
		// logical rule are pure matchers.
		var rule struct {
			Action   string `json:"action"`
			Outbound string `json:"outbound"`
		}
		if json.Unmarshal(rawRule, &rule) != nil {
			continue
		}
		switch rule.Action {
		case "", "route", "bypass":
			if rule.Outbound == tag {
				refs = append(refs, "route.rules["+strconv.Itoa(i)+"]")
			}
		}
	}
	return refs, nil
}

// checkRouteReferences refuses to remove or rename tag while the route still
// points at it, unless another outbound or endpoint keeps answering to it.
func checkRouteReferences(tx *gorm.DB, kind string, tag string) error {
	if tag == "" {
		return nil
	}
	refs, err := routeReferences(tx, tag)
	if err != nil || len(refs) == 0 {
		return err
	}
	var outbounds, endpoints int64
	if err = tx.Model(model.Outbound{}).Where("tag = ?", tag).Count(&outbounds).Error; err != nil {
		return err
	}
	if err = tx.Model(model.Endpoint{}).Where("tag = ?", tag).Count(&endpoints).Error; err != nil {
		return err
	}
	if outbounds+endpoints > 1 {
		return nil
	}
	return common.NewErrorf("%s [%s] is used by %s; update the route first", kind, tag, strings.Join(refs, ", "))
}
