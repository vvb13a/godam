package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/vvb13a/godam/domain"
)

func TestCatalogIsConsistent(t *testing.T) {
	catalog := ImgProxyCatalog()
	if len(catalog.Formats) == 0 || len(catalog.Options) == 0 {
		t.Fatal("catalog is empty")
	}
	seen := make(map[string]bool, len(catalog.Options))
	for _, option := range catalog.Options {
		if seen[option.Name] {
			t.Fatalf("duplicate catalog option %q", option.Name)
		}
		seen[option.Name] = true
		if option.Raw && len(option.Args) != 0 {
			t.Fatalf("raw option %q has structured args", option.Name)
		}
		if !option.Raw && len(option.Args) == 0 {
			t.Fatalf("structured option %q has no args", option.Name)
		}
	}
	for alias, canonical := range catalogAliases {
		if !seen[canonical] {
			t.Fatalf("alias %q points at unknown option %q", alias, canonical)
		}
	}
}

func TestBuildProcessing(t *testing.T) {
	params := domain.PresetParameters{
		Format: "webp",
		Options: []domain.PresetOption{
			{Name: "resize", Args: []string{"fit", "800", "600"}},
			{Name: "quality", Args: []string{"82"}},
		},
	}
	processing, err := BuildProcessing(params)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if processing != "resize:fit:800:600:0:0/quality:82" {
		t.Fatalf("processing = %q", processing)
	}
}

func TestValidatePreset(t *testing.T) {
	valid := domain.PresetParameters{
		Format:  "webp",
		Options: []domain.PresetOption{{Name: "quality", Args: []string{"82"}}},
	}
	label, name, _, err := ValidatePreset("  Thumbnail  ", " Thumbnail ", valid)
	if err != nil {
		t.Fatalf("valid preset: %v", err)
	}
	if label != "Thumbnail" || name != "thumbnail" {
		t.Fatalf("label=%q name=%q", label, name)
	}

	invalid := []struct {
		name   string
		params domain.PresetParameters
	}{
		{name: "empty options", params: domain.PresetParameters{Format: "webp"}},
		{name: "unknown option", params: domain.PresetParameters{Format: "webp", Options: []domain.PresetOption{{Name: "nope"}}}},
		{name: "unsafe arg", params: domain.PresetParameters{Format: "webp", Options: []domain.PresetOption{{Name: "cachebuster", Args: []string{"a/b"}}}}},
		{name: "duplicate", params: domain.PresetParameters{Format: "webp", Options: []domain.PresetOption{{Name: "quality", Args: []string{"80"}}, {Name: "quality", Args: []string{"81"}}}}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, _, err := ValidatePreset("Label", "label", tc.params); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	if _, _, _, err := ValidatePreset("Label", "800x600", valid); err == nil {
		t.Fatal("expected a size-shaped name to be reserved")
	}
}

func TestParseProcessing(t *testing.T) {
	options, err := ParseProcessing("rs:fit:800:600/q:82")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(options) != 2 || options[0].Name != "resize" || options[1].Name != "quality" {
		t.Fatalf("options = %+v", options)
	}
	if _, err := ParseProcessing("nope:1"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestSignRequestRoundTrip(t *testing.T) {
	proxy, err := NewImgProxy("http://imgproxy:8080", "http://godam:8080", "736563726574", "68656c6c6f")
	if err != nil {
		t.Fatalf("new imgproxy: %v", err)
	}
	segment, err := proxy.SignRequest("asset-1", "webp", "rs:fit:100:100")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	format, processing, err := proxy.VerifyRequest("asset-1", segment)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if format != "webp" || processing != "rs:fit:100:100" {
		t.Fatalf("format=%q processing=%q", format, processing)
	}
	if _, _, err := proxy.VerifyRequest("asset-2", segment); err == nil {
		t.Fatal("expected asset-bound signature to fail")
	}
	tampered := strings.Replace(segment, ".webp", ".png", 1)
	if _, _, err := proxy.VerifyRequest("asset-1", tampered); err == nil {
		t.Fatal("expected tampered signature to fail")
	}
}
