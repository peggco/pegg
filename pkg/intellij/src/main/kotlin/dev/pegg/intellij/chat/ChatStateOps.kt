package dev.pegg.intellij.chat

import dev.pegg.intellij.acp.AttachmentInfo
import dev.pegg.intellij.acp.ChatItem
import dev.pegg.intellij.acp.ChatState
import dev.pegg.intellij.acp.ComposerAttachment
import dev.pegg.intellij.acp.PendingInteraction
import dev.pegg.intellij.acp.PlanEntry
import dev.pegg.intellij.acp.PromptResult
import dev.pegg.intellij.acp.SessionUpdate
import dev.pegg.intellij.acp.TaskState
import dev.pegg.intellij.acp.ToolState
import dev.pegg.intellij.acp.UsageState
import dev.pegg.intellij.acp.elementString
import dev.pegg.intellij.acp.textOf
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive

object ChatStateOps {

    fun initialState(mode: String = "auto"): ChatState = ChatState(mode = mode)

    fun snapshot(state: ChatState): ChatState {
        val pending = state.pending.map { item ->
            if (item is PendingInteraction.Question) {
                item.copy(answers = LinkedHashMap(item.answers))
            } else {
                item
            }
        }
        return state.copy(
            items = state.items.toMutableList(),
            plan = state.plan.toMutableList(),
            commands = state.commands.toMutableList(),
            pending = pending.toMutableList(),
            composerAttachments = state.composerAttachments.toMutableList(),
        )
    }

    private fun nextId(state: ChatState, prefix: String): String {
        state.counter += 1
        return "$prefix-${state.counter}"
    }

    private fun findItem(state: ChatState, id: String): ChatItem? = state.items.firstOrNull { it.id == id }

    fun applyHistory(input: ChatState, updates: List<SessionUpdate>): ChatState {
        val history = initialState()
        history.counter = input.counter
        for (update in updates) applyUpdate(history, update)
        val finalized = history.items.map { item ->
            if (item is ChatItem.Tool && item.tool.task != null) {
                item.copy(tool = finalizeRestoredTask(item.tool))
            } else {
                item
            }
        }
        input.items.addAll(0, finalized)
        input.counter = maxOf(input.counter, history.counter)
        return input
    }

    fun applyUpdate(input: ChatState, update: SessionUpdate): ChatState {
        val state = input
        val now = System.currentTimeMillis()
        when (update.sessionUpdate) {
            "agent_message_chunk", "user_message_chunk" -> applyMessage(state, update, now)
            "agent_thought_chunk" -> applyThought(state, update, now)
            "tool_call", "tool_call_update" -> applyTool(state, update, now)
            "subagent" -> applySubagent(state, update, now)
            "plan" -> if (update.entries != null) state.plan = update.entries.toMutableList()
            "usage_update" -> state.usage = UsageState(
                used = update.used ?: state.usage.used,
                size = update.size ?: state.usage.size,
                cost = update.cost?.amount ?: state.usage.cost,
            )
            "current_mode_update" -> if (update.currentModeId != null) state.mode = update.currentModeId
            "config_option_update" -> applyConfigOptions(state, update.configOptions)
            "run_state_update" -> {
                state.running = update.runState == "running"
                val error = update.metaError
                if (!error.isNullOrBlank()) {
                    state.items.add(ChatItem.Error(nextId(state, "error"), error, now))
                    state.error = error
                }
            }
            "run_ask_resolved", "agent_warning_clear" -> Unit
            "available_commands_update" -> if (update.availableCommands != null) {
                state.commands = update.availableCommands.toMutableList()
            }
            "session_info_update" -> if (!update.title.isNullOrBlank()) state.title = update.title
            "notice" -> applyNotice(state, update, now)
            "compaction_update" -> applyCompaction(state, update, now)
            "compaction_summary_chunk" -> Unit
            "error" -> {
                val text = update.error ?: update.metaError ?: "Unknown error"
                state.items.add(ChatItem.Error(nextId(state, "error"), text, now))
                state.error = text
            }
        }
        return state
    }

    private fun applyConfigOptions(state: ChatState, options: List<dev.pegg.intellij.acp.SessionConfigOption>?) {
        if (options == null) return
        for (option in options) {
            when (option.id) {
                "mode" -> option.currentString?.let { state.mode = it }
                "model" -> option.currentString?.let { state.model = it }
            }
        }
    }

