// Package questionnaires owns the questionnaire data contract and answer rules.
package questionnaires

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

type Settings struct {
	Min         *float64 `json:"min"`
	Max         *float64 `json:"max"`
	Unit        string   `json:"unit"`
	Precision   string   `json:"precision"`
	AllowFuture *bool    `json:"allowFuture"`
}
type Option struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
type Question struct {
	ID, Type, Audience, Dependency string
	DependencyValue                any
	Settings                       Settings
	Options                        []Option
}
type Definition struct{ Questions []Question }

func Load(app core.App, id string) (Definition, error) {
	d := Definition{}
	form, err := app.FindRecordById("questionnaires", id)
	if err != nil {
		return d, fmt.Errorf("questionnaire unavailable: %w", err)
	}
	if !form.GetBool("enabled") {
		return d, fmt.Errorf("questionnaire is disabled")
	}
	if form.GetString("occurance") != "once" {
		return d, fmt.Errorf("only one-time questionnaires are supported")
	}
	seen := map[string]bool{}
	for _, id := range form.GetStringSlice("questions") {
		if seen[id] {
			return d, fmt.Errorf("duplicate question %s", id)
		}
		seen[id] = true
		r, err := app.FindRecordById("questions", id)
		if err != nil {
			return d, err
		}
		q := Question{ID: id, Type: r.GetString("type"), Audience: r.GetString("audience"), Dependency: r.GetString("dependency")}
		if r.GetString("valueFromQuestion") != "" {
			return d, fmt.Errorf("valueFromQuestion is not supported")
		}
		raw := map[string]json.RawMessage{}
		if err := r.UnmarshalJSONField("settings", &raw); err != nil {
			return d, err
		}
		allowed := map[string]bool{}
		switch q.Type {
		case "integer":
			allowed = map[string]bool{"min": true, "max": true, "unit": true}
		case "date":
			allowed = map[string]bool{"precision": true, "allowFuture": true}
		case "text", "singleChoice":
		default:
			return d, fmt.Errorf("unsupported question type %s", q.Type)
		}
		for key := range raw {
			if !allowed[key] {
				return d, fmt.Errorf("unsupported setting %s on %s", key, id)
			}
		}
		if err := r.UnmarshalJSONField("settings", &q.Settings); err != nil {
			return d, err
		}
		if q.Settings.Precision == "" {
			q.Settings.Precision = "day"
		}
		if q.Settings.Precision != "day" && q.Settings.Precision != "month" {
			return d, fmt.Errorf("invalid date precision")
		}
		for _, bound := range []*float64{q.Settings.Min, q.Settings.Max} {
			if bound != nil && (math.IsNaN(*bound) || math.IsInf(*bound, 0) || math.Trunc(*bound) != *bound || math.Abs(*bound) > 9007199254740991) {
				return d, fmt.Errorf("invalid integer bound")
			}
		}
		if q.Settings.Min != nil && q.Settings.Max != nil && *q.Settings.Min > *q.Settings.Max {
			return d, fmt.Errorf("minimum exceeds maximum")
		}
		if q.Audience != "" && q.Audience != "all" && q.Audience != "female" && q.Audience != "male" {
			return d, fmt.Errorf("invalid audience")
		}
		if err := r.UnmarshalJSONField("dependencyValue", &q.DependencyValue); err != nil {
			return d, err
		}
		if q.Type == "singleChoice" {
			options, err := app.FindRecordById("questionOptions", r.GetString("options"))
			if err != nil {
				return d, err
			}
			if err := options.UnmarshalJSONField("value", &q.Options); err != nil {
				return d, err
			}
			values := map[string]bool{}
			if len(q.Options) == 0 {
				return d, fmt.Errorf("choice question has no options")
			}
			for _, o := range q.Options {
				if o.Label == "" || o.Value == "" || values[o.Value] {
					return d, fmt.Errorf("invalid or duplicate option")
				}
				values[o.Value] = true
			}
		}
		d.Questions = append(d.Questions, q)
	}
	return d, d.Check()
}

