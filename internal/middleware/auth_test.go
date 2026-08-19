package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/gobook/internal/middleware"
	"github.com/scriptertoufiq/gobook/pkg/jwt"
)

func init() { gin.SetMode(gin.TestMode) }

func newManager() *jwt.Manager {
	return jwt.NewManager(strings.Repeat("k", 32), "gobook-test", 15*time.Minute)
}

// call drives a request through Auth plus the guard under test.
func call(t *testing.T, guard gin.HandlerFunc, path, target string, userID uint, role string) int {
	t.Helper()

	manager := newManager()
	router := gin.New()
	router.GET(path, middleware.Auth(manager), guard, func(c *gin.Context) {
		c.String(http.StatusOK, "reached")
	})

	access, _, err := manager.Issue(userID, role, true)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer "+access)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

func TestRequireRoleAdmitsOnlyTheListedRoles(t *testing.T) {
	guard := middleware.RequireRole("admin")

	if got := call(t, guard, "/x", "/x", 1, "admin"); got != http.StatusOK {
		t.Errorf("admin should pass, got %d", got)
	}
	if got := call(t, guard, "/x", "/x", 2, "user"); got != http.StatusForbidden {
		t.Errorf("plain user should be forbidden, got %d", got)
	}
}

// The ownership rule: your own record yes, anyone else's no.
func TestRequireSelfOrRoleAllowsOwnRecordOnly(t *testing.T) {
	guard := middleware.RequireSelfOrRole("id", "admin")

	if got := call(t, guard, "/users/:id", "/users/7", 7, "user"); got != http.StatusOK {
		t.Errorf("a user editing their own record should pass, got %d", got)
	}
	if got := call(t, guard, "/users/:id", "/users/1", 7, "user"); got != http.StatusForbidden {
		t.Errorf("a user reaching for another record must be forbidden, got %d", got)
	}
	if got := call(t, guard, "/users/:id", "/users/1", 7, "admin"); got != http.StatusOK {
		t.Errorf("an admin should reach any record, got %d", got)
	}
}

// A malformed id must not somehow satisfy the ownership comparison.
func TestRequireSelfOrRoleRejectsUnparseableIDs(t *testing.T) {
	guard := middleware.RequireSelfOrRole("id", "admin")

	if got := call(t, guard, "/users/:id", "/users/abc", 7, "user"); got != http.StatusForbidden {
		t.Errorf("a non-numeric id should be forbidden, got %d", got)
	}
}

// Both guards must fail closed when Auth has not populated the context —
// otherwise mounting them in the wrong order would silently grant access.
func TestGuardsFailClosedWithoutAuth(t *testing.T) {
	cases := map[string]gin.HandlerFunc{
		"RequireRole":       middleware.RequireRole("admin"),
		"RequireSelfOrRole": middleware.RequireSelfOrRole("id", "admin"),
	}

	for name, guard := range cases {
		t.Run(name, func(t *testing.T) {
			router := gin.New()
			router.GET("/users/:id", guard, func(c *gin.Context) {
				c.String(http.StatusOK, "reached")
			})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/1", nil))

			if rec.Code == http.StatusOK {
				t.Errorf("%s allowed a request with no authenticated identity", name)
			}
		})
	}
}

func TestHasRoleIsFalseWithoutAuth(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	if middleware.HasRole(c, "admin") {
		t.Error("HasRole must fail closed when Auth has not run")
	}
}
