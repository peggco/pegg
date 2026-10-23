package dev.pegg.intellij.chat

import com.intellij.notification.NotificationAction
import com.intellij.notification.NotificationGroupManager
import com.intellij.notification.NotificationType
import com.intellij.notification.Notifications
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.Messages
import com.intellij.openapi.wm.ToolWindowManager
import dev.pegg.intellij.acp.ACP_JSON
import dev.pegg.intellij.acp.AcpClient
import dev.pegg.intellij.acp.AskParams
import dev.pegg.intellij.acp.ChatItem
import dev.pegg.intellij.acp.ChatState
import dev.pegg.intellij.acp.ContentBlock
import dev.pegg.intellij.acp.InitializeResult
import dev.pegg.intellij.acp.ModelListResult
import dev.pegg.intellij.acp.PeggCapabilities
import dev.pegg.intellij.acp.PendingInteraction
import dev.pegg.intellij.acp.PermissionOption
import dev.pegg.intellij.acp.PermissionParams
import dev.pegg.intellij.acp.PromptResult
import dev.pegg.intellij.acp.SessionForkResult
import dev.pegg.intellij.acp.SessionListItem
import dev.pegg.intellij.acp.SessionListResult
import dev.pegg.intellij.acp.SessionNewResult
import dev.pegg.intellij.acp.SessionUpdate
import dev.pegg.intellij.acp.UsageState
import dev.pegg.intellij.acp.VoiceResult
import dev.pegg.intellij.acp.textOf
import dev.pegg.intellij.binary.BinaryManager
import dev.pegg.intellij.binary.searchDirs
import dev.pegg.intellij.log.PeggLog
import dev.pegg.intellij.settings.PeggProjectState
import dev.pegg.intellij.settings.PeggSettings
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonArray
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.encodeToJsonElement
import kotlinx.serialization.json.put
import java.io.File
import java.util.concurrent.CompletableFuture
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.TimeUnit

class ChatController(
    private val project: Project,
    private val binaryManager: BinaryManager,
    private val scope: CoroutineScope,
) {
    @Volatile
    var state: ChatState = ChatState()
        private set

    private val stateLock = Any()
    private val startMutex = Mutex()
    private val sessionMutex = Mutex()
    private val stateListeners = CopyOnWriteArrayList<(ChatState) -> Unit>()
    private val sessionsListeners = CopyOnWriteArrayList<() -> Unit>()
    private val composerTextListeners = CopyOnWriteArrayList<(String) -> Unit>()
    private val runFinishedListeners = CopyOnWriteArrayList<(ChatState) -> Unit>()

    private var client: AcpClient? = null
    @Volatile private var capabilities: dev.pegg.intellij.acp.AgentCapabilities? = null
    @Volatile private var started = false
    @Volatile private var disposed = false
    @Volatile private var legacyStreaming = false
    @Volatile private var loadingSession = false
    private var voiceStarting = false
    private var forceFallback = false
    private val resolvers = ConcurrentHashMap<String, CompletableFuture<JsonElement>>()

    private var chatVisible: () -> Boolean = { false }

    fun addStateListener(listener: (ChatState) -> Unit) = stateListeners.addIfAbsent(listener)
    fun removeStateListener(listener: (ChatState) -> Unit) = stateListeners.remove(listener)
    fun addSessionsListener(listener: () -> Unit) = sessionsListeners.addIfAbsent(listener)
    fun removeSessionsListener(listener: () -> Unit) = sessionsListeners.remove(listener)
    fun addComposerTextListener(listener: (String) -> Unit) = composerTextListeners.addIfAbsent(listener)
    fun removeComposerTextListener(listener: (String) -> Unit) = composerTextListeners.remove(listener)
    fun addRunFinishedListener(listener: (ChatState) -> Unit) = runFinishedListeners.addIfAbsent(listener)
    fun removeRunFinishedListener(listener: (ChatState) -> Unit) = runFinishedListeners.remove(listener)

    fun setChatVisible(provider: () -> Boolean) {
        chatVisible = provider
    }

    fun setForceFallback(value: Boolean) {
        forceFallback = value
    }

    val historySupported: Boolean
        get() = capabilities?.pegg?.history == true

    fun snapshotState(): ChatState = synchronized(stateLock) {
        state.historySupported = historySupported
        ChatStateOps.snapshot(state)
    }

    suspend fun start() {
        ensureStarted()
        sessionMutex.withLock {
            if (state.acpSessionId != null) return
            val stored = state.peggSessionId
                ?: PeggProjectState.getInstance(project).state.peggSessionId
            if (stored != null) {
                val resumed = runCatching { resumeSessionInternal(stored) }
                    .onFailure { PeggLog.warn("Could not restore the stored session: ${it.message}") }
                    .isSuccess
                if (!resumed) {
                    PeggProjectState.getInstance(project).save(null, null)
                    synchronized(stateLock) {
                        state = ChatState(
                            connected = true,
                            mode = PeggSettings.getInstance().state.defaultMode,
                        )
                    }
                    emitState()
                }
            }
        }
    }

    suspend fun activate() {
        ensureStarted()
        sessionMutex.withLock {
            if (state.acpSessionId != null) return
            val peggId = state.peggSessionId
                ?: PeggProjectState.getInstance(project).state.peggSessionId
            if (peggId != null) {
                val resumed = runCatching { resumeSessionInternal(peggId) }
                    .onFailure { PeggLog.warn("Could not resume session after restart: ${it.message}") }
                    .isSuccess
                if (resumed) return
            }
            newSessionInternal()
        }
    }

    suspend fun ensureSession() {
        ensureStarted()
        if (state.acpSessionId == null) activate()
    }

    suspend fun newSession() {
        ensureStarted()
        sessionMutex.withLock { newSessionInternal() }
    }

    private suspend fun newSessionInternal() {
        val root = projectRoot() ?: throw IllegalStateException("Open a folder to use Pegg.")
        val settings = PeggSettings.getInstance().state
        val previous = state
        synchronized(stateLock) {
            state = ChatState(mode = settings.defaultMode, connected = true)
        }
        emitState()
        try {
            val result = request<SessionNewResult>(
                "session/new",
                buildJsonObject { put("cwd", root) },
                60_000,
            )
            synchronized(stateLock) {
                state.acpSessionId = result.sessionId
                state.peggSessionId = result.peggSessionId
                state.mode = result.mode ?: settings.defaultMode
                state.provider = result.provider
                state.model = result.model
                state.permissionMode = result.permissionMode
                state.commands = (result.commands ?: emptyList()).toMutableList()
                state.title = "New session"
                if (legacyStreaming) {
                    ChatStateOps.addNotice(
                        state,
                        "Using an older Pegg binary. Update it for streaming tool details and interactive approvals.",
                        "warn",
                    )
                }
            }
            emitState()
            saveSession()
            emitSessions()
            if (
                settings.provider.isNotBlank() && settings.model.isNotBlank() &&
                capabilities?.pegg?.models == true &&
                (settings.provider != result.provider || settings.model != result.model)
            ) {
                runCatching { setModel(settings.provider, settings.model) }
                    .onFailure { PeggLog.warn("Could not apply preferred model: ${it.message}") }
            }
        } catch (e: Throwable) {
            synchronized(stateLock) {
                state = previous
                ChatStateOps.addNotice(state, e.message ?: e.javaClass.simpleName, "error")
            }
            emitState()
            throw e
        }
    }

    suspend fun resumeSession(peggSessionId: String) {
        ensureStarted()
        sessionMutex.withLock { resumeSessionInternal(peggSessionId) }
    }

    private suspend fun resumeSessionInternal(peggSessionId: String) {
        val root = projectRoot() ?: throw IllegalStateException("Open a folder to use Pegg.")
        synchronized(stateLock) {
            state = ChatState(
                acpSessionId = peggSessionId,
                peggSessionId = peggSessionId,
                connected = true,
            )
        }
        emitState()
        loadingSession = true
        try {
            request<JsonElement>(
                "session/load",
                buildJsonObject {
                    put("sessionId", peggSessionId)
                    put("cwd", root)
                },
                60_000,
            )
        } finally {
            loadingSession = false
        }
        val meta = runCatching { listSessions(false) }.getOrDefault(emptyList())
            .firstOrNull { it.id == peggSessionId }
        synchronized(stateLock) {
            state.title = meta?.title ?: "Session"
            state.provider = meta?.provider ?: state.provider
            state.model = meta?.model ?: state.model
            state.mode = meta?.mode ?: state.mode
            if (meta?.usage != null) {
                state.usage = UsageState(
                    used = (meta.usage.totalTokens ?: 0).toInt(),
                    size = 0,
                    cost = meta.usage.cost,
                )
            }
            if (legacyStreaming) {
                ChatStateOps.addNotice(
                    state,
                    "Using an older Pegg binary. Update it for streaming tool details and interactive approvals.",
                    "warn",
                )
            }
        }
        emitState()
        saveSession()
        emitSessions()
    }

    suspend fun listSessions(all: Boolean = false): List<SessionListItem> {
        ensureStarted()
        val root = projectRoot()
        val params = buildJsonObject {
            if (root != null) put("cwd", root)
            put("all", all)
            put("limit", 100)
        }
        val result = request<SessionListResult>("session/list", params, 60_000)
        return result.sessions
    }

    suspend fun deleteSession(peggSessionId: String) {
        ensureStarted()
        if (state.peggSessionId == peggSessionId) {
            val acpId = state.acpSessionId
            if (acpId != null) {
                runCatching {
                    request<JsonElement>("session/close", buildJsonObject { put("sessionId", acpId) }, 30_000)
                }
            }
        }
        request<JsonElement>("session/delete", buildJsonObject { put("sessionId", peggSessionId) }, 60_000)
        if (state.peggSessionId == peggSessionId) newSession()
        emitSessions()
    }

    suspend fun renameSession(peggSessionId: String, title: String) {
        ensureStarted()
        val sessionId = if (state.peggSessionId == peggSessionId) state.acpSessionId else peggSessionId
        request<JsonElement>(
            "session/rename",
            buildJsonObject {
                put("sessionId", sessionId ?: peggSessionId)
                put("title", title)
            },
            60_000,
        )
        if (state.peggSessionId == peggSessionId) {
            synchronized(stateLock) { state.title = title }
            emitState()
        }
        emitSessions()
    }

    suspend fun sendPrompt(
        text: String,
        context: String? = null,
        attachments: List<dev.pegg.intellij.acp.ComposerAttachment>? = null,
        includeActiveContext: Boolean = true,
    ) {
        ensureSession()
        val selected = attachments ?: state.composerAttachments
        if (text.isBlank() && selected.isEmpty()) return
        val settings = PeggSettings.getInstance().state
        val display = text.trim().ifEmpty { "(attachments)" }
        synchronized(stateLock) {
            if (state.running) throw IllegalStateException("Pegg is already working.")
            ChatStateOps.appendUser(state, display)
            state.running = true
            state.error = null
        }
        emitState()

        val blocks = mutableListOf<ContentBlock>()
        val promptParts = mutableListOf<String>()
        if (text.isNotBlank()) promptParts.add(text)
        val activeContext = context ?: if (includeActiveContext) {
            runCatching { ContextBuilder.activeContext(project) }.getOrDefault("")
        } else {
            ""
        }
        if (activeContext.isNotBlank()) promptParts.add(activeContext)
        if (promptParts.isNotEmpty()) blocks.add(ContentBlock("text", text = promptParts.joinToString("\n\n")))
        for (attachment in selected) {
            runCatching { Attachments.attachmentToBlock(attachment) }
                .onSuccess { block -> if (block != null) blocks.add(block) }
                .onFailure {
                    PeggLog.warn("Failed to attach file ${attachment.name}: ${it.message}")
                    blocks.add(ContentBlock("text", text = "[Could not read attachment ${attachment.name}]"))
                }
        }
        synchronized(stateLock) { ChatStateOps.clearAttachments(state) }
        emitState()

        try {
            val prompt = JsonArray(blocks.map { ACP_JSON.encodeToJsonElement(ContentBlock.serializer(), it) })
            val result = request<PromptResult>(
                "session/prompt",
                buildJsonObject {
                    put("sessionId", state.acpSessionId ?: "")
                    put("prompt", prompt)
                },
                24 * 60 * 60 * 1000L,
            )
            finishRun(result)
            if (settings.notifyOnComplete && !ApplicationManager.getApplication().isActive) {
                notifyComplete(lastAssistantText())
            }
        } catch (e: Throwable) {
            val message = e.message ?: e.javaClass.simpleName
            synchronized(stateLock) {
                state.running = false
                if (Regex("Pegg server (exited|stopped)", RegexOption.IGNORE_CASE).containsMatchIn(message)) {
                    state.connected = false
                } else {
                    ChatStateOps.addNotice(state, message, "error")
                }
            }
            emitState()
            throw e
        }
    }

    suspend fun cancel() {
        val acpId = state.acpSessionId
        if (client != null && acpId != null) client?.cancelPrompt(acpId)
        rejectPending("Cancelled")
        synchronized(stateLock) { state.running = false }
        emitState()
    }

    suspend fun setMode(mode: String) {
        ensureSession()
        request<JsonElement>(
            "session/set_mode",
            buildJsonObject {
                put("sessionId", state.acpSessionId ?: "")
                put("modeId", mode)
            },
            30_000,
        )
        synchronized(stateLock) { state.mode = mode }
        emitState()
    }

    suspend fun setModel(provider: String, model: String, reasoningEffort: String? = null) {
        ensureSession()
        if (capabilities?.pegg?.models != true) {
            throw IllegalStateException("This Pegg binary does not support model switching. Update the Pegg binary first.")
        }
        val params = buildJsonObject {
            put("sessionId", state.acpSessionId ?: "")
            put("provider", provider)
            put("model", model)
            if (!reasoningEffort.isNullOrBlank()) put("reasoningEffort", reasoningEffort)
        }
        request<JsonElement>("session/set_model", params, 30_000)
        synchronized(stateLock) {
            state.provider = provider
            state.model = model
        }
        emitState()
        saveSession()
    }

    suspend fun models(): List<dev.pegg.intellij.acp.ModelEntry> {
        ensureStarted()
        if (capabilities?.pegg?.models != true) {
            throw IllegalStateException("This Pegg binary does not support model listing. Update the Pegg binary first.")
        }
        return request<ModelListResult>("pegg/models", buildJsonObject { }, 60_000).models
    }

    suspend fun startVoice() {
        synchronized(this) {
            if (voiceStarting) throw IllegalStateException("A recording is already in progress.")
            voiceStarting = true
        }
        try {
            ensureStarted()
            request<JsonElement>("stt/record/start", buildJsonObject { }, 30_000)
        } catch (e: Throwable) {
            synchronized(this) { voiceStarting = false }
            throw e
        }
    }

    suspend fun stopVoice(): String {
        synchronized(this) { voiceStarting = false }
        ensureStarted()
        return request<VoiceResult>("stt/record/stop", buildJsonObject { }, 120_000).text
    }

    suspend fun transcribe(audio: String, mediaType: String, fileName: String?): String {
        ensureStarted()
        val params = buildJsonObject {
            put("audio", audio)
            put("mediaType", mediaType)
            if (!fileName.isNullOrBlank()) put("fileName", fileName)
        }
        return request<VoiceResult>("stt/transcribe", params, 120_000).text
    }

    suspend fun forkMessage(itemId: String) {
        val item = synchronized(stateLock) {
            state.items.firstOrNull { it.id == itemId && it is ChatItem.User && it.agentId == null }
        } as? ChatItem.User ?: throw IllegalStateException("Message not found")
        ensureSession()
        if (state.running) throw IllegalStateException("Wait for Pegg to finish before forking.")
        if (!historySupported) throw IllegalStateException("This Pegg binary does not support forking. Update it first.")
        val result = request<SessionForkResult>("session/fork", messageRef(item), 60_000)
        resumeSession(result.sessionId)
        composerTextListeners.forEach { it(item.text) }
        emitSessions()
    }

    suspend fun revertMessage(itemId: String) {
        val item = synchronized(stateLock) {
            state.items.firstOrNull { it.id == itemId && it is ChatItem.User && it.agentId == null }
        } as? ChatItem.User ?: throw IllegalStateException("Message not found")
        ensureSession()
        if (state.running) throw IllegalStateException("Wait for Pegg to finish before reverting.")
        if (!historySupported) throw IllegalStateException("This Pegg binary does not support reverting. Update it first.")
        request<JsonElement>("session/revert", messageRef(item), 60_000)
        synchronized(stateLock) {
            ChatStateOps.truncateFrom(state, itemId)
            state.usage = UsageState()
            state.plan.clear()
        }
        emitState()
        composerTextListeners.forEach { it(item.text) }
        emitSessions()
    }

    suspend fun restart() {
        rejectPending("Server restarting")
        binaryManager.invalidate()
        runCatching { client?.close() }
        client = null
        started = false
        synchronized(stateLock) {
            state.connected = false
            state.running = false
        }
        emitState()
        val peggId = state.peggSessionId
        ensureStarted()
        if (peggId != null) {
            runCatching { resumeSession(peggId) }
                .onFailure { newSession() }
        } else {
            newSession()
        }
    }

    fun dispose() {
        disposed = true
        rejectPending("Pegg was disposed")
        runCatching { client?.close() }
        client = null
        started = false
    }

    fun addComposerAttachment(attachment: dev.pegg.intellij.acp.ComposerAttachment) {
        synchronized(stateLock) { ChatStateOps.addAttachment(state, attachment) }
        emitState()
    }

    fun removeComposerAttachment(id: String) {
        synchronized(stateLock) { ChatStateOps.removeAttachment(state, id) }
        emitState()
    }

    fun respondPermission(id: String, optionId: String) {
        val pending = synchronized(stateLock) {
            state.pending.firstOrNull { it.id == id && it is PendingInteraction.Permission }
        } ?: return
        val future = resolvers.remove(pending.id) ?: return
        synchronized(stateLock) { state.pending.removeAll { it.id == id } }
        future.complete(permissionResult(optionId))
        emitState()
    }

    fun respondQuestion(id: String, answers: Map<String, String>) {
        val pending = synchronized(stateLock) {
            state.pending.firstOrNull { it.id == id && it is PendingInteraction.Question }
        } ?: return
        val future = resolvers.remove(pending.id) ?: return
        synchronized(stateLock) { state.pending.removeAll { it.id == id } }
        future.complete(answersResult(answers))
        emitState()
    }

    fun snapshotSessions(): List<SessionListItem> = emptyList()

    private suspend fun ensureStarted() {
        if (disposed) throw IllegalStateException("Pegg controller is disposed")
        if (started) return
        startMutex.withLock {
            if (started) return
            startClient()
        }
    }

    private suspend fun startClient() {
        val root = projectRoot() ?: throw IllegalStateException("Open a folder to use Pegg.")
        if (!isTrusted()) throw IllegalStateException("Pegg requires a trusted project.")
        val preparing = NotificationGroupManager.getInstance()
            .getNotificationGroup("Pegg")
            .createNotification("Pegg", "Preparing the Pegg binary…", NotificationType.INFORMATION)
        val showPreparing = java.util.concurrent.atomic.AtomicBoolean(true)
        scope.launch {
            kotlinx.coroutines.delay(1500)
            if (showPreparing.get()) Notifications.Bus.notify(preparing, project)
        }
        val binary = try {
            binaryManager.ensure()
        } finally {
            showPreparing.set(false)
            preparing.expire()
        }
        val env = LinkedHashMap<String, String>()
        runCatching { env["PATH"] = searchDirs().joinToString(File.pathSeparator) }
        env.putAll(PeggSettings.getInstance().state.env)
        val version = com.intellij.ide.plugins.PluginManagerCore
            .getPlugin(com.intellij.openapi.extensions.PluginId.getId("dev.pegg.intellij"))?.version ?: "0.2.1"
        val acp = AcpClient(
            cwd = File(root),
            binary = binary,
            environment = env,
            version = version,
            scope = scope,
            onNotification = ::onNotification,
            onServerRequest = ::onServerRequest,
            onExit = ::onExit,
        )
        client = acp
        try {
            acp.start()
            val init = acp.initialize()
            capabilities = init.agentCapabilities
            legacyStreaming = init.agentCapabilities?.pegg == null
            PeggLog.info("Pegg capabilities: ${init.agentCapabilities?.pegg}")
            started = true
            synchronized(stateLock) { state.connected = true }
            emitState()
            if (init.agentCapabilities?.pegg?.permissions != true) {
                PeggLog.warn("The installed Pegg CLI does not advertise interactive permission support.")
            }
        } catch (e: Throwable) {
            started = false
            client = null
            runCatching { acp.close() }
            synchronized(stateLock) {
                state.connected = false
                ChatStateOps.addNotice(state, e.message ?: e.javaClass.simpleName, "error")
            }
            emitState()
            throw e
        }
    }

    private fun onNotification(method: String, params: JsonElement?) {
        if (method != "session/update") {
            PeggLog.debug("ACP notification: $method")
            return
        }
        val obj = params as? JsonObject ?: return
        val sessionId = (obj["sessionId"] as? JsonPrimitive)?.content
        val updateElement = obj["update"] ?: return
        val update = runCatching {
            ACP_JSON.decodeFromJsonElement(SessionUpdate.serializer(), updateElement)
        }.getOrElse {
            PeggLog.warn("Invalid session update: ${it.message}")
            return
        }
        if (sessionId != null && state.acpSessionId != null && sessionId != state.acpSessionId && !loadingSession) return
        if (loadingSession && sessionId != null && state.acpSessionId != sessionId) {
            synchronized(stateLock) { state.acpSessionId = sessionId }
        }

        if (
            legacyStreaming && !loadingSession &&
            update.sessionUpdate == "agent_message_chunk" && update.agentName == null
        ) {
            val text = textOf(update.content)
            synchronized(stateLock) {
                val last = state.items.lastOrNull()
                if (text.isNotEmpty() && last is ChatItem.Assistant) {
                    state.items[state.items.lastIndex] = last.copy(text = last.text + text)
                } else {
                    ChatStateOps.applyUpdate(state, update)
                }
            }
            emitState()
            return
        }

        synchronized(stateLock) { ChatStateOps.applyUpdate(state, update) }
        emitState()
    }

    private suspend fun onServerRequest(method: String, params: JsonElement?): JsonElement {
        return when (method) {
            "session/request_permission" -> requestPermission(params)
            "pegg/ask" -> requestAnswers(params)
            else -> throw IllegalStateException("Unsupported client method: $method")
        }
    }

    private suspend fun requestPermission(params: JsonElement?): JsonElement {
        val decoded = params?.let {
            runCatching { ACP_JSON.decodeFromJsonElement(PermissionParams.serializer(), it) }.getOrNull()
        } ?: PermissionParams()
        val options = decoded.options.ifEmpty {
            listOf(
                PermissionOption("allow_once", "Allow", "allow_once"),
                PermissionOption("allow_always", "Allow All", "allow_always"),
                PermissionOption("reject_once", "Reject", "reject_once"),
                PermissionOption("reject_always", "Reject Always", "reject_always"),
            )
        }
        val id = "perm-${System.currentTimeMillis()}-${(0..999999).random()}"
        val pending = PendingInteraction.Permission(
            id = id,
            question = decoded.question ?: decoded.toolCall?.title ?: "Allow this tool call?",
            options = options,
        )
        val future = CompletableFuture<JsonElement>()
        resolvers[id] = future
        synchronized(stateLock) { state.pending.add(pending) }
        emitState()

        if (forceFallback || !chatVisible()) {
            ApplicationManager.getApplication().invokeLater {
                val choice = Messages.showDialog(
                    project,
                    pending.question,
                    "Pegg Permission",
                    options.map { it.name }.toTypedArray(),
                    0,
                    null,
                )
                val option = options.getOrNull(choice) ?: options.firstOrNull { it.kind.startsWith("reject") }
                completePermission(pending.id, option?.optionId ?: "reject_once")
            }
        }

        return try {
            withContext(Dispatchers.IO) { future.get(10, TimeUnit.MINUTES) }
        } catch (e: Throwable) {
            resolvers.remove(id)
            synchronized(stateLock) { state.pending.removeAll { it.id == id } }
            emitState()
            permissionResult("reject_once")
        }
    }

    private suspend fun requestAnswers(params: JsonElement?): JsonElement {
        val decoded = params?.let {
            runCatching { ACP_JSON.decodeFromJsonElement(AskParams.serializer(), it) }.getOrNull()
        } ?: AskParams()
        val id = "ask-${System.currentTimeMillis()}-${(0..999999).random()}"
        val pending = PendingInteraction.Question(
            id = id,
            agentName = decoded.agentName,
            questions = decoded.questions,
            answers = linkedMapOf(),
        )
        val future = CompletableFuture<JsonElement>()
        resolvers[id] = future
        synchronized(stateLock) { state.pending.add(pending) }
        emitState()

        if (forceFallback || !chatVisible()) {
            ApplicationManager.getApplication().invokeLater {
                askSequentially(pending, 0, linkedMapOf())
            }
        }

        return try {
            withContext(Dispatchers.IO) { future.get(30, TimeUnit.MINUTES) }
        } catch (e: Throwable) {
            resolvers.remove(id)
            synchronized(stateLock) { state.pending.removeAll { it.id == id } }
            emitState()
            answersResult(emptyMap())
        }
    }

    private fun askSequentially(
        pending: PendingInteraction.Question,
        index: Int,
        answers: MutableMap<String, String>,
    ) {
        if (index >= pending.questions.size) {
            val future = resolvers.remove(pending.id)
            synchronized(stateLock) { state.pending.removeAll { it.id == pending.id } }
            future?.complete(answersResult(answers))
            emitState()
            return
        }
        val question = pending.questions[index]
        val answer = when (question.type) {
            "boolean" -> {
                val yes = Messages.showYesNoDialog(project, question.question, "Pegg", null) == Messages.YES
                if (yes) "true" else "false"
            }
            "select", "multiselect" -> {
                val options = question.options ?: emptyList()
                if (question.type == "multiselect") {
                    Messages.showInputDialog(
                        project,
                        "${question.question}\nOptions: ${options.joinToString(", ")}",
                        "Pegg",
                        null,
                        "",
                        null,
                    ) ?: ""
                } else {
                    Messages.showEditableChooseDialog(
                        question.question,
                        "Pegg",
                        null,
                        options.toTypedArray(),
                        options.firstOrNull() ?: "",
                        null,
                    ) ?: ""
                }
            }
            else -> Messages.showInputDialog(
                project,
                question.question,
                "Pegg",
                null,
            ) ?: ""
        }
        answers[question.id] = answer
        askSequentially(pending, index + 1, answers)
    }

    private fun completePermission(id: String, optionId: String) {
        val future = resolvers.remove(id) ?: return
        synchronized(stateLock) { state.pending.removeAll { it.id == id } }
        future.complete(permissionResult(optionId))
        emitState()
    }

    private fun finishRun(result: PromptResult) {
        val snapshot = synchronized(stateLock) {
            ChatStateOps.finishRun(state, result)
            state.historySupported = historySupported
            ChatStateOps.snapshot(state)
        }
        emitState()
        emitSessions()
        runFinishedListeners.forEach { it(snapshot) }
    }

    private fun rejectPending(reason: String) {
        for ((id, future) in resolvers) {
            future.completeExceptionally(IllegalStateException(reason))
            resolvers.remove(id)
        }
        synchronized(stateLock) {
            if (state.pending.isNotEmpty()) {
                state.pending.clear()
            }
        }
        emitState()
    }

    private fun onExit(code: Int?) {
        started = false
        client = null
        rejectPending("Pegg server exited")
        synchronized(stateLock) {
            state.acpSessionId = null
            state.connected = false
            state.running = false
        }
        emitState()
    }

    private fun messageRef(item: ChatItem.User): JsonObject = buildJsonObject {
        put("sessionId", state.acpSessionId ?: "")
        if (!item.messageId.isNullOrBlank()) {
            put("messageId", item.messageId)
            return@buildJsonObject
        }
        val ordinal = synchronized(stateLock) { ChatStateOps.userMessageOrdinal(state, item.id) }
        if (ordinal != null) put("userMessageIndex", ordinal)
    }

    private fun permissionResult(optionId: String): JsonElement = buildJsonObject {
        put("outcome", buildJsonObject {
            put("outcome", "selected")
            put("optionId", optionId)
        })
    }

    private fun answersResult(answers: Map<String, String>): JsonElement = buildJsonObject {
        put("answers", buildJsonObject { answers.forEach { (key, value) -> put(key, value) } })
    }

    private fun lastAssistantText(): String = synchronized(stateLock) {
        state.items.asReversed().firstOrNull { it is ChatItem.Assistant }
            ?.let { (it as ChatItem.Assistant).text }
            ?: ""
    }

    private fun notifyComplete(text: String) {
        val summary = text.replace(Regex("\\s+"), " ").trim().take(140).ifBlank { "Task completed" }
        val notification = NotificationGroupManager.getInstance()
            .getNotificationGroup("Pegg")
            .createNotification("Pegg finished", summary, NotificationType.INFORMATION)
            .addAction(
                NotificationAction.createSimple("Open Chat") {
                    ToolWindowManager.getInstance(project).getToolWindow("Pegg")?.activate(null)
                },
            )
        Notifications.Bus.notify(notification, project)
    }

    private fun emitState() {
        val snapshot = synchronized(stateLock) {
            state.historySupported = historySupported
            ChatStateOps.snapshot(state)
        }
        stateListeners.forEach { runCatching { it(snapshot) } }
    }

    private fun emitSessions() = sessionsListeners.forEach { runCatching { it() } }

    private fun saveSession() {
        val current = state
        PeggProjectState.getInstance(project).save(current.acpSessionId, current.peggSessionId)
    }

    private fun projectRoot(): String? = ContextBuilder.projectRoot(project)?.path

    private fun isTrusted(): Boolean =
        runCatching { com.intellij.ide.trustedProjects.TrustedProjects.isProjectTrusted(project) }.getOrDefault(false)

    private suspend inline fun <reified T> request(method: String, params: JsonElement? = null, timeoutMs: Long): T {
        val result = client?.request(method, params, timeoutMs)
            ?: throw IllegalStateException("Pegg server is not running")
        return ACP_JSON.decodeFromJsonElement(result)
    }

    private suspend fun requestRaw(method: String, params: JsonElement? = null, timeoutMs: Long): JsonElement =
        client?.request(method, params, timeoutMs)
            ?: throw IllegalStateException("Pegg server is not running")
}