    private fun applyNotice(state: ChatState, update: SessionUpdate, now: Long) {
        val title = update.title ?: update.description ?: "Notice"
        val text = if (!update.description.isNullOrBlank() && update.description != title) {
            "$title: ${update.description}"
        } else {
            title
        }
        val level = when (update.severity) {
            "error" -> "error"
            "warning" -> "warn"
            else -> "info"
        }
        if (level == "error") {
            state.items.add(ChatItem.Error(nextId(state, "error"), text, now))
            state.error = text
        } else {
            state.items.add(ChatItem.Notice(nextId(state, "notice"), text, level, now))
        }
    }

    private fun applyCompaction(state: ChatState, update: SessionUpdate, now: Long) {
        val summary = update.summary?.mapNotNull { it.text }?.joinToString("\n").orEmpty()
        val text = summary.ifBlank {
            when (update.status) {
                "failed" -> update.error ?: "Context compaction failed"
                "cancelled" -> "Context compaction cancelled"
                else -> "Context compacted"
            }
        }
        state.items.add(ChatItem.Notice(nextId(state, "notice"), text, "info", now))
    }

    private fun applyMessage(state: ChatState, update: SessionUpdate, now: Long) {
        val text = textOf(update.content)
        val attachments = update.attachments.orEmpty()
        if (text.isEmpty() && attachments.isEmpty()) return
        val isUser = update.sessionUpdate == "user_message_chunk"
        val id = update.messageId ?: nextId(state, "message")
        val index = state.items.indexOfFirst {
            it.id == id && (it is ChatItem.User || it is ChatItem.Assistant)
        }
        if (index >= 0) {
            when (val item = state.items[index]) {
                is ChatItem.Assistant -> state.items[index] = item.copy(text = item.text + text)
                is ChatItem.User -> state.items[index] = item.copy(
                    text = item.text + text,
                    attachments = mergeAttachments(item.attachments, if (isUser) attachments else null),
                )
                else -> Unit
            }
            return
        }
        val last = state.items.lastOrNull()
        if (isUser && last is ChatItem.User) {
            state.items[state.items.lastIndex] = last.copy(
                text = last.text + text,
                attachments = mergeAttachments(last.attachments, attachments),
            )
            return
        }
        if (!isUser && last is ChatItem.Assistant && last.agentName == update.agentName) {
            state.items[state.items.lastIndex] = last.copy(text = last.text + text)
            return
        }
        state.items.add(
            if (isUser) {
                ChatItem.User(
                    id = id,
                    text = text,
                    createdAt = now,
                    messageId = update.messageId,
                    agentId = update.agentId,
                    attachments = attachments,
                )
            } else {
                ChatItem.Assistant(
                    id = id,
                    text = text,
                    createdAt = now,
                    agentId = update.agentId,
                    agentName = update.agentName,
                )
            },
        )
    }

    private fun mergeAttachments(
        existing: List<AttachmentInfo>?,
        incoming: List<AttachmentInfo>?,
    ): List<AttachmentInfo> {
        if (incoming.isNullOrEmpty()) return existing.orEmpty()
        if (existing.isNullOrEmpty()) return incoming
        val seen = existing.map { "${it.name}|${it.path ?: ""}" }.toMutableSet()
        val merged = existing.toMutableList()
        for (attachment in incoming) {
            if (!seen.add("${attachment.name}|${attachment.path ?: ""}")) continue
            merged.add(attachment)
        }
        return merged
    }

    private fun mergeFiles(
        existing: List<AttachmentInfo>?,
        incoming: List<AttachmentInfo>?,
    ): List<AttachmentInfo>? {
        if (incoming.isNullOrEmpty()) return existing
        if (existing.isNullOrEmpty()) return incoming
        val seen = existing.map { "${it.name}|${it.path ?: ""}" }.toMutableSet()
        val merged = existing.toMutableList()
        for (file in incoming) {
            if (!seen.add("${file.name}|${file.path ?: ""}")) continue
            merged.add(file)
        }
        return merged
    }

    private fun applyThought(state: ChatState, update: SessionUpdate, now: Long) {
        val text = textOf(update.content)
        if (text.isEmpty()) return
        val id = update.messageId ?: nextId(state, "thought")
        val index = state.items.indexOfFirst { it.id == id && it is ChatItem.Reasoning }
        if (index >= 0) {
            val item = state.items[index] as ChatItem.Reasoning
            state.items[index] = item.copy(text = item.text + text)
            return
        }
        val last = state.items.lastOrNull()
        if (last is ChatItem.Reasoning && last.agentName == update.agentName) {
            state.items[state.items.lastIndex] = last.copy(text = last.text + text)
            return
        }
        state.items.add(ChatItem.Reasoning(id, text, now, update.agentId, update.agentName))
    }

