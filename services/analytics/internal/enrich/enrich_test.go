package enrich

import (
	"errors"
	"net"
	"testing"
)

type fakeGeo map[string]string

func (f fakeGeo) Country(ip net.IP) (string, error) {
	if code, ok := f[ip.String()]; ok {
		return code, nil
	}
	return "", errors.New("not found")
}

func TestEnrich_UserAgent(t *testing.T) {
	tests := []struct {
		name, ua                string
		browser, os, device string
	}{
		{"iPhone Safari", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", "Safari", "iOS", DeviceMobile},
		{"Android phone", "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Mobile Safari/537.36", "Chrome", "Android", DeviceMobile},
		{"Android tablet", "Mozilla/5.0 (Linux; Android 13; SM-X700) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36", "Chrome", "Android", DeviceTablet},
		{"iPad", "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", "Safari", "iPadOS", DeviceTablet},
		{"Windows Chrome", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Chrome", "Windows", DeviceDesktop},
		{"Mac Firefox", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14.0; rv:121.0) Gecko/20100101 Firefox/121.0", "Firefox", "Mac OS X", DeviceDesktop},
		{"Googlebot", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "Googlebot", "Unknown", DeviceBot},
		{"empty", "", "Unknown", "Unknown", DeviceDesktop},
	}
	e := New(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := e.Enrich("", tt.ua)
			if r.Browser != tt.browser || r.Device != tt.device {
				t.Errorf("browser=%q device=%q, want %q %q", r.Browser, r.Device, tt.browser, tt.device)
			}
			if tt.os != "" && r.OS != tt.os {
				t.Errorf("os = %q, want %q", r.OS, tt.os)
			}
		})
	}
}

func TestEnrich_Country(t *testing.T) {
	geo := fakeGeo{"36.68.0.1": "ID", "2001:4860::1": "US"}
	tests := []struct {
		ip   string
		geo  CountryLookup
		want string
	}{
		{"36.68.0.1", geo, "ID"},
		{" 36.68.0.1 ", geo, "ID"},
		{"2001:4860::1", geo, "US"},
		{"8.8.4.4", geo, UnknownCountry}, // lookup gagal
		{"127.0.0.1", geo, UnknownCountry},
		{"::1", geo, UnknownCountry},
		{"192.168.1.10", geo, UnknownCountry},
		{"10.0.0.5", geo, UnknownCountry},
		{"not-an-ip", geo, UnknownCountry},
		{"", geo, UnknownCountry},
		{"36.68.0.1", nil, UnknownCountry}, // GeoIP nonaktif
	}
	for _, tt := range tests {
		if got := New(tt.geo).Enrich(tt.ip, "").Country; got != tt.want {
			t.Errorf("country(%q) = %q, want %q", tt.ip, got, tt.want)
		}
	}
}

func TestOpenGeoLite2_MissingFile(t *testing.T) {
	if _, err := OpenGeoLite2("does-not-exist.mmdb"); err == nil {
		t.Error("want error for missing database file")
	}
}
