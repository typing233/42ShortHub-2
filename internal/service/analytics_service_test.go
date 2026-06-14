package service

import (
	"testing"

	"github.com/42ShortHub/shortlink/internal/model"
)

func TestParseUserAgent(t *testing.T) {
	tests := []struct {
		ua         string
		wantDevice string
		wantBrowser string
		wantOS     string
	}{
		{
			ua:         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			wantDevice: model.DeviceDesktop, wantBrowser: "Chrome", wantOS: "Windows",
		},
		{
			ua:         "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			wantDevice: model.DeviceMobile, wantBrowser: "Safari", wantOS: "iOS",
		},
		{
			ua:         "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			wantDevice: model.DeviceTablet, wantBrowser: "Safari", wantOS: "iOS",
		},
		{
			ua:         "Mozilla/5.0 (Linux; Android 13; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
			wantDevice: model.DeviceMobile, wantBrowser: "Chrome", wantOS: "Android",
		},
		{
			ua:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			wantDevice: model.DeviceDesktop, wantBrowser: "Edge", wantOS: "macOS",
		},
		{
			ua:         "Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0",
			wantDevice: model.DeviceDesktop, wantBrowser: "Firefox", wantOS: "Linux",
		},
	}

	for _, tt := range tests {
		device, browser, os := parseUserAgent(tt.ua)
		if device != tt.wantDevice {
			t.Errorf("parseUserAgent(%q) device = %q, want %q", tt.ua[:30], device, tt.wantDevice)
		}
		if browser != tt.wantBrowser {
			t.Errorf("parseUserAgent(%q) browser = %q, want %q", tt.ua[:30], browser, tt.wantBrowser)
		}
		if os != tt.wantOS {
			t.Errorf("parseUserAgent(%q) os = %q, want %q", tt.ua[:30], os, tt.wantOS)
		}
	}
}

func TestBotDetection(t *testing.T) {
	svc := &AnalyticsService{}

	bots := []string{
		"Googlebot/2.1 (+http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; Bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		"curl/7.68.0",
		"python-requests/2.28.0",
		"Go-http-client/1.1",
		"facebookexternalhit/1.1",
		"Twitterbot/1.0",
		"LinkedInBot/1.0",
		"WhatsApp/2.21.12.21",
		"TelegramBot (like TwitterBot)",
		"AhrefsBot/7.0",
		"SemrushBot/7",
		"PetalBot",
	}
	for _, ua := range bots {
		if !svc.detectBot(ua) {
			t.Errorf("detectBot(%q) = false, want true", ua)
		}
	}

	humans := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0",
	}
	for _, ua := range humans {
		if svc.detectBot(ua) {
			t.Errorf("detectBot(%q) = true, want false", ua[:40])
		}
	}
}