    private fun applyTool(state: ChatState, update: SessionUpdate, now: Long) {
        val id = update.toolCallId ?: nextId(state, "tool")
        val diff = update.toolContents?.firstOrNull { it.type == "diff" && it.diff != null }?.diff
        val error = update.metaError ?: update.error
        val index = state.items.indexOfFirst { it.id == id && it is ChatItem.Tool }
        val tool: ToolState
        if (index >= 0) {
            val item = state.items[index] as ChatItem.Tool
            val old = item.tool
            val newTitle = update.title ?: old.title
            tool = enrichTask(
                old.copy(
                    name = update.name ?: old.name,
                    title = newTitle,
                    kind = inferToolKind(update.kind, newTitle, old.kind),
                    status = normalizeStatus(update.status) ?: old.status,
                    input = nonNull(update.rawInput) ?: old.input,
                    output = nonNull(update.rawOutput) ?: old.output,
                    error = error ?: old.error,
                    locations = update.locations ?: old.locations,
                    diff = diff ?: old.diff,
                    files = mergeFiles(old.files, update.attachments),
                    agentId = update.agentId ?: old.agentId,
                    agentName = update.agentName ?: old.agentName,
                ),
                update,
            )
            state.items[index] = item.copy(tool = tool)
        } else {
            val title = update.title ?: update.toolCallId ?: "Tool"
            tool = enrichTask(
                ToolState(
                    id = id,
                    name = update.name,
                    title = title,
                    kind = inferToolKind(update.kind, title, null),
                    status = normalizeStatus(update.status) ?: "pending",
                    input = nonNull(update.rawInput),
                    output = nonNull(update.rawOutput),
                    error = error,
                    locations = update.locations,
                    diff = diff,
                    files = update.attachments?.takeIf { it.isNotEmpty() },
                    agentId = update.agentId,
                    agentName = update.agentName,
                ),
                update,
            )
            state.items.add(ChatItem.Tool(id = id, createdAt = now, tool = tool))
        }
        val activity = update.title ?: update.name
        if (!isTaskTool(tool) && !update.agentId.isNullOrEmpty() && !activity.isNullOrBlank()) {
            val linkedIndex = state.items.indexOfFirst {
                it is ChatItem.Tool && it.tool.task?.agentId == update.agentId && it.tool.task?.status == "running"
            }
            if (linkedIndex >= 0) {
                val linked = state.items[linkedIndex] as ChatItem.Tool
                val task = linked.tool.task
                if (task != null) {
                    state.items[linkedIndex] = linked.copy(tool = linked.tool.copy(task = task.copy(activity = activity)))
                }
            }
        }
    }

    private fun applySubagent(state: ChatState, update: SessionUpdate, now: Long) {
        val agentId = update.agentId ?: return
        val status = when (update.status) {
            "finished" -> "finished"
            "error" -> "error"
            else -> "running"
        }
        val taskName = update.taskName
        val linked = findSubagentTask(state, agentId, taskName)
        val index = state.items.indexOfFirst { it is ChatItem.Subagent && it.agentId == agentId }
        if (index >= 0) {
            val item = state.items[index] as ChatItem.Subagent
            state.items[index] = item.copy(
                status = status,
                agentName = update.agentName ?: item.agentName,
                task = update.title ?: item.task,
                output = if (nonNull(update.rawOutput) != null) elementString(update.rawOutput) else item.output,
                error = update.metaError ?: item.error,
                linkedTaskId = linked?.id ?: item.linkedTaskId,
            )
        } else {
            state.items.add(
                ChatItem.Subagent(
                    id = "subagent-$agentId",
                    agentId = agentId,
                    agentName = update.agentName,
                    status = status,
                    task = update.title,
                    output = if (nonNull(update.rawOutput) != null) elementString(update.rawOutput) else null,
                    error = update.metaError,
                    linkedTaskId = linked?.id,
                    createdAt = now,
                ),
            )
        }
        if (linked != null) {
            val linkedIndex = state.items.indexOfFirst { it.id == linked.id }
            val item = state.items[linkedIndex] as ChatItem.Tool
            var task: TaskState = item.tool.task ?: return
            task = task.copy(agentId = agentId)
            val usage = update.metaUsage
            if (usage != null) task = task.copy(usage = usage)
            val duration = update.durationMs
            if (duration != null) task = task.copy(durationMs = duration)
            task = when (status) {
                "finished" -> task.copy(
                    status = "finished",
                    output = if (update.rawOutput != null) elementString(update.rawOutput) else "",
                    activity = null,
                )
                "error" -> task.copy(status = "error", error = update.metaError ?: "subagent failed", activity = null)
                else -> task.copy(status = "running")
            }
            state.items[linkedIndex] = item.copy(tool = item.tool.copy(task = task))
        }
        if (!update.title.isNullOrBlank()) pushSubagentPrompt(state, agentId, update.title, now)
    }

