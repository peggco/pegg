export type Meta = Record<string, unknown>;

export type SessionMode = 'auto' | 'plan' | 'build';

export interface Annotations {
  _meta?: Meta;
  audience?: string[];
  priority?: number;
  lastModified?: string;
}

export interface EmbeddedResource {
  _meta?: Meta;
  uri: string;
  mimeType?: string;
  text?: string;
  blob?: string;
  annotations?: Annotations;
}

export interface ContentBlock {
  _meta?: Meta;
  type: 'text' | 'image' | 'audio' | 'resource' | 'resource_link';
  text?: string;
  data?: string;
  mimeType?: string;
  uri?: string;
  name?: string;
  title?: string;
  description?: string;
  size?: number;
  resource?: EmbeddedResource;
  annotations?: Annotations;
}

export interface ToolCallLocation {
  _meta?: Meta;
  path: string;
  line?: number;
}

export interface Diff {
  _meta?: Meta;
  path: string;
  oldText?: string;
  newText: string;
}

export interface ToolCallContent {
  _meta?: Meta;
  type: string;
  content?: ContentBlock;
  diff?: Diff;
  terminalId?: string;
}

export interface AttachmentInfo {
  name: string;
  mediaType?: string;
  type?: string;
  size?: number;
  path?: string;
}

export interface PlanEntry {
  _meta?: Meta;
  content: string;
  priority?: string;
  status?: string;
}

export interface Cost {
  _meta?: Meta;
  amount: number;
  currency: string;
}

export interface AvailableCommandInput {
  _meta?: Meta;
  hint: string;
}

export interface AvailableCommand {
  _meta?: Meta;
  name: string;
  description: string;
  input?: AvailableCommandInput;
}

export type SlashCommand = AvailableCommand;

export interface SessionModeInfo {
  _meta?: Meta;
  id: string;
  name: string;
  description?: string;
}

export interface SessionModeState {
  _meta?: Meta;
  currentModeId: string;
  availableModes: SessionModeInfo[];
}

export type SessionConfigOptionCategory = 'mode' | 'model' | 'model_config' | 'thought_level' | string;

export interface SessionConfigSelectOption {
  _meta?: Meta;
  value: string;
  name: string;
  description?: string;
}

export interface SessionConfigSelectGroup {
  _meta?: Meta;
  group: string;
  name: string;
  options: SessionConfigSelectOption[];
}

export interface SessionConfigOption {
  _meta?: Meta;
  id: string;
  name: string;
  description?: string;
  category?: SessionConfigOptionCategory;
  type: 'select' | 'boolean' | string;
  currentValue: string | boolean;
  options?: SessionConfigSelectOption[] | SessionConfigSelectGroup[];
}

export type NoticeSeverity = 'info' | 'warning' | 'error';

export interface Notice {
  _meta?: Meta;
  severity: NoticeSeverity;
  title: string;
  description?: string;
}

export type CompactionStatus = 'in_progress' | 'completed' | 'failed' | 'cancelled';

export interface CompactionUpdate {
  _meta?: Meta;
  compactionId: string;
  status: CompactionStatus;
  summary?: ContentBlock[];
  error?: string;
}

export interface CompactionSummaryChunk {
  _meta?: Meta;
  compactionId: string;
  content: ContentBlock;
}

export interface SessionUpdate {
  _meta?: Meta;
  sessionUpdate: string;
  messageId?: string;
  content?: ContentBlock | ToolCallContent[] | string;
  toolCallId?: string;
  title?: string;
  name?: string;
  kind?: string;
  status?: string;
  locations?: ToolCallLocation[];
  rawInput?: unknown;
  rawOutput?: unknown;
  entries?: PlanEntry[];
  availableCommands?: AvailableCommand[];
  currentModeId?: string;
  configOptions?: SessionConfigOption[];
  updatedAt?: string;
  used?: number;
  size?: number;
  cost?: Cost;
  severity?: NoticeSeverity;
  description?: string;
  compactionId?: string;
  summary?: ContentBlock[];
  error?: string;
}

export type PeggUpdate = SessionUpdate;

export interface SessionInfo {
  _meta?: Meta;
  sessionId: string;
  cwd: string;
  title?: string;
  updatedAt?: string;
  additionalDirectories?: string[];
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
  _meta?: Meta;
  sessionId: string;
  modes?: SessionModeState;
  configOptions?: SessionConfigOption[];
}

export interface SessionStateResult {
  _meta?: Meta;
  sessionId?: string;
  modes?: SessionModeState;
  configOptions?: SessionConfigOption[];
}

export type SessionLoadResult = SessionStateResult;

export interface SessionForkResult {
  _meta?: Meta;
  sessionId: string;
  title?: string;
  parentId?: string;
}

