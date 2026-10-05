package email

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"memdoor/pkg/shared"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// Service handles email sending
type Service struct {
	provider string
	config   SMTPConfig
	logger   *slog.Logger
}

// SMTPConfig holds SMTP server configuration
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	FromName string
}

// NewService creates a new email service
func NewService(logger *slog.Logger) (*Service, error) {
	provider := os.Getenv("EMAIL_PROVIDER")
	if provider == "" {
		provider = "local" // Default to local file-based delivery for self-hosted
	}

	config := SMTPConfig{
		Host:     getEnvOrDefault("SMTP_HOST", "localhost"),
		Port:     getEnvOrDefault("SMTP_PORT", "587"),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     getEnvOrDefault("EMAIL_FROM", "noreply@memdoor.ai"),
		FromName: getEnvOrDefault("EMAIL_FROM_NAME", "Memdoor"),
	}

	logger.Info("Email service initialized",
		slog.String("provider", provider),
		slog.String("mode", getEmailModeDescription(provider)))

	return &Service{
		provider: provider,
		config:   config,
		logger:   logger,
	}, nil
}

// getEmailModeDescription returns a user-friendly description of the email mode
func getEmailModeDescription(provider string) string {
	switch provider {
	case "local":
		return "File-based (verification links logged to console)"
	case "console":
		return "Console-only (emails printed to stdout)"
	case "smtp":
		return "SMTP server"
	case "resend":
		return "Resend API (resend.com)"
	default:
		return "Unknown"
	}
}

// getEnvOrDefault gets environment variable or returns default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Email represents an email message
type Email struct {
	To      string
	Subject string
	Body    string
	IsHTML  bool
}

// Send sends an email
func (s *Service) Send(email Email) error {
	switch s.provider {
	case "local":
		return s.sendLocal(email)
	case "console":
		return s.sendConsole(email)
	case "smtp":
		return s.sendSMTP(email)
	case "resend":
		return s.sendResend(email)
	default:
		return fmt.Errorf("unsupported email provider: %s", s.provider)
	}
}

// sendSMTP sends email via SMTP
func (s *Service) sendSMTP(email Email) error {
	// Build email message
	var message bytes.Buffer
	message.WriteString(fmt.Sprintf("From: %s <%s>\r\n", s.config.FromName, s.config.From))
	message.WriteString(fmt.Sprintf("To: %s\r\n", email.To))
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", email.Subject))

	if email.IsHTML {
		message.WriteString("MIME-Version: 1.0\r\n")
		message.WriteString("Content-Type: text/html; charset=utf-8\r\n")
	} else {
		message.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	}

	message.WriteString("\r\n")
	message.WriteString(email.Body)

	// Setup authentication
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)

	// Try to send with TLS
	err := s.sendWithTLS(addr, auth, email.To, message.Bytes())
	if err != nil {
		s.logger.Error("Failed to send email via SMTP",
			slog.String("to", email.To),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to send email: %w", err)
	}

	s.logger.Info("Email sent successfully",
		slog.String("to", email.To),
		slog.String("subject", email.Subject))

	return nil
}

// sendWithTLS sends email with TLS encryption
func (s *Service) sendWithTLS(addr string, auth smtp.Auth, to string, msg []byte) error {
	// Connect to server
	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer client.Close()

	// Start TLS
	tlsConfig := &tls.Config{
		ServerName: s.config.Host,
	}

	if err = client.StartTLS(tlsConfig); err != nil {
		return err
	}

	// Authenticate
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}

	// Set sender
	if err = client.Mail(s.config.From); err != nil {
		return err
	}

	// Set recipient
	if err = client.Rcpt(to); err != nil {
		return err
	}

	// Send message
	w, err := client.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

// sendResend sends email via Resend API (https://resend.com/docs/api-reference/emails/send-email)
func (s *Service) sendResend(email Email) error {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("RESEND_API_KEY environment variable is required")
	}

	from := fmt.Sprintf("%s <%s>", s.config.FromName, s.config.From)

	payload := map[string]interface{}{
		"from":    from,
		"to":      []string{email.To},
		"subject": email.Subject,
	}
	if email.IsHTML {
		payload["html"] = email.Body
	} else {
		payload["text"] = email.Body
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("Resend API error",
			slog.Int("status", resp.StatusCode),
			slog.String("response", string(respBody)))
		return fmt.Errorf("resend API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	s.logger.Info("Email sent via Resend",
		slog.String("to", email.To),
		slog.String("subject", email.Subject))

	return nil
}

