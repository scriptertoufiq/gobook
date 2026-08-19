package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/gobook/pkg/response"
)

type signupBody struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8,max=64"`
	RefreshToken string `json:"refresh_token" binding:"omitempty"`
	Role         string `json:"role" binding:"omitempty,oneof=user admin"`
	Age          int    `json:"age" binding:"omitempty,gte=18"`
}

func init() {
	gin.SetMode(gin.TestMode)
	response.UseJSONFieldNames()
}

// post runs one request through a handler that binds and reports errors the
// same way every controller does.
func post(t *testing.T, body string) (int, response.Envelope) {
	t.Helper()

	router := gin.New()
	router.POST("/", func(c *gin.Context) {
		var req signupBody
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ValidationError(c, err)
			return
		}
		response.OK(c, gin.H{"ok": true})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	var env response.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("response was not valid JSON: %v\nbody: %s", err, rec.Body.String())
	}
	return rec.Code, env
}

// The whole point of UseJSONFieldNames: a frontend must be able to match error
// keys to its own form fields.
func TestFieldKeysUseJSONNamesNotGoNames(t *testing.T) {
	code, env := post(t, `{"email":"x@y.co","password":"supersecret","role":"wizard"}`)

	if code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", code)
	}
	if _, ok := env.Error.Fields["role"]; !ok {
		t.Errorf("expected a `role` key, got %v", env.Error.Fields)
	}
	if _, leaked := env.Error.Fields["Role"]; leaked {
		t.Error("error keys are exposing Go struct names")
	}
}

func TestMessagesReadAsSentences(t *testing.T) {
	cases := []struct {
		name, body, field, want string
	}{
		{
			name: "required", body: `{}`,
			field: "email", want: "Email is required.",
		},
		{
			name: "email format", body: `{"email":"not-an-email","password":"supersecret"}`,
			field: "email", want: "Email must be a valid email address.",
		},
		{
			name: "min length", body: `{"email":"x@y.co","password":"short"}`,
			field: "password", want: "Password must be at least 8 characters.",
		},
		{
			name: "oneof", body: `{"email":"x@y.co","password":"supersecret","role":"wizard"}`,
			field: "role", want: "Role must be one of: user, admin.",
		},
		{
			name: "gte", body: `{"email":"x@y.co","password":"supersecret","age":12}`,
			field: "age", want: "Age must be 18 or more.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, env := post(t, tc.body)

			got := env.Error.Fields[tc.field]
			if got != tc.want {
				t.Errorf("field %q:\n  got  %q\n  want %q", tc.field, got, tc.want)
			}
		})
	}
}

// A snake_case field must be labelled readably, not echoed raw.
func TestMultiWordFieldNamesAreHumanised(t *testing.T) {
	router := gin.New()
	router.POST("/", func(c *gin.Context) {
		var req struct {
			RefreshToken string `json:"refresh_token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ValidationError(c, err)
			return
		}
		response.OK(c, nil)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)))

	var env response.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}

	if got := env.Error.Fields["refresh_token"]; got != "Refresh token is required." {
		t.Errorf("got %q, want %q", got, "Refresh token is required.")
	}
}

// A single problem should be stated up front, not hidden behind a generic
// "the given data was invalid".
func TestSingleProblemIsSummarisedInTheTopLevelMessage(t *testing.T) {
	_, env := post(t, `{"email":"x@y.co"}`)

	if env.Error.Message != "Password is required." {
		t.Errorf("message = %q, want the single field problem restated", env.Error.Message)
	}
}

func TestMultipleProblemsAreCounted(t *testing.T) {
	_, env := post(t, `{}`)

	if !strings.Contains(env.Error.Message, "2 fields") {
		t.Errorf("message = %q, want a count of the failing fields", env.Error.Message)
	}
	if len(env.Error.Fields) != 2 {
		t.Errorf("expected 2 field errors, got %d: %v", len(env.Error.Fields), env.Error.Fields)
	}
}

// An unparseable body is a different problem from a rule violation, and must
// not send the caller hunting through field rules.
func TestUnparseableBodiesReport400WithAnExplanation(t *testing.T) {
	cases := []struct {
		name, body, wantContains string
	}{
		{"malformed json", `{"email":`, "not valid JSON"},
		{"empty body", ``, "empty"},
		{"wrong type", `{"email":123,"password":"supersecret"}`, "must be a string"},
		{"wrong type number", `{"email":"x@y.co","password":"supersecret","age":"old"}`, "must be a number"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, env := post(t, tc.body)

			if code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 (unparseable, not a rule failure)", code)
			}
			if !strings.Contains(env.Error.Message, tc.wantContains) {
				t.Errorf("message %q does not mention %q", env.Error.Message, tc.wantContains)
			}
		})
	}
}

func TestValidRequestStillSucceeds(t *testing.T) {
	code, env := post(t, `{"email":"x@y.co","password":"supersecret","role":"admin","age":30}`)

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; error: %+v", code, env.Error)
	}
	if !env.Success {
		t.Error("success should be true")
	}
}
