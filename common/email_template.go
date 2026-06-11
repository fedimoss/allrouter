package common

import (
	"embed"
	"fmt"
	"html/template"
	"strings"
	"sync"
)

//go:embed email_templates/*.html
var emailTemplatesFS embed.FS

var (
	emailTemplates     *template.Template
	emailTemplatesOnce sync.Once
	emailTemplatesErr  error
)

// initEmailTemplates parses all embedded email templates exactly once.
func initEmailTemplates() error {
	emailTemplatesOnce.Do(func() {
		entries, err := emailTemplatesFS.ReadDir("email_templates")
		if err != nil {
			emailTemplatesErr = fmt.Errorf("failed to read email templates directory: %w", err)
			return
		}

		tmpl := template.New("email")
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			data, readErr := emailTemplatesFS.ReadFile("email_templates/" + entry.Name())
			if readErr != nil {
				emailTemplatesErr = fmt.Errorf("failed to read email template %s: %w", entry.Name(), readErr)
				return
			}
			if _, parseErr := tmpl.New(entry.Name()).Parse(string(data)); parseErr != nil {
				emailTemplatesErr = fmt.Errorf("failed to parse email template %s: %w", entry.Name(), parseErr)
				return
			}
		}
		emailTemplates = tmpl
	})
	return emailTemplatesErr
}

// RenderEmailTemplate renders an email template with the given data and returns the HTML content.
func RenderEmailTemplate(templateName string, data any) (string, error) {
	if err := initEmailTemplates(); err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := emailTemplates.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", fmt.Errorf("failed to render email template %s: %w", templateName, err)
	}
	return buf.String(), nil
}
