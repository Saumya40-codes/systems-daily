package email

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Attachment is a file attached to the message.
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// Message is an email (plain body + optional attachments).
type Message struct {
	From        string
	To          []string
	Subject     string
	Body        string // text/plain
	Attachments []Attachment
}

// SMTPConfig configures the outbound mail path.
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Pass     string
	UseTLS   bool // STARTTLS (typical for 587)
	Insecure bool // skip cert verify
}

// Send delivers msg via SMTP.
func Send(cfg SMTPConfig, msg Message) error {
	if cfg.Host == "" {
		return fmt.Errorf("SMTP host is empty")
	}
	if msg.From == "" {
		return fmt.Errorf("From is empty")
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("To is empty")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	toHeader := strings.Join(msg.To, ", ")
	raw := buildMIME(msg.From, toHeader, msg.Subject, msg.Body, msg.Attachments)

	// Port 465: implicit TLS
	if cfg.Port == 465 {
		return sendImplicitTLS(cfg, addr, msg, raw)
	}

	// Default: dial plain, then STARTTLS if requested (587)
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return fmt.Errorf("dial SMTP: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if cfg.UseTLS {
		tlsCfg := &tls.Config{
			ServerName:         cfg.Host,
			InsecureSkipVerify: cfg.Insecure, //nolint:gosec // explicit opt-in for local dev
			MinVersion:         tls.VersionTLS12,
		}
		if err := client.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("STARTTLS: %w", err)
		}
	}

	if cfg.User != "" {
		auth := smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}

	if err := client.Mail(msg.From); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	for _, rcpt := range msg.To {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("RCPT TO %s: %w", rcpt, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func sendImplicitTLS(cfg SMTPConfig, addr string, msg Message, raw []byte) error {
	tlsCfg := &tls.Config{
		ServerName:         cfg.Host,
		InsecureSkipVerify: cfg.Insecure, //nolint:gosec
		MinVersion:         tls.VersionTLS12,
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 30 * time.Second}, "tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("TLS dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return err
	}
	defer client.Close()

	if cfg.User != "" {
		auth := smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}
	if err := client.Mail(msg.From); err != nil {
		return err
	}
	for _, rcpt := range msg.To {
		if err := client.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func buildMIME(from, to, subject, body string, atts []Attachment) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", sanitizeHeader(subject))
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))

	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\n", "\r\n")
	if !strings.HasSuffix(body, "\r\n") {
		body += "\r\n"
	}

	if len(atts) == 0 {
		fmt.Fprintf(&b, "Content-Type: text/plain; charset=UTF-8\r\n")
		fmt.Fprintf(&b, "Content-Transfer-Encoding: 8bit\r\n")
		fmt.Fprintf(&b, "\r\n")
		b.WriteString(body)
		return []byte(b.String())
	}

	boundary := fmt.Sprintf("systems-daily-%d", time.Now().UnixNano())
	fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=%s\r\n", boundary)
	fmt.Fprintf(&b, "\r\n")
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&b, "Content-Transfer-Encoding: 8bit\r\n")
	fmt.Fprintf(&b, "\r\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\r\n") {
		b.WriteString("\r\n")
	}

	for _, att := range atts {
		ct := att.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		name := att.Filename
		if name == "" {
			name = "attachment"
		}
		fmt.Fprintf(&b, "--%s\r\n", boundary)
		fmt.Fprintf(&b, "Content-Type: %s; name=\"%s\"\r\n", ct, sanitizeHeader(name))
		fmt.Fprintf(&b, "Content-Disposition: attachment; filename=\"%s\"\r\n", sanitizeHeader(name))
		fmt.Fprintf(&b, "Content-Transfer-Encoding: base64\r\n")
		fmt.Fprintf(&b, "\r\n")
		b.WriteString(wrapBase64(base64.StdEncoding.EncodeToString(att.Data)))
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String())
}

func wrapBase64(s string) string {
	const col = 76
	var b strings.Builder
	for len(s) > col {
		b.WriteString(s[:col])
		b.WriteString("\r\n")
		s = s[col:]
	}
	if len(s) > 0 {
		b.WriteString(s)
		b.WriteString("\r\n")
	}
	return b.String()
}

func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\"", "'")
	return strings.TrimSpace(s)
}