    private data class TaskInfo(val name: String, val type: String?, val prompt: String, val background: Boolean)

    private fun isTaskTool(tool: ToolState): Boolean {
        val name = tool.name?.lowercase() ?: ""
        return name == "task" || tool.title.lowercase().startsWith("task:")
    }

    private fun taskInfo(input: JsonElement?): TaskInfo? {
        val obj = input as? JsonObject ?: return null
        val name = (obj["name"] as? JsonPrimitive)?.contentOrNull
            ?: (obj["description"] as? JsonPrimitive)?.contentOrNull
            ?: return null
        return TaskInfo(
            name = name,
            type = (obj["subagent_type"] as? JsonPrimitive)?.contentOrNull,
            prompt = (obj["prompt"] as? JsonPrimitive)?.contentOrNull ?: "",
            background = (obj["background"] as? JsonPrimitive)?.booleanOrNull == true,
        )
    }

    private fun isBackgroundStart(output: String): Boolean {
        val out = output.trim()
        if (out.startsWith("Started subagent in background:") || out.startsWith("Started subagent in backgro")) return true
        return out.startsWith("Resumed subagent ") && (out.contains(" in background") || out.endsWith("backgro…"))
    }

    private fun taskResponse(output: String): String {
        val marker = "\nResponse:\n"
        val i = output.indexOf(marker)
        return if (i >= 0) output.substring(i + marker.length).trim() else output.trim()
    }

    private fun taskFailure(output: String): String? {
        val out = output.trim()
        if (!out.startsWith("Subagent ")) return null
        val i = out.indexOf(" failed: ")
        if (i < 0) return null
        return out.substring(i + " failed: ".length).trim()
    }

    private fun enrichTask(tool: ToolState, update: SessionUpdate): ToolState {
        if (!isTaskTool(tool)) return tool
        val info = taskInfo(tool.input)
        var task: TaskState = tool.task ?: TaskState(
            name = info?.name ?: return tool,
            type = info?.type,
            prompt = info?.prompt ?: "",
            background = info?.background == true,
            status = "running",
        )
        val duration = update.durationMs
        if (duration != null) task = task.copy(durationMs = duration)
        val usage = update.metaUsage
        if (usage != null) task = task.copy(usage = usage)
        if (update.metaError != null) task = task.copy(status = "error", error = update.metaError)
        val raw = update.rawOutput?.let { elementString(it) } ?: return tool.copy(task = task)
        if (isBackgroundStart(raw)) {
            task = task.copy(background = true)
            if (task.status != "finished" && task.status != "error") {
                task = task.copy(status = "running", output = null)
            }
            return tool.copy(task = task)
        }
        val failure = taskFailure(raw)
        val trimmed = raw.trim()
        task = when {
            failure != null -> task.copy(status = "error", error = failure, output = null)
            trimmed.startsWith("Error: ") -> task.copy(
                status = "error",
                error = trimmed.removePrefix("Error: ").trim(),
                output = null,
            )
            task.agentId != null && task.status == "running" -> if (trimmed.isNotEmpty()) {
                task.copy(output = taskResponse(raw))
            } else {
                task
            }
            trimmed.isEmpty() -> if (task.agentId == null) {
                task.copy(status = if (task.background) "background" else "unknown")
            } else {
                task
            }
            else -> task.copy(status = "finished", output = taskResponse(raw))
        }
        return tool.copy(task = task)
    }

    private fun findSubagentTask(state: ChatState, agentId: String, name: String?): ChatItem.Tool? {
        for (i in state.items.indices.reversed()) {
            val item = state.items[i]
            if (item is ChatItem.Tool && item.tool.task?.agentId == agentId) {
                return item
            }
        }
        return if (name != null) findTaskTool(state, name) else null
    }

    private fun findTaskTool(state: ChatState, name: String): ChatItem.Tool? {
        for (i in state.items.indices.reversed()) {
            val item = state.items[i]
            if (item is ChatItem.Tool && item.tool.task?.name == name && item.tool.task?.agentId == null) {
                return item
            }
        }
        return null
    }

