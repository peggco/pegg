package selfupdate

type State string

const (
	StateIdle        State = "idle"
	StateChecking    State = "checking"
	StateAvailable   State = "available"
	StateDownloading State = "downloading"
	StateReady       State = "ready"
	StateUnsupported State = "unsupported"
	StateError       State = "error"
)

type Status struct {
	CurrentVersion   string `json:"currentVersion"`
	LatestVersion    string `json:"latestVersion,omitempty"`
	State            State  `json:"state"`
	Available        bool   `json:"available"`
	Supported        bool   `json:"supported"`
	DesktopAvailable bool   `json:"desktopAvailable"`
	DesktopReady     bool   `json:"desktopReady"`
	Ready            bool   `json:"ready"`
	Progress         int    `json:"progress"`
	Error            string `json:"error,omitempty"`
	ReleaseURL       string `json:"releaseUrl,omitempty"`
	TargetPath       string `json:"targetPath,omitempty"`

	AgentCurrentVersion string `json:"agentCurrentVersion,omitempty"`
	AgentLatestVersion  string `json:"agentLatestVersion,omitempty"`
	AgentAvailable      bool   `json:"agentAvailable"`
	AgentSupported      bool   `json:"agentSupported"`
	AgentReady          bool   `json:"agentReady"`
	AgentTargetPath     string `json:"agentTargetPath,omitempty"`
}
