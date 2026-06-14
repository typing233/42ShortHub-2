package service

import (
	"testing"
)

func TestQRCodePNG_NotEmpty(t *testing.T) {
	svc := &QRCodeService{baseURL: "http://localhost:8080"}
	data, err := svc.GeneratePNG("abc123", 256)
	if err != nil {
		t.Fatalf("GeneratePNG error: %v", err)
	}
	if len(data) == 0 {
		t.Error("PNG output is empty")
	}
	// PNG starts with magic bytes
	if data[0] != 0x89 || data[1] != 'P' || data[2] != 'N' || data[3] != 'G' {
		t.Error("output is not a valid PNG (bad magic bytes)")
	}
}

func TestQRCodeSVG_NotEmpty(t *testing.T) {
	svc := &QRCodeService{baseURL: "http://localhost:8080"}
	data, err := svc.GenerateSVG("abc123", 256)
	if err != nil {
		t.Fatalf("GenerateSVG error: %v", err)
	}
	if len(data) == 0 {
		t.Error("SVG output is empty")
	}
	svg := string(data)
	if svg[:4] != "<svg" {
		t.Errorf("SVG output doesn't start with <svg: %q", svg[:20])
	}
	if svg[len(svg)-6:] != "</svg>" {
		t.Errorf("SVG output doesn't end with </svg>")
	}
}

func TestQRCodePNG_DifferentSizes(t *testing.T) {
	svc := &QRCodeService{baseURL: "http://localhost:8080"}
	small, _ := svc.GeneratePNG("x", 64)
	large, _ := svc.GeneratePNG("x", 512)
	if len(large) <= len(small) {
		t.Error("larger size should produce more data")
	}
}
