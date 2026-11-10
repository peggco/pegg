package dev.pegg.intellij.acp

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.longOrNull
import kotlinx.serialization.json.doubleOrNull
import kotlinx.serialization.json.jsonPrimitive

val ACP_JSON: Json = Json {
    ignoreUnknownKeys = true
    explicitNulls = false
    encodeDefaults = true
    classDiscriminator = "kind"
    isLenient = true
}

fun metaString(meta: JsonObject?, key: String): String? =
    (meta?.get(key) as? JsonPrimitive)?.contentOrNull

fun metaInt(meta: JsonObject?, key: String): Int? =
    (meta?.get(key) as? JsonPrimitive)?.intOrNull

fun metaBool(meta: JsonObject?, key: String): Boolean? =
    (meta?.get(key) as? JsonPrimitive)?.booleanOrNull

fun metaAttachments(meta: JsonObject?): List<AttachmentInfo>? {
    val element = meta?.get("com.pegg/attachments") ?: return null
    return runCatching {
        ACP_JSON.decodeFromJsonElement<List<AttachmentInfo>>(element)
    }.getOrNull()
}

@Serializable
data class ContentBlock(
    val type: String,
    val text: String? = null,
    val data: String? = null,
    val mimeType: String? = null,
    val name: String? = null,
    val uri: String? = null,
    val resource: EmbeddedResource? = null,
)

@Serializable
data class AttachmentInfo(
    val name: String = "",
    val mediaType: String? = null,
    val type: String? = null,
    val size: Long? = null,
    val path: String? = null,
)

@Serializable
data class EmbeddedResource(
    val uri: String,
    val mimeType: String? = null,
    val text: String? = null,
    val blob: String? = null,
)

@Serializable
data class ToolCallLocation(val path: String, val line: Int? = null)

@Serializable
data class Diff(val path: String, val oldText: String? = null, val newText: String)

@Serializable
data class ToolCallContent(
    val type: String,
    val diff: Diff? = null,
    val content: ContentBlock? = null,
)

@Serializable
data class PlanEntry(
    val content: String,
    val priority: String? = null,
    val status: String? = null,
)

@Serializable
data class SlashCommand(val name: String, val description: String? = null)

@Serializable
data class HubSkill(
    val name: String,
    val skillId: String,
    val source: String,
    val official: Boolean = false,
)

@Serializable
data class HubSkillsResult(
    val items: List<HubSkill> = emptyList(),
    val nextCursor: Int? = null,
)

@Serializable
data class SkillInstallResult(
    val name: String? = null,
    val path: String? = null,
    val commands: List<SlashCommand> = emptyList(),
)

@Serializable
data class Cost(val amount: Double, val currency: String? = null)

@Serializable
data class SessionUpdate(
    @SerialName("_meta") val meta: JsonObject? = null,
    val sessionUpdate: String,
    val messageId: String? = null,
    val content: JsonElement? = null,
    val toolCallId: String? = null,
    val title: String? = null,
    val name: String? = null,
    val kind: String? = null,
    val status: String? = null,
    val locations: List<ToolCallLocation>? = null,
    val rawInput: JsonElement? = null,
    val rawOutput: JsonElement? = null,
    val entries: List<PlanEntry>? = null,
    val availableCommands: List<SlashCommand>? = null,
    val currentModeId: String? = null,
    val configOptions: List<SessionConfigOption>? = null,
    val updatedAt: String? = null,
    val used: Int? = null,
    val size: Int? = null,
    val cost: Cost? = null,
    val severity: String? = null,
    val description: String? = null,
    val compactionId: String? = null,
    val summary: List<ContentBlock>? = null,
    val error: String? = null,
) {
    val agentId: String? get() = metaString(meta, "com.pegg/agentId")
    val agentName: String? get() = metaString(meta, "com.pegg/agentName")
    val seq: Int? get() = metaInt(meta, "com.pegg/seq")
    val baseSeq: Int? get() = metaInt(meta, "com.pegg/baseSeq")
    val historyPage: Boolean get() = metaBool(meta, "com.pegg/historyPage") == true
    val attachments: List<AttachmentInfo>? get() = metaAttachments(meta)
    val metaError: String? get() = metaString(meta, "com.pegg/error")
    val runId: String? get() = metaString(meta, "com.pegg/runId")
    val runState: String? get() = metaString(meta, "com.pegg/runState")
    val taskName: String? get() = metaString(meta, "com.pegg/taskName")
    val durationMs: Long? get() = (meta?.get("com.pegg/durationMs") as? JsonPrimitive)?.longOrNull
    val metaUsage: UsageState? get() = runCatching {
        (meta?.get("com.pegg/usage") as? JsonObject)?.let { obj ->
            UsageState(
                used = (obj["used"] as? JsonPrimitive)?.intOrNull ?: 0,
                size = (obj["size"] as? JsonPrimitive)?.intOrNull ?: 0,
                cost = (obj["cost"] as? JsonPrimitive)?.doubleOrNull,
            )
        }
    }.getOrNull()
    val toolContents: List<ToolCallContent>? get() = (content as? JsonArray)?.let { array ->
        runCatching {
            ACP_JSON.decodeFromJsonElement<List<ToolCallContent>>(array)
        }.getOrNull()
    }
}

@Serializable
data class SessionMode(
    val id: String,
    val name: String = "",
    val description: String? = null,
)

@Serializable
data class SessionModeState(
    @SerialName("_meta") val meta: JsonObject? = null,
    val currentModeId: String = "auto",
    val availableModes: List<SessionMode> = emptyList(),
)

@Serializable
data class SessionConfigOption(
    @SerialName("_meta") val meta: JsonObject? = null,
    val id: String,
    val name: String = "",
    val description: String? = null,
    val category: String? = null,
    val type: String = "select",
    val currentValue: JsonElement? = null,
    val options: JsonElement? = null,
) {
    val currentString: String? get() = (currentValue as? JsonPrimitive)?.contentOrNull
}

@Serializable
data class InitializeResult(
    @SerialName("_meta") val meta: JsonObject? = null,
    val protocolVersion: Int = 1,
    val agentCapabilities: AgentCapabilities? = null,
    val agentInfo: AgentInfo? = null,
)

@Serializable
data class AgentCapabilities(
    @SerialName("_meta") val meta: JsonObject? = null,
    val loadSession: Boolean? = null,
    val promptCapabilities: PromptCapabilities? = null,
    val sessionCapabilities: JsonObject? = null,
    val auth: JsonObject? = null,
) {
    val pegg: PeggCapabilities? get() = meta?.get("com.pegg/capabilities")?.let { element ->
        runCatching {
            ACP_JSON.decodeFromJsonElement(PeggCapabilities.serializer(), element)
        }.getOrNull()
    }
}

@Serializable
data class PromptCapabilities(
    val image: Boolean? = null,
    val audio: Boolean? = null,
    val embeddedContext: Boolean? = null,
)

@Serializable
data class PeggCapabilities(
    val ask: Boolean? = null,
    val permissions: Boolean? = null,
    val models: Boolean? = null,
    val commands: Boolean? = null,
    val attachments: Boolean? = null,
    val diffs: Boolean? = null,
    val plan: Boolean? = null,
    val history: Boolean? = null,
    val terminals: Boolean? = null,
    val elicitation: Boolean? = null,
)

@Serializable
data class AgentInfo(
    val name: String? = null,
    val title: String? = null,
    val version: String? = null,
)

@Serializable
data class SessionNewResult(
    @SerialName("_meta") val meta: JsonObject? = null,
    val sessionId: String,
    val modes: SessionModeState? = null,
    val configOptions: List<SessionConfigOption>? = null,
) {
    val peggSessionId: String? get() = metaString(meta, "com.pegg/sessionId")
    val permissionMode: String? get() = metaString(meta, "com.pegg/permissionMode")
    val mode: String? get() = modes?.currentModeId
    val provider: String? get() = null
    val model: String? get() = configOptions?.firstOrNull { it.id == "model" }?.currentString
}

@Serializable
data class SessionStateResult(
    @SerialName("_meta") val meta: JsonObject? = null,
    val modes: SessionModeState? = null,
    val configOptions: List<SessionConfigOption>? = null,
) {
    val oldestSeq: Int? get() = metaInt(meta, "com.pegg/oldestSeq")
    val hasMore: Boolean? get() = metaBool(meta, "com.pegg/hasMore")
    val mode: String? get() = modes?.currentModeId
    val model: String? get() = configOptions?.firstOrNull { it.id == "model" }?.currentString
}

@Serializable
data class SessionUsage(
    @SerialName("prompt_tokens") val promptTokens: Long? = null,
    @SerialName("completion_tokens") val completionTokens: Long? = null,
    @SerialName("total_tokens") val totalTokens: Long? = null,
    val cost: Double? = null,
)

@Serializable
data class SessionListItem(
    @SerialName("_meta") val meta: JsonObject? = null,
    val sessionId: String,
    val cwd: String = "",
    val title: String = "Untitled session",
    val updatedAt: String? = null,
) {
    val id: String get() = sessionId
    val projectDir: String? get() = cwd.ifBlank { null }
    val provider: String? get() = metaString(meta, "com.pegg/provider")
    val model: String? get() = metaString(meta, "com.pegg/model")
    val mode: String? get() = metaString(meta, "com.pegg/mode")
    val createdAt: String? get() = metaString(meta, "com.pegg/createdAt")
    val usage: SessionUsage? get() = meta?.get("com.pegg/usage")?.let { element ->
        runCatching {
            ACP_JSON.decodeFromJsonElement(SessionUsage.serializer(), element)
        }.getOrNull()
    }
}

