package event

const (
	TopicAppMounted  = "app.mounted"
	TopicAppShutdown = "app.shutdown"

	TopicAppFocused = "app.focused"
	TopicAppBlurred = "app.blurred"

	TopicProviderAdded   = "provider.added"
	TopicProviderRemoved = "provider.removed"

	TopicModelSelect  = "llm.model.select"
	TopicModelsLoaded = "llm.models.loaded"

	TopicSTTStarted  = "stt.started"
	TopicSTTPartial  = "stt.partial"
	TopicSTTFinished = "stt.finished"
	TopicSTTError    = "stt.error"
)

type STTEvent struct {
	Source     string
	StreamID   string
	Text       string
	Final      bool
	DurationMS int64
	Message    string
}
