package plugin

type ViewSpan struct {
	Text  string `json:"text"`
	Style string `json:"style,omitempty"`
}

type ViewLine struct {
	Text  string     `json:"text,omitempty"`
	Spans []ViewSpan `json:"spans,omitempty"`
}

type ViewFragment struct {
	ID       string     `json:"id,omitempty"`
	Title    string     `json:"title,omitempty"`
	Badge    string     `json:"badge,omitempty"`
	Style    string     `json:"style,omitempty"`
	Lines    []ViewLine `json:"lines,omitempty"`
	Priority int        `json:"priority,omitempty"`
}

type ViewMenuItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Detail   string `json:"detail,omitempty"`
	Badge    string `json:"badge,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

type ViewSection struct {
	Tab         string      `json:"tab,omitempty"`
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	Fields      []TUIField  `json:"fields,omitempty"`
	Actions     []TUIAction `json:"actions,omitempty"`
}

type ViewKeyBinding struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type ViewContext struct {
	Mode      string `json:"mode,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Messages  int    `json:"messages,omitempty"`
	Running   bool   `json:"running,omitempty"`
}

type StatusContext struct {
	Mode            string     `json:"mode,omitempty"`
	Running         bool       `json:"running,omitempty"`
	Provider        string     `json:"provider,omitempty"`
	Model           string     `json:"model,omitempty"`
	ReasoningEffort string     `json:"reasoning_effort,omitempty"`
	Cost            float64    `json:"cost,omitempty"`
	TotalTokens     int        `json:"total_tokens,omitempty"`
	MaxTokens       int        `json:"max_tokens,omitempty"`
	Current         []ViewSpan `json:"current,omitempty"`
}

type StatusHook struct {
	StatusContext
	Spans   []ViewSpan `json:"spans,omitempty"`
	Replace bool       `json:"replace,omitempty"`
}

type HeaderHook struct {
	ViewContext
	Title string     `json:"title,omitempty"`
	Lines []ViewLine `json:"lines,omitempty"`
}

type FooterHook struct {
	ViewContext
	Hint  string     `json:"hint,omitempty"`
	Lines []ViewLine `json:"lines,omitempty"`
}

type ChatItemsHook struct {
	ViewContext
	Items []ViewFragment `json:"items,omitempty"`
}

type MessageDecorateHook struct {
	AgentID   string `json:"agent_id,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Suffix    string `json:"suffix,omitempty"`
	Style     string `json:"style,omitempty"`
}

type PromptHook struct {
	ViewContext
	Placeholder string `json:"placeholder,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
}

type MenuHook struct {
	Menu  string         `json:"menu"`
	Items []ViewMenuItem `json:"items,omitempty"`
}

type KeyBindingsHook struct {
	ViewContext
	Bindings []ViewKeyBinding `json:"bindings,omitempty"`
}

type CommandHook struct {
	ID  string `json:"id"`
	Key string `json:"key,omitempty"`
}

type SettingsSectionsHook struct {
	Tab      string        `json:"tab,omitempty"`
	Sections []ViewSection `json:"sections,omitempty"`
}

type SettingsValuesHook struct {
	Tab    string            `json:"tab,omitempty"`
	Values map[string]string `json:"values,omitempty"`
}

type SettingsSaveHook struct {
	Tab    string            `json:"tab"`
	Values map[string]string `json:"values"`
}

type DesktopViewContext struct {
	Platform  string `json:"platform,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Theme     string `json:"theme,omitempty"`
	Running   bool   `json:"running,omitempty"`
	Messages  int    `json:"messages,omitempty"`
}

type DesktopView struct {
	Header        []ViewLine       `json:"header,omitempty"`
	Footer        []ViewLine       `json:"footer,omitempty"`
	StatusLeft    []ViewSpan       `json:"status_left,omitempty"`
	StatusRight   []ViewSpan       `json:"status_right,omitempty"`
	ReplaceStatus bool             `json:"replace_status,omitempty"`
	ChatItems     []ViewFragment   `json:"chat_items,omitempty"`
	EmptyItems    []ViewFragment   `json:"empty_items,omitempty"`
	Placeholder   string           `json:"placeholder,omitempty"`
	MenuItems     []ViewMenuItem   `json:"menu_items,omitempty"`
	KeyBindings   []ViewKeyBinding `json:"key_bindings,omitempty"`
	Settings      []ViewSection    `json:"settings,omitempty"`
}

type DesktopViewHook struct {
	DesktopViewContext
	View DesktopView `json:"view"`
}
