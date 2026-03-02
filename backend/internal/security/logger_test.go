package security

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLogEvent_OutputContainsFields(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)

	LogEvent(SecurityEvent{
		Type:   EventLoginFailed,
		IP:     "192.168.1.1",
		UserID: 42,
		Path:   "/api/v1/auth/login",
		Detail: "invalid password",
	})

	output := buf.String()
	if !strings.Contains(output, "[SECURITY]") {
		t.Error("expected [SECURITY] prefix in log output")
	}
	if !strings.Contains(output, "login_failed") {
		t.Error("expected event type in log output")
	}
	if !strings.Contains(output, "192.168.1.1") {
		t.Error("expected IP in log output")
	}
}
