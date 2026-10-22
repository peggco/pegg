export type SessionMode = 'auto' | 'plan' | 'build';

export interface ContentBlock {
  type: 'text' | 'image' | 'audio' | 'video' | 'resource' | 'resource_link';
  text?: string;
  data?: string;
  mimeType?: string;
  uri?: string;
  name?: string;
  resource?: { uri: string; mimeType?: string; text?: string; blob?: string };
}

export interface ToolCallLocation {
  path: string;
  line?: number;
}

export interface Diff {
  path: string;
  oldText?: string;
  newText: string;
}

export interface ToolCallContent {
  type: string;
  diff?: Diff;
  content?: ContentBlock;
}

export interface SessionUpdate {
  sessionUpdate: string;
  messageId?: string;
  content?: ContentBlock | string;
  toolCallId?: string;
  title?: string;
  kind?: string;
  status?: string;
  locations?: ToolCallLocation[];
  rawInput?: unknown;
  rawOutput?: unknown;
  entries?: PlanEntry[];
  used?: number;
  size?: number;
  cost?: { amount: number; currency: string };
  mode?: string;
  provider?: string;
  model?: string;
  commands?: SlashCommand[];
  agentId?: string;
  agentName?: string;
  error?: string;
  contents?: ToolCallContent[];
}

export interface PlanEntry {
  content: string;
  priority?: string;
  status?: string;
}

export interface SlashCommand {
  name: string;
  description?: string;
}

export interface SessionListItem {
  id: string;
  title: string;
  projectDir?: string;
  provider?: string;
  model?: string;
  mode?: string;
  usage?: { prompt_tokens?: number; completion_tokens?: number; total_tokens?: number; cost?: number };
  createdAt: string;
  updatedAt: string;
}

export interface SessionNewResult {
  sessionId: string;
  peggSessionId?: string;
  mode?: string;
  provider?: string;
  model?: string;
  permissionMode?: string;
  commands?: SlashCommand[];
}

export interface SessionForkResult {
  sessionId: string;
  title?: string;
  parentId?: string;
}

export interface SessionRevertResult {
  removed: number;
  snapshotId?: string;
}

export interface SessionListResult {
  sessions: SessionListItem[];
  nextCursor?: string;
}

export interface AskQuestion {
  id: string;
  question: string;
  type: 'text' | 'select' | 'multiselect' | 'boolean' | string;
  options?: string[];
  required?: boolean;
}

export interface AskParams {
  sessionId: string;
  agentId?: string;
  agentName?: string;
  questions: AskQuestion[];
}

export interface PermissionOption {
  optionId: string;
  name: string;
  kind: string;
}

export interface PermissionParams {
  sessionId: string;
  question?: string;
  toolCall?: { toolCallId?: string; title?: string; kind?: string; rawInput?: unknown };
  options: PermissionOption[];
}

export interface InitializeResult {
  protocolVersion: number;
  agentCapabilities?: {
    loadSession?: boolean;
    promptCapabilities?: { image?: boolean; audio?: boolean; embeddedContext?: boolean };
    pegg?: {
      ask?: boolean;
      permissions?: boolean;
      models?: boolean;
      commands?: boolean;
      attachments?: boolean;
      diffs?: boolean;
      plan?: boolean;
      history?: boolean;
    };
  };
  agentInfo?: { name?: string; title?: string; version?: string };
}

export interface ModelEntry {
  provider: string;
  model: { id: string; name?: string; owned_by?: string };
}

export interface ModelListResult {
  models: ModelEntry[];
}

export interface PromptResult {
  stopReason: string;
  usage?: { used: number; size: number; cost?: { amount: number; currency: string } };
}

export type ServerRequestHandler = (method: string, params: unknown) => Promise<unknown>;
