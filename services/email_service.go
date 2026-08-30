package services

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"html"
	"log"
	"net"
	"net/smtp"
	"time"

	"github.com/nabungyuk/nabungyuk/config"
)

// EmailService handles sending email via SMTP
type EmailService struct {
	host     string
	port     string
	username string
	password string
	from     string
}

// NewEmailService creates an EmailService from environment config
func NewEmailService() *EmailService {
	username := config.GetEnv("SMTP_USERNAME", "")
	return &EmailService{
		host:     config.GetEnv("SMTP_HOST", "smtp.gmail.com"),
		port:     config.GetEnv("SMTP_PORT", "587"),
		username: username,
		password: config.GetEnv("SMTP_PASSWORD", ""),
		from:     username,
	}
}

// IsConfigured returns true if SMTP credentials are set in the environment
func (s *EmailService) IsConfigured() bool {
	return s.username != "" && s.password != ""
}

// SendReminder sends a saving reminder email
func (s *EmailService) SendReminder(to, userName, goalName string, amount, progress int64, deadline string) error {
	subject := "🌱 Waktunya Menabung!"
	body := buildReminderBody(userName, goalName, amount, progress, deadline)
	return s.send(to, subject, body)
}

// SendGeneric sends a generic email with subject and HTML body
func (s *EmailService) send(to, subject, body string) error {
	if !s.IsConfigured() {
		return fmt.Errorf("SMTP belum dikonfigurasi. Set SMTP_USERNAME dan SMTP_PASSWORD di .env")
	}

	// Build MIME message (UTF-8 + HTML)
	msg := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"Subject: " + encodeSubject(subject) + "\r\n" +
		"From: " + s.from + "\r\n" +
		"To: " + to + "\r\n" +
		"\r\n" +
		body

	addr := net.JoinHostPort(s.host, s.port)
	auth := smtp.PlainAuth("", s.username, s.password, s.host)

	var err error
	if s.port == "465" {
		err = s.sendSSL(addr, s.username, to, auth, []byte(msg))
	} else {
		err = s.sendSTARTTLS(addr, s.username, to, auth, []byte(msg))
	}

	if err != nil {
		log.Printf("[email] failed to send to %s: %v", to, err)
		return err
	}
	log.Printf("[email] sent reminder to %s (subject: %s)", to, subject)
	return nil
}

// sendSSL uses implicit TLS, which is required by SMTPS on port 465.
func (s *EmailService) sendSSL(addr, from, to string, auth smtp.Auth, msg []byte) error {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// sendSTARTTLS uses the normal SMTP flow and explicitly requires STARTTLS.
func (s *EmailService) sendSTARTTLS(addr, from, to string, auth smtp.Auth, msg []byte) error {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("SMTP server tidak mendukung STARTTLS")
	}
	if err := client.StartTLS(&tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}); err != nil {
		return err
	}
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// encodeSubject encodes a non-ASCII subject line for email headers
func encodeSubject(subject string) string {
	// Use RFC 2047 encoded-word for UTF-8 subject
	return "=?UTF-8?B?" + base64Encode(subject) + "?="
}

// base64Encode encodes a string to base64.
func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// buildReminderBody constructs the HTML email body
func buildReminderBody(userName, goalName string, suggestedAmount, progress int64, deadline string) string {
	userName = html.EscapeString(userName)
	goalName = html.EscapeString(goalName)
	deadline = html.EscapeString(deadline)
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="margin:0;padding:0;background-color:#f4f7f6;font-family:Arial,Helvetica,sans-serif;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f4f7f6;padding:24px;">
    <tr><td align="center">
      <table width="520" cellpadding="0" cellspacing="0" style="max-width:520px;background:#ffffff;border-radius:16px;overflow:hidden;box-shadow:0 4px 24px rgba(0,0,0,.08);">
        <tr>
          <td style="background:#16a34a;padding:24px 32px;text-align:center;">
            <span style="font-size:40px;">🌱</span>
            <h1 style="color:#ffffff;margin:8px 0 0;font-size:24px;">Waktunya Menabung!</h1>
          </td>
        </tr>
        <tr>
          <td style="padding:32px;">
            <p style="margin:0 0 16px;color:#334155;font-size:15px;line-height:1.6;">Halo <strong>%s</strong>,</p>
            <p style="margin:0 0 24px;color:#334155;font-size:15px;line-height:1.6;">Jangan lupa menabung hari ini ya! Tetap konsisten menuju target kamu. 💪</p>

            <table width="100%%" cellspacing="0" style="margin-bottom:24px;">
              <tr><td style="padding:16px;background:#f0fdf4;border-radius:12px;border:1px solid #bbf7d0;">
                <table width="100%%">
                  <tr>
                    <td style="color:#475569;font-size:13px;padding:4px 0;">🎯 Target</td>
                    <td style="color:#0f172a;font-size:15px;font-weight:bold;text-align:right;padding:4px 0;">%s</td>
                  </tr>
                  <tr>
                    <td style="color:#475569;font-size:13px;padding:4px 0;">📈 Progress</td>
                    <td style="color:#0f172a;font-size:15px;font-weight:bold;text-align:right;padding:4px 0;">%d%%</td>
                  </tr>
                  <tr>
                    <td style="color:#475569;font-size:13px;padding:4px 0;">💡 Saran menabung</td>
                    <td style="color:#16a34a;font-size:15px;font-weight:bold;text-align:right;padding:4px 0;">Rp%d</td>
                  </tr>
                  <tr>
                    <td style="color:#475569;font-size:13px;padding:4px 0;">⏰ Deadline</td>
                    <td style="color:#0f172a;font-size:15px;font-weight:bold;text-align:right;padding:4px 0;">%s</td>
                  </tr>
                </table>
              </td></tr>
            </table>

            <p style="margin:0 0 8px;color:#475569;font-size:14px;">Tetap konsisten!</p>
            <p style="margin:0;color:#94a3b8;font-size:13px;">— NabungYuk · Kelola uang, capai impian.</p>
          </td>
        </tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, userName, goalName, progress, suggestedAmount, deadline)
}
