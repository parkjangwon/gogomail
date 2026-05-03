# gogomail agent instructions

## Mail standards are mandatory

gogomail is a mail server project. Correct RFC/international-standard compliance is a core requirement, not an optional enhancement.

When implementing SMTP, message parsing, storage, headers, MIME, authentication, DNS, delivery, bounce handling, or IMAP-related behavior, prefer standards-compliant behavior over shortcuts.

Important standards to keep in mind include, but are not limited to:

- RFC 5321: SMTP
- RFC 5322: Internet Message Format, successor to RFC 822
- RFC 2045/2046/2047/2049: MIME
- RFC 6531/6532: SMTPUTF8 and internationalized email headers
- RFC 6376: DKIM
- RFC 7208: SPF
- RFC 7489: DMARC
- RFC 3461/3464: DSN and delivery status notifications
- RFC 3501 and successors: IMAP, when IMAP is implemented

Use well-maintained libraries for protocol parsing/serialization where possible instead of ad-hoc string parsing.

Tests for mail behavior should include protocol edge cases and RFC-shaped examples whenever practical.
