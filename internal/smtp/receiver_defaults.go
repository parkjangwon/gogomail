package smtpd

import (
	"context"
	"net"
	"time"

	gosmtp "github.com/emersion/go-smtp"
)

func clockOrDefault(clock func() time.Time) func() time.Time {
	if clock != nil {
		return clock
	}
	return time.Now
}

type noopRecorder struct{}

func (noopRecorder) Record(context.Context, ReceivedMessage) (string, error) {
	return "", nil
}

func recorderOrDefault(recorder MessageRecorder) MessageRecorder {
	if recorder != nil {
		return recorder
	}
	return noopRecorder{}
}

type noopDeduplicator struct{}

func (noopDeduplicator) CheckAndSet(context.Context, DedupKey) (bool, error) {
	return true, nil
}

func deduplicatorOrDefault(deduplicator Deduplicator) Deduplicator {
	if deduplicator != nil {
		return deduplicator
	}
	return noopDeduplicator{}
}

type noopRateLimiter struct{}

func (noopRateLimiter) Allow(context.Context, RateLimitKey) (bool, error) {
	return true, nil
}

func rateLimiterOrDefault(rateLimiter RateLimiter) RateLimiter {
	if rateLimiter != nil {
		return rateLimiter
	}
	return noopRateLimiter{}
}

func remoteAddrFromConn(conn *gosmtp.Conn) string {
	if conn == nil || conn.Conn() == nil {
		return ""
	}
	addr := conn.Conn().RemoteAddr()
	if addr == nil {
		return ""
	}
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return tcpAddr.IP.String()
	}
	return addr.String()
}

type noopBackpressure struct{}

func (noopBackpressure) Accept(context.Context) (bool, error) {
	return true, nil
}

func backpressureOrDefault(backpressure Backpressure) Backpressure {
	if backpressure != nil {
		return backpressure
	}
	return noopBackpressure{}
}
