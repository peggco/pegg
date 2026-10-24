package dev.pegg.intellij.chat

import dev.pegg.intellij.acp.ChatItem
import dev.pegg.intellij.acp.ChatState
import dev.pegg.intellij.acp.ComposerAttachment
import dev.pegg.intellij.acp.PendingInteraction
import dev.pegg.intellij.acp.PlanEntry
import dev.pegg.intellij.acp.PromptResult
import dev.pegg.intellij.acp.SessionUpdate
import dev.pegg.intellij.acp.ToolState
import dev.pegg.intellij.acp.UsageState
import dev.pegg.intellij.acp.elementString
import dev.pegg.intellij.acp.textOf
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonElement

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
            "current_mode_update" -> if (update.mode != null) state.mode = update.mode
            "run_state_update" -> state.running = update.runState == "running"
            "current_model_update" -> {
                if (update.provider != null) state.provider = update.provider
                if (update.model != null) state.model = update.model
            }
            "available_commands_update" -> if (update.commands != null) {
                state.commands = update.commands.toMutableList()
            }
            "error" -> {
                val text = update.error ?: "Unknown error"
                state.items.add(ChatItem.Error(nextId(state, "error"), text, now))
                state.error = text
            }
        }
        return state
    }

    private fun applyMessage(state: ChatState, update: SessionUpdate, now: Long) {
        val text = textOf(update.content)
        if (text.isEmpty()) return
        val isUser = update.sessionUpdate == "user_message_chunk"
        val id = update.messageId ?: nextId(state, "message")
        val index = state.items.indexOfFirst {
            it.id == id && (it is ChatItem.User || it is ChatItem.Assistant)
        }
        if (index >= 0) {
            when (val item = state.items[index]) {
                is ChatItem.Assistant -> state.items[index] = item.copy(text = item.text + text)
                is ChatItem.User -> state.items[index] = item.copy(text = item.text + text)
                else -> Unit
            }
            return
        }
        val last = state.items.lastOrNull()
        if (isUser && last is ChatItem.User) {
            state.items[state.items.lastIndex] = last.copy(text = last.text + text)
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
        val toolName = update.toolName?.lowercase()
        val taskTitle = update.title?.startsWith("task:", ignoreCase = true) == true
        if (toolName == "task" || taskTitle) {
            state.items.removeAll { it.id == id }
            return
        }
        val diff = update.contents?.firstOrNull { it.type == "diff" && it.diff != null }?.diff
        val index = state.items.indexOfFirst { it.id == id && it is ChatItem.Tool }
        if (index >= 0) {
            val item = state.items[index] as ChatItem.Tool
            val old = item.tool
            val newTitle = update.title ?: old.title
            val newTool = old.copy(
                name = update.toolName ?: old.name,
                title = newTitle,
                kind = inferToolKind(update.kind, newTitle, old.kind),
                status = normalizeStatus(update.status) ?: old.status,
                input = nonNull(update.rawInput) ?: old.input,
                output = nonNull(update.rawOutput) ?: old.output,
                error = update.error ?: old.error,
                locations = update.locations ?: old.locations,
                diff = diff ?: old.diff,
                agentId = update.agentId ?: old.agentId,
                agentName = update.agentName ?: old.agentName,
            )
            state.items[index] = item.copy(tool = newTool)
            return
        }
        val title = update.title ?: update.toolCallId ?: "Tool"
        state.items.add(
            ChatItem.Tool(
                id = id,
                createdAt = now,
                tool = ToolState(
                    id = id,
                    name = update.toolName,
                    title = title,
                    kind = inferToolKind(update.kind, title, null),
                    status = normalizeStatus(update.status) ?: "pending",
                    input = nonNull(update.rawInput),
                    output = nonNull(update.rawOutput),
                    error = update.error,
                    locations = update.locations,
                    diff = diff,
                    agentId = update.agentId,
                    agentName = update.agentName,
                ),
            ),
        )
    }

    private fun applySubagent(state: ChatState, update: SessionUpdate, now: Long) {
        val agentId = update.agentId ?: return
        val status = when (update.status) {
            "finished" -> "finished"
            "error" -> "error"
            else -> "running"
        }
        val index = state.items.indexOfFirst { it is ChatItem.Subagent && it.agentId == agentId }
        if (index >= 0) {
            val item = state.items[index] as ChatItem.Subagent
            state.items[index] = item.copy(
                status = status,
                agentName = update.agentName ?: item.agentName,
                task = update.title ?: item.task,
                output = if (nonNull(update.rawOutput) != null) elementString(update.rawOutput) else item.output,
                error = update.error ?: item.error,
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
                    error = update.error,
                    createdAt = now,
                ),
            )
        }
        if (!update.title.isNullOrBlank()) pushSubagentPrompt(state, agentId, update.title, now)
    }

    private fun pushSubagentPrompt(state: ChatState, agentId: String, text: String, now: Long) {
        val id = "subagent-input-$agentId"
        if (state.items.any { it.id == id }) return
        state.items.add(ChatItem.User(id, text, now, agentId = agentId))
    }

    fun appendUser(state: ChatState, text: String): ChatState {
        state.counter += 1
        state.items.add(ChatItem.User("user-${state.counter}", text, System.currentTimeMillis()))
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
        val usage = result.usage
        if (usage != null && usage.used > 0) {
            state.usage = UsageState(usage.used, usage.size, usage.cost?.amount)
        }
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