@Serializable
data class SessionListResult(
    @SerialName("_meta") val meta: JsonObject? = null,
    val sessions: List<SessionListItem> = emptyList(),
    val nextCursor: String? = null,
)

@Serializable
data class SessionForkResult(
    val sessionId: String,
    val title: String? = null,
    val parentId: String? = null,
)

@Serializable
data class SessionRevertResult(
    val removed: Int = 0,
    val snapshotId: String? = null,
)

@Serializable
data class ModelInfo(
    val id: String,
    val name: String? = null,
    @SerialName("owned_by") val ownedBy: String? = null,
)

@Serializable
data class ModelEntry(val provider: String, val model: ModelInfo)

@Serializable
data class ModelListResult(val models: List<ModelEntry> = emptyList())

@Serializable
data class PromptUsage(
    val used: Int = 0,
    val size: Int = 0,
    val cost: Cost? = null,
)

@Serializable
data class PromptResult(
    @SerialName("_meta") val meta: JsonObject? = null,
    val stopReason: String = "end_turn",
)

@Serializable
data class AskQuestion(
    val id: String,
    val question: String,
    val type: String = "text",
    val options: List<String>? = null,
    val required: Boolean? = null,
)

@Serializable
data class AskParams(
    val sessionId: String? = null,
    val agentId: String? = null,
    val agentName: String? = null,
    val questions: List<AskQuestion> = emptyList(),
)

@Serializable
data class PermissionOption(
    val optionId: String,
    val name: String,
    val kind: String,
)

@Serializable
data class PermissionToolCall(
    val toolCallId: String? = null,
    val title: String? = null,
    val kind: String? = null,
    val rawInput: JsonElement? = null,
)

@Serializable
data class PermissionParams(
    @SerialName("_meta") val meta: JsonObject? = null,
    val sessionId: String? = null,
    val toolCall: PermissionToolCall? = null,
    val options: List<PermissionOption> = emptyList(),
) {
    val question: String? get() = metaString(meta, "com.pegg/question")
}

@Serializable
data class EnvVar(val name: String, val value: String)

@Serializable
data class TerminalCreateParams(
    @SerialName("_meta") val meta: JsonObject? = null,
    val sessionId: String? = null,
    val command: String,
    val args: List<String> = emptyList(),
    val env: List<EnvVar> = emptyList(),
    val cwd: String? = null,
    val outputByteLimit: Int? = null,
)

@Serializable
data class TerminalIdParams(
    @SerialName("_meta") val meta: JsonObject? = null,
    val sessionId: String? = null,
    val terminalId: String,
)

@Serializable
data class TerminalExitStatus(
    @SerialName("_meta") val meta: JsonObject? = null,
    val exitCode: Int? = null,
    val signal: String? = null,
)

@Serializable
data class ElicitationProperty(
    val type: String = "string",
    val title: String? = null,
    val description: String? = null,
    @SerialName("enum") val enumValues: List<String>? = null,
    val items: ElicitationItems? = null,
)

@Serializable
data class ElicitationItems(
    val type: String = "string",
    @SerialName("enum") val enumValues: List<String>? = null,
)

@Serializable
data class ElicitationSchema(
    @SerialName("_meta") val meta: JsonObject? = null,
    val type: String = "object",
    val title: String? = null,
    val description: String? = null,
    val properties: Map<String, ElicitationProperty> = emptyMap(),
    val required: List<String> = emptyList(),
)

@Serializable
data class ElicitationCreateParams(
    @SerialName("_meta") val meta: JsonObject? = null,
    val mode: String = "form",
    val message: String = "",
    val sessionId: String? = null,
    val toolCallId: String? = null,
    val requestId: JsonElement? = null,
    val requestedSchema: ElicitationSchema? = null,
    val elicitationId: String? = null,
    val url: String? = null,
)

@Serializable
data class VoiceResult(val text: String = "")

@Serializable
data class UsageState(
    val used: Int = 0,
    val size: Int = 0,
    val cost: Double? = null,
)

@Serializable
data class TaskState(
    val name: String,
    val type: String? = null,
    val prompt: String = "",
    val background: Boolean = false,
    val agentId: String? = null,
    val status: String = "running",
    val activity: String? = null,
    val output: String? = null,
    val error: String? = null,
    val usage: UsageState? = null,
    val durationMs: Long? = null,
)