// sendLocal saves emails to local files for self-hosted deployments
// This is perfect for development and self-hosted instances where SMTP is not configured
func (s *Service) sendLocal(email Email) error {
	// Get emails directory from config
	emailsDir := shared.MemdoorHome("emails")

	// Create emails directory
	if err := os.MkdirAll(emailsDir, 0755); err != nil {
		return fmt.Errorf("failed to create emails directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(emailsDir, fmt.Sprintf("%s-%s.txt", timestamp, sanitizeFilename(email.Subject)))

	// Create email content
	content := fmt.Sprintf("To: %s\nFrom: %s <%s>\nSubject: %s\nDate: %s\n\n%s",
		email.To,
		s.config.FromName,
		s.config.From,
		email.Subject,
		time.Now().Format(time.RFC1123),
		email.Body)

	// Write to file
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write email file: %w", err)
	}

	// Log to console with clickable link extraction
	s.logger.Info("📧 Email saved locally (self-hosted mode)",
		slog.String("to", email.To),
		slog.String("subject", email.Subject),
		slog.String("file", filename))

	// Extract and log verification/reset links for easy access
	s.logExtractedLinks(email)

	return nil
}

// sendConsole logs the full email instead of delivering it (development/testing)
func (s *Service) sendConsole(email Email) error {
	s.logger.Info("📧 Email (console mode)",
		slog.String("to", email.To),
		slog.String("from", fmt.Sprintf("%s <%s>", s.config.FromName, s.config.From)),
		slog.String("subject", email.Subject),
		slog.String("body", email.Body))

	// Extract and log verification/reset links
	s.logExtractedLinks(email)

	return nil
}

// logExtractedLinks extracts and logs verification/reset URLs from email body
func (s *Service) logExtractedLinks(email Email) {
	// Extract URLs from email body
	// Look for http:// or https:// followed by non-whitespace characters
	body := email.Body
	startIdx := 0

	for {
		// Find next URL
		httpIdx := indexAny(body[startIdx:], "http://", "https://")
		if httpIdx == -1 {
			break
		}

		urlStart := startIdx + httpIdx
		urlEnd := urlStart

		// Find end of URL (whitespace, <, or end of string)
		for urlEnd < len(body) && body[urlEnd] != ' ' && body[urlEnd] != '\n' && body[urlEnd] != '<' && body[urlEnd] != '"' {
			urlEnd++
		}

		url := body[urlStart:urlEnd]

		// Log the URL with appropriate context
		if strings.Contains(url, "verify-email") {
			s.logger.Info("🔗 Email Verification Link",
				slog.String("url", url),
				slog.String("action", "Click to verify email"))
		} else if strings.Contains(url, "reset-password") {
			s.logger.Info("🔗 Password Reset Link",
				slog.String("url", url),
				slog.String("action", "Click to reset password"))
		}

		startIdx = urlEnd
	}
}

// indexAny finds the first occurrence of any of the substrings in s
func indexAny(s string, substrs ...string) int {
	minIdx := -1
	for _, substr := range substrs {
		idx := strings.Index(s, substr)
		if idx != -1 && (minIdx == -1 || idx < minIdx) {
			minIdx = idx
		}
	}
	return minIdx
}

// sanitizeFilename removes characters that are invalid in filenames
func sanitizeFilename(s string) string {
	// Replace invalid characters with underscores
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	result := s
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	// Limit length
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

// SendEmailVerification sends an email verification email.
// If workspace is non-empty, URLs are prefixed: /{workspace}/verify-email?token=...
func (s *Service) SendEmailVerification(to, name, token, baseURL string, workspace ...string) error {
	wsPrefix := ""
	if len(workspace) > 0 && workspace[0] != "" {
		wsPrefix = "/" + workspace[0]
	}
	verificationURL := fmt.Sprintf("%s%s/verify-email?token=%s", baseURL, wsPrefix, url.QueryEscape(token))

	body, err := renderTemplate(emailVerificationTemplate, map[string]string{
		"Name":            name,
		"VerificationURL": verificationURL,
		"BaseURL":         baseURL,
	})
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	return s.Send(Email{
		To:      to,
		Subject: "Verify your email - Memdoor",
		Body:    body,
		IsHTML:  true,
	})
}

// SendPasswordReset sends a password reset email.
// If workspace is non-empty, URLs are prefixed: /{workspace}/reset-password?token=...
func (s *Service) SendPasswordReset(to, name, token, baseURL string, workspace ...string) error {
	wsPrefix := ""
	if len(workspace) > 0 && workspace[0] != "" {
		wsPrefix = "/" + workspace[0]
	}
	resetURL := fmt.Sprintf("%s%s/reset-password?token=%s", baseURL, wsPrefix, url.QueryEscape(token))

	body, err := renderTemplate(passwordResetTemplate, map[string]string{
		"Name":     name,
		"ResetURL": resetURL,
		"BaseURL":  baseURL,
	})
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	return s.Send(Email{
		To:      to,
		Subject: "Reset your password - Memdoor",
		Body:    body,
		IsHTML:  true,
	})
}

// SendInvite sends a workspace invite email.
// If workspace is non-empty, URLs are prefixed: /{workspace}/?register=true
func (s *Service) SendInvite(to, inviterName, channelName, message, baseURL string, workspace ...string) error {
	wsPrefix := ""
	if len(workspace) > 0 && workspace[0] != "" {
		wsPrefix = "/" + workspace[0]
	}
	inviteURL := baseURL + wsPrefix + "/?register=true"
	if channelName != "" {
		inviteURL += "&channel=" + url.QueryEscape(channelName)
	}

	body, err := renderTemplate(inviteTemplate, map[string]string{
		"InviterName": inviterName,
		"ChannelName": channelName,
		"Message":     message,
		"InviteURL":   inviteURL,
		"BaseURL":     baseURL,
	})
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	subject := fmt.Sprintf("%s invited you to Memdoor", inviterName)

	return s.Send(Email{
		To:      to,
		Subject: subject,
		Body:    body,
		IsHTML:  true,
	})
}

// SendInviteWithToken sends a workspace invite email with an invite token for invite-only registration.
// If workspace is non-empty, URLs are prefixed: /{workspace}/?invite=...
func (s *Service) SendInviteWithToken(to, inviterName, channelName, message, baseURL, inviteToken string, workspace ...string) error {
	wsPrefix := ""
	if len(workspace) > 0 && workspace[0] != "" {
		wsPrefix = "/" + workspace[0]
	}
	inviteURL := baseURL + wsPrefix + "/?invite=" + url.QueryEscape(inviteToken)
	if channelName != "" {
		inviteURL += "&channel=" + url.QueryEscape(channelName)
	}

	body, err := renderTemplate(inviteTemplate, map[string]string{
		"InviterName": inviterName,
		"ChannelName": channelName,
		"Message":     message,
		"InviteURL":   inviteURL,
		"BaseURL":     baseURL,
	})
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	subject := fmt.Sprintf("%s invited you to Memdoor", inviterName)

	return s.Send(Email{
		To:      to,
		Subject: subject,
		Body:    body,
		IsHTML:  true,
	})
}

// SendDemoRequest sends a styled demo request notification email
func (s *Service) SendDemoRequest(to, requesterEmail, youtubeChannel string) error {
	if youtubeChannel == "" {
		youtubeChannel = "(not provided)"
	}

	body, err := renderTemplate(demoRequestTemplate, map[string]string{
		"Email":          requesterEmail,
		"YouTubeChannel": youtubeChannel,
		"Time":           time.Now().Format(time.RFC1123),
	})
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	return s.Send(Email{
		To:      to,
		Subject: "New Demo Request — " + requesterEmail,
		Body:    body,
		IsHTML:  true,
	})
}

// renderTemplate renders an email template
func renderTemplate(templateStr string, data map[string]string) (string, error) {
	tmpl, err := template.New("email").Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
