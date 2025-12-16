package mailer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"

	"stu/internal/config"
)

// Mailer sends e-mails via SMTP (Mailpit) or MailerSend HTTP API.
type Mailer struct {
	cfg config.MailerConfig
}

func New(cfg config.MailerConfig) *Mailer {
	return &Mailer{cfg: cfg}
}

// SendVerification delivers verification code for user signup/login.
func (m *Mailer) SendVerification(toEmail, code string) error {
	subject := "Код подтверждения Stu"
	body := fmt.Sprintf("Ваш код подтверждения Stu: %s\nОн действует 15 минут.", code)
	return m.send(toEmail, subject, body)
}

// SendAdminCode sends MFA code for admin login.
func (m *Mailer) SendAdminCode(toEmail, code string) error {
	subject := "Stu: код для входа в админку"
	body := fmt.Sprintf("Код безопасности для входа в админку Stu: %s\nСрок действия 10 минут.", code)
	return m.send(toEmail, subject, body)
}

func (m *Mailer) send(toEmail, subject, body string) error {
	if strings.ToLower(m.cfg.Mode) == "mailersend" {
		return m.sendMailerSend(toEmail, subject, body)
	}
	return m.sendSMTP(toEmail, subject, body)
}

func (m *Mailer) sendSMTP(toEmail, subject, body string) error {
	var auth smtp.Auth
	if m.cfg.SMTPUser != "" || m.cfg.SMTPPass != "" {
		auth = smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPass, m.cfg.SMTPHost)
	}
	addr := fmt.Sprintf("%s:%d", m.cfg.SMTPHost, m.cfg.SMTPPort)
	msg := strings.Join([]string{
		fmt.Sprintf("From: %s", m.cfg.From),
		fmt.Sprintf("To: %s", toEmail),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")
	return smtp.SendMail(addr, auth, m.cfg.From, []string{toEmail}, []byte(msg))
}

func (m *Mailer) sendMailerSend(toEmail, subject, body string) error {
	if m.cfg.APIKey == "" {
		return fmt.Errorf("mailersend api key missing")
	}
	payload := map[string]any{
		"from": map[string]string{
			"email": m.cfg.From,
			"name":  m.cfg.FromName,
		},
		"to": []map[string]string{
			{"email": toEmail},
		},
		"subject": subject,
		"text":    body,
	}
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.mailersend.com/v1/email", strings.NewReader(string(buf)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.cfg.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("mailersend status %d", resp.StatusCode)
	}
	return nil
}
