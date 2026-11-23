// Package enrich menurunkan data analitik (negara, browser, OS, device) dari IP & User-Agent mentah.
package enrich

import (
	"fmt"
	"net"
	"strings"

	"github.com/mssola/useragent"
	"github.com/oschwald/geoip2-golang"
)

// UnknownCountry adalah kode ISO 3166 "user-assigned" untuk negara yang tidak diketahui.
const UnknownCountry = "ZZ"

const (
	DeviceDesktop = "desktop"
	DeviceMobile  = "mobile"
	DeviceTablet  = "tablet"
	DeviceBot     = "bot"
)

type Result struct {
	Country string
	Browser string
	OS      string
	Device  string
}

// CountryLookup memetakan IP ke kode negara ISO (mis. "ID"). Kosong = tidak diketahui.
type CountryLookup interface {
	Country(ip net.IP) (string, error)
}

type Enricher struct {
	geo CountryLookup // nil = GeoIP nonaktif
}

func New(geo CountryLookup) *Enricher {
	return &Enricher{geo: geo}
}

func (e *Enricher) Enrich(ip, userAgent string) Result {
	browser, os, device := parseUserAgent(userAgent)
	return Result{
		Country: e.country(ip),
		Browser: browser,
		OS:      os,
		Device:  device,
	}
}

func (e *Enricher) country(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if e.geo == nil || ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() {
		return UnknownCountry
	}
	code, err := e.geo.Country(ip)
	if err != nil || code == "" {
		return UnknownCountry
	}
	return code
}

func parseUserAgent(raw string) (browser, os, device string) {
	if strings.TrimSpace(raw) == "" {
		return "Unknown", "Unknown", DeviceDesktop
	}
	ua := useragent.New(raw)

	browser, _ = ua.Browser()
	if browser == "" {
		browser = "Unknown"
	}
	os = normalizeOS(ua.OSInfo().Name, raw)

	switch {
	case ua.Bot():
		device = DeviceBot
	case isTablet(raw):
		device = DeviceTablet
	case ua.Mobile():
		device = DeviceMobile
	default:
		device = DeviceDesktop
	}
	return browser, os, device
}

// normalizeOS menyeragamkan nama OS untuk dashboard. mssola/useragent melaporkan iPad sebagai
// "OS" dan iPhone sebagai "iPhone OS".
func normalizeOS(name, raw string) string {
	switch {
	case strings.Contains(raw, "iPad"):
		return "iPadOS"
	case name == "iPhone OS":
		return "iOS"
	case name == "":
		return "Unknown"
	}
	return name
}

// isTablet: iPad, atau Android tanpa token "Mobile" (konvensi Google untuk tablet).
func isTablet(raw string) bool {
	return strings.Contains(raw, "iPad") || strings.Contains(raw, "Tablet") ||
		(strings.Contains(raw, "Android") && !strings.Contains(raw, "Mobile"))
}

// GeoLite2 membungkus database MaxMind GeoLite2-Country (.mmdb).
type GeoLite2 struct {
	db *geoip2.Reader
}

func OpenGeoLite2(path string) (*GeoLite2, error) {
	db, err := geoip2.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open geoip database: %w", err)
	}
	return &GeoLite2{db: db}, nil
}

func (g *GeoLite2) Country(ip net.IP) (string, error) {
	rec, err := g.db.Country(ip)
	if err != nil {
		return "", err
	}
	return rec.Country.IsoCode, nil
}

func (g *GeoLite2) Close() error {
	return g.db.Close()
}