@Serializable
data class ToolState(
    val id: String,
    val name: String? = null,
    val title: String,
    val kind: String? = null,
    val status: String = "pending",
    val input: JsonElement? = null,
    val output: JsonElement? = null,
    val error: String? = null,
    val locations: List<ToolCallLocation>? = null,
    val diff: Diff? = null,
    val files: List<AttachmentInfo>? = null,
    val agentId: String? = null,
    val agentName: String? = null,
    val task: TaskState? = null,
)

@Serializable
sealed class ChatItem {
    abstract val id: String
    abstract val createdAt: Long

    @Serializable
    @SerialName("user")
    data class User(
        override val id: String,
        val text: String,
        override val createdAt: Long,
        val messageId: String? = null,
        val agentId: String? = null,
        val attachments: List<AttachmentInfo> = emptyList(),
    ) : ChatItem()

    @Serializable
    @SerialName("assistant")
    data class Assistant(
        override val id: String,
        val text: String,
        override val createdAt: Long,
        val agentId: String? = null,
        val agentName: String? = null,
    ) : ChatItem()

    @Serializable
    @SerialName("reasoning")
    data class Reasoning(
        override val id: String,
        val text: String,
        override val createdAt: Long,
        val agentId: String? = null,
        val agentName: String? = null,
    ) : ChatItem()

    @Serializable
    @SerialName("tool")
    data class Tool(
        override val id: String,
        val tool: ToolState,
        override val createdAt: Long,
    ) : ChatItem()

    @Serializable
    @SerialName("subagent")
    data class Subagent(
        override val id: String,
        val agentId: String,
        val agentName: String? = null,
        val status: String = "running",
        val task: String? = null,
        val output: String? = null,
        val error: String? = null,
        val linkedTaskId: String? = null,
        override val createdAt: Long,
    ) : ChatItem()

    @Serializable
    @SerialName("notice")
    data class Notice(
        override val id: String,
        val text: String,
        val level: String = "info",
        override val createdAt: Long,
    ) : ChatItem()

    @Serializable
    @SerialName("error")
    data class Error(
        override val id: String,
        val text: String,
        override val createdAt: Long,
    ) : ChatItem()
}

@Serializable
data class ComposerAttachment(
    val id: String,
    val name: String,
    val mediaType: String,
    val size: Long? = null,
    val path: String? = null,
    val data: String? = null,
    val text: String? = null,
)

@Serializable
sealed class PendingInteraction {
    abstract val id: String

    @Serializable
    @SerialName("permission")
    data class Permission(
        override val id: String,
        val question: String,
        val options: List<PermissionOption>,
    ) : PendingInteraction()

    @Serializable
    @SerialName("question")
    data class Question(
        override val id: String,
        val agentName: String? = null,
        val questions: List<AskQuestion>,
        val answers: MutableMap<String, String> = linkedMapOf(),
    ) : PendingInteraction()
}

@Serializable
data class ChatState(
    var acpSessionId: String? = null,
    var peggSessionId: String? = null,
    var title: String = "New session",
    var mode: String = "auto",
    var provider: String? = null,
    var model: String? = null,
    var permissionMode: String? = null,
    var connected: Boolean = false,
    var running: Boolean = false,
    var items: MutableList<ChatItem> = mutableListOf(),
    var plan: MutableList<PlanEntry> = mutableListOf(),
    var usage: UsageState = UsageState(),
    var commands: MutableList<SlashCommand> = mutableListOf(),
    var pending: MutableList<PendingInteraction> = mutableListOf(),
    var composerAttachments: MutableList<ComposerAttachment> = mutableListOf(),
    var error: String? = null,
    var counter: Int = 0,
    var historySupported: Boolean = false,
    var historyOldestSeq: Int = 0,
    var historyHasMore: Boolean = false,
    var historyLoading: Boolean = false,
)

@Serializable
data class ChatSettings(
    val sendOnEnter: String = "enter",
    val defaultMode: String = "auto",
)

fun textOf(content: JsonElement?): String {
    if (content == null || content is JsonNull) return ""
    if (content is JsonPrimitive) return content.contentOrNull ?: ""
    if (content is JsonObject) return content["text"]?.jsonPrimitive?.contentOrNull ?: ""
    return ""
}

fun elementString(value: JsonElement?): String {
    if (value == null || value is JsonNull) return ""
    if (value is JsonPrimitive) return value.contentOrNull ?: value.toString()
    return value.toString()
}

fun elementBoolean(value: JsonElement?): Boolean? =
    (value as? JsonPrimitive)?.booleanOrNull

fun elementArray(value: JsonElement?): JsonArray? = value as? JsonArray