    private fun finalizeRestoredTask(tool: ToolState): ToolState {
        val task = tool.task ?: return tool
        var next = task.copy(agentId = null)
        if (tool.error != null) return tool.copy(task = next.copy(status = "error", error = tool.error))
        val raw = if (tool.output == null) "" else elementString(tool.output)
        val failure = taskFailure(raw)
        val trimmed = raw.trim()
        next = when {
            failure != null -> next.copy(status = "error", error = failure, output = null)
            trimmed.startsWith("Error: ") -> next.copy(
                status = "error",
                error = trimmed.removePrefix("Error: ").trim(),
                output = null,
            )
            next.background && isBackgroundStart(raw) -> next.copy(status = "background", output = null)
            trimmed.isEmpty() -> next.copy(status = if (next.background) "background" else "unknown")
            else -> next.copy(status = "finished", output = taskResponse(raw))
        }
        return tool.copy(task = next)
    }

    private fun pushSubagentPrompt(state: ChatState, agentId: String, text: String, now: Long) {
        val id = "subagent-input-$agentId"
        if (state.items.any { it.id == id }) return
        state.items.add(ChatItem.User(id, text, now, agentId = agentId))
    }

    fun appendUser(
        state: ChatState,
        text: String,
        attachments: List<AttachmentInfo> = emptyList(),
    ): ChatState {
        state.counter += 1
        state.items.add(
            ChatItem.User(
                id = "user-${state.counter}",
                text = text,
                createdAt = System.currentTimeMillis(),
                attachments = attachments,
            ),
        )
        return state
    }

    fun addNotice(state: ChatState, text: String, level: String = "info"): ChatState {
        state.counter += 1
        state.items.add(ChatItem.Notice("notice-${state.counter}", text, level, System.currentTimeMillis()))
        return state
    }

    fun addAttachment(state: ChatState, attachment: ComposerAttachment): ChatState {
        state.composerAttachments.add(attachment)
        return state
    }

    fun removeAttachment(state: ChatState, id: String): ChatState {
        state.composerAttachments.removeAll { it.id == id }
        return state
    }

    fun clearAttachments(state: ChatState): ChatState {
        state.composerAttachments.clear()
        return state
    }

    fun clearConversation(state: ChatState): ChatState {
        state.items.clear()
        state.plan.clear()
        state.usage = UsageState()
        state.error = null
        state.counter = 0
        return state
    }

    fun truncateFrom(state: ChatState, itemId: String): ChatState {
        val index = state.items.indexOfFirst { it.id == itemId }
        if (index < 0) return state
        state.items = state.items.take(index).toMutableList()
        return state
    }

    fun userMessageOrdinal(state: ChatState, itemId: String): Int? {
        var ordinal = 0
        for (item in state.items) {
            if (item !is ChatItem.User || item.agentId != null) continue
            if (item.id == itemId) return ordinal
            ordinal += 1
        }
        return null
    }

    fun finishRun(state: ChatState, result: PromptResult): ChatState {
        state.running = false
        state.items = state.items.map { item ->
            if (item !is ChatItem.Tool) return@map item
            if (item.tool.status != "pending" && item.tool.status != "in_progress") return@map item
            val status = if (result.stopReason == "cancelled") "cancelled" else "completed"
            item.copy(tool = item.tool.copy(status = status))
        }.toMutableList()
        when (result.stopReason) {
            "error" -> addNotice(state, "Pegg stopped with an error.", "error")
            "cancelled" -> addNotice(state, "Run cancelled.", "warn")
        }
        return state
    }

    private fun nonNull(value: JsonElement?): JsonElement? =
        if (value == null || value is JsonNull) null else value

    fun inferToolKind(kind: String?, title: String?, previous: String?): String? {
        if (!kind.isNullOrEmpty() && kind != "other") return kind
        val name = (title ?: "").lowercase()
        return when {
            Regex("^bash\\b|\\bbash\\b").containsMatchIn(name) -> "execute"
            Regex("^(read|list)\\b").containsMatchIn(name) -> "read"
            Regex("^(write|edit)\\b").containsMatchIn(name) -> "edit"
            Regex("^(glob|grep)\\b").containsMatchIn(name) -> "search"
            Regex("^(webfetch|websearch)\\b").containsMatchIn(name) -> "fetch"
            Regex("^(task|todo)").containsMatchIn(name) -> "think"
            else -> kind ?: previous
        }
    }

    fun normalizeStatus(status: String?): String? = when (status) {
        "pending", "in_progress", "completed", "failed", "cancelled" -> status
        else -> null
    }

    fun planFromEntries(entries: List<PlanEntry>): MutableList<PlanEntry> = entries.toMutableList()
}
