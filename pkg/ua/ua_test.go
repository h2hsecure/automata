package ua

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		ua       string
		device   Device
		browser  Browser
	}{
		{
			name:    "ios safari",
			ua:      "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
			device:  DeviceMobile,
			browser: BrowserSafari,
		},
		{
			name:    "ios safari in chrome mode",
			ua:      "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/119.0.6045.66 Mobile/15E148 Safari/604.1",
			device:  DeviceMobile,
			browser: BrowserChrome,
		},
		{
			name:    "chrome android",
			ua:      "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
			device:  DeviceMobile,
			browser: BrowserChrome,
		},
		{
			name:    "ipad",
			ua:      "Mozilla/5.0 (iPad; CPU OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
			device:  DeviceTablet,
			browser: BrowserSafari,
		},
		{
			name:    "firefox linux",
			ua:      "Mozilla/5.0 (X11; Linux x86_64; rv:126.0) Gecko/20100101 Firefox/126.0",
			device:  DeviceDesktop,
			browser: BrowserFirefox,
		},
		{
			name:    "edge windows",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0",
			device:  DeviceDesktop,
			browser: BrowserEdge,
		},
		{
			name:    "chrome desktop",
			ua:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			device:  DeviceDesktop,
			browser: BrowserChrome,
		},
		{
			name:    "empty ua",
			ua:      "",
			device:  DeviceDesktop,
			browser: BrowserOther,
		},
		{
			name:    "garbage ua",
			ua:      "curl/8.5.0",
			device:  DeviceDesktop,
			browser: BrowserOther,
		},
		{
			name:    "kindle tablet (chromium-based)",
			ua:      "Mozilla/5.0 (Linux; Android 13; Kindle) AppleWebKit/537.36 (KHTML, like Gecko) Version/14.0 Chrome/119.0.0.0 Mobile Safari/537.36",
			device:  DeviceTablet,
			browser: BrowserChrome,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device, browser := Parse(tt.ua)
			if device != tt.device {
				t.Errorf("device = %q, want %q (ua: %s)", device, tt.device, tt.ua)
			}
			if browser != tt.browser {
				t.Errorf("browser = %q, want %q (ua: %s)", browser, tt.browser, tt.ua)
			}
		})
	}
}