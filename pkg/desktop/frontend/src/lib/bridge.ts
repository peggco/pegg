import * as GoApp from '../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import type {
  AcpStatus,
  AskRequest,
  Attachment,
  BinaryProgress,
  BinaryStatus,
  BootstrapInfo,
  ModelListResult,
  PermissionRequest,
  Prefs,
  PromptAttachment,
  PromptResult,
  Recent,
  SessionInfo,
  SessionListResult,
  SettingsSnapshot,
  SlashCommand,
  ProviderCatalogEntry,
  GeneralSettings,
  CompactionSettings,
  PermissionsSettings,
  MemorySettings,
  Theme,
  ConnectSettings,
  ConnectStatus,
  TeamStatus,
} from '../types';

export interface NewSessionInput {
  cwd: string;
  mode?: string;
  provider?: string;
  model?: string;
  reasoningEffort?: string;
}

export interface LoadSessionInput {
  peggId: string;
  cwd?: string;
  title?: string;
  mode?: string;
  provider?: string;
  model?: string;
}

export interface PromptInput {
  sessionId: string;
  text: string;
  attachments?: PromptAttachment[];
}

export interface ForkResult {
  sessionId: string;
  title?: string;
  parentId?: string;
}

export interface RevertResult {
  removed: number;
  snapshotId?: string;
}

export interface STTResult {
  text: string;
  provider?: string;
  model?: string;
}

const App = GoApp as unknown as {
  Bootstrap(): Promise<BootstrapInfo>;
  Platform(): Promise<string>;
  Log(message: string): Promise<void>;
  Notify(title: string, body: string): Promise<void>;
  SearchFiles(query: string, limit: number): Promise<string[]>;
  SettingsSnapshot(): Promise<SettingsSnapshot>;
  SlashCommands(): Promise<SlashCommand[]>;
  ProviderCatalog(): Promise<ProviderCatalogEntry[]>;
  AddProvider(name: string, apiKey: string, baseURL: string): Promise<void>;
  RemoveProvider(name: string): Promise<void>;
  AddSTTProvider(name: string, apiKey: string, baseURL: string): Promise<void>;
  RemoveSTTProvider(name: string): Promise<void>;
  SaveGeneralSettings(g: GeneralSettings): Promise<void>;
  SaveCompactionSettings(c: CompactionSettings): Promise<void>;
  SavePermissionSettings(p: PermissionsSettings): Promise<void>;
  SaveMemorySettings(m: MemorySettings): Promise<void>;
  TogglePluginSettings(name: string): Promise<boolean>;
  SetReasoningEffort(sessionId: string, provider: string, model: string, effort: string): Promise<SessionInfo>;
  Version(): Promise<string>;
  LogPath(): Promise<string>;
  BinaryStatus(): Promise<BinaryStatus>;
  FindBinary(): Promise<BinaryStatus>;
  SetBinaryPath(path: string): Promise<BinaryStatus>;
  PickBinary(): Promise<string>;
  InstallBinary(): Promise<BinaryStatus>;
  PickWorkspace(): Promise<string>;
  OpenWorkspace(path: string): Promise<string>;
  SavePrefs(p: Prefs): Promise<Prefs>;
  SetTheme(name: string, mode: string): Promise<Prefs>;
  ApplyWindowTheme(dark: boolean): Promise<void>;
  Themes(): Promise<Theme[]>;
  Recents(): Promise<Recent[]>;
  WindowMinimise(): Promise<void>;
  WindowToggleMaximise(): Promise<void>;
  WindowClose(): Promise<void>;
  WindowMaximised(): Promise<boolean>;
  OpenExternal(url: string): Promise<void>;
  RevealPath(path: string): Promise<void>;
  ReadAttachment(path: string): Promise<Attachment>;
  AttachFiles(): Promise<Attachment[]>;
  NewSession(input: NewSessionInput): Promise<SessionInfo>;
  ResetSession(): Promise<void>;
  LoadSession(input: LoadSessionInput): Promise<SessionInfo>;
  ListSessions(cwd: string, all: boolean): Promise<SessionListResult>;
  SendPrompt(input: PromptInput): Promise<PromptResult>;
  Cancel(sessionId: string): Promise<void>;
  SetMode(sessionId: string, mode: string): Promise<SessionInfo>;
  SetModel(sessionId: string, provider: string, model: string): Promise<SessionInfo>;
  ListModels(provider: string): Promise<ModelListResult>;
  RenameSession(sessionId: string, title: string): Promise<void>;
  DeleteSession(sessionId: string): Promise<void>;
  ForkSession(sessionId: string, messageId: string, userMessageIndex: number): Promise<ForkResult>;
  RevertSession(sessionId: string, messageId: string, userMessageIndex: number): Promise<RevertResult>;
  RecordStart(): Promise<void>;
  RecordStop(): Promise<STTResult>;
  RespondPermission(requestId: string, optionId: string): Promise<void>;
  RespondAsk(requestId: string, answers: Record<string, string>): Promise<void>;
  ConnectSettings(): Promise<ConnectSettings>;
  SaveConnectSettings(settings: ConnectSettings): Promise<ConnectSettings>;
  ConnectStart(): Promise<ConnectStatus>;
  ConnectStop(): Promise<ConnectStatus>;
  ConnectLogout(): Promise<ConnectSettings>;
  ConnectResetDevice(): Promise<ConnectSettings>;
  TeamStatus(): Promise<TeamStatus>;
  TeamConnect(): Promise<TeamStatus>;
  TeamCancel(): Promise<TeamStatus>;
  TeamRefresh(provider: string): Promise<TeamStatus>;
  TeamRemove(provider: string): Promise<TeamStatus>;
};

