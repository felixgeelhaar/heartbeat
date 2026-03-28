package domain

import "time"

// TemplateMetric describes one dimension of a health check (e.g. "Easy to Release").
// The Good/Bad descriptions anchor the green and red ends of the scale.
type TemplateMetric struct {
	ID              string
	TemplateID      string
	Name            string
	DescriptionGood string
	DescriptionBad  string
	SortOrder       int
}

// Template is an aggregate root that groups a set of metrics into a reusable health check format.
type Template struct {
	ID          string
	Name        string
	Description string
	BuiltIn     bool
	Metrics     []TemplateMetric
	CreatedAt   time.Time
}

// TemplateRepository defines persistence operations for the Template aggregate.
type TemplateRepository interface {
	CreateTemplate(template *Template) error
	FindTemplateByID(id string) (*Template, error)
	FindTemplateByName(name string) (*Template, error)
	FindAllTemplates() ([]*Template, error)
	DeleteTemplate(id string) error
}
