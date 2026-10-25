package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/peggco/pegg/internal/utils/search"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/agents"
	"github.com/peggco/pegg/internal/agent/attach"
	"github.com/peggco/pegg/internal/builtin/middlewares/compaction"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/router"
	runpkg "github.com/peggco/pegg/internal/run"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/utils/query"
)

type runOptions struct {
	provider      string
	model         string
	selectModel   bool
	session       string
	selectSession bool
	files         []string
	showThinking  bool
	showSubagent  bool
	chat          bool
	attach        bool
	follow        bool
	stt           bool
	mode          agent.Mode
	modeSet       bool
}

func (c *CLI) newRunCommand() *cobra.Command {
	var opts runOptions
	var modePlan, modeBuild, modeAuto bool

	cmd := &cobra.Command{
		Use:   "run [message]",
		Short: "Run the orchestrator agent on a message",
		Long:  "Run the orchestrator agent on the given message, streaming its progress, tools, thinking and responses live. Without a message (or with --chat), starts an interactive chat with the agent in the same session. Use --session or --select-session to continue an existing session.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, explicit, err := resolveRunMode(modePlan, modeBuild, modeAuto)
			if err != nil {
				return err
			}
			opts.mode = mode
			opts.modeSet = explicit
			return c.runRun(cmd.OutOrStdout(), cmd.InOrStdin(), strings.Join(args, " "), opts)
		},
	}
	cmd.Flags().StringVar(&opts.provider, "provider", "", "provider to use (random if omitted)")
	cmd.Flags().StringVar(&opts.model, "model", "", "model to use (random if omitted)")
	cmd.Flags().BoolVar(&opts.selectModel, "select-model", false, "list available models and let you pick one (overrides --model)")
	cmd.Flags().StringVar(&opts.session, "session", "", "session ID to continue")
	cmd.Flags().BoolVar(&opts.selectSession, "select-session", false, "list sessions and let you pick one to continue")
	cmd.Flags().StringArrayVar(&opts.files, "file", nil, "file to attach to the agent (repeatable)")
	cmd.Flags().BoolVar(&opts.showThinking, "show-thinking", false, "show the model's raw thinking (reasoning) content instead of a Thinking indicator")
	cmd.Flags().BoolVar(&opts.showSubagent, "show-subagent", false, "show subagent messages (their content) instead of a Subagent indicator")
	cmd.Flags().BoolVar(&opts.chat, "chat", false, "keep chatting with the agent after the run finishes (type exit/quit to end)")
	cmd.Flags().BoolVar(&opts.attach, "attach", false, "attach to a session that is already running in another client (requires --session)")
	cmd.Flags().BoolVar(&opts.follow, "follow", false, "with --attach, keep following later runs on the session")
	cmd.Flags().BoolVar(&opts.stt, "stt", false, "record voice input; press Enter to stop and use the transcript as the message")
	cmd.Flags().BoolVar(&modePlan, "plan", false, "start in plan mode (read-only planning)")
	cmd.Flags().BoolVar(&modeBuild, "build", false, "start in build mode (execute without planning tools)")
	cmd.Flags().BoolVar(&modeAuto, "auto", false, "start in auto mode, letting the agent plan on its own (default)")
	return cmd
}

func resolveRunMode(plan, build, auto bool) (agent.Mode, bool, error) {
	n := 0
	for _, b := range []bool{plan, build, auto} {
		if b {
			n++
		}
	}
	if n > 1 {
		return "", false, errors.New("cli: only one of --plan, --build or --auto may be set")
	}
	switch {
	case plan:
		return agent.ModePlan, true, nil
	case build:
		return agent.ModeBuild, true, nil
	case auto:
		return agent.ModeAuto, true, nil
	}
	return agent.ModeAuto, false, nil
}

