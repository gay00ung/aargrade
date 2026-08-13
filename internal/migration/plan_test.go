package migration

import (
	"path/filepath"
	"testing"
)

func TestCreateAGP9PlanFromFixture(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", "kotlin-library"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Create(Options{ProjectPath: root, TargetAGP: "9.0.1", ToolVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Ready || plan.CurrentAGP != "8.8.0" || plan.Toolchain.MinimumGradle != "9.1.0" {
		t.Fatalf("plan = %#v", plan)
	}
	for _, requiredID := range []string{"agp9.kotlin-android-plugin", "agp9.legacy-api", "r8.consumer-global-option", "verify.candidate", "matrix.consumers"} {
		if !hasStep(plan, requiredID) {
			t.Errorf("plan missing step %s", requiredID)
		}
	}
}

func TestCreateBlocksNonUpgrade(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "library-only"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Create(Options{ProjectPath: root, TargetAGP: "9.2.0", ToolVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Ready || len(plan.Blockers) == 0 {
		t.Fatalf("plan should be blocked: %#v", plan)
	}
}

func hasStep(plan Plan, id string) bool {
	for _, step := range plan.Steps {
		if step.ID == id {
			return true
		}
	}
	return false
}
