export type SessionMode = 'auto' | 'plan' | 'build';

export interface ContentBlock {
  type: string;
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

export interface PlanEntry {
  content: string;
  priority?: string;
  status?: string;
}

export interface SlashCommand {
  name: string;
  description?: string;
}

export interface SessionUpdate {
  sessionUpdate: string;
  messageId?: string;
  content?: ContentBlock | string;
  toolCallId?: string;
  toolName?: string;
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
  runId?: string;
  runState?: string;
  baseSeq?: number;
  seq?: number;
  historyPage?: boolean;
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

export interface SessionListResult {
  sessions: SessionListItem[];
  nextCursor?: string;
}

export interface SessionInfo {
  acpId: string;
  peggId: string;
  cwd: string;
  title: string;
  mode: SessionMode | string;
  provider: string;
  model: string;
  reasoningEffort?: string;
  permissionMode?: string;
  commands?: SlashCommand[];
  historyOldestSeq?: number;
  historyHasMore?: boolean;
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

export interface PermissionOption {
  optionId: string;
  name: string;
  kind: string;
}

export interface PermissionParams {
  sessionId: string;
  question?: string;
  toolCall?: { toolCallId?: string; title?: string; kind?: string; rawInput?: unknown };
  options?: PermissionOption[];
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

export interface Theme {
  name: string;
  label: string;
  dark: boolean;
  tokens: Record<string, string>;
}

export interface WindowPrefs {
  width?: number;
  height?: number;
  x?: number;
  y?: number;
}

export interface Recent {
  path: string;
  name: string;
  openedAt: string;
}

export interface Prefs {
  theme: string;
  themeMode: 'auto' | 'light' | 'dark';
  workspace: string;
  recents: Recent[];
  binaryPath: string;
  autoDownload: boolean;
  sendOnEnter: boolean;
  fontScale: number;
  sidebarCollapsed: boolean;
  lastSession: string;
  window: WindowPrefs;
}

export interface BinaryStatus {
  found: boolean;
  path?: string;
  version?: string;
  source?: string;
  managed?: boolean;
  error?: string;
}

export interface BootstrapInfo {
  platform: string;
  arch: string;
  version: string;
  home: string;
  prefs: Prefs;
  themes: Theme[];
  binary: BinaryStatus;
  defaultWorkspace: string;
  workspace: string;
  logPath: string;
  language: string;
  nativeTitlebar: boolean;
}

export interface Attachment {
  id: string;
  name: string;
  mediaType: string;
  size: number;
  path?: string;
  data?: string;
  text?: string;
}

export interface PromptAttachment {
  name: string;
  mediaType: string;
  data?: string;
  text?: string;
  path?: string;
  size?: number;
}

export interface BinaryProgress {
  stage: string;
  percent: number;
  detail?: string;
}

export interface AcpStatus {
  state: 'connected' | 'exited' | 'error' | string;
  message?: string;
  code?: number;
}

export interface PermissionRequest {
  requestId: string;
  params: PermissionParams;
}

export interface AskRequest {
  requestId: string;
  params: AskParams;
}

export interface LanguageOption {
  code: string;
  label: string;
}

export interface ModelOption {
  id: string;
  name?: string;
  reasoningEfforts?: string[];
}

export interface ProviderCatalogEntry {
  name: string;
  configured: boolean;
  kind: string;
  hint: string;
}

export interface ProviderOption {
  name: string;
  driver?: string;
  models: ModelOption[];
}

export interface STTProviderEntry {
  name: string;
  configured: boolean;
  models: string[];
}

export interface STTSettings {
  providers: string[];
  configured: string[];
  provider: string;
  model: string;
  models: string[];
  modelsByProvider: Record<string, string[]>;
  entries: STTProviderEntry[];
  recordCommand: string;
  maxDuration: number;
}

export interface GeneralSettings {
  provider: string;
  model: string;
  reasoningEffort: string;
  language: string;
  languages: LanguageOption[];
  providers: ProviderOption[];
  stt: STTSettings;
}

export interface SessionSettings {
  acpId: string;
  peggId: string;
  title: string;
  cwd: string;
  mode: string;
  provider: string;
  model: string;
}

export interface CompactionSettings {
  enabled: boolean;
  strategies: string[];
  allStrategies: string[];
  threshold: number;
  maxMessages: number;
  maxToolOutputChars: number;
}

export interface ToolPermission {
  name: string;
  mode: string;
  group: string;
}

export interface PermissionsSettings {
  default: string;
  modes: string[];
  tools: ToolPermission[];
  judgeProviders: string[];
  judgeProvider: string;
  judgeModel: string;
  judgeThreshold: number;
  defaults: Record<string, string>;
}

export interface MemorySettings {
  enabled: boolean;
  provider: string;
  model: string;
  gate: string;
  gates: string[];
  threshold: number;
  contextBudget: number;
  maxResults: number;
  consolidate: boolean;
  entries: number;
  lastEntry: string;
}

export interface MCPServerSetting {
  name: string;
  type: string;
  command?: string;
  args?: string[];
  url?: string;
  env: number;
  headers: number;
}

export interface SkillSetting {
  name: string;
  description: string;
  source: string;
  path: string;
}

export interface RuleSetting {
  name: string;
  source: string;
  path: string;
  preview: string;
}

export interface PluginSetting {
  name: string;
  path: string;
  enabled: boolean;
  status: string;
}

export interface SystemSettings {
  appVersion: string;
  agentVersion: string;
  platform: string;
  arch: string;
  binaryPath: string;
  binaryVersion: string;
  binarySource: string;
  configPath: string;
  configDir: string;
  projectDir: string;
  logPath: string;
  memoryDir: string;
  skillsDir: string;
  rulesDir: string;
  pluginsDir: string;
}

export interface SettingsSnapshot {
  general: GeneralSettings;
  session: SessionSettings;
  compaction: CompactionSettings;
  permissions: PermissionsSettings;
  memory: MemorySettings;
  system: SystemSettings;
  pluginsEnabled: boolean;
  mcp: MCPServerSetting[];
  skills: SkillSetting[];
  rules: RuleSetting[];
  plugins: PluginSetting[];
}

export interface ConnectStatus {
  state: string;
  pid?: number;
  startedAt?: string;
  error?: string;
  logs: string[];
}

export interface ConnectSettings {
  enabled: boolean;
  relayUrl: string;
  token?: string;
  clearToken?: boolean;
  tokenMasked: string;
  hasToken: boolean;
  deviceName: string;
  deviceId: string;
  publicKey: string;
  autoApprove: boolean;
  maxConcurrency: number;
  responseTimeout: number;
  allowedMethods: string[];
  status: ConnectStatus;
}

export interface TeamProvider {
  name: string;
  baseUrl: string;
  apiKeyMasked: string;
}

export interface TeamStatus {
  state: string;
  url?: string;
  error?: string;
  logs: string[];
  providers: TeamProvider[];
}

export interface MCPServerInput {
  name: string;
  command: string;
  args: string[];
  env: Record<string, string>;
  url: string;
  headers: Record<string, string>;
}