export interface SessionRevertResult {
  _meta?: Meta;
  removed: number;
  snapshotId?: string;
}

export interface SessionHistoryResult {
  _meta?: Meta;
  oldestSeq: number;
  hasMore: boolean;
}

export interface SessionListResult {
  _meta?: Meta;
  sessions: SessionInfo[];
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
  _meta?: Meta;
  sessionId: string;
  agentId?: string;
  agentName?: string;
  questions: AskQuestion[];
}

export interface PermissionOption {
  _meta?: Meta;
  optionId: string;
  name: string;
  kind: string;
}

export interface ToolCallUpdate {
  _meta?: Meta;
  toolCallId: string;
  title?: string;
  name?: string;
  kind?: string;
  status?: string;
  content?: ToolCallContent[];
  locations?: ToolCallLocation[];
  rawInput?: unknown;
  rawOutput?: unknown;
}

export interface PermissionParams {
  _meta?: Meta;
  sessionId: string;
  toolCall: ToolCallUpdate;
  options: PermissionOption[];
}

export interface TerminalExitStatus {
  _meta?: Meta;
  exitCode?: number;
  signal?: string;
}

export interface TerminalOutputResult {
  _meta?: Meta;
  output: string;
  truncated: boolean;
  exitStatus?: TerminalExitStatus;
}

export interface TerminalCreateParams {
  _meta?: Meta;
  sessionId: string;
  command: string;
  args?: string[];
  env?: { name: string; value: string }[];
  cwd?: string;
  outputByteLimit?: number;
}

export interface TerminalIdParams {
  _meta?: Meta;
  sessionId: string;
  terminalId: string;
}

export type ElicitationMode = 'form' | 'url';
export type ElicitationAction = 'accept' | 'decline' | 'cancel';

export interface EnumOption {
  const: string;
  title: string;
  description?: string;
}

export interface MultiSelectItems {
  type: 'string';
  enum?: string[];
  enumNames?: string[];
}

export interface ElicitationPropertySchema {
  type: 'string' | 'number' | 'integer' | 'boolean' | 'array';
  title?: string;
  description?: string;
  enum?: string[];
  enumNames?: string[];
  format?: string;
  items?: MultiSelectItems;
  default?: unknown;
  minimum?: number;
  maximum?: number;
  minItems?: number;
  maxItems?: number;
}

export interface ElicitationSchema {
  _meta?: Meta;
  type: 'object';
  title?: string;
  description?: string;
  properties: Record<string, ElicitationPropertySchema>;
  required?: string[];
}

export interface ElicitationCreateParams {
  _meta?: Meta;
  mode: ElicitationMode;
  message: string;
  sessionId?: string;
  toolCallId?: string;
  requestId?: string | number;
  requestedSchema?: ElicitationSchema;
  elicitationId?: string;
  url?: string;
}

export interface ElicitationCreateResult {
  _meta?: Meta;
  action: ElicitationAction;
  content?: Record<string, unknown>;
}

export interface Implementation {
  _meta?: Meta;
  name: string;
  title?: string;
  version: string;
}

export interface AuthMethod {
  _meta?: Meta;
  id: string;
  name: string;
  description?: string;
  type?: string;
  args?: string[];
  env?: Record<string, string>;
}

export interface PeggCapabilities {
  ask?: boolean;
  permissions?: boolean;
  models?: boolean;
  commands?: boolean;
  attachments?: boolean;
  diffs?: boolean;
  plan?: boolean;
  history?: boolean;
  terminals?: boolean;
  elicitation?: boolean;
}

export interface AgentCapabilities {
  _meta?: Meta & { 'com.pegg/capabilities'?: PeggCapabilities };
  loadSession?: boolean;
  promptCapabilities?: { image?: boolean; audio?: boolean; embeddedContext?: boolean };
  mcpCapabilities?: { http?: boolean; sse?: boolean };
  sessionCapabilities?: {
    delete?: boolean;
    resume?: boolean;
    close?: boolean;
    list?: boolean;
    additionalDirectories?: boolean;
    systemPrompt?: { mode?: { override?: boolean } };
  };
  auth?: { logout?: boolean };
}

export interface InitializeResult {
  _meta?: Meta;
  protocolVersion: number;
  agentCapabilities?: AgentCapabilities;
  agentInfo?: Implementation;
  authMethods?: AuthMethod[];
}

export interface ModelEntry {
  provider: string;
  model: { id: string; name?: string; owned_by?: string };
}

export interface ModelListResult {
  models: ModelEntry[];
}

export interface PromptResult {
  _meta?: Meta;
  stopReason: string;
}

export type ServerRequestHandler = (method: string, params: unknown) => Promise<unknown>;
