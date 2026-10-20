// Package ua classifies raw user-agent strings into a small fixed set of
// device and browser buckets. It intentionally avoids third-party UA
// databases: unknown agents fall back to desktop/Other.
package ua

import "strings"

// Device is a coarse device class.
type Device string

const (
	DeviceDesktop Device = "desktop"
	DeviceMobile  Device = "mobile"
	DeviceTablet  Device = "tablet"
)

// Browser is a coarse browser class.
type Browser string

const (
	BrowserChrome  Browser = "Chrome"
	BrowserFirefox Browser = "Firefox"
	BrowserSafari  Browser = "Safari"
	BrowserEdge    Browser = "Edge"
	BrowserOther   Browser = "Other"
)

// Parse classifies a raw user-agent string into device and browser buckets.
// An empty or unrecognized UA returns (DeviceDesktop, BrowserOther).
func Parse(userAgent string) (Device, Browser) {
	return parseDevice(userAgent), parseBrowser(userAgent)
}

func parseDevice(userAgent string) Device {
	upper := strings.ToUpper(userAgent)
	switch {
	case strings.Contains(upper, "IPAD") ||
		strings.Contains(upper, "TABLET") ||
		strings.Contains(upper, "KINDLE") ||
		strings.Contains(upper, "SILK") ||
		strings.Contains(upper, "PLAYBOOK"):
		return DeviceTablet
	case strings.Contains(upper, "IPHONE") ||
		strings.Contains(upper, "ANDROID") && !strings.Contains(upper, "KINDLE") ||
		strings.Contains(upper, "MOBILE") ||
		strings.Contains(upper, "WINDOWS PHONE"):
		return DeviceMobile
	default:
		return DeviceDesktop
	}
}

func parseBrowser(userAgent string) Browser {
	switch {
	case strings.Contains(userAgent, "Edg/"):
		return BrowserEdge
	case strings.Contains(userAgent, "Firefox"):
		return BrowserFirefox
	case strings.Contains(userAgent, "Safari") && !strings.Contains(userAgent, "Chrome") && !strings.Contains(userAgent, "CriOS"):
		return BrowserSafari
	case strings.Contains(userAgent, "Chrome") || strings.Contains(userAgent, "CriOS"):
		return BrowserChrome
	default:
		return BrowserOther
	}
}