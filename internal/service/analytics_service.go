package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/42ShortHub/shortlink/internal/cache"
	"github.com/42ShortHub/shortlink/internal/config"
	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/repository"
)

var botPatterns = regexp.MustCompile(`(?i)(bot|crawler|spider|scraper|curl|wget|python-requests|go-http|java/|php/|ruby|perl|libwww|httpclient|fetcher|slurp|mediapartners|adsbot|bingpreview|facebookexternalhit|twitterbot|linkedinbot|whatsapp|telegrambot|discordbot|applebot|yandex|baidu|sogou|semrush|ahrefs|mj12bot|dotbot|petalbot)`)

type AnalyticsService struct {
	accessLogRepo *repository.AccessLogRepo
	linkRepo      *repository.LinkRepo
	cache         *cache.RedisCache
	cfg           *config.Config
}

func NewAnalyticsService(
	accessLogRepo *repository.AccessLogRepo,
	linkRepo *repository.LinkRepo,
	rc *cache.RedisCache,
	cfg *config.Config,
) *AnalyticsService {
	return &AnalyticsService{
		accessLogRepo: accessLogRepo,
		linkRepo:      linkRepo,
		cache:         rc,
		cfg:           cfg,
	}
}

func (s *AnalyticsService) EnrichAccessLog(entry *model.AccessLog) {
	entry.DeviceType, entry.Browser, entry.OS = parseUserAgent(entry.UserAgent)
	entry.IsBot = s.detectBot(entry.UserAgent)
	if entry.IsBot {
		entry.DeviceType = model.DeviceBot
	}
	entry.Country, entry.City = s.lookupGeo(entry.IP)
}

func (s *AnalyticsService) CheckDedup(linkID uint, ip string) bool {
	ctx := context.Background()
	dedupKey := fmt.Sprintf("dedup:%d:%s", linkID, ip)

	exists, err := s.cache.Get(ctx, dedupKey)
	if err == nil && exists != "" {
		return false
	}

	window := time.Duration(s.cfg.Analytics.DedupWindowSec) * time.Second
	s.cache.Set(ctx, dedupKey, "1", window)
	return true
}

func (s *AnalyticsService) RecordRealtime(linkID uint) {
	ctx := context.Background()
	bucket := time.Now().Unix() / 60
	key := fmt.Sprintf("rt:%d:%d", linkID, bucket)
	s.cache.Incr(ctx, key)
	s.cache.Expire(ctx, key, 10*time.Minute)
}

func (s *AnalyticsService) GetRealtime(linkID uint, minutes int) int64 {
	ctx := context.Background()
	now := time.Now().Unix() / 60
	var total int64
	for i := 0; i < minutes; i++ {
		key := fmt.Sprintf("rt:%d:%d", linkID, now-int64(i))
		if val, err := s.cache.Get(ctx, key); err == nil {
			var count int64
			fmt.Sscanf(val, "%d", &count)
			total += count
		}
	}
	return total
}

func (s *AnalyticsService) GetSummary(linkID uint, from, to time.Time) (*model.AnalyticsSummary, error) {
	return s.accessLogRepo.Summary(linkID, from, to)
}

func (s *AnalyticsService) GetTimeseries(linkID uint, from, to time.Time, granularity string) ([]model.TimeseriesPoint, error) {
	return s.accessLogRepo.Timeseries(linkID, from, to, granularity)
}

func (s *AnalyticsService) GetReferers(linkID uint, from, to time.Time, limit int) ([]model.BreakdownItem, error) {
	return s.accessLogRepo.RefererBreakdown(linkID, from, to, limit)
}

func (s *AnalyticsService) GetDevices(linkID uint, from, to time.Time) ([]model.BreakdownItem, error) {
	return s.accessLogRepo.DeviceBreakdown(linkID, from, to)
}

func (s *AnalyticsService) GetBrowsers(linkID uint, from, to time.Time, limit int) ([]model.BreakdownItem, error) {
	return s.accessLogRepo.BrowserBreakdown(linkID, from, to, limit)
}

func (s *AnalyticsService) GetGeo(linkID uint, from, to time.Time, limit int) ([]model.GeoItem, error) {
	return s.accessLogRepo.GeoBreakdown(linkID, from, to, limit)
}

func (s *AnalyticsService) detectBot(ua string) bool {
	return botPatterns.MatchString(ua)
}

func (s *AnalyticsService) lookupGeo(ip string) (country, city string) {
	// GeoIP lookup is optional; return empty if no DB configured
	// In production, integrate MaxMind GeoLite2:
	//   reader, _ := geoip2.Open(s.cfg.Analytics.GeoIPDBPath)
	//   record, _ := reader.City(net.ParseIP(ip))
	//   country = record.Country.IsoCode
	//   city = record.City.Names["en"]
	return "", ""
}

func parseUserAgent(ua string) (deviceType, browser, os string) {
	uaLower := strings.ToLower(ua)

	switch {
	case strings.Contains(uaLower, "ipad"):
		deviceType = model.DeviceTablet
	case strings.Contains(uaLower, "mobile") || (strings.Contains(uaLower, "android") && !strings.Contains(uaLower, "tablet")):
		deviceType = model.DeviceMobile
	case strings.Contains(uaLower, "tablet"):
		deviceType = model.DeviceTablet
	default:
		deviceType = model.DeviceDesktop
	}

	switch {
	case strings.Contains(uaLower, "firefox"):
		browser = "Firefox"
	case strings.Contains(uaLower, "edg"):
		browser = "Edge"
	case strings.Contains(uaLower, "chrome") && !strings.Contains(uaLower, "edg"):
		browser = "Chrome"
	case strings.Contains(uaLower, "safari") && !strings.Contains(uaLower, "chrome"):
		browser = "Safari"
	case strings.Contains(uaLower, "opera") || strings.Contains(uaLower, "opr"):
		browser = "Opera"
	default:
		browser = "Other"
	}

	switch {
	case strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad"):
		os = "iOS"
	case strings.Contains(uaLower, "android"):
		os = "Android"
	case strings.Contains(uaLower, "windows"):
		os = "Windows"
	case strings.Contains(uaLower, "mac os"):
		os = "macOS"
	case strings.Contains(uaLower, "linux"):
		os = "Linux"
	default:
		os = "Other"
	}

	return
}
