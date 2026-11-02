package databricks

import (
	"strings"
	"testing"
)

func TestDefaultBaseURLRequiresEnv(t *testing.T) {
	t.Setenv("DATABRICKS_HOST", "")
	if _, err := defaultBaseURL(); err == nil || !strings.Contains(err.Error(), "DATABRICKS_HOST") {
		t.Fatalf("err = %v, want a DATABRICKS_HOST error", err)
	}
}

func TestDefaultBaseURLExpandsEnv(t *testing.T) {
	t.Setenv("DATABRICKS_HOST", "example.databricks.com")
	got, err := defaultBaseURL()
	if err != nil {
		t.Fatalf("defaultBaseURL: %v", err)
	}
	if want := "https://example.databricks.com/ai-gateway/mlflow/v1"; got != want {
		t.Fatalf("defaultBaseURL = %q, want %q", got, want)
	}
}
