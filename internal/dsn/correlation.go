package dsn

import (
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/outbound"
)

type BounceCorrelation struct {
	ReturnPath        string
	BaseAddress       string
	OriginalRecipient string
	Token             string
}

func CorrelateVERPReturnPath(returnPath string) (BounceCorrelation, error) {
	parsed, ok := outbound.ParseVERPReturnPath(returnPath)
	if !ok {
		return BounceCorrelation{}, fmt.Errorf("return path is not a gogomail VERP address")
	}
	base := parsed.BaseLocal + "@" + parsed.Domain
	normalizedBase, err := mail.NormalizeAddress(base)
	if err != nil {
		return BounceCorrelation{}, fmt.Errorf("invalid VERP base address: %w", err)
	}
	normalizedReturnPath, err := mail.NormalizeAddress(strings.Trim(returnPath, "<>"))
	if err != nil {
		return BounceCorrelation{}, fmt.Errorf("invalid VERP return path: %w", err)
	}
	return BounceCorrelation{
		ReturnPath:        normalizedReturnPath,
		BaseAddress:       normalizedBase,
		OriginalRecipient: parsed.Recipient,
		Token:             parsed.Token,
	}, nil
}
