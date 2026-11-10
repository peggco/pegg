---
icon: lucide/puzzle
---

# Extending

Register custom tools, middlewares, agents, providers, and skills to shape the
engine's behavior.

## Tools

A tool implements four methods:

```go
type Tool interface {
	Name() string
	Description() string
	Parameters() any
	Execute(ctx context.Context, args string) (string, error)
}
```

The ergonomic way is `sdk.NewTool`, which wraps a function and a JSON schema:

```go
greet := sdk.NewTool(
	"greet",
	"greets the user",
	map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
		"required": []string{"name"},
	},
	func(ctx context.Context, args string) (string, error) {
		return "hello, " + args, nil
	},
)

err := eng.RegisterTool(greet)
```

`Parameters` is a JSON schema describing the arguments. The model invokes the tool
with a JSON string that your `Execute` function parses.

Registry operations:

```go
eng.RegisterTool(greet)        // sdk.ErrToolNil / ErrToolEmpty / ErrToolDuplicate
eng.UnregisterTool("greet")    // bool
eng.Tools()                    // []Tool, sorted by name
```

Tools registered this way are available to agents that enable them by name (see
`ChatRequest.Tools`).

## Middlewares

Middlewares wrap the agent lifecycle. Embed `sdk.BaseMiddleware` to get no-op
defaults for all seven hooks and override the ones you need:

```go
type myMW struct {
	sdk.BaseMiddleware
}

func (m *myMW) BeforeLLM(ctx context.Context, req *sdk.Request) error {
	// inspect or mutate the outgoing request
	return nil
}

eng.RegisterMiddleware("my-mw", &myMW{})   // sdk.ErrMiddlewareNil / Duplicate
eng.UnregisterMiddleware("my-mw")           // bool
```

The full hook set: `BeforeRun`, `AfterRun`, `BeforeLLM`, `AfterLLM`, `BeforeTool`,
`AfterTool`, `OnError`.

## Agents

Register a factory for a new agent type. The factory is invoked immediately to learn
the agent's name:

```go
eng.RegisterAgent(func() (*sdk.Agent, error) {
	a := &sdk.Agent{
		Name:          "auditor",
		Description:   "Audits code for security issues",
		SystemPrompt:  "You are a security auditor...",
		MaxIterations: 30,
	}
	return a, nil
})
```

The built-in agent types are `orchestrator`, `explorer`, and `developer`,
and are what `Chat` and the `task` tool run. The `sdk.Agent` type exposes all
its fields and run methods, but note that a fully wired agent also needs a populated
tool registry and an event bus; the built-in agents come pre-wired through the
engine. Registered agent types become available to the `task` tool and the TUI
mention picker.

## Providers

Register a new LLM provider factory:

```go
eng.RegisterProvider("my-provider", func(cfg sdk.LLMConfig) (sdk.Provider, error) {
	// return a provider implementing Chat / ChatStream / ListModels
	return provider, nil
})
```

The provider becomes available to model resolution under its name.

## Skills

Load `SKILL.md`-based skill directories or register skills directly:

```go
eng.LoadSkills("~/skills", "./skills")

for _, sk := range eng.Skills() {
	fmt.Println(sk.Name, sk.Description)
}

eng.RegisterSkill(sk)   // name must match ^[a-z0-9]+(?:-[a-z0-9]+)*$
```

A `*sdk.Skill` has `Name`, `Description`, `License`, `Compatibility`, `Metadata`,
`AllowedTools`, `Instructions`, `Source`, `Path`, and `ScriptsPath`. Loading a skill
also wires `/name` expansion into agent inputs.

## VFS access

The engine exposes its sandbox directly if you need lower-level control:

```go
fs := eng.Workspace()          // *sdk.VFS
root := fs.Root()              // workspace root
```

See [Files](files.md) for the high-level operations.

## Events

See [Events & Errors](events-errors.md) for subscribing to agent and session
events from custom components.