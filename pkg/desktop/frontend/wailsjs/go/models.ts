export namespace acpclient {
	
	export class Cost {
	    amount: number;
	    currency: string;
	
	    static createFrom(source: any = {}) {
	        return new Cost(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.amount = source["amount"];
	        this.currency = source["currency"];
	    }
	}
	export class ModelInfo {
	    id: string;
	    name?: string;
	    owned_by?: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.owned_by = source["owned_by"];
	    }
	}
	export class ModelEntry {
	    provider: string;
	    model: ModelInfo;
	
	    static createFrom(source: any = {}) {
	        return new ModelEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.model = this.convertValues(source["model"], ModelInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ModelListResult {
	    models: ModelEntry[];
	
	    static createFrom(source: any = {}) {
	        return new ModelListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.models = this.convertValues(source["models"], ModelEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PeggCapabilities {
	    ask?: boolean;
	    permissions?: boolean;
	    models?: boolean;
	    commands?: boolean;
	    attachments?: boolean;
	    diffs?: boolean;
	    plan?: boolean;
	    history?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PeggCapabilities(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ask = source["ask"];
	        this.permissions = source["permissions"];
	        this.models = source["models"];
	        this.commands = source["commands"];
	        this.attachments = source["attachments"];
	        this.diffs = source["diffs"];
	        this.plan = source["plan"];
	        this.history = source["history"];
	    }
	}
	export class STTResult {
	    text: string;
	    provider?: string;
	    model?: string;
	
	    static createFrom(source: any = {}) {
	        return new STTResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	    }
	}
	export class SessionForkResult {
	    sessionId: string;
	    title?: string;
	    parentId?: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionForkResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.title = source["title"];
	        this.parentId = source["parentId"];
	    }
	}
	export class SessionHistoryResult {
	    oldestSeq: number;
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SessionHistoryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.oldestSeq = source["oldestSeq"];
	        this.hasMore = source["hasMore"];
	    }
	}
	export class SessionUsage {
	    prompt_tokens?: number;
	    completion_tokens?: number;
	    total_tokens?: number;
	    cost?: number;
	
	    static createFrom(source: any = {}) {
	        return new SessionUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prompt_tokens = source["prompt_tokens"];
	        this.completion_tokens = source["completion_tokens"];
	        this.total_tokens = source["total_tokens"];
	        this.cost = source["cost"];
	    }
	}
	export class SessionListItem {
	    id: string;
	    title: string;
	    projectDir?: string;
	    provider?: string;
	    model?: string;
	    mode?: string;
	    usage?: SessionUsage;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionListItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.projectDir = source["projectDir"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.mode = source["mode"];
	        this.usage = this.convertValues(source["usage"], SessionUsage);
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SessionListResult {
	    sessions: SessionListItem[];
	    nextCursor?: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessions = this.convertValues(source["sessions"], SessionListItem);
	        this.nextCursor = source["nextCursor"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Usage {
	    used: number;
	    size: number;
	    cost?: Cost;
	    total_tokens?: number;
	
	    static createFrom(source: any = {}) {
	        return new Usage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.used = source["used"];
	        this.size = source["size"];
	        this.cost = this.convertValues(source["cost"], Cost);
	        this.total_tokens = source["total_tokens"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SessionPromptResult {
	    stopReason: string;
	    usage?: Usage;
	
	    static createFrom(source: any = {}) {
	        return new SessionPromptResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stopReason = source["stopReason"];
	        this.usage = this.convertValues(source["usage"], Usage);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SessionRevertResult {
	    removed: number;
	    snapshotId?: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionRevertResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.removed = source["removed"];
	        this.snapshotId = source["snapshotId"];
	    }
	}
	
	export class SlashCommand {
	    name: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new SlashCommand(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	    }
	}

}

export namespace binary {
	
	export class Status {
	    found: boolean;
	    path?: string;
	    version?: string;
	    source?: string;
	    managed?: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.found = source["found"];
	        this.path = source["path"];
	        this.version = source["version"];
	        this.source = source["source"];
	        this.managed = source["managed"];
	        this.error = source["error"];
	    }
	}

}

export namespace main {
	
	export class BootstrapInfo {
	    platform: string;
	    arch: string;
	    version: string;
	    home: string;
	    prefs: prefs.Prefs;
	    themes: themes.Theme[];
	    binary: binary.Status;
	    defaultWorkspace: string;
	    workspace: string;
	    logPath: string;
	    language: string;
	    nativeTitlebar: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BootstrapInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.platform = source["platform"];
	        this.arch = source["arch"];
	        this.version = source["version"];
	        this.home = source["home"];
	        this.prefs = this.convertValues(source["prefs"], prefs.Prefs);
	        this.themes = this.convertValues(source["themes"], themes.Theme);
	        this.binary = this.convertValues(source["binary"], binary.Status);
	        this.defaultWorkspace = source["defaultWorkspace"];
	        this.workspace = source["workspace"];
	        this.logPath = source["logPath"];
	        this.language = source["language"];
	        this.nativeTitlebar = source["nativeTitlebar"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CompactionSettings {
	    enabled: boolean;
	    strategies: string[];
	    allStrategies: string[];
	    threshold: number;
	    maxMessages: number;
	    maxToolOutputChars: number;
	
	    static createFrom(source: any = {}) {
	        return new CompactionSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.strategies = source["strategies"];
	        this.allStrategies = source["allStrategies"];
	        this.threshold = source["threshold"];
	        this.maxMessages = source["maxMessages"];
	        this.maxToolOutputChars = source["maxToolOutputChars"];
	    }
	}
	export class ConnectStatus {
	    state: string;
	    pid?: number;
	    startedAt?: string;
	    error?: string;
	    logs: string[];
	
	    static createFrom(source: any = {}) {
	        return new ConnectStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.pid = source["pid"];
	        this.startedAt = source["startedAt"];
	        this.error = source["error"];
	        this.logs = source["logs"];
	    }
	}
	export class ConnectSettings {
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
	
	    static createFrom(source: any = {}) {
	        return new ConnectSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.relayUrl = source["relayUrl"];
	        this.token = source["token"];
	        this.clearToken = source["clearToken"];
	        this.tokenMasked = source["tokenMasked"];
	        this.hasToken = source["hasToken"];
	        this.deviceName = source["deviceName"];
	        this.deviceId = source["deviceId"];
	        this.publicKey = source["publicKey"];
	        this.autoApprove = source["autoApprove"];
	        this.maxConcurrency = source["maxConcurrency"];
	        this.responseTimeout = source["responseTimeout"];
	        this.allowedMethods = source["allowedMethods"];
	        this.status = this.convertValues(source["status"], ConnectStatus);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class STTProviderEntry {
	    name: string;
	    configured: boolean;
	    models: string[];
	
	    static createFrom(source: any = {}) {
	        return new STTProviderEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.configured = source["configured"];
	        this.models = source["models"];
	    }
	}
	export class STTSettings {
	    providers: string[];
	    configured: string[];
	    provider: string;
	    model: string;
	    models: string[];
	    modelsByProvider: Record<string, Array<string>>;
	    entries: STTProviderEntry[];
	    recordCommand: string;
	    maxDuration: number;
	
	    static createFrom(source: any = {}) {
	        return new STTSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.providers = source["providers"];
	        this.configured = source["configured"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.models = source["models"];
	        this.modelsByProvider = source["modelsByProvider"];
	        this.entries = this.convertValues(source["entries"], STTProviderEntry);
	        this.recordCommand = source["recordCommand"];
	        this.maxDuration = source["maxDuration"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ModelOption {
	    id: string;
	    name?: string;
	    reasoningEfforts?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ModelOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.reasoningEfforts = source["reasoningEfforts"];
	    }
	}
	export class ProviderOption {
	    name: string;
	    driver?: string;
	    models: ModelOption[];
	
	    static createFrom(source: any = {}) {
	        return new ProviderOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.driver = source["driver"];
	        this.models = this.convertValues(source["models"], ModelOption);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LanguageOption {
	    code: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new LanguageOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.label = source["label"];
	    }
	}
	export class GeneralSettings {
	    provider: string;
	    model: string;
	    reasoningEffort: string;
	    language: string;
	    languages: LanguageOption[];
	    providers: ProviderOption[];
	    stt: STTSettings;
	
	    static createFrom(source: any = {}) {
	        return new GeneralSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.reasoningEffort = source["reasoningEffort"];
	        this.language = source["language"];
	        this.languages = this.convertValues(source["languages"], LanguageOption);
	        this.providers = this.convertValues(source["providers"], ProviderOption);
	        this.stt = this.convertValues(source["stt"], STTSettings);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HubSkill {
	    name: string;
	    skillId: string;
	    source: string;
	    url: string;
	    license?: string;
	    official: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HubSkill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.skillId = source["skillId"];
	        this.source = source["source"];
	        this.url = source["url"];
	        this.license = source["license"];
	        this.official = source["official"];
	    }
	}
	export class HubSearchResult {
	    items: HubSkill[];
	    nextCursor?: number;
	
	    static createFrom(source: any = {}) {
	        return new HubSearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], HubSkill);
	        this.nextCursor = source["nextCursor"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class LoadHistoryRequest {
	    sessionId: string;
	    cwd?: string;
	    beforeSeq: number;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new LoadHistoryRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.cwd = source["cwd"];
	        this.beforeSeq = source["beforeSeq"];
	        this.limit = source["limit"];
	    }
	}
	export class LoadSessionRequest {
	    peggId: string;
	    cwd?: string;
	    title?: string;
	    mode?: string;
	    provider?: string;
	    model?: string;
	
	    static createFrom(source: any = {}) {
	        return new LoadSessionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.peggId = source["peggId"];
	        this.cwd = source["cwd"];
	        this.title = source["title"];
	        this.mode = source["mode"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	    }
	}
	export class MCPServerInput {
	    name: string;
	    command: string;
	    args: string[];
	    env: Record<string, string>;
	    url: string;
	    headers: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new MCPServerInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.env = source["env"];
	        this.url = source["url"];
	        this.headers = source["headers"];
	    }
	}
	export class MCPServerSetting {
	    name: string;
	    type: string;
	    command?: string;
	    args?: string[];
	    url?: string;
	    env: number;
	    headers: number;
	
	    static createFrom(source: any = {}) {
	        return new MCPServerSetting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.url = source["url"];
	        this.env = source["env"];
	        this.headers = source["headers"];
	    }
	}
	export class MemorySettings {
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
	
	    static createFrom(source: any = {}) {
	        return new MemorySettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.gate = source["gate"];
	        this.gates = source["gates"];
	        this.threshold = source["threshold"];
	        this.contextBudget = source["contextBudget"];
	        this.maxResults = source["maxResults"];
	        this.consolidate = source["consolidate"];
	        this.entries = source["entries"];
	        this.lastEntry = source["lastEntry"];
	    }
	}
	
	export class NewSessionRequest {
	    cwd: string;
	    mode?: string;
	    provider?: string;
	    model?: string;
	    reasoningEffort?: string;
	
	    static createFrom(source: any = {}) {
	        return new NewSessionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cwd = source["cwd"];
	        this.mode = source["mode"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.reasoningEffort = source["reasoningEffort"];
	    }
	}
	export class ToolPermission {
	    name: string;
	    mode: string;
	    group: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolPermission(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.group = source["group"];
	    }
	}
	export class PermissionsSettings {
	    default: string;
	    modes: string[];
	    tools: ToolPermission[];
	    judgeProviders: string[];
	    judgeProvider: string;
	    judgeModel: string;
	    judgeThreshold: number;
	    defaults: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new PermissionsSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.default = source["default"];
	        this.modes = source["modes"];
	        this.tools = this.convertValues(source["tools"], ToolPermission);
	        this.judgeProviders = source["judgeProviders"];
	        this.judgeProvider = source["judgeProvider"];
	        this.judgeModel = source["judgeModel"];
	        this.judgeThreshold = source["judgeThreshold"];
	        this.defaults = source["defaults"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PluginSetting {
	    name: string;
	    path: string;
	    enabled: boolean;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new PluginSetting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.enabled = source["enabled"];
	        this.status = source["status"];
	    }
	}
	export class PromptAttachment {
	    name: string;
	    mediaType: string;
	    data?: string;
	    text?: string;
	    path?: string;
	    size?: number;
	
	    static createFrom(source: any = {}) {
	        return new PromptAttachment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mediaType = source["mediaType"];
	        this.data = source["data"];
	        this.text = source["text"];
	        this.path = source["path"];
	        this.size = source["size"];
	    }
	}
	export class PromptRequest {
	    sessionId: string;
	    text: string;
	    attachments?: PromptAttachment[];
	
	    static createFrom(source: any = {}) {
	        return new PromptRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.text = source["text"];
	        this.attachments = this.convertValues(source["attachments"], PromptAttachment);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProviderCatalogEntry {
	    name: string;
	    configured: boolean;
	    kind: string;
	    hint: string;
	
	    static createFrom(source: any = {}) {
	        return new ProviderCatalogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.configured = source["configured"];
	        this.kind = source["kind"];
	        this.hint = source["hint"];
	    }
	}
	
	export class RuleSetting {
	    name: string;
	    source: string;
	    path: string;
	    preview: string;
	
	    static createFrom(source: any = {}) {
	        return new RuleSetting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.source = source["source"];
	        this.path = source["path"];
	        this.preview = source["preview"];
	    }
	}
	
	
	export class SessionInfo {
	    acpId: string;
	    peggId: string;
	    cwd: string;
	    title: string;
	    mode: string;
	    provider: string;
	    model: string;
	    permissionMode: string;
	    reasoningEffort?: string;
	    commands: acpclient.SlashCommand[];
	    historyOldestSeq?: number;
	    historyHasMore?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.acpId = source["acpId"];
	        this.peggId = source["peggId"];
	        this.cwd = source["cwd"];
	        this.title = source["title"];
	        this.mode = source["mode"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.permissionMode = source["permissionMode"];
	        this.reasoningEffort = source["reasoningEffort"];
	        this.commands = this.convertValues(source["commands"], acpclient.SlashCommand);
	        this.historyOldestSeq = source["historyOldestSeq"];
	        this.historyHasMore = source["historyHasMore"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SessionSettings {
	    acpId: string;
	    peggId: string;
	    title: string;
	    cwd: string;
	    mode: string;
	    provider: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.acpId = source["acpId"];
	        this.peggId = source["peggId"];
	        this.title = source["title"];
	        this.cwd = source["cwd"];
	        this.mode = source["mode"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	    }
	}
	export class SkillSetting {
	    name: string;
	    description: string;
	    source: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new SkillSetting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.source = source["source"];
	        this.path = source["path"];
	    }
	}
	export class SystemSettings {
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
	
	    static createFrom(source: any = {}) {
	        return new SystemSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appVersion = source["appVersion"];
	        this.agentVersion = source["agentVersion"];
	        this.platform = source["platform"];
	        this.arch = source["arch"];
	        this.binaryPath = source["binaryPath"];
	        this.binaryVersion = source["binaryVersion"];
	        this.binarySource = source["binarySource"];
	        this.configPath = source["configPath"];
	        this.configDir = source["configDir"];
	        this.projectDir = source["projectDir"];
	        this.logPath = source["logPath"];
	        this.memoryDir = source["memoryDir"];
	        this.skillsDir = source["skillsDir"];
	        this.rulesDir = source["rulesDir"];
	        this.pluginsDir = source["pluginsDir"];
	    }
	}
	export class SettingsSnapshot {
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
	
	    static createFrom(source: any = {}) {
	        return new SettingsSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.general = this.convertValues(source["general"], GeneralSettings);
	        this.session = this.convertValues(source["session"], SessionSettings);
	        this.compaction = this.convertValues(source["compaction"], CompactionSettings);
	        this.permissions = this.convertValues(source["permissions"], PermissionsSettings);
	        this.memory = this.convertValues(source["memory"], MemorySettings);
	        this.system = this.convertValues(source["system"], SystemSettings);
	        this.pluginsEnabled = source["pluginsEnabled"];
	        this.mcp = this.convertValues(source["mcp"], MCPServerSetting);
	        this.skills = this.convertValues(source["skills"], SkillSetting);
	        this.rules = this.convertValues(source["rules"], RuleSetting);
	        this.plugins = this.convertValues(source["plugins"], PluginSetting);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SkillMutationResult {
	    path?: string;
	    commands?: acpclient.SlashCommand[];
	
	    static createFrom(source: any = {}) {
	        return new SkillMutationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.commands = this.convertValues(source["commands"], acpclient.SlashCommand);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class TeamProvider {
	    name: string;
	    baseUrl: string;
	    apiKeyMasked: string;
	
	    static createFrom(source: any = {}) {
	        return new TeamProvider(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.baseUrl = source["baseUrl"];
	        this.apiKeyMasked = source["apiKeyMasked"];
	    }
	}
	export class TeamStatus {
	    state: string;
	    url?: string;
	    error?: string;
	    logs: string[];
	    providers: TeamProvider[];
	
	    static createFrom(source: any = {}) {
	        return new TeamStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.url = source["url"];
	        this.error = source["error"];
	        this.logs = source["logs"];
	        this.providers = this.convertValues(source["providers"], TeamProvider);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace prefs {
	
	export class Window {
	    width?: number;
	    height?: number;
	    x?: number;
	    y?: number;
	
	    static createFrom(source: any = {}) {
	        return new Window(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	}
	export class Recent {
	    path: string;
	    name: string;
	    // Go type: time
	    openedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Recent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.openedAt = this.convertValues(source["openedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Prefs {
	    theme: string;
	    themeMode: string;
	    workspace: string;
	    recents: Recent[];
	    binaryPath: string;
	    autoDownload: boolean;
	    sendOnEnter: boolean;
	    fontScale: number;
	    sidebarCollapsed: boolean;
	    lastSession: string;
	    window: Window;
	
	    static createFrom(source: any = {}) {
	        return new Prefs(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.themeMode = source["themeMode"];
	        this.workspace = source["workspace"];
	        this.recents = this.convertValues(source["recents"], Recent);
	        this.binaryPath = source["binaryPath"];
	        this.autoDownload = source["autoDownload"];
	        this.sendOnEnter = source["sendOnEnter"];
	        this.fontScale = source["fontScale"];
	        this.sidebarCollapsed = source["sidebarCollapsed"];
	        this.lastSession = source["lastSession"];
	        this.window = this.convertValues(source["window"], Window);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace themes {
	
	export class Theme {
	    name: string;
	    label: string;
	    dark: boolean;
	    tokens: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Theme(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.label = source["label"];
	        this.dark = source["dark"];
	        this.tokens = source["tokens"];
	    }
	}

}

export namespace workspace {
	
	export class Attachment {
	    id: string;
	    name: string;
	    mediaType: string;
	    size: number;
	    path?: string;
	    data?: string;
	    text?: string;
	
	    static createFrom(source: any = {}) {
	        return new Attachment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.mediaType = source["mediaType"];
	        this.size = source["size"];
	        this.path = source["path"];
	        this.data = source["data"];
	        this.text = source["text"];
	    }
	}

}

