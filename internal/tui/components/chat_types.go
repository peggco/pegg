package components

import (
	"time"

	"github.com/peggco/pegg/internal/llm"
)

type ItemKind int

const (
	ItemUser ItemKind = iota
	ItemAssistant
	ItemThinking
	ItemTool
	ItemSubagent
	ItemFinished
	ItemError
	ItemCompaction
	ItemCompactionDivider
	ItemNotice
)

type ChatItem struct {
	Kind ItemKind
	ID   string

	MsgID        string
	MsgSessionID string

	Text      string
	Reasoning string

	ToolName     string
	ToolArgs     string
	ToolOutput   string
	ToolPartial  string
	ToolErr      string
	ToolDone     bool
	Diff         []DiffHunk
	WriteContent string
	Activity     string

	ToolTaskName       string
	ToolTaskType       string
	ToolTaskPrompt     string
	ToolTaskBackground bool
	ToolTaskAgentID    string
	ToolTaskStatus     string
	ToolTaskUsage      llm.Usage
	TaskStartedAt      time.Time

	AgentID          string
	SubagentName     string
	SubagentStatus   string
	SubagentTask     string
	SubagentOutput   string
	SubagentUsage    llm.Usage
	SubagentActivity string

	Attachments []llm.Attachment

	Duration time.Duration
	Expanded bool
}

type flatItem struct {
	ItemIdx int
	Start   int
	End     int
}

type Chat struct {
	items       []*ChatItem
	scroll      int
	autoScroll  bool
	wasAtBottom bool
	back        bool
	hasMore     bool

	onActivate        func(*ChatItem)
	onOpenFile        func(path string)
	onBack            func()
	onLoadMore        func()
	onSubagentHistory func(agentID string)

	flat       []Line
	flatItems  []flatItem
	flatRev    int
	itemCursor int
	follow     bool
	now        time.Duration

	indicatorX0, indicatorX1, indicatorY int
	indicatorVisible                     bool
	backX0, backX1, backY                int
	backVisible                          bool

	historyButtons map[int]string
	fileButtons    map[int]string

	lastWidth   int
	lastVisible int

	cache      [][]Line
	cacheWidth int
	dirty      bool

	selActive  bool
	selStart   selPos
	selEnd     selPos
	mouseDown  bool
	mouseStart selPos
	dragged    bool

	hscroll         map[*ChatItem]int
	itemMaxWidth    map[*ChatItem]int
	itemTableBounds map[*ChatItem][]int
	tableHintOn     bool
	tableLeftX0     int
	tableLeftX1     int
	tableLeftY      int
	tableLeftOn     bool
	tableRightX0    int
	tableRightX1    int
	tableRightY     int
	tableRightOn    bool
}

type selPos struct {
	line int
	cell int
}
