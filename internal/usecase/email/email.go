package email

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/hekemen/automata/internal/domain/email"
	"github.com/hekemen/automata/internal/infrastructure/queue"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// EmailUsecase handles business logic for email template rendering and sending.
type EmailUsecase struct {
	repo  email.Repository
	queue queue.Queue
	log   zerolog.Logger
}

// NewEmailUsecase creates a new EmailUsecase.
func NewEmailUsecase(repo email.Repository, q queue.Queue) *EmailUsecase {
	return &EmailUsecase{
		repo:  repo,
		queue: q,
		log:   zerolog.New(os.Stdout).With().Str("service", "email_usecase").Logger(),
	}
}

// RenderTemplate resolves a template by key and renders it with the given variables.
func (u *EmailUsecase) RenderTemplate(contextID, templateKey string, variables map[string]string) (subject, body, htmlBody string, err error) {
	tmpl, err := u.repo.GetByKey(contextID, templateKey)
	if err != nil {
		return "", "", "", fmt.Errorf("template %q not found: %w", templateKey, err)
	}

	// Render subject
	subjTmpl, err := template.New("subject").Parse(tmpl.Subject)
	if err != nil {
		return "", "", "", fmt.Errorf("parse subject template: %w", err)
	}

	var subjBuf strings.Builder
	if err := subjTmpl.Execute(&subjBuf, variables); err != nil {
		return "", "", "", fmt.Errorf("render subject: %w", err)
	}

	// Render body_text
	bodyTmpl, err := template.New("body").Parse(tmpl.BodyText)
	if err != nil {
		return "", "", "", fmt.Errorf("parse body template: %w", err)
	}

	var bodyBuf strings.Builder
	if err := bodyTmpl.Execute(&bodyBuf, variables); err != nil {
		return "", "", "", fmt.Errorf("render body: %w", err)
	}

	htmlBody = tmpl.BodyHTML
	if htmlBody != "" {
		htmlTmpl, err := template.New("html").Parse(htmlBody)
		if err != nil {
			return "", "", "", fmt.Errorf("parse html template: %w", err)
		}

		var htmlBuf strings.Builder
		if err := htmlTmpl.Execute(&htmlBuf, variables); err != nil {
			return "", "", "", fmt.Errorf("render html: %w", err)
		}

		htmlBody = htmlBuf.String()
	}

	return subjBuf.String(), bodyBuf.String(), htmlBody, nil
}

// SendEmail resolves a template, renders it, and enqueues an email job.
func (u *EmailUsecase) SendEmail(contextID string, req email.SendEmailRequest) (string, error) {
	// Normalize recipients
	toAddresses := normalizeRecipients(req.To, req.ToArray)
	if len(toAddresses) == 0 {
		return "", fmt.Errorf("no recipients specified")
	}

	// Render template
	renderedSubject, renderedBody, renderedHTML, err := u.RenderTemplate(contextID, req.Template, req.Variables)
	if err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}

	// Apply overrides
	subject := renderedSubject
	if req.SubjectOverride != nil && *req.SubjectOverride != "" {
		subject = *req.SubjectOverride
	}

	body := renderedBody
	htmlBody := renderedHTML
	if req.BodyOverride != nil && *req.BodyOverride != "" {
		body = *req.BodyOverride
		htmlBody = *req.BodyOverride
	}

	// Build email job
	job := &queue.EmailJob{
		ContextID:  contextID,
		To:         toAddresses,
		Subject:    subject,
		Body:       body,
		HTMLBody:   htmlBody,
		MaxRetries: 3,
	}

	if err := u.queue.Enqueue(job); err != nil {
		return "", fmt.Errorf("enqueue email job: %w", err)
	}

	log.Info().Strs("to", toAddresses).Str("template", req.Template).Msg("email job enqueued")
	return job.ID, nil
}

// TestSend renders a template with the given variables and enqueues a test delivery.
func (u *EmailUsecase) TestSend(contextID string, req email.TestSendRequest) (jobID, renderedSubject, preview string, err error) {
	// Render template
	subject, body, _, err := u.RenderTemplate(contextID, req.Template, req.Variables)
	if err != nil {
		return "", "", "", fmt.Errorf("render template: %w", err)
	}

	// Build preview (first 200 chars of body)
	preview = body
	if len(preview) > 200 {
		preview = preview[:200]
	}

	// Enqueue test job
	job := &queue.EmailJob{
		ContextID:  contextID,
		To:         []string{req.To},
		Subject:    subject,
		Body:       body,
		MaxRetries: 3,
	}

	if err := u.queue.Enqueue(job); err != nil {
		return "", "", "", fmt.Errorf("enqueue test email: %w", err)
	}

	log.Info().Str("to", req.To).Str("template", req.Template).Msg("test email enqueued")
	return job.ID, subject, preview, nil
}

// Repo returns the email repository (for handler access).
func (u *EmailUsecase) Repo() email.Repository {
	return u.repo
}

// normalizeRecipients accepts a single "to" string or an array, returning a deduplicated list.
func normalizeRecipients(to string, toArray []string) []string {
	if to != "" {
		return []string{to}
	}
	if len(toArray) > 0 {
		return toArray
	}
	return nil
}
