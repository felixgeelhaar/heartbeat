package seed_test

import (
	"testing"

	"github.com/felixgeelhaar/go-teamhealthcheck/internal/seed"
)

func validateTemplate(t *testing.T, tmpl interface {
	GetName() string
	GetDescription() string
	GetBuiltIn() bool
	GetMetrics() interface {
		Len() int
		Get(i int) interface {
			GetName() string
			GetDescGood() string
			GetDescBad() string
			GetOrder() int
		}
	}
}) {
	t.Helper()
}

// assertTemplate validates the common invariants for any built-in template.
func assertTemplate(t *testing.T, name string, metricCount int, getter func() interface {
	GetBuiltIn() bool
}) {
	t.Helper()
}

func TestTuckmanTemplate(t *testing.T) {
	tmpl := seed.TuckmanTemplate()

	if tmpl.Name != "Team Maturity (Tuckman)" {
		t.Errorf("unexpected name: %q", tmpl.Name)
	}
	if tmpl.Description == "" {
		t.Error("expected non-empty description")
	}
	if !tmpl.BuiltIn {
		t.Error("expected BuiltIn=true")
	}
	if len(tmpl.Metrics) != 8 {
		t.Errorf("expected 8 metrics, got %d", len(tmpl.Metrics))
	}

	for _, m := range tmpl.Metrics {
		if m.Name == "" {
			t.Error("metric has empty name")
		}
		if m.DescriptionGood == "" {
			t.Errorf("metric %q has empty DescriptionGood", m.Name)
		}
		if m.DescriptionBad == "" {
			t.Errorf("metric %q has empty DescriptionBad", m.Name)
		}
	}

	for i, m := range tmpl.Metrics {
		if m.SortOrder != i+1 {
			t.Errorf("metric %q has sort order %d, expected %d", m.Name, m.SortOrder, i+1)
		}
	}
}

func TestPsychologicalSafetyTemplate(t *testing.T) {
	tmpl := seed.PsychologicalSafetyTemplate()

	if tmpl.Name != "Psychological Safety (Edmondson)" {
		t.Errorf("unexpected name: %q", tmpl.Name)
	}
	if tmpl.Description == "" {
		t.Error("expected non-empty description")
	}
	if !tmpl.BuiltIn {
		t.Error("expected BuiltIn=true")
	}
	if len(tmpl.Metrics) != 7 {
		t.Errorf("expected 7 metrics, got %d", len(tmpl.Metrics))
	}

	for _, m := range tmpl.Metrics {
		if m.Name == "" {
			t.Error("metric has empty name")
		}
		if m.DescriptionGood == "" {
			t.Errorf("metric %q has empty DescriptionGood", m.Name)
		}
		if m.DescriptionBad == "" {
			t.Errorf("metric %q has empty DescriptionBad", m.Name)
		}
	}

	for i, m := range tmpl.Metrics {
		if m.SortOrder != i+1 {
			t.Errorf("metric %q has sort order %d, expected %d", m.Name, m.SortOrder, i+1)
		}
	}
}

func TestDORATemplate(t *testing.T) {
	tmpl := seed.DORATemplate()

	if tmpl.Name != "DORA Metrics (DevOps)" {
		t.Errorf("unexpected name: %q", tmpl.Name)
	}
	if tmpl.Description == "" {
		t.Error("expected non-empty description")
	}
	if !tmpl.BuiltIn {
		t.Error("expected BuiltIn=true")
	}
	if len(tmpl.Metrics) != 8 {
		t.Errorf("expected 8 metrics, got %d", len(tmpl.Metrics))
	}

	for _, m := range tmpl.Metrics {
		if m.Name == "" {
			t.Error("metric has empty name")
		}
		if m.DescriptionGood == "" {
			t.Errorf("metric %q has empty DescriptionGood", m.Name)
		}
		if m.DescriptionBad == "" {
			t.Errorf("metric %q has empty DescriptionBad", m.Name)
		}
	}

	for i, m := range tmpl.Metrics {
		if m.SortOrder != i+1 {
			t.Errorf("metric %q has sort order %d, expected %d", m.Name, m.SortOrder, i+1)
		}
	}
}

func TestAllBuiltInTemplates_Count(t *testing.T) {
	templates := seed.AllBuiltInTemplates()

	if len(templates) != 4 {
		t.Errorf("expected 4 built-in templates, got %d", len(templates))
	}
}

