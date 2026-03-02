package security

import (
	"log"
	"time"
)

type EventType string

const (
	EventLoginFailed    EventType = "login_failed"
	EventLoginSuccess   EventType = "login_success"
	EventPermissionDeny EventType = "permission_denied"
	EventRateLimit      EventType = "rate_limited"
	EventInvalidInput   EventType = "invalid_input"
	EventSuspicious     EventType = "suspicious_activity"
)

type SecurityEvent struct {
	Type      EventType
	IP        string
	UserID    int64
	UserAgent string
	Path      string
	Detail    string
	Timestamp time.Time
}

func LogEvent(event SecurityEvent) {
	event.Timestamp = time.Now()
	log.Printf("[SECURITY] type=%s ip=%s user_id=%d path=%s detail=%s",
		event.Type, event.IP, event.UserID, event.Path, event.Detail)
}
