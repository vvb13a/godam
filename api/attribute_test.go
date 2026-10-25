package api

import (
	"testing"

	"github.com/vvb13a/godam/domain"
)

func TestAssetVisibleToActor(t *testing.T) {
	asset := &domain.Asset{Attributes: []domain.AssetAttribute{
		{Key: "tenant", Value: "acme"},
		{Key: "entity", Value: "news"},
		{Key: "kind", Value: "logo"},
	}}
	actor := &domain.Actor{
		Token: true,
		Scope: domain.Scope{Attributes: []domain.AttributeGrant{
			{Key: "tenant", Values: []string{"acme"}, Mode: domain.AttributeModeHidden},
			{Key: "entity", Values: []string{"news", "press"}, Mode: domain.AttributeModeLocked},
			{Key: "kind", Values: []string{"icon"}, Mode: domain.AttributeModeActive},
		}},
	}
	if !assetVisibleToActor(asset, actor) {
		t.Fatal("asset should be visible")
	}
	actor.Scope.Attributes[0].Values = []string{"beta"}
	if assetVisibleToActor(asset, actor) {
		t.Fatal("cross-tenant asset should be hidden")
	}
}
