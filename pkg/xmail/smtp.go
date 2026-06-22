// Package xmail wraps net/smtp with a small senders interface so callers
// can swap real SMTP for a stdout-logging stub in dev / tests / when SMTP
// is not configured.
//
// Magic-link password reset is the only consumer today; keep API minimal.
package xmail

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

// Sender abstracts outbound email so we can stub it.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// Message is the minimum we need to send a transactional email. HTML is
// optional — falls back to plain Text body.
type Message struct {
	To      []string
	Subject string
	Text    string
	HTML    string // optional
}

// Config carries SMTP credentials. Empty Host means we won't actually
// dial — caller should swap to NoopSender (or LoggerSender for dev).
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// IsConfigured returns true when Host + From are set. We allow empty
// username/password so unauthenticated relays (internal MTA) still work.
func (c Config) IsConfigured() bool {
	return strings.TrimSpace(c.Host) != "" && strings.TrimSpace(c.From) != ""
}

// SMTPSender talks to a real SMTP server using STARTTLS on port 587.
// For port 465 implicit TLS, callers should prefer port 587 with STARTTLS;
// add a separate sender when the need arises.
type SMTPSender struct {
	cfg Config
}

// NewSMTPSender builds a Sender bound to cfg.
func NewSMTPSender(cfg Config) *SMTPSender { return &SMTPSender{cfg: cfg} }

// Send dials cfg.Host:cfg.Port, upgrades with STARTTLS, AUTHs with the
// configured credentials (PLAIN) and submits a single message.
// The context's deadline becomes the dial deadline.
func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if !s.cfg.IsConfigured() {
		return fmt.Errorf("smtp: not configured (host/from empty)")
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("smtp: no recipients")
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	dialer := &smtpDialer{addr: addr, host: s.cfg.Host}

	deadline := time.Now().Add(30 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}

	cli, err := dialer.dial(deadline)
	if err != nil {
		return fmt.Errorf("smtp: dial: %w", err)
	}
	defer func() { _ = cli.Quit() }()

	if err := cli.Hello("localhost"); err != nil {
		return fmt.Errorf("smtp: hello: %w", err)
	}
	if ok, _ := cli.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}
		if err := cli.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("smtp: starttls: %w", err)
		}
	}
	if s.cfg.Username != "" {
		auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
		if err := cli.Auth(auth); err != nil {
			return fmt.Errorf("smtp: auth: %w", err)
		}
	}
	// SMTP MAIL FROM cần bare email (không kèm display name). Header From:
	// trong MIME có thể giữ "Display Name <email>" — handled by buildMIME.
	if err := cli.Mail(extractBareEmail(s.cfg.From)); err != nil {
		return fmt.Errorf("smtp: mail from: %w", err)
	}
	for _, to := range msg.To {
		if err := cli.Rcpt(to); err != nil {
			return fmt.Errorf("smtp: rcpt %s: %w", to, err)
		}
	}
	wc, err := cli.Data()
	if err != nil {
		return fmt.Errorf("smtp: data: %w", err)
	}
	defer wc.Close()

	if _, err := wc.Write(buildMIME(s.cfg.From, msg)); err != nil {
		return fmt.Errorf("smtp: write body: %w", err)
	}
	return nil
}

// LoggerSender is a fallback used in dev when SMTP isn't configured.
// It writes the email body to a callback (typically a structured logger)
// so devs can copy-paste the magic link from stdout / log file.
type LoggerSender struct {
	Log func(msg Message)
}

// Send prints `msg` via the configured callback.
func (s *LoggerSender) Send(_ context.Context, msg Message) error {
	if s.Log != nil {
		s.Log(msg)
	}
	return nil
}

// buildMIME formats a multipart/alternative MIME body so clients prefer
// the HTML version when available but fall back to plain text.
func buildMIME(from string, msg Message) []byte {
	boundary := fmt.Sprintf("app-%d", time.Now().UnixNano())
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(msg.To, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", encodeHeader(msg.Subject))
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	if msg.HTML != "" {
		fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary)
		// plain
		fmt.Fprintf(&b, "--%s\r\n", boundary)
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
		b.WriteString(msg.Text)
		b.WriteString("\r\n\r\n")
		// html
		fmt.Fprintf(&b, "--%s\r\n", boundary)
		b.WriteString("Content-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
		b.WriteString(msg.HTML)
		fmt.Fprintf(&b, "\r\n\r\n--%s--\r\n", boundary)
	} else {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
		b.WriteString(msg.Text)
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}

// encodeHeader wraps non-ASCII subject in RFC 2047 'B' (base64) encoding.
// Vietnamese subjects need this or Gmail will mangle the display.
func encodeHeader(s string) string {
	if isAllASCII(s) {
		return s
	}
	return "=?UTF-8?B?" + base64Encode(s) + "?="
}

func isAllASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// base64Encode without importing encoding/base64 just to avoid pulling
// dependencies into pkg level — used only for subject header.
func base64Encode(s string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	src := []byte(s)
	var out strings.Builder
	for len(src) >= 3 {
		n := int(src[0])<<16 | int(src[1])<<8 | int(src[2])
		out.WriteByte(alphabet[(n>>18)&63])
		out.WriteByte(alphabet[(n>>12)&63])
		out.WriteByte(alphabet[(n>>6)&63])
		out.WriteByte(alphabet[n&63])
		src = src[3:]
	}
	if len(src) > 0 {
		n := int(src[0]) << 16
		if len(src) == 2 {
			n |= int(src[1]) << 8
		}
		out.WriteByte(alphabet[(n>>18)&63])
		out.WriteByte(alphabet[(n>>12)&63])
		if len(src) == 2 {
			out.WriteByte(alphabet[(n>>6)&63])
			out.WriteByte('=')
		} else {
			out.WriteString("==")
		}
	}
	return out.String()
}

// extractBareEmail returns the address-spec part of an RFC 5322 address.
// Input "Display Name <user@host>" → "user@host". Input "user@host" → "user@host".
func extractBareEmail(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "<"); i >= 0 {
		if j := strings.LastIndex(s, ">"); j > i {
			return strings.TrimSpace(s[i+1 : j])
		}
	}
	return s
}

// smtpDialer wraps smtp.Dial with a deadline + host hint for STARTTLS.
type smtpDialer struct {
	addr string
	host string
}

func (d *smtpDialer) dial(deadline time.Time) (*smtp.Client, error) {
	timeout := time.Until(deadline)
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	conn, err := dialTimeout("tcp", d.addr, timeout)
	if err != nil {
		return nil, err
	}
	return smtp.NewClient(conn, d.host)
}