func TestAllBuiltInTemplates_Names(t *testing.T) {
	templates := seed.AllBuiltInTemplates()

	expectedNames := map[string]bool{
		"Spotify Squad Health Check":       false,
		"Team Maturity (Tuckman)":          false,
		"Psychological Safety (Edmondson)": false,
		"DORA Metrics (DevOps)":            false,
	}

	for _, tmpl := range templates {
		if _, ok := expectedNames[tmpl.Name]; !ok {
			t.Errorf("unexpected template name: %q", tmpl.Name)
		}
		expectedNames[tmpl.Name] = true
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("expected template %q not found in AllBuiltInTemplates", name)
		}
	}
}

func TestAllBuiltInTemplates_AllBuiltIn(t *testing.T) {
	templates := seed.AllBuiltInTemplates()
	for _, tmpl := range templates {
		if !tmpl.BuiltIn {
			t.Errorf("template %q has BuiltIn=false, expected true", tmpl.Name)
		}
	}
}

func TestAllBuiltInTemplates_AllHaveMetrics(t *testing.T) {
	templates := seed.AllBuiltInTemplates()
	for _, tmpl := range templates {
		if len(tmpl.Metrics) == 0 {
			t.Errorf("template %q has no metrics", tmpl.Name)
		}
	}
}

func TestAllBuiltInTemplates_AllHaveDescriptions(t *testing.T) {
	templates := seed.AllBuiltInTemplates()
	for _, tmpl := range templates {
		if tmpl.Description == "" {
			t.Errorf("template %q has empty description", tmpl.Name)
		}
		for _, m := range tmpl.Metrics {
			if m.DescriptionGood == "" {
				t.Errorf("template %q, metric %q has empty DescriptionGood", tmpl.Name, m.Name)
			}
			if m.DescriptionBad == "" {
				t.Errorf("template %q, metric %q has empty DescriptionBad", tmpl.Name, m.Name)
			}
		}
	}
}

func TestAllBuiltInTemplates_MetricSortOrder(t *testing.T) {
	templates := seed.AllBuiltInTemplates()
	for _, tmpl := range templates {
		for i, m := range tmpl.Metrics {
			if m.SortOrder != i+1 {
				t.Errorf("template %q, metric %q: sort order %d, expected %d",
					tmpl.Name, m.Name, m.SortOrder, i+1)
			}
		}
	}
}

func TestAllBuiltInTemplates_IsCopy(t *testing.T) {
	// Each call returns a fresh slice — mutations must not affect subsequent calls.
	first := seed.AllBuiltInTemplates()
	first[0].Name = "Mutated"

	second := seed.AllBuiltInTemplates()
	if second[0].Name == "Mutated" {
		t.Error("AllBuiltInTemplates should return independent copies, not shared state")
	}
}

func TestTuckmanTemplate_SpecificMetrics(t *testing.T) {
	tmpl := seed.TuckmanTemplate()

	metricNames := make(map[string]bool, len(tmpl.Metrics))
	for _, m := range tmpl.Metrics {
		metricNames[m.Name] = true
	}

	expected := []string{"Trust & Safety", "Healthy Conflict", "Commitment", "Accountability"}
	for _, name := range expected {
		if !metricNames[name] {
			t.Errorf("expected metric %q in Tuckman template", name)
		}
	}
}

func TestPsychologicalSafetyTemplate_SpecificMetrics(t *testing.T) {
	tmpl := seed.PsychologicalSafetyTemplate()

	metricNames := make(map[string]bool, len(tmpl.Metrics))
	for _, m := range tmpl.Metrics {
		metricNames[m.Name] = true
	}

	expected := []string{"Speaking Up", "Asking for Help", "Admitting Mistakes"}
	for _, name := range expected {
		if !metricNames[name] {
			t.Errorf("expected metric %q in Psychological Safety template", name)
		}
	}
}

func TestDORATemplate_SpecificMetrics(t *testing.T) {
	tmpl := seed.DORATemplate()

	metricNames := make(map[string]bool, len(tmpl.Metrics))
	for _, m := range tmpl.Metrics {
		metricNames[m.Name] = true
	}

	expected := []string{"Deployment Frequency", "Lead Time for Changes", "Change Failure Rate", "Time to Restore"}
	for _, name := range expected {
		if !metricNames[name] {
			t.Errorf("expected metric %q in DORA template", name)
		}
	}
}
