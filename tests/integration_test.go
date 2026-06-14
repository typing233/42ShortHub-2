package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/42ShortHub/shortlink/internal/cache"
	"github.com/42ShortHub/shortlink/internal/config"
	"github.com/42ShortHub/shortlink/internal/handler"
	"github.com/42ShortHub/shortlink/internal/middleware"
	"github.com/42ShortHub/shortlink/internal/model"
	"github.com/42ShortHub/shortlink/internal/repository"
	"github.com/42ShortHub/shortlink/internal/service"
)

type testEnv struct {
	router      *gin.Engine
	db          *gorm.DB
	linkSvc     *service.LinkService
	cfg         *config.Config
	userAToken  string
	userBToken  string
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	cfg := config.Load()
	cfg.App.ShortCodeLen = 6
	cfg.App.MaxBatchSize = 50
	cfg.App.RateLimitPerMin = 1000

	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("database not available, skipping integration test: %v", err)
	}

	db.Exec("DROP TABLE IF EXISTS access_logs")
	db.Exec("DROP TABLE IF EXISTS short_links")
	db.Exec("DROP TABLE IF EXISTS users")

	if err := db.AutoMigrate(&model.User{}, &model.ShortLink{}, &model.AccessLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	redisCache := cache.NewRedisCache(cfg.Redis)
	if err := redisCache.Ping(context.Background()); err != nil {
		t.Skipf("redis not available, skipping integration test: %v", err)
	}

	userRepo := repository.NewUserRepo(db)
	linkRepo := repository.NewLinkRepo(db)
	accessLogRepo := repository.NewAccessLogRepo(db)

	authSvc := service.NewAuthService(userRepo, cfg)
	linkSvc := service.NewLinkService(linkRepo, redisCache, cfg)
	linkSvc.StartLogWorker(accessLogRepo, 2, "")

	authHandler := handler.NewAuthHandler(authSvc)
	qrSvc := service.NewQRCodeService(cfg)
	auditSvc := service.NewAuditService(repository.NewAuditLogRepo(db))
	auditSvc.Start(1)
	linkHandler := handler.NewLinkHandler(linkSvc, qrSvc, auditSvc)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.LoadHTMLGlob("../web/templates/*")

	r.GET("/s/:code", linkHandler.Redirect)

	api := r.Group("/api/v1")
	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)

	authed := api.Group("")
	authed.Use(middleware.AuthMiddleware(cfg.JWT.Secret))
	authed.POST("/links", linkHandler.Create)
	authed.POST("/links/batch", linkHandler.BatchCreate)
	authed.GET("/links", linkHandler.List)
	authed.GET("/links/:id", linkHandler.Get)
	authed.PUT("/links/:id", linkHandler.Update)
	authed.DELETE("/links/:id", linkHandler.Delete)

	env := &testEnv{router: r, db: db, linkSvc: linkSvc, cfg: cfg}

	env.userAToken = env.registerAndLogin(t, "usera", "usera@test.com", "password123")
	env.userBToken = env.registerAndLogin(t, "userb", "userb@test.com", "password123")

	return env
}

