package app

import (
	"log/slog"

	"github.com/gogomail/gogomail/internal/protocolmetrics"
)

func newProtocolGatewayMetrics(logger *slog.Logger) *protocolmetrics.GatewayMetrics {
	metrics := protocolmetrics.NewGatewayMetrics()
	metrics.SetLogger(protocolmetrics.NewLoggerWithSlog(logger))
	return metrics
}
