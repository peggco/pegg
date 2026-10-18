package explorer

import "testing"

func TestExplorerAgentParameters(t *testing.T) {
	a, err := newExplorerAgent()
	if err != nil {
		t.Fatal(err)
	}
	if a.Temperature != agentTemperature || a.TopP != agentTopP {
		t.Fatalf("sampling = %v/%v, want %v/%v", a.Temperature, a.TopP, agentTemperature, agentTopP)
	}
	if a.MaxTokens != agentMaxTokens || a.MaxIterations != agentMaxIterations {
		t.Fatalf("limits = %d/%d, want %d/%d", a.MaxTokens, a.MaxIterations, agentMaxTokens, agentMaxIterations)
	}
}