func (e *testEnv) registerAndLogin(t *testing.T, username, email, password string) string {
	t.Helper()

	body, _ := json.Marshal(map[string]string{
		"username": username, "email": email, "password": password,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("register %s failed: %d %s", username, w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	loginBody, _ := json.Marshal(map[string]string{"username": username, "password": password})
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login %s failed: %d %s", username, w.Code, w.Body.String())
	}

	var resp model.APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	dataMap := resp.Data.(map[string]interface{})
	return dataMap["token"].(string)
}

func (e *testEnv) doRequest(method, path, token string, body interface{}) *httptest.ResponseRecorder {
	var reqBody *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	} else {
		reqBody = bytes.NewReader(nil)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	e.router.ServeHTTP(w, req)
	return w
}

func TestCreateLink(t *testing.T) {
	env := setupTestEnv(t)
	defer env.linkSvc.Shutdown(2 * time.Second)

	w := env.doRequest("POST", "/api/v1/links", env.userAToken, map[string]string{
		"url": "https://github.com",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	link := resp.Data.(map[string]interface{})
	if link["short_code"] == "" {
		t.Error("short_code should not be empty")
	}
	if link["original_url"] != "https://github.com" {
		t.Error("original_url mismatch")
	}
}

func TestCreateLink_CustomCode(t *testing.T) {
	env := setupTestEnv(t)
	defer env.linkSvc.Shutdown(2 * time.Second)

	w := env.doRequest("POST", "/api/v1/links", env.userAToken, map[string]string{
		"url": "https://google.com", "custom_code": "goog",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	link := resp.Data.(map[string]interface{})
	if link["short_code"] != "goog" {
		t.Errorf("expected short_code=goog, got %v", link["short_code"])
	}
}

func TestCreateLink_DuplicateCode(t *testing.T) {
	env := setupTestEnv(t)
	defer env.linkSvc.Shutdown(2 * time.Second)

	env.doRequest("POST", "/api/v1/links", env.userAToken, map[string]string{
		"url": "https://example.com", "custom_code": "dup1",
	})

	w := env.doRequest("POST", "/api/v1/links", env.userAToken, map[string]string{
		"url": "https://other.com", "custom_code": "dup1",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Message == "" {
		t.Error("conflict response should have a message")
	}
}

func TestCreateLink_DuplicateCodeCrossUser(t *testing.T) {
	env := setupTestEnv(t)
	defer env.linkSvc.Shutdown(2 * time.Second)

	env.doRequest("POST", "/api/v1/links", env.userAToken, map[string]string{
		"url": "https://example.com", "custom_code": "shared",
	})

	w := env.doRequest("POST", "/api/v1/links", env.userBToken, map[string]string{
		"url": "https://other.com", "custom_code": "shared",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBatchCreate(t *testing.T) {
	env := setupTestEnv(t)
	defer env.linkSvc.Shutdown(2 * time.Second)

	body := map[string]interface{}{
		"links": []map[string]string{
			{"url": "https://a.com"},
			{"url": "https://b.com"},
			{"url": "https://c.com"},
		},
	}
	w := env.doRequest("POST", "/api/v1/links/batch", env.userAToken, body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	created := data["created"].([]interface{})
	if len(created) != 3 {
		t.Errorf("expected 3 created, got %d", len(created))
	}
}

func TestBatchCreate_PartialFailure(t *testing.T) {
	env := setupTestEnv(t)
	defer env.linkSvc.Shutdown(2 * time.Second)

	env.doRequest("POST", "/api/v1/links", env.userAToken, map[string]string{
		"url": "https://existing.com", "custom_code": "taken",
	})

	body := map[string]interface{}{
		"links": []map[string]string{
			{"url": "https://new1.com"},
			{"url": "https://new2.com", "custom_code": "taken"},
			{"url": "https://new3.com"},
		},
	}
	w := env.doRequest("POST", "/api/v1/links/batch", env.userAToken, body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	created := data["created"].([]interface{})
	errors := data["errors"].([]interface{})
	if len(created) != 2 {
		t.Errorf("expected 2 created, got %d", len(created))
	}
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
}

func TestCrossUserPermission(t *testing.T) {
	env := setupTestEnv(t)
	defer env.linkSvc.Shutdown(2 * time.Second)

	w := env.doRequest("POST", "/api/v1/links", env.userAToken, map[string]string{
		"url": "https://private.com", "custom_code": "priv1",
	})
	var resp model.APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	link := resp.Data.(map[string]interface{})
	linkID := fmt.Sprintf("%v", link["id"])

	// User B tries to read User A's link
	w = env.doRequest("GET", "/api/v1/links/"+linkID, env.userBToken, nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-user GET, got %d", w.Code)
	}

	// User B tries to update User A's link
	w = env.doRequest("PUT", "/api/v1/links/"+linkID, env.userBToken, map[string]string{
		"title": "hacked",
	})
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-user PUT, got %d", w.Code)
	}

	// User B tries to delete User A's link
	w = env.doRequest("DELETE", "/api/v1/links/"+linkID, env.userBToken, nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-user DELETE, got %d", w.Code)
	}
}

func TestRedirect_Active(t *testing.T) {
	env := setupTestEnv(t)
	defer env.linkSvc.Shutdown(2 * time.Second)

	env.doRequest("POST", "/api/v1/links", env.userAToken, map[string]string{
		"url": "https://example.com/target", "custom_code": "redir1",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/s/redir1", nil)
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "https://example.com/target" {
		t.Errorf("expected redirect to target, got %s", loc)
	}
}

func TestRedirect_Inactive(t *testing.T) {
	env := setupTestEnv(t)
	defer env.linkSvc.Shutdown(2 * time.Second)

	env.doRequest("POST", "/api/v1/links", env.userAToken, map[string]string{
		"url": "https://example.com/disabled", "custom_code": "dis1",
	})

	// Get link ID from list
	w := env.doRequest("GET", "/api/v1/links?keyword=dis1", env.userAToken, nil)
	var resp model.APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	items := data["items"].([]interface{})
	linkID := fmt.Sprintf("%.0f", items[0].(map[string]interface{})["id"].(float64))

	// Disable it
	inactive := "inactive"
	env.doRequest("PUT", "/api/v1/links/"+linkID, env.userAToken, map[string]*string{
		"status": &inactive,
	})

	// Try to redirect
	w2 := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/s/dis1", nil)
	env.router.ServeHTTP(w2, req)

	if w2.Code != http.StatusForbidden {
		t.Errorf("expected 403 for inactive link, got %d", w2.Code)
	}
}

func TestRedirect_Expired(t *testing.T) {
	env := setupTestEnv(t)
	defer env.linkSvc.Shutdown(2 * time.Second)

	past := time.Now().Add(-1 * time.Hour)
	body := map[string]interface{}{
		"url":        "https://example.com/expired",
		"custom_code": "exp1",
		"expires_at": past.Format(time.RFC3339),
	}
	env.doRequest("POST", "/api/v1/links", env.userAToken, body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/s/exp1", nil)
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected 410 for expired link, got %d", w.Code)
	}
}

func TestRedirect_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	defer env.linkSvc.Shutdown(2 * time.Second)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/s/nonexist", nil)
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAccessLog_Persisted(t *testing.T) {
	env := setupTestEnv(t)

	env.doRequest("POST", "/api/v1/links", env.userAToken, map[string]string{
		"url": "https://example.com/logged", "custom_code": "log1",
	})

	// Hit the redirect several times
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/s/log1", nil)
		req.Header.Set("User-Agent", fmt.Sprintf("TestBot/%d", i))
		req.Header.Set("Referer", "https://referrer.com")
		env.router.ServeHTTP(w, req)
	}

	// Shutdown flushes logs to DB
	env.linkSvc.Shutdown(5 * time.Second)

	// Verify logs in database
	var count int64
	env.db.Model(&model.AccessLog{}).Where("short_link_id IN (SELECT id FROM short_links WHERE short_code = 'log1')").Count(&count)

	if count != 5 {
		t.Errorf("expected 5 access logs persisted, got %d", count)
	}

	// Verify log content
	var logs []model.AccessLog
	env.db.Where("short_link_id IN (SELECT id FROM short_links WHERE short_code = 'log1')").
		Order("accessed_at").Find(&logs)

	for i, l := range logs {
		if l.UserAgent != fmt.Sprintf("TestBot/%d", i) {
			t.Errorf("log %d: expected UA TestBot/%d, got %s", i, i, l.UserAgent)
		}
		if l.Referer != "https://referrer.com" {
			t.Errorf("log %d: expected referer, got %s", i, l.Referer)
		}
		if l.AccessedAt.IsZero() {
			t.Errorf("log %d: accessed_at should not be zero", i)
		}
	}
}

func TestCreateLink_BlockedURLs(t *testing.T) {
	env := setupTestEnv(t)
	defer env.linkSvc.Shutdown(2 * time.Second)

	blocked := []string{
		"http://192.168.1.1",
		"http://10.0.0.1",
		"http://172.16.0.1",
		"http://127.0.0.1",
		"http://localhost",
		"http://169.254.1.1",
		"ftp://example.com",
	}

	for _, url := range blocked {
		w := env.doRequest("POST", "/api/v1/links", env.userAToken, map[string]string{"url": url})
		if w.Code != http.StatusBadRequest {
			t.Errorf("URL %q should be blocked, got status %d", url, w.Code)
		}
	}
}
