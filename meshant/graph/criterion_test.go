// criterion_test.go tests the EquivalenceCriterion type — its zero-value
// detection, layer ordering, and Validate() enforcement.
//
// Package graph_test (black-box) — tests use only the exported API.
package graph_test

import (
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// TestEquivalenceCriterionIsZero verifies zero-value detection across all
// combinations of empty and populated fields.
func TestEquivalenceCriterionIsZero(t *testing.T) {
	t.Run("zero value (all empty) is zero", func(t *testing.T) {
		var c graph.EquivalenceCriterion
		if !c.IsZero() {
			t.Error("zero-value EquivalenceCriterion should be zero")
		}
	})

	t.Run("empty slices (not nil) is still zero", func(t *testing.T) {
		c := graph.EquivalenceCriterion{
			Preserve: []string{},
			Ignore:   []string{},
		}
		if !c.IsZero() {
			t.Error("EquivalenceCriterion with empty (non-nil) slices should be zero")
		}
	})

	t.Run("non-zero with only Name", func(t *testing.T) {
		c := graph.EquivalenceCriterion{Name: "operational-meaning"}
		if c.IsZero() {
			t.Error("EquivalenceCriterion with Name should not be zero")
		}
	})

	t.Run("non-zero with only Declaration", func(t *testing.T) {
		c := graph.EquivalenceCriterion{
			Declaration: "Preserve operational meaning, ignore representational variation",
		}
		if c.IsZero() {
			t.Error("EquivalenceCriterion with Declaration should not be zero")
		}
	})

	t.Run("non-zero with Preserve populated", func(t *testing.T) {
		c := graph.EquivalenceCriterion{
			Declaration: "Preserve target and obligation level",
			Preserve:    []string{"target", "obligation_level"},
		}
		if c.IsZero() {
			t.Error("EquivalenceCriterion with Preserve should not be zero")
		}
	})

	t.Run("non-zero with Ignore populated", func(t *testing.T) {
		c := graph.EquivalenceCriterion{
			Declaration: "Ignore display format",
			Ignore:      []string{"display_format"},
		}
		if c.IsZero() {
			t.Error("EquivalenceCriterion with Ignore should not be zero")
		}
	})
}

// TestEquivalenceCriterionValidate verifies layer-ordering enforcement.
// Layer 1 (Declaration) must be present before Layer 2 (Preserve/Ignore).
func TestEquivalenceCriterionValidate(t *testing.T) {
	t.Run("zero value is valid (no criterion declared)", func(t *testing.T) {
		var c graph.EquivalenceCriterion
		if err := c.Validate(); err != nil {
			t.Errorf("zero-value criterion should be valid, got: %v", err)
		}
	})

	t.Run("Name only is valid (handle without grounds)", func(t *testing.T) {
		c := graph.EquivalenceCriterion{Name: "operational-meaning"}
		if err := c.Validate(); err != nil {
			t.Errorf("name-only criterion should be valid (structurally), got: %v", err)
		}
	})

	t.Run("Declaration only is valid", func(t *testing.T) {
		c := graph.EquivalenceCriterion{
			Declaration: "Preserve operational meaning, ignore representational variation",
		}
		if err := c.Validate(); err != nil {
			t.Errorf("declaration-only criterion should be valid, got: %v", err)
		}
	})

	t.Run("Declaration + Preserve is valid", func(t *testing.T) {
		c := graph.EquivalenceCriterion{
			Declaration: "Preserve target and obligation level",
			Preserve:    []string{"target", "obligation_level"},
		}
		if err := c.Validate(); err != nil {
			t.Errorf("Declaration+Preserve should be valid, got: %v", err)
		}
	})

	t.Run("Declaration + Ignore is valid", func(t *testing.T) {
		c := graph.EquivalenceCriterion{
			Declaration: "Ignore display format",
			Ignore:      []string{"display_format", "wording"},
		}
		if err := c.Validate(); err != nil {
			t.Errorf("Declaration+Ignore should be valid, got: %v", err)
		}
	})

	t.Run("Declaration + Preserve + Ignore is valid", func(t *testing.T) {
		c := graph.EquivalenceCriterion{
			Declaration: "Preserve target; ignore display format",
			Preserve:    []string{"target"},
			Ignore:      []string{"display_format"},
		}
		if err := c.Validate(); err != nil {
			t.Errorf("full criterion should be valid, got: %v", err)
		}
	})

	t.Run("Preserve without Declaration is invalid (layer ordering violation)", func(t *testing.T) {
		c := graph.EquivalenceCriterion{
			Preserve: []string{"target"},
		}
		if err := c.Validate(); err == nil {
			t.Error("Preserve without Declaration should be invalid — layer ordering violation")
		}
	})

	t.Run("Ignore without Declaration is invalid (layer ordering violation)", func(t *testing.T) {
		c := graph.EquivalenceCriterion{
			Ignore: []string{"display_format"},
		}
		if err := c.Validate(); err == nil {
			t.Error("Ignore without Declaration should be invalid — layer ordering violation")
		}
	})

	t.Run("Preserve and Ignore without Declaration is invalid", func(t *testing.T) {
		c := graph.EquivalenceCriterion{
			Preserve: []string{"target"},
			Ignore:   []string{"display_format"},
		}
		if err := c.Validate(); err == nil {
			t.Error("Preserve+Ignore without Declaration should be invalid")
		}
	})

	t.Run("Name + Preserve without Declaration is invalid", func(t *testing.T) {
		c := graph.EquivalenceCriterion{
			Name:     "operational-meaning",
			Preserve: []string{"target"},
		}
		if err := c.Validate(); err == nil {
			t.Error("Name+Preserve without Declaration should be invalid — name does not substitute for grounds")
		}
	})

	t.Run("Name + Ignore without Declaration is invalid", func(t *testing.T) {
		c := graph.EquivalenceCriterion{
			Name:   "operational-meaning",
			Ignore: []string{"display_format"},
		}
		if err := c.Validate(); err == nil {
			t.Error("Name+Ignore without Declaration should be invalid — name does not substitute for grounds")
		}
	})
}

// TestEquivalenceCriterionFields verifies structural stability — fields are
// accessible and carry their values correctly.
func TestEquivalenceCriterionFields(t *testing.T) {
	c := graph.EquivalenceCriterion{
		Name:        "operational-meaning",
		Declaration: "Preserve operational meaning, ignore representational variation",
		Preserve:    []string{"target", "obligation_level"},
		Ignore:      []string{"display_format", "wording"},
	}

	if c.Name != "operational-meaning" {
		t.Errorf("Name: got %q, want %q", c.Name, "operational-meaning")
	}
	if c.Declaration != "Preserve operational meaning, ignore representational variation" {
		t.Errorf("Declaration: got %q", c.Declaration)
	}
	if len(c.Preserve) != 2 || c.Preserve[0] != "target" || c.Preserve[1] != "obligation_level" {
		t.Errorf("Preserve: got %v", c.Preserve)
	}
	if len(c.Ignore) != 2 || c.Ignore[0] != "display_format" || c.Ignore[1] != "wording" {
		t.Errorf("Ignore: got %v", c.Ignore)
	}
}