export const api = App;

export const hasWails = typeof window !== 'undefined' && typeof (window as any).go !== 'undefined';

type Unsubscribe = () => void;

function on<T>(event: string, cb: (payload: T) => void): Unsubscribe {
  if (!hasWails) return () => undefined;
  return EventsOn(event, (payload: T) => cb(payload));
}

export function onUpdate(cb: (sessionId: string, updates: unknown[]) => void): Unsubscribe {
  return on<string>('acp:update', (raw) => {
    try {
      const payload = JSON.parse(raw) as { sessionId: string; updates: unknown[] };
      cb(payload.sessionId, payload.updates ?? []);
    } catch {
      /* ignore malformed frame */
    }
  });
}

export function onPermission(cb: (req: PermissionRequest) => void): Unsubscribe {
  return on<PermissionRequest>('acp:permission', cb);
}

export function onAsk(cb: (req: AskRequest) => void): Unsubscribe {
  return on<AskRequest>('acp:ask', cb);
}

export function onStatus(cb: (status: AcpStatus) => void): Unsubscribe {
  return on<AcpStatus>('acp:status', cb);
}

export function onLoading(cb: (payload: { sessionId: string; loading: boolean }) => void): Unsubscribe {
  return on('acp:loading', cb);
}

export function onPendingClear(cb: () => void): Unsubscribe {
  return on('acp:pending-clear', cb);
}

export function onBinaryProgress(cb: (progress: BinaryProgress) => void): Unsubscribe {
  return on<BinaryProgress>('binary:progress', cb);
}

export function onFileDrop(cb: (paths: string[]) => void): Unsubscribe {
  if (!hasWails) return () => undefined;
  return EventsOn('wails:file-drop', (_x: number, _y: number, paths: string[]) => {
    if (Array.isArray(paths) && paths.length) cb(paths);
  });
}

export function onConnectStatus(cb: (status: ConnectStatus) => void): Unsubscribe {
  return on<ConnectStatus>('connect:status', cb);
}

export function onTeamStatus(cb: (status: TeamStatus) => void): Unsubscribe {
  return on<TeamStatus>('team:status', cb);
}

export function offAll(events: string[]): void {
  if (!hasWails) return;
  for (const event of events) EventsOff(event);
}
