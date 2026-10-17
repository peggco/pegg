---
icon: lucide/history
---

# Sessions

Persistent, resumable conversations — the same session store the CLI, TUI, and HTTP
API use.

## Creating and loading

```go
s, err := eng.CreateSession(sdk.CreateSessionOptions{
	Title:  "demo",
	Provider: "openai",
	Model:  "gpt-4o",
	Mode:   sdk.ModeAuto, // sdk.ModeBuild | sdk.ModePlan | sdk.ModeAuto
})
if err != nil {
	return err
}

got, err := eng.GetSession(s.ID)   // sdk.ErrNoSession if missing
```

An empty `Title` produces `"New Session <timestamp>"`.

## Modes

Sessions run in a mode that controls planning: `build` (no plan reminder, no plan
tools), `plan` (read-only planning), or `auto` (default; the agent may plan on its
own). Set it when creating (`CreateSessionOptions.Mode`), override it per run with
`ChatRequest.Mode`, or update it later:

```go
err = eng.SetSessionMode(s.ID, sdk.ModePlan)
```

When `ChatRequest.Mode` is empty and `SessionID` is set, the session's stored mode
is used.

## Using a session in a chat

```go
_, err = eng.Chat(context.Background(), sdk.ChatRequest{
	Input:     "Continue the session",
	SessionID: s.ID,
})
```

The session's messages are replayed into context and the run re-attaches to the same
session. After the run, `SessionMessages` reflects the new turns.

## Listing and deleting

```go
sessions, total, err := eng.ListSessions(1, 50)   // newest first; total = count

err = eng.DeleteSession(s.ID)
err = eng.SetSessionTitle(s.ID, "New title")      // sdk.ErrSessionEmptyTitle if empty
```

## Reading history

```go
msgs, err := eng.SessionMessages(s.ID)
for _, m := range msgs {
	fmt.Println(m.Seq, m.Role, m.Content)
}
```

`SessionMessage` fields: `ID`, `SessionID`, `Seq`, `Role`, `Content`, `Reasoning`,
`Name`, `ToolCallID`, `ToolCalls`, `CreatedAt`.

## Forking

`ForkSession` copies a session up to and including a given message into a new
session:

```go
fork, err := eng.ForkSession(s.ID, atMessageID)
// fork.ParentID == s.ID
```

The fork keeps the message history up to `atMessageID` and starts with an empty
future. This is the same operation as the session layer's fork, exposed here
programmatically.

## Snapshots

Reverting a session stores the removed messages as a snapshot:

```go
snaps, err := eng.SessionSnapshots(s.ID)
```

## Appending messages

```go
msg, err := eng.AppendMessage(s.ID, sdk.UserMessage("manual entry"))
// msg.Seq is auto-incremented
```

`AppendMessage` does not run the agent — it writes a message directly into the
session's history.

## Types

`Session` fields: `ID`, `Title`, `Provider`, `Model`, `ReasoningEffort`, `Mode`,
`ProjectDir`, `ParentID`, `CreatedAt`, `UpdatedAt`, `Usage`.

`SessionSnapshot` fields: `ID`, `SessionID`, `HeadMessageID`, `CreatedAt`,
`Messages`.

## Note on persistence

Sessions are stored in the configured session store (SQLite by default). If you
opened the engine with `DisableRecorder`, running `Chat` will not auto-attach to a
session; use `SessionID` explicitly and read `resp.SessionID` from the result.