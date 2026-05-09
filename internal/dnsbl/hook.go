package dnsbl

import (
	"context"
	"fmt"
	"log/slog"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

// Policy controls what action the hook takes when an IP is listed.
type Policy string

const (
	PolicyReject  Policy = "reject"
	PolicyMonitor Policy = "monitor"
	PolicyTag     Policy = "tag"
)

// HookOptions configures the DNSBL hook.
type HookOptions struct {
	Checker *Checker
	Policy  Policy
	Logger  *slog.Logger
}

// Hook returns a smtpd.Hook that runs DNSBL checks at StageBackpressureChecked.
// On lookup error the hook fails open (allows the message through).
func Hook(opts HookOptions) smtpd.Hook {
	return func(ctx context.Context, event smtpd.Event) error {
		if event.Stage != smtpd.StageBackpressureChecked || opts.Checker == nil {
			return nil
		}
		result, err := opts.Checker.CheckAddr(event.RemoteAddr)
		if err != nil {
			if opts.Logger != nil {
				opts.Logger.WarnContext(ctx, "dnsbl lookup error", "addr", event.RemoteAddr, "error", err)
			}
			return nil // fail open
		}
		if !result.Listed {
			return nil
		}
		if opts.Logger != nil {
			opts.Logger.InfoContext(ctx, "dnsbl match", "addr", event.RemoteAddr, "zone", result.Zone, "code", result.Code)
		}
		if opts.Policy == PolicyReject {
			return fmt.Errorf("dnsbl: %s listed in %s", event.RemoteAddr, result.Zone)
		}
		return nil // monitor or tag: pass through
	}
}
