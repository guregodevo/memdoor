package gateway

import (
	"memdoor/gateway/email"
	"memdoor/pkg/auth"
)

// emailServiceAdapter adapts gateway/email.Service to auth.EmailService interface
type emailServiceAdapter struct {
	svc *email.Service
}

// NewEmailServiceAdapter creates an adapter for the email service
func NewEmailServiceAdapter(svc *email.Service) auth.EmailService {
	return &emailServiceAdapter{svc: svc}
}

// SendVerificationEmail sends an email verification email
func (a *emailServiceAdapter) SendVerificationEmail(to, name, token, baseURL string) error {
	return a.svc.SendEmailVerification(to, name, token, baseURL)
}

// SendPasswordResetEmail sends a password reset email
func (a *emailServiceAdapter) SendPasswordResetEmail(to, name, token, baseURL string) error {
	return a.svc.SendPasswordReset(to, name, token, baseURL)
}
