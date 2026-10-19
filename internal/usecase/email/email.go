package email

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/google/uuid"
	cd "github.com/hekemen/automata/internal/domain/contact"
	"github.com/hekemen/automata/internal/domain/email"
	fwform "github.com/hekemen/automata/internal/domain/form"
	"github.com/hekemen/automata/internal/infrastructure/queue"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Sentinel errors for SendToContact, mapped to HTTP status codes by the handler.
var (
	ErrTemplateNotFound = errors.New("template not found")
	ErrContactNotFound  = errors.New("contact not found")
	ErrNotStandalone    = errors.New("template is not standalone")
	ErrNoEmail          = errors.New("contact has no email address")
)

// EmailUsecase handles business logic for email template rendering and sending.
type EmailUsecase struct {
	repo        email.Repository
	queue       queue.Queue
	contactRepo cd.Repository
	log         zerolog.Logger
}

// NewEmailUsecase creates a new EmailUsecase without contact access.
// Contact-dependent methods (SendToContact) are unavailable and return errors.
func NewEmailUsecase(repo email.Repository, q queue.Queue) *EmailUsecase {
	return NewEmailUsecaseWithContact(repo, q, nil)
}

// NewEmailUsecaseWithContact creates a new EmailUsecase with contact access
// for standalone template sending and activity tracking.
func NewEmailUsecaseWithContact(repo email.Repository, q queue.Queue, contactRepo cd.Repository) *EmailUsecase {
	return &EmailUsecase{
		repo:        repo,
		queue:       q,
		contactRepo: contactRepo,
		log:         zerolog.New(os.Stdout).With().Str("service", "email_usecase").Logger(),
	}
}

// RenderTemplate resolves a template by key and renders it with the given variables.
// Variables may contain nested maps (e.g. "custom", "form") for template dot-access.
func (u *EmailUsecase) RenderTemplate(contextID, templateKey string, variables map[string]interface{}) (subject, body, htmlBody string, err error) {
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
	renderedSubject, renderedBody, renderedHTML, err := u.RenderTemplate(contextID, req.Template, toInterfaceVars(req.Variables))
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
	subject, body, _, err := u.RenderTemplate(contextID, req.Template, toInterfaceVars(req.Variables))
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

// SendToContact renders a standalone template with the contact's variables,
// enqueues an email job to the contact's address, and records an email_sent
// activity on the contact. Returns the enqueued job ID.
func (u *EmailUsecase) SendToContact(contextID, templateID, contactID string) (string, error) {
	if u.contactRepo == nil {
		return "", ErrContactNotFound
	}

	tmpl, err := u.repo.GetByID(templateID)
	if err != nil {
		return "", ErrTemplateNotFound
	}
	if tmpl.ContextID != contextID {
		return "", ErrTemplateNotFound
	}
	if tmpl.Kind != email.TemplateKindStandalone {
		return "", ErrNotStandalone
	}

	contact, err := u.contactRepo.GetByID(contactID)
	if err != nil {
		return "", ErrContactNotFound
	}
	if contact.ContextID != contextID {
		return "", ErrContactNotFound
	}
	if contact.Email == nil || *contact.Email == "" {
		return "", ErrNoEmail
	}

	vars := BuildContactVariables(contact)
	subject, body, htmlBody, err := u.renderByID(tmpl, vars)
	if err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}

	job := &queue.EmailJob{
		ContextID:  contextID,
		To:         []string{*contact.Email},
		Subject:    subject,
		Body:       body,
		HTMLBody:   htmlBody,
		MaxRetries: 3,
	}
	if err := u.queue.Enqueue(job); err != nil {
		return "", fmt.Errorf("enqueue email job: %w", err)
	}

	activity := &cd.Activity{
		ID:        uuid.New().String(),
		ContactID: contactID,
		ContextID: contextID,
		Type:      cd.ActivityEmailSent,
		Data: map[string]interface{}{
			"subject":       subject,
			"template_name": tmpl.Key,
		},
	}
	if err := u.contactRepo.CreateActivity(activity); err != nil {
		log.Error().Err(err).Str("contact_id", contactID).Msg("failed to record email_sent activity")
	}

	log.Info().Str("to", *contact.Email).Str("template", tmpl.Key).Msg("standalone email enqueued")
	return job.ID, nil
}

// SendForSubmission sends all form-integrated templates for the form to the
// submitter's email address, recorded from the first email-type field in the
// submission. All failures are logged, never returned: form submission must
// not fail because of email.
func (u *EmailUsecase) SendForSubmission(f *fwform.Form, s *fwform.Submission) {
	templates, err := u.repo.ListByForm(f.ContextID, f.ID)
	if err != nil {
		log.Error().Err(err).Str("form_id", f.ID).Msg("list form email templates")
		return
	}
	if len(templates) == 0 {
		return
	}

	to := submissionEmail(f, s)
	if to == "" {
		return
	}

	var contact *cd.Contact
	if u.contactRepo != nil {
		if c, err := u.contactRepo.FindByEmail(f.ContextID, to); err == nil {
			contact = c
		}
	}

	vars := BuildFormVariables(f, s, contact)
	for _, tmpl := range templates {
		subject, body, htmlBody, err := u.renderByID(tmpl, vars)
		if err != nil {
			log.Error().Err(err).Str("template", tmpl.Key).Msg("render form email template")
			continue
		}

		job := &queue.EmailJob{
			ContextID:  f.ContextID,
			To:         []string{to},
			Subject:    subject,
			Body:       body,
			HTMLBody:   htmlBody,
			MaxRetries: 3,
		}
		if err := u.queue.Enqueue(job); err != nil {
			log.Error().Err(err).Str("template", tmpl.Key).Msg("enqueue form email")
			continue
		}

		if contact != nil {
			activity := &cd.Activity{
				ID:        uuid.New().String(),
				ContactID: contact.ID,
				ContextID: f.ContextID,
				Type:      cd.ActivityEmailSent,
				Data: map[string]interface{}{
					"subject":       subject,
					"template_name": tmpl.Key,
				},
			}
			if err := u.contactRepo.CreateActivity(activity); err != nil {
				log.Error().Err(err).Str("contact_id", contact.ID).Msg("failed to record email_sent activity")
			}
		}

		log.Info().Str("to", to).Str("template", tmpl.Key).Msg("form email enqueued")
	}
}

// submissionEmail returns the value of the first email-type field in the
// submission data, or "" when the submission carries no email.
func submissionEmail(f *fwform.Form, s *fwform.Submission) string {
	if s == nil || s.Data == nil {
		return ""
	}
	for _, field := range f.Fields {
		if field.Type != "email" {
			continue
		}
		if v, ok := s.Data[field.Slug].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// renderByID renders a template entity (subject, body, html) with the given variables.
func (u *EmailUsecase) renderByID(tmpl *email.EmailTemplate, variables map[string]interface{}) (subject, body, htmlBody string, err error) {
	subjTmpl, err := template.New("subject").Parse(tmpl.Subject)
	if err != nil {
		return "", "", "", fmt.Errorf("parse subject template: %w", err)
	}
	var subjBuf strings.Builder
	if err := subjTmpl.Execute(&subjBuf, variables); err != nil {
		return "", "", "", fmt.Errorf("render subject: %w", err)
	}

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

// toInterfaceVars converts a flat string map to interface values so it can be
// passed to RenderTemplate, which accepts nested interface values.
func toInterfaceVars(vars map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(vars))
	for k, v := range vars {
		out[k] = v
	}
	return out
}
