//go:build gui

package main

import "encoding/json"

const (
	HookBootstrap    = "desktop.bootstrap"
	HookEmit         = "desktop.emit"
	HookTheme        = "desktop.theme"
	HookNotify       = "desktop.notify"
	HookChatSend     = "desktop.chat.send"
	HookSession      = "desktop.session"
	HookView         = "desktop.view"
	HookViewCommand  = "desktop.command"
	HookViewSettings = "desktop.settings.save"
)

type BootstrapHook struct {
	Platform  string `json:"platform"`
	Version   string `json:"version"`
	Workspace string `json:"workspace"`
	Language  string `json:"language"`
}

type EmitHook struct {
	Name string          `json:"name"`
	Data json.RawMessage `json:"data,omitempty"`
}

type ThemeHook struct {
	Name string `json:"name"`
	Mode string `json:"mode,omitempty"`
}

type NotifyHook struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type ChatSendHook struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

type SessionHook struct {
	Action    string `json:"action"`
	SessionID string `json:"session_id"`
}
