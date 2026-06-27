package notification

import (
	"bytes"
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templatesFS embed.FS

var welcomeTemplate = template.Must(template.ParseFS(templatesFS, "templates/welcome.html"))

type welcomeTemplateData struct {
	Name string
}

func renderWelcomeEmail(name string) (string, error) {
	var buf bytes.Buffer
	if err := welcomeTemplate.Execute(&buf, welcomeTemplateData{Name: name}); err != nil {
		return "", err
	}
	return buf.String(), nil
}
