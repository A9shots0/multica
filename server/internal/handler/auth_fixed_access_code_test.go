package handler

import "testing"

func TestDevVerificationCodeIsDisabledInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv(devVerificationCodeEnv, "123456")

	if isDevVerificationCode("123456") {
		t.Fatal("development verification code authenticated in production")
	}
}

func TestPrivateAccessCodeRequiresExplicitValidConfiguration(t *testing.T) {
	t.Setenv(privateAccessCodeEnabledEnv, "false")
	t.Setenv(privateAccessCodeEnv, "123456")
	if isPrivateAccessCode("123456") {
		t.Fatal("private access code authenticated while mode was disabled")
	}

	t.Setenv(privateAccessCodeEnabledEnv, "true")
	if !privateAccessCodeLoginEnabled() {
		t.Fatal("valid explicit private access-code mode was not enabled")
	}
	if !isPrivateAccessCode("123456") {
		t.Fatal("configured private access code did not authenticate")
	}
	if isPrivateAccessCode("654321") {
		t.Fatal("wrong private access code authenticated")
	}

	t.Setenv(privateAccessCodeEnv, "not-six-digits")
	if privateAccessCodeLoginEnabled() {
		t.Fatal("malformed private access code enabled the mode")
	}
}
