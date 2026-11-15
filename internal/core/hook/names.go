package hook

const (
	CoreConfigLoaded = "core.config.loaded"
	CoreStartup      = "core.startup"
	CoreShutdown     = "core.shutdown"
	CoreCommand      = "core.command"
	CoreNotification = "core.notification"
)

type ConfigLoaded struct {
	Version   string   `json:"version"`
	Theme     string   `json:"theme"`
	Language  string   `json:"language"`
	Providers []string `json:"providers"`
}

type Startup struct {
	Version   string `json:"version"`
	Workspace string `json:"workspace"`
}

type Shutdown struct {
	Reason string `json:"reason"`
}

type Command struct {
	Args []string `json:"args"`
}

type Notice struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Level string `json:"level,omitempty"`
}
