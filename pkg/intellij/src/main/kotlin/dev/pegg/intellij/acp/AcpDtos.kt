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
import kotlinx.serialization.json.jsonPrimitive

val ACP_JSON: Json = Json {
    ignoreUnknownKeys = true
    explicitNulls = false
    encodeDefaults = true
    classDiscriminator = "kind"
    isLenient = true
}

@Serializable
data class ContentBlock(
    val type: String,
    val text: String? = null,
    val data: String? = null,
    val mimeType: String? = null,
    val uri: String? = null,
    val resource: EmbeddedResource? = null,
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
data class Cost(val amount: Double, val currency: String? = null)

@Serializable
data class SessionUpdate(
    val sessionUpdate: String,
    val messageId: String? = null,
    val content: JsonElement? = null,
    val toolCallId: String? = null,
    val toolName: String? = null,
    val title: String? = null,
    val kind: String? = null,
    val status: String? = null,
    val locations: List<ToolCallLocation>? = null,
    val rawInput: JsonElement? = null,
    val rawOutput: JsonElement? = null,
    val entries: List<PlanEntry>? = null,
    val used: Int? = null,
    val size: Int? = null,
    val cost: Cost? = null,
    val mode: String? = null,
    val provider: String? = null,
    val model: String? = null,
    val commands: List<SlashCommand>? = null,
    val agentId: String? = null,
    val agentName: String? = null,
    val error: String? = null,
    val contents: List<ToolCallContent>? = null,
    val runId: String? = null,
    val runState: String? = null,
    val baseSeq: Int? = null,
    val seq: Int? = null,
    val historyPage: Boolean = false,
)

@Serializable
data class InitializeResult(
    val protocolVersion: Int = 1,
    val agentCapabilities: AgentCapabilities? = null,
    val agentInfo: AgentInfo? = null,
)

@Serializable
data class AgentCapabilities(
    val loadSession: Boolean? = null,
    val promptCapabilities: PromptCapabilities? = null,
    val pegg: PeggCapabilities? = null,
)

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
)

@Serializable
data class AgentInfo(
    val name: String? = null,
    val title: String? = null,
    val version: String? = null,
)

@Serializable
data class SessionNewResult(
    val sessionId: String,
    val peggSessionId: String? = null,
    val mode: String? = null,
    val provider: String? = null,
    val model: String? = null,
    val permissionMode: String? = null,
    val commands: List<SlashCommand>? = null,
)

@Serializable
data class SessionUsage(
    @SerialName("prompt_tokens") val promptTokens: Long? = null,
    @SerialName("completion_tokens") val completionTokens: Long? = null,
    @SerialName("total_tokens") val totalTokens: Long? = null,
    val cost: Double? = null,
)

@Serializable
data class SessionListItem(
    val id: String,
    val title: String = "Untitled session",
    val projectDir: String? = null,
    val provider: String? = null,
    val model: String? = null,
    val mode: String? = null,
    val usage: SessionUsage? = null,
    val createdAt: String? = null,
    val updatedAt: String? = null,
)

@Serializable
data class SessionListResult(
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
    val stopReason: String = "end_turn",
    val usage: PromptUsage? = null,
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
    val sessionId: String? = null,
    val question: String? = null,
    val toolCall: PermissionToolCall? = null,
    val options: List<PermissionOption> = emptyList(),
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
    val agentId: String? = null,
    val agentName: String? = null,
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
