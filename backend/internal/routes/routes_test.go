package routes

import (
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"go.yaml.in/yaml/v3"

	"opensex/backend/internal/config"
	"opensex/backend/internal/services"
)

func testRoutes(t *testing.T) []RouteSpec {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Env: config.EnvTest, AccessTTL: time.Minute, AdminAccessTTL: time.Minute, IPHashKey: []byte("k"), SwaggerOn: true}
	svc := services.New(services.Deps{Cfg: cfg})
	_, specs := New(Deps{Cfg: cfg, Svc: svc, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), RateLimitDisabled: true})
	return specs
}

// The access table and the OpenAPI contract must describe the same routes (T4.7, R17).
func TestOpenAPICoversEveryRoute(t *testing.T) {
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Skip("api/openapi.yaml not found")
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	param := regexp.MustCompile(`:([A-Za-z]+)`)
	declared := map[string]bool{}
	for _, r := range testRoutes(t) {
		path := param.ReplaceAllString(r.Path, "{$1}")
		key := strings.ToLower(r.Method) + " " + path
		declared[key] = true
		if _, ok := doc.Paths[path][strings.ToLower(r.Method)]; !ok {
			t.Errorf("route missing from openapi.yaml: %s", key)
		}
	}
	for path, ops := range doc.Paths {
		for method := range ops {
			if !declared[method+" "+path] {
				t.Errorf("openapi.yaml documents an unknown route: %s %s", method, path)
			}
		}
	}
}

// Every route has exactly one access kind and protected routes list roles.
func TestAccessTable(t *testing.T) {
	for _, r := range testRoutes(t) {
		if !r.Access.valid() {
			t.Errorf("%s %s: invalid access", r.Method, r.Path)
		}
		if strings.HasPrefix(r.Path, "/api/v1/admin/") && (len(r.Access.Roles) != 1 || r.Access.Roles[0] != "ADMIN") {
			t.Errorf("%s %s must be ADMIN only", r.Method, r.Path)
		}
		if strings.HasPrefix(r.Path, "/api/v1/manager/") || strings.HasPrefix(r.Path, "/api/v1/billing/subscription") {
			if len(r.Access.Roles) != 1 || r.Access.Roles[0] != "MANAGER" {
				t.Errorf("%s %s must be MANAGER only", r.Method, r.Path)
			}
		}
	}
}

// The contract is a valid OpenAPI 3 document.
func TestOpenAPIValid(t *testing.T) {
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Skip("api/openapi.yaml not found")
	}
	doc, err := openapi3.NewLoader().LoadFromData(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(t.Context()); err != nil {
		t.Fatal(err)
	}
}