func (c *CLI) runRun(out io.Writer, in io.Reader, message string, opts runOptions) error {
	orch, err := agents.New("orchestrator")
	if err != nil {
		return fmt.Errorf("cli: create orchestrator: %w", err)
	}

	var fileAttachments []llm.Attachment
	if len(opts.files) > 0 {
		fileAttachments, err = loadAttachments(opts.files)
		if err != nil {
			return err
		}
	}

	resume, err := c.resolveSession(opts)
	if err != nil {
		return err
	}
	var resumeMsgs []session.Message
	if resume != nil {
		resumeMsgs, err = c.sessions.Messages(resume.ID)
		if err != nil {
			return fmt.Errorf("cli: get session messages: %w", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if opts.attach {
		if opts.session == "" {
			return fmt.Errorf("cli: --attach requires --session")
		}
		return c.attachSession(ctx, out, in, opts.session, opts.follow)
	}

	if opts.stt {
		transcript, err := c.recordSTT(ctx, in, out)
		if err != nil {
			return err
		}
		if strings.TrimSpace(message) == "" {
			message = transcript
		} else {
			message = strings.TrimSpace(message) + " " + transcript
		}
		opts.stt = false
	}

	var prov llm.Provider
	var mdl llm.Model
	if resume != nil && opts.provider == "" && opts.model == "" && !opts.selectModel {
		prov, mdl, err = c.selectModel(resume.Provider, resume.Model, false)
	} else {
		prov, mdl, err = c.selectModel(opts.provider, opts.model, opts.selectModel)
	}
	if err != nil {
		return fmt.Errorf("cli: select model: %w", err)
	}
	orch.SetModelProvider(mdl, prov)
	orch.Bus = c.bus

	mode := opts.mode
	if !opts.modeSet && resume != nil {
		mode = agent.NormalizeMode(agent.Mode(resume.Mode))
	}
	orch.SetMode(mode)
	if opts.modeSet && resume != nil {
		_ = c.sessions.SetMode(resume.ID, string(mode))
	}

	if len(fileAttachments) > 0 {
		orch.Attachments = fileAttachments
	}

	renderer := newRunRenderer(out, in, orch.ID, opts.showThinking, opts.showSubagent)
	renderer.bus = c.bus
	renderer.ctx = ctx
	if err := renderer.subscribe(c.bus); err != nil {
		return err
	}
	defer renderer.unsubscribe(c.bus)

	modelName := mdl.ID
	if mdl.Name != "" {
		modelName = mdl.Name
	}
	bannerMode := ""
	if mode != agent.ModeAuto {
		bannerMode = string(mode)
	}
	renderer.banner(prov.Name(), modelName, bannerMode)
	if len(opts.files) > 0 {
		renderer.attachments(opts.files)
	}
	if resume != nil {
		renderer.renderSessionHistory(resumeMsgs)
	}

	history := make([]llm.Message, 0, len(resumeMsgs)+1)
	if resume != nil {
		if orch.SystemPrompt != "" {
			history = append(history, llm.SystemMessage(orch.SystemPrompt))
		}
		history = append(history, session.MessagesToLLM(resumeMsgs)...)
		c.bus.Publish(session.TopicSessionResume, session.SessionResume{
			AgentID:   orch.ID,
			SessionID: resume.ID,
		})
	}

	if resume == nil && (strings.TrimSpace(message) != "" || opts.chat) {
		sess, cerr := c.createRunSession(prov, mdl, orch.ReasoningEffort, mode)
		if cerr != nil {
			return cerr
		}
		resume = sess
		c.bus.Publish(session.TopicSessionResume, session.SessionResume{
			AgentID:   orch.ID,
			SessionID: sess.ID,
		})
	}

	var lease *runpkg.Lease
	if strings.TrimSpace(message) != "" {
		lease, err = c.claimRun(ctx, resume, orch, prov, mdl)
		if err != nil {
			return err
		}
	}

	if strings.TrimSpace(message) != "" {
		var (
			result *agent.RunResult
			runErr error
		)
		if c.runCoord != nil {
			result, runErr = c.runCoord.Stream(ctx, lease, orch, message, history, func(agent.StreamEvent) error { return nil })
		} else if len(history) > 0 {
			result, runErr = orch.ResumeStream(ctx, message, history, func(agent.StreamEvent) error { return nil })
		} else {
			result, runErr = orch.RunStream(ctx, message, func(agent.StreamEvent) error { return nil })
		}
		if runErr != nil {
			if !opts.chat || !errors.Is(runErr, context.Canceled) || result == nil {
				return fmt.Errorf("orchestrator: %w", runErr)
			}
		}
		if result != nil {
			history = result.History
		}
		if !opts.chat {
			return nil
		}
	}
	return c.runChatLoop(ctx, orch, renderer, history, in, resume)
}

func (c *CLI) createRunSession(prov llm.Provider, mdl llm.Model, effort string, mode agent.Mode) (*session.Session, error) {
	dir, _ := os.Getwd()
	provider := ""
	if prov != nil {
		provider = prov.Name()
	}
	sess, err := c.sessions.Create(session.CreateOptions{
		Provider:        provider,
		Model:           mdl.ID,
		ReasoningEffort: effort,
		Mode:            string(mode),
		ProjectDir:      dir,
	})
	if err != nil {
		return nil, fmt.Errorf("cli: create session: %w", err)
	}
	return sess, nil
}

func (c *CLI) claimRun(ctx context.Context, sess *session.Session, orch *agent.Agent, prov llm.Provider, mdl llm.Model) (*runpkg.Lease, error) {
	if sess == nil || c.runCoord == nil {
		return nil, nil
	}
	provider := ""
	if prov != nil {
		provider = prov.Name()
	}
	lease, err := c.runCoord.Claim(ctx, runpkg.ClaimRequest{
		SessionID: sess.ID,
		AgentID:   orch.ID,
		AgentName: orch.Name,
		Provider:  provider,
		Model:     mdl.ID,
		BaseSeq:   c.runCoord.BaseSeq(sess.ID),
	})
	if err != nil {
		var busy *runpkg.BusyError
		if errors.As(err, &busy) {
			runID := ""
			if busy.Run != nil {
				runID = busy.Run.ID
			}
			return nil, fmt.Errorf("session is already running in another client (run %s); cancel it there first: %w", runID, busy)
		}
		return nil, fmt.Errorf("cli: claim session: %w", err)
	}
	return lease, nil
}

func (c *CLI) resolveSession(opts runOptions) (*session.Session, error) {
	if opts.session != "" {
		return c.resolveChainHead(opts.session)
	}
	if !opts.selectSession {
		return nil, nil
	}
	id, err := c.selectSession()
	if err != nil {
		return nil, err
	}
	return c.resolveChainHead(id)
}

func (c *CLI) resolveChainHead(id string) (*session.Session, error) {
	latest, err := c.sessions.LatestInChain(id)
	if err != nil {
		return nil, fmt.Errorf("cli: get session: %w", err)
	}
	return latest, nil
}

func (c *CLI) selectSession() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cli: get working directory: %w", err)
	}
	page, size := 1, 10
	for {
		q := query.Query{
			Page: query.Page{Number: page, Size: size},
			Filters: []query.Filter{{Column: "project_dir", Operator: query.OpEqual, Value: dir},
				session.MainSessionsFilter(),
				{Column: "compaction_parent_id", Operator: query.OpEqual, Value: ""}},
		}
		sessions, total, err := c.sessions.List(q)
		if err != nil {
			return "", fmt.Errorf("cli: list sessions: %w", err)
		}
		if len(sessions) == 0 {
			if page > 1 {
				page--
				continue
			}
			return "", errors.New("cli: no sessions found")
		}
		items := make([]string, 0, len(sessions)+2)
		for _, s := range sessions {
			items = append(items, fmt.Sprintf("%-24s %-20s %s",
				truncate(s.Title, 24), s.Provider+"/"+s.Model, s.CreatedAt.Format("2006-01-02 15:04")))
		}
		if page > 1 {
			items = append(items, "← previous page")
		}
		if page*size < total {
			items = append(items, fmt.Sprintf("→ next page (%d more)", total-page*size))
		}
		idx, err := c.picker(items, "Select session")
		if err != nil {
			return "", err
		}
		if idx < 0 || idx >= len(items) {
			return "", errors.New("cli: invalid session selection")
		}
		if idx < len(sessions) {
			return sessions[idx].ID, nil
		}
		nav := items[idx]
		switch {
		case strings.HasPrefix(nav, "←"):
			page--
		case strings.HasPrefix(nav, "→"):
			page++
		}
	}
}

func (c *CLI) runChatLoop(ctx context.Context, orch *agent.Agent, renderer *runRenderer, history []llm.Message, in io.Reader, sess *session.Session) error {
	renderer.chatMode = true
	renderer.write("\n%s\n", renderer.dim("chat mode: type a message, exit/quit to end"))

	tty := isTerminal(in)
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lines := make(chan string)
	go func() {
		for scanner.Scan() {
			lines <- strings.TrimSpace(scanner.Text())
		}
		close(lines)
	}()

	wake := make(chan struct{}, 1)
	onNotification := func(e agent.SubAgentNotification) {
		if e.ParentAgentID != orch.ID {
			return
		}
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	if err := c.bus.Subscribe(agent.TopicSubAgentNotification, onNotification); err != nil {
		return fmt.Errorf("cli: subscribe subagent notification: %w", err)
	}
	defer c.bus.Unsubscribe(agent.TopicSubAgentNotification, onNotification)

	for {
		if tty {
			renderer.prompt()
		}
		select {
		case line, ok := <-lines:
			if !ok {
				return nil
			}
			if line == "" {
				continue
			}
			if isChatExit(line) {
				return nil
			}
			if !tty {
				renderer.write("> %s\n", line)
			}
			var err error
			history, err = c.runChatAgent(ctx, orch, line, history, sess)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					renderer.write("%s\n", renderer.dim("run cancelled; send a new message to continue"))
					continue
				}
				return err
			}
		case <-wake:
			if tty {
				renderer.write("\n")
			}
			lease, lerr := c.claimRun(ctx, sess, orch, orch.Provider, orch.Model)
			if lerr != nil {
				return lerr
			}
			var (
				result *agent.RunResult
				err    error
			)
			if lease != nil {
				result, err = orch.Continue(lease.Context(), history)
				if err == nil {
					lease.Finish(session.RunStateDone, "")
				} else {
					lease.Finish(session.RunStateError, err.Error())
				}
			} else {
				result, err = orch.Continue(ctx, history)
			}
			if err != nil {
				return fmt.Errorf("orchestrator: %w", err)
			}
			if result != nil {
				history = result.History
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (c *CLI) runChatAgent(ctx context.Context, orch *agent.Agent, line string, history []llm.Message, sess *session.Session) ([]llm.Message, error) {
	lease, err := c.claimRun(ctx, sess, orch, orch.Provider, orch.Model)
	if err != nil {
		return history, err
	}
	var result *agent.RunResult
	if c.runCoord != nil {
		result, err = c.runCoord.Stream(ctx, lease, orch, line, history, func(agent.StreamEvent) error { return nil })
	} else if len(history) > 0 {
		result, err = orch.ResumeStream(ctx, line, history, func(agent.StreamEvent) error { return nil })
	} else {
		result, err = orch.RunStream(ctx, line, func(agent.StreamEvent) error { return nil })
	}
	if result != nil {
		history = result.History
	}
	if err != nil {
		return history, err
	}
	return history, nil
}

func isChatExit(line string) bool {
	switch line {
	case "exit", "quit", "/exit", "/quit", "bye":
		return true
	}
	return false
}

func loadAttachments(paths []string) ([]llm.Attachment, error) {
	var out []llm.Attachment
	for _, p := range paths {
		att, err := attach.FromPath(p)
		if err != nil {
			return nil, fmt.Errorf("cli: %w", err)
		}
		out = append(out, att)
	}
	return out, nil
}

type modelChoice struct {
	provider string
	model    llm.Model
}

func (c *CLI) selectModel(provider, model string, interactive bool) (llm.Provider, llm.Model, error) {
	if c.llmMgr == nil {
		return nil, llm.Model{}, errors.New("cli: llm manager unavailable")
	}

	if interactive {
		choices, err := c.availableModels(provider)
		if err != nil {
			return nil, llm.Model{}, err
		}
		if len(choices) == 0 {
			return nil, llm.Model{}, errors.New("cli: no models available")
		}
		idx, err := c.pickModel(choices, "Select model")
		if err != nil {
			return nil, llm.Model{}, err
		}
		return c.resolveChoice(choices[idx])
	}

	if model != "" && provider != "" {
		choice, err := c.findModel(provider, model)
		if err != nil {
			return nil, llm.Model{}, err
		}
		return c.resolveChoice(choice)
	}

	if model != "" {
		var matches []modelChoice
		for _, p := range c.cfg.Providers {
			if choice, err := c.findModel(p.Provider, model); err == nil {
				matches = append(matches, choice)
			}
		}
		if len(matches) == 0 {
			if res := c.llmMgr.Select(llm.SelectRequest{Mode: llm.SelectModeExact, Model: model}); res.Err == nil {
				matches = append(matches, modelChoice{provider: res.Provider, model: res.Model})
			}
		}
		if len(matches) == 0 {
			return nil, llm.Model{}, fmt.Errorf("cli: model %q not found", model)
		}
		if len(matches) == 1 {
			return c.resolveChoice(matches[0])
		}
		idx, err := c.pickModel(matches, "Multiple providers have this model; select one")
		if err != nil {
			return nil, llm.Model{}, err
		}
		return c.resolveChoice(matches[idx])
	}

	if provider != "" {
		choices, err := c.availableModels(provider)
		if err != nil {
			return nil, llm.Model{}, err
		}
		if len(choices) == 0 {
			return nil, llm.Model{}, fmt.Errorf("cli: no models available for provider %q", provider)
		}
		idx, err := c.pickModel(choices, "Select model")
		if err != nil {
			return nil, llm.Model{}, err
		}
		return c.resolveChoice(choices[idx])
	}

	if c.cfg != nil {
		if def, ok := c.cfg.AgentDefaults["orchestrator"]; ok {
			if choice, ok := c.resolveAgentDefault(def); ok {
				return c.resolveChoice(choice)
			}
		}
	}

	mode := llm.SelectModePreferred

	reply := "llm.select.reply." + uuid.NewString()
	resultCh := make(chan llm.SelectResult, 1)
	handler := func(res llm.SelectResult) {
		select {
		case resultCh <- res:
		default:
		}
	}
	if err := c.bus.SubscribeOnce(reply, handler); err != nil {
		return nil, llm.Model{}, fmt.Errorf("cli: subscribe model select reply: %w", err)
	}
	defer c.bus.Unsubscribe(reply, handler)

	c.bus.Publish(event.TopicModelSelect, llm.SelectRequest{
		Mode:       mode,
		ReplyTopic: reply,
	})

	select {
	case res := <-resultCh:
		if res.Err != nil {
			return nil, llm.Model{}, res.Err
		}
		return c.resolveChoice(modelChoice{provider: res.Provider, model: res.Model})
	case <-time.After(30 * time.Second):
		return nil, llm.Model{}, errors.New("cli: timed out selecting model")
	}
}

func (c *CLI) resolveAgentDefault(def config.AgentModelDefault) (modelChoice, bool) {
	try := func(provider, model string) (modelChoice, bool) {
		if model == "" {
			return modelChoice{}, false
		}
		if provider != "" {
			if choice, err := c.findModel(provider, model); err == nil {
				return choice, true
			}
		}
		if res := c.llmMgr.Select(llm.SelectRequest{Mode: llm.SelectModeExact, Model: model}); res.Err == nil {
			return modelChoice{provider: res.Provider, model: res.Model}, true
		}
		return modelChoice{}, false
	}
	if choice, ok := try(def.Provider, def.Model); ok {
		return choice, true
	}
	if choice, ok := try(def.FallbackProvider, def.FallbackModel); ok {
		return choice, true
	}
	return modelChoice{}, false
}

func (c *CLI) availableModels(provider string) ([]modelChoice, error) {
	if provider != "" {
		models, err := c.llmMgr.Models(provider)
		if err != nil {
			return nil, err
		}
		return toChoices(provider, models), nil
	}
	var out []modelChoice
	for _, opt := range router.ModelOptionsHook.Apply(nil) {
		out = append(out, modelChoice{provider: opt.Provider, model: llm.Model{ID: opt.ModelID, Name: opt.Label}})
	}
	for _, p := range c.cfg.Providers {
		models, err := c.llmMgr.Models(p.Provider)
		if err != nil {
			continue
		}
		out = append(out, toChoices(p.Provider, models)...)
	}
	return out, nil
}

func (c *CLI) findModel(provider, model string) (modelChoice, error) {
	models, err := c.llmMgr.Models(provider)
	if err != nil {
		return modelChoice{}, err
	}
	for _, m := range models {
		if m.ID == model || m.Name == model {
			return modelChoice{provider: provider, model: m}, nil
		}
	}
	return modelChoice{}, fmt.Errorf("cli: model %q not found in provider %q", model, provider)
}

func (c *CLI) pickModel(choices []modelChoice, label string) (int, error) {
	providers := make(map[string]bool, len(choices))
	for _, ch := range choices {
		providers[ch.provider] = true
	}
	withProvider := len(providers) > 1
	items := make([]string, len(choices))
	for i, ch := range choices {
		items[i] = displayChoice(ch, withProvider)
	}
	if c.picker == nil {
		return 0, errors.New("cli: model selector unavailable")
	}
	idx, err := c.picker(items, label)
	if err != nil {
		return 0, fmt.Errorf("cli: select model: %w", err)
	}
	if idx < 0 || idx >= len(items) {
		return 0, errors.New("cli: invalid model selection")
	}
	return idx, nil
}

func (c *CLI) resolveChoice(ch modelChoice) (llm.Provider, llm.Model, error) {
	prov, err := c.llmMgr.Provider(ch.provider)
	if err != nil {
		return nil, llm.Model{}, err
	}
	return prov, ch.model, nil
}

func toChoices(provider string, models []llm.Model) []modelChoice {
	out := make([]modelChoice, 0, len(models))
	for _, m := range models {
		out = append(out, modelChoice{provider: provider, model: m})
	}
	return out
}

func displayChoice(ch modelChoice, withProvider bool) string {
	if ch.model.ID == router.SmartRouterModel {
		return "Smart Router (auto-select per task)"
	}
	name := ch.model.ID
	if ch.model.Name != "" {
		name = ch.model.Name
	}
	if withProvider {
		return ch.provider + "/" + name
	}
	return name
}

func defaultPicker(items []string, label string) (int, error) {
	p := promptui.Select{
		Label: label,
		Items: items,
		Size:  10,
		Searcher: func(input string, index int) bool {
			return search.Score(input, items[index]) >= 0
		},
	}
	idx, _, err := p.Run()
	return idx, err
}

type runAgentState struct {
	inThinking      bool
	streamed        bool
	contentLineOpen bool
}

type runRenderer struct {
	out          io.Writer
	in           io.Reader
	bus          event.Bus
	ctx          context.Context
	mu           sync.Mutex
	colors       bool
	showThinking bool
	showSubagent bool
	chatMode     bool
	mainID       string
	states       map[string]*runAgentState

	thinkingCount int
	subRunning    map[string]bool
	subOrder      []string
	subCurrent    string
	animLabel     string
	animActive    bool
	animStop      chan struct{}
	animDone      chan struct{}
}

func newRunRenderer(out io.Writer, in io.Reader, mainID string, showThinking, showSubagent bool) *runRenderer {
	return &runRenderer{
		out:          out,
		in:           in,
		colors:       isTerminal(out),
		showThinking: showThinking,
		showSubagent: showSubagent,
		mainID:       mainID,
		states:       make(map[string]*runAgentState),
		subRunning:   make(map[string]bool),
	}
}

func isTerminal(v any) bool {
	f, ok := v.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func (r *runRenderer) subscribe(bus event.Bus) error {
	subs := []struct {
		topic string
		fn    any
	}{
		{agent.TopicAgentStarted, r.onStarted},
		{agent.TopicAgentInput, r.onInput},
		{agent.TopicAgentToken, r.onToken},
		{agent.TopicAgentMessage, r.onMessage},
		{agent.TopicAgentToolCall, r.onToolCall},
		{agent.TopicAgentToolResult, r.onToolResult},
		{agent.TopicAgentFinished, r.onFinished},
		{agent.TopicAgentError, r.onError},
		{agent.TopicAgentAsk, r.onAsk},
		{compaction.TopicCompactionFinished, r.onCompaction},
	}
	for _, s := range subs {
		if err := bus.Subscribe(s.topic, s.fn); err != nil {
			return fmt.Errorf("cli: subscribe %s: %w", s.topic, err)
		}
	}
	return nil
}

func (r *runRenderer) unsubscribe(bus event.Bus) {
	subs := []struct {
		topic string
		fn    any
	}{
		{agent.TopicAgentStarted, r.onStarted},
		{agent.TopicAgentInput, r.onInput},
		{agent.TopicAgentToken, r.onToken},
		{agent.TopicAgentMessage, r.onMessage},
		{agent.TopicAgentToolCall, r.onToolCall},
		{agent.TopicAgentToolResult, r.onToolResult},
		{agent.TopicAgentFinished, r.onFinished},
		{agent.TopicAgentError, r.onError},
		{agent.TopicAgentAsk, r.onAsk},
		{compaction.TopicCompactionFinished, r.onCompaction},
	}
	for _, s := range subs {
		_ = bus.Unsubscribe(s.topic, s.fn)
	}
}

func (r *runRenderer) write(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writeLocked(format, args...)
}

func (r *runRenderer) writeLocked(format string, args ...any) {
	if r.animActive && !strings.HasSuffix(format, "\r") {
		fmt.Fprintf(r.out, "\r%-40s\r", "")
	}
	fmt.Fprintf(r.out, format, args...)
}

func (r *runRenderer) state(id string) *runAgentState {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.states[id]
	if !ok {
		st = &runAgentState{}
		r.states[id] = st
	}
	return st
}

func (r *runRenderer) startThinking(st *runAgentState) {
	if st.inThinking {
		return
	}
	st.inThinking = true
	if r.showThinking {
		r.write("\n%s\n", r.dim("Thinking:"))
		return
	}
	r.mu.Lock()
	r.thinkingCount++
	r.syncAnim()
	r.mu.Unlock()
}

func (r *runRenderer) endThinking(st *runAgentState) {
	if !st.inThinking {
		return
	}
	st.inThinking = false
	if r.showThinking {
		r.write("\n")
		return
	}
	r.mu.Lock()
	r.thinkingCount--
	if r.thinkingCount < 0 {
		r.thinkingCount = 0
	}
	r.syncAnim()
	r.mu.Unlock()
}

func (r *runRenderer) subStarted(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == "" || r.subRunning[name] {
		return
	}
	r.subRunning[name] = true
	r.subOrder = append(r.subOrder, name)
	if r.subCurrent == "" {
		r.subCurrent = name
	}
	r.syncAnim()
}

func (r *runRenderer) subFinished(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.subRunning[name] {
		return
	}
	delete(r.subRunning, name)
	if name == r.subCurrent {
		r.subCurrent = ""
		for _, n := range r.subOrder {
			if r.subRunning[n] {
				r.subCurrent = n
				break
			}
		}
	}
	r.syncAnim()
}

func (r *runRenderer) desiredLabel() string {
	if r.thinkingCount > 0 {
		return "Thinking"
	}
	if len(r.subRunning) > 0 && r.subCurrent != "" {
		return "Subagent " + r.subCurrent
	}
	return ""
}

func (r *runRenderer) syncAnim() {
	desired := r.desiredLabel()
	switch {
	case desired == r.animLabel:
		return
	case r.animActive && desired != "":
		r.animLabel = desired
		return
	case desired == "":
		if r.animActive {
			r.animActive = false
			close(r.animStop)
		}
		r.resolveAnimLocked(r.animLabel)
		r.animLabel = ""
	default:
		if !r.colors && r.animLabel != "" {
			r.writeLocked("%s\n", r.animLabel)
		}
		r.animLabel = desired
		if !r.colors {
			r.writeLocked("%s...\n", desired)
			return
		}
		r.animActive = true
		r.animStop = make(chan struct{})
		r.animDone = make(chan struct{})
		go r.animate(r.animStop, r.animDone)
	}
}

func (r *runRenderer) resolveAnimLocked(label string) {
	if label == "" {
		return
	}
	if r.colors {
		r.writeLocked("\r%-40s\n", label)
	} else {
		r.writeLocked("%s\n", label)
	}
}

func (r *runRenderer) animate(stop <-chan struct{}, done chan<- struct{}) {
	dots := 0
	t := time.NewTicker(150 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stop:
			close(done)
			return
		case <-t.C:
			r.mu.Lock()
			select {
			case <-stop:
				r.mu.Unlock()
				continue
			default:
			}
			dots = dots%3 + 1
			fmt.Fprintf(r.out, "\r%-40s", r.animLabel+strings.Repeat(".", dots))
			r.mu.Unlock()
		}
	}
}

func (r *runRenderer) closeThinking(st *runAgentState) {
	r.endThinking(st)
	if st.contentLineOpen {
		st.contentLineOpen = false
		r.write("\n")
	}
}

func (r *runRenderer) agentTag(agentID, agentName string) string {
	if agentID == r.mainID || agentName == "" {
		return ""
	}
	return r.dim("[" + agentName + "] ")
}

func (r *runRenderer) banner(provider, model, mode string) {
	if mode != "" {
		r.write("\n%s %s (%s/%s, %s)\n\n", r.bold("Running"), r.bold("orchestrator"), provider, model, mode)
		return
	}
	r.write("\n%s %s (%s/%s)\n\n", r.bold("Running"), r.bold("orchestrator"), provider, model)
}

func (r *runRenderer) attachments(paths []string) {
	r.write("%s %s\n", r.dim("attached"), strings.Join(paths, ", "))
}

func (r *runRenderer) renderSessionHistory(msgs []session.Message) {
	r.write("%s\n", r.dim("--- session history ---"))
	for _, m := range msgs {
		lm := llm.Message{
			Role:       m.Role,
			Content:    m.Content,
			Reasoning:  m.Reasoning,
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
			ToolCalls:  m.ToolCalls,
		}
		switch lm.Role {
		case llm.RoleUser:
			if c := messageContent(lm); c != "" {
				r.write("%s\n", r.bold("> "+c))
			}
		case llm.RoleAssistant:
			if reasoning := messageReasoning(lm); reasoning != "" && r.showThinking {
				r.write("%s\n%s\n", r.dim("Thinking:"), r.dim(reasoning))
			}
			if c := messageContent(lm); c != "" {
				r.write("%s\n", c)
			}
			for _, tc := range lm.ToolCalls {
				r.write("%s %s\n", r.green("tool"), formatToolCall(tc))
			}
		case llm.RoleTool:
			r.write("%s %s\n", r.dim("result"), truncate(fmt.Sprint(lm.Content), 500))
		}
	}
	r.write("\n")
}

func (r *runRenderer) onStarted(e agent.AgentStarted) {
	if e.AgentID != r.mainID && !r.showSubagent {
		r.subStarted(e.AgentName)
	}
	model := e.Model.ID
	if e.Model.Name != "" {
		model = e.Model.Name
	}
	prov := ""
	if e.Provider != nil {
		prov = e.Provider.Name() + "/"
	}
	r.write("%s\n", r.dim("• "+e.AgentName+" started ("+prov+model+")"))
}

func (r *runRenderer) onInput(e agent.AgentInput) {
	if e.AgentID != r.mainID || r.chatMode {
		return
	}
	r.write("%s\n", r.bold("> "+e.Input))
}

func (r *runRenderer) prompt() {
	st := r.state(r.mainID)
	r.closeThinking(st)
	r.write("> ")
}

func (r *runRenderer) onToken(e agent.AgentToken) {
	st := r.state(e.AgentID)
	if e.Reasoning != "" {
		if !st.inThinking {
			r.startThinking(st)
		}
		if r.showThinking {
			r.write("%s", r.dim(e.Reasoning))
		}
		st.streamed = true
	}
	if e.Content != "" {
		if st.inThinking {
			r.endThinking(st)
		}
		st.streamed = true
		st.contentLineOpen = !strings.HasSuffix(e.Content, "\n")
		r.write("%s", e.Content)
	}
}

func (r *runRenderer) onMessage(e agent.AgentMessage) {
	st := r.state(e.AgentID)
	if st.streamed {
		st.streamed = false
		return
	}
	if e.AgentID != r.mainID && !r.showSubagent {
		return
	}
	reasoning := messageReasoning(e.Message)
	if reasoning != "" {
		r.closeThinking(st)
		if r.showThinking {
			r.write("\n%s\n%s\n", r.dim("Thinking:"), r.dim(reasoning))
		} else {
			r.write("Thinking\n")
		}
	}
	content := messageContent(e.Message)
	if content != "" {
		r.write("\n%s\n", content)
	}
}

func (r *runRenderer) onToolCall(e agent.AgentToolCall) {
	if e.AgentID != r.mainID && !r.showSubagent {
		return
	}
	st := r.state(e.AgentID)
	r.closeThinking(st)
	r.write("%s %s%s\n", r.green("tool"), r.agentTag(e.AgentID, e.AgentName), formatToolCall(e.Call))
}

func (r *runRenderer) onToolResult(e agent.AgentToolResult) {
	if e.AgentID != r.mainID && !r.showSubagent {
		return
	}
	st := r.state(e.AgentID)
	r.closeThinking(st)
	if e.Err != nil {
		r.write("%s %s%s: %s\n", r.red("error"), r.agentTag(e.AgentID, e.AgentName), e.ToolName, truncate(e.Output, 500))
		return
	}
	out := truncate(e.Output, 500)
	if strings.TrimSpace(out) == "" {
		out = "(no output)"
	}
	r.write("%s %s%s: %s\n", r.dim("result"), r.agentTag(e.AgentID, e.AgentName), e.ToolName, out)
}

func (r *runRenderer) onFinished(e agent.AgentFinished) {
	st := r.state(e.AgentID)
	r.closeThinking(st)
	if e.AgentID != r.mainID {
		r.subFinished(e.AgentName)
	}
	parts := []string{fmt.Sprintf("%d iterations", e.Iterations)}
	if e.Usage.TotalTokens > 0 {
		parts = append(parts, fmt.Sprintf("%d tokens", e.Usage.TotalTokens))
	}
	if e.Usage.Cost > 0 {
		parts = append(parts, fmt.Sprintf("$%.6f", e.Usage.Cost))
	}
	r.write("%s\n", r.dim("• "+e.AgentName+" finished ("+strings.Join(parts, ", ")+")"))
}

func (r *runRenderer) onError(e agent.AgentError) {
	st := r.state(e.AgentID)
	r.closeThinking(st)
	if e.AgentID != r.mainID {
		r.subFinished(e.AgentName)
		r.write("%s %s: %v\n", r.red("error"), e.AgentName, e.Err)
	}
}

func (r *runRenderer) readLine() (string, bool) {
	ch := make(chan string, 1)
	go func() {
		var b [1]byte
		var line []byte
		for {
			n, err := r.in.Read(b[:])
			if n > 0 {
				if b[0] == '\n' {
					ch <- string(line)
					return
				}
				line = append(line, b[0])
			}
			if err != nil {
				ch <- ""
				return
			}
		}
	}()
	select {
	case line := <-ch:
		return line, true
	case <-r.ctx.Done():
		return "", false
	}
}

func (r *runRenderer) onCompaction(e compaction.Event) {
	r.write("\n%s Context compacted (%s) — %d messages, %d tokens\n",
		r.green("↻"), e.Strategy, e.Messages, e.Tokens)
}

func (r *runRenderer) onAsk(e agent.AgentAsk) {
	if e.AgentID != r.mainID {
		return
	}
	r.write("\n%s %s is asking:\n", r.green("askuserquestion"), e.AgentName)
	answers := make(map[string]string)

	for _, q := range e.Questions {
		for {
			r.write("\n%s (%s)", r.bold(q.Question), q.Type)
			if q.Required {
				r.write(" [required]")
			}
			r.write(":\n")

			switch q.Type {
			case "text":
				r.write("> ")
				val, ok := r.readLine()
				if !ok {
					goto done
				}
				val = strings.TrimSpace(val)
				if val == "" && q.Required {
					r.write("%s Answer is required.\n", r.red("!"))
					continue
				}
				answers[q.ID] = val
			case "select":
				for i, opt := range q.Options {
					r.write("  %d) %s\n", i+1, opt)
				}
				r.write("  %d) Custom answer...\n", len(q.Options)+1)
				r.write("> ")
				val, ok := r.readLine()
				if !ok {
					goto done
				}
				val = strings.TrimSpace(val)
				if val == "" && q.Required {
					r.write("%s Answer is required.\n", r.red("!"))
					continue
				}
				idx := 0
				if _, err := fmt.Sscanf(val, "%d", &idx); idx >= 1 && idx <= len(q.Options) && err == nil {
					answers[q.ID] = q.Options[idx-1]
				} else {
					answers[q.ID] = val
				}
			case "multiselect":
				for i, opt := range q.Options {
					r.write("  %d) %s\n", i+1, opt)
				}
				r.write("  (comma-separated numbers, e.g. 1,3)\n> ")
				val, ok := r.readLine()
				if !ok {
					goto done
				}
				labels := parseMultiSelection(val, q.Options)
				if len(labels) == 0 && q.Required {
					r.write("%s Select at least one option.\n", r.red("!"))
					continue
				}
				answers[q.ID] = strings.Join(labels, ", ")
			case "boolean":
				r.write("  1) Yes\n  2) No\n> ")
				val, ok := r.readLine()
				if !ok {
					goto done
				}
				switch strings.ToLower(strings.TrimSpace(val)) {
				case "1", "y", "yes", "true":
					answers[q.ID] = "yes"
				case "2", "n", "no", "false":
					answers[q.ID] = "no"
				default:
					if q.Required {
						r.write("%s Answer is required.\n", r.red("!"))
						continue
					}
				}
			}
			break
		}
	}

done:
	r.bus.Publish(agent.TopicAgentAskAnswer, agent.AgentAskAnswer{
		AgentID: e.AgentID,
		Answers: answers,
	})
}

func parseMultiSelection(val string, options []string) []string {
	fields := strings.FieldsFunc(val, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
	seen := make(map[int]bool)
	labels := make([]string, 0, len(fields))
	for _, f := range fields {
		idx := 0
		if _, err := fmt.Sscanf(f, "%d", &idx); err != nil {
			continue
		}
		if idx < 1 || idx > len(options) || seen[idx] {
			continue
		}
		seen[idx] = true
		labels = append(labels, options[idx-1])
	}
	return labels
}

func (r *runRenderer) bold(s string) string { return r.color("\x1b[1m", s) }
func (r *runRenderer) dim(s string) string  { return r.color("\x1b[2m", s) }
func (r *runRenderer) green(s string) string {
	return r.color("\x1b[32m", s)
}
func (r *runRenderer) red(s string) string { return r.color("\x1b[31m", s) }

func (r *runRenderer) color(code, s string) string {
	if !r.colors {
		return s
	}
	return code + s + "\x1b[0m"
}

func formatToolCall(call llm.ToolCall) string {
	args := strings.TrimSpace(call.Function.Arguments)
	if args == "" {
		return call.Function.Name + "()"
	}
	return call.Function.Name + "(" + truncate(args, 300) + ")"
}

func messageContent(m llm.Message) string {
	switch c := m.Content.(type) {
	case string:
		return c
	case []any:
		for _, item := range c {
			if mm, ok := item.(map[string]any); ok {
				if t, ok := mm["type"].(string); ok && t == "text" {
					if text, ok := mm["text"].(string); ok {
						return text
					}
				}
			}
		}
	}
	return ""
}

func messageReasoning(m llm.Message) string {
	if s, ok := m.Reasoning.(string); ok {
		return s
	}
	return ""
}
