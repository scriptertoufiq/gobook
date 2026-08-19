package config_test

import (
	"strings"
	"testing"

	"github.com/scriptertoufiq/gobook/config"
)

// Debug returns panic detail to clients and logs every query. A deploy that
// forgets to set APP_DEBUG must get the safe default, not the convenient one.
func TestDebugDefaultsOnlyInDevelopment(t *testing.T) {
	cases := map[string]bool{
		"local":       true,
		"development": true,
		"test":        true,
		"staging":     false,
		"production":  false,
		"typo-env":    false,
		"":            false, // unset → production
	}

	for env, wantDebug := range cases {
		t.Run(env, func(t *testing.T) {
			t.Setenv("APP_ENV", env)
			t.Setenv("APP_DEBUG", "") // unset: exercise the default

			if got := config.Load().App.Debug; got != wantDebug {
				t.Errorf("APP_ENV=%s → Debug=%v, want %v", env, got, wantDebug)
			}
		})
	}
}

// An explicit setting always wins over the environment-derived default.
func TestExplicitDebugOverridesTheDefault(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_DEBUG", "true")

	if !config.Load().App.Debug {
		t.Error("an explicit APP_DEBUG=true should be honoured even in production")
	}
}

// Seeding inserts accounts with a published password, so anything unrecognised
// must fail closed rather than be treated as a development box.
func TestIsDevelopmentIsAnAllowList(t *testing.T) {
	for env, want := range map[string]bool{
		"local": true, "development": true, "dev": true, "test": true, "testing": true,
		"staging": false, "production": false, "prod": false, "Local": false,
		// Unset falls back to production — see config.Load.
		"": false,
	} {
		t.Setenv("APP_ENV", env)

		if got := config.Load().App.IsDevelopment(); got != want {
			t.Errorf("IsDevelopment(%q) = %v, want %v", env, got, want)
		}
	}
}

func TestValidateRejectsWeakSigningKeys(t *testing.T) {
	cases := []struct {
		name, secret, wantIn string
	}{
		{"empty", "", "JWT_SECRET is required"},
		{"too short", strings.Repeat("k", 31), "at least 32 characters"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET", tc.secret)

			err := config.Load().Validate()
			if err == nil {
				t.Fatal("expected a validation error")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q does not mention %q", err, tc.wantIn)
			}
		})
	}
}

func TestValidateAcceptsASufficientKey(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("k", 32))
	t.Setenv("AUTH_REQUIRE_EMAIL_VERIFICATION", "false")

	if err := config.Load().Validate(); err != nil {
		t.Errorf("expected a 32-char secret to validate, got: %v", err)
	}
}

// Mail settings only become mandatory once something will actually send.
func TestValidateRequiresSenderWhenVerificationIsOn(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("k", 32))
	t.Setenv("AUTH_REQUIRE_EMAIL_VERIFICATION", "true")
	t.Setenv("MAIL_FROM_ADDRESS", "")

	err := config.Load().Validate()
	if err == nil || !strings.Contains(err.Error(), "MAIL_FROM_ADDRESS") {
		t.Errorf("expected MAIL_FROM_ADDRESS to be required, got: %v", err)
	}
}