func (d Definition) Check() error {
	byID := map[string]Question{}
	for _, q := range d.Questions {
		if _, ok := byID[q.ID]; ok {
			return fmt.Errorf("duplicate question")
		}
		byID[q.ID] = q
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if done[id] {
			return nil
		}
		if visiting[id] {
			return fmt.Errorf("cyclic dependency")
		}
		q, ok := byID[id]
		if !ok {
			return fmt.Errorf("missing dependency %s", id)
		}
		visiting[id] = true
		if q.Dependency != "" {
			if err := visit(q.Dependency); err != nil {
				return err
			}
			values, ok := q.DependencyValue.([]any)
			if !ok {
				values = []any{q.DependencyValue}
			}
			if len(values) == 0 {
				return fmt.Errorf("empty dependency value")
			}
			parent := byID[q.Dependency]
			for _, v := range values {
				if v == nil || parent.Validate(v, time.Now()) != "" {
					return fmt.Errorf("invalid dependency value")
				}
			}
		} else if q.DependencyValue != nil {
			return fmt.Errorf("dependency value without parent")
		}
		delete(visiting, id)
		done[id] = true
		return nil
	}
	for id := range byID {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func matches(value, expected any) bool {
	if values, ok := expected.([]any); ok {
		for _, v := range values {
			if matches(value, v) {
				return true
			}
		}
		return false
	}
	// JSON primitives compare consistently across decoding and record fields.
	a, _ := json.Marshal(value)
	b, _ := json.Marshal(expected)
	return string(a) == string(b)
}

func (d Definition) Visible(answers map[string]any, gender string) map[string]bool {
	byID := map[string]Question{}
	for _, q := range d.Questions {
		byID[q.ID] = q
	}
	visible := map[string]bool{}
	visited := map[string]bool{}
	var show func(string) bool
	show = func(id string) bool {
		if visited[id] {
			return visible[id]
		}
		visited[id] = true
		q, ok := byID[id]
		if !ok {
			return false
		}
		result := q.Audience == "" || q.Audience == "all" || q.Audience == gender
		if q.Dependency != "" {
			v := answers[q.Dependency]
			result = result && show(q.Dependency) && v != nil && v != "" && matches(v, q.DependencyValue)
		}
		visible[id] = result
		return result
	}
	for id := range byID {
		show(id)
	}
	return visible
}

func (q Question) Validate(value any, now time.Time) string {
	switch q.Type {
	case "text":
		if _, ok := value.(string); !ok {
			return "Ange text."
		}
	case "integer":
		n, ok := value.(float64)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) || math.Trunc(n) != n || math.Abs(n) > 9007199254740991 {
			return "Ange ett heltal."
		}
		if q.Settings.Min != nil && n < *q.Settings.Min {
			return fmt.Sprintf("Ange minst %g.", *q.Settings.Min)
		}
		if q.Settings.Max != nil && n > *q.Settings.Max {
			return fmt.Sprintf("Ange högst %g.", *q.Settings.Max)
		}
	case "singleChoice":
		s, ok := value.(string)
		if !ok {
			return "Välj ett svarsalternativ."
		}
		for _, o := range q.Options {
			if o.Value == s {
				return ""
			}
		}
		return "Välj ett giltigt svarsalternativ."
	case "date":
		s, ok := value.(string)
		if !ok {
			return "Ange ett giltigt datum."
		}
		layout := "2006-01-02"
		if q.Settings.Precision == "month" {
			layout = "2006-01"
		}
		parsed, err := time.Parse(layout, s)
		if err != nil || parsed.Year() < 1 || parsed.Format(layout) != s {
			return "Ange ett giltigt datum."
		}
		if q.Settings.AllowFuture != nil && !*q.Settings.AllowFuture && s > now.Format(layout) {
			return "Datumet får inte ligga i framtiden."
		}
	default:
		return "Frågetypen stöds inte."
	}
	return ""
}

func (d Definition) Validate(answers map[string]any, gender string, now time.Time) map[string]string {
	errors := map[string]string{}
	visible := d.Visible(answers, gender)
	byID := map[string]Question{}
	for _, q := range d.Questions {
		byID[q.ID] = q
	}
	for id, value := range answers {
		q, ok := byID[id]
		if !ok {
			errors[id] = "Frågan tillhör inte formuläret."
			continue
		}
		if !visible[id] {
			errors[id] = "Frågan är inte tillgänglig."
			continue
		}
		if value == nil || value == "" {
			continue
		}
		if message := q.Validate(value, now); message != "" {
			errors[id] = message
		}
	}
	return errors
}
