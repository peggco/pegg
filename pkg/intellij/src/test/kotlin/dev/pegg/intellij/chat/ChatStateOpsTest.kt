package dev.pegg.intellij.chat

import dev.pegg.intellij.acp.ACP_JSON
import dev.pegg.intellij.acp.AttachmentInfo
import dev.pegg.intellij.acp.ChatItem
import dev.pegg.intellij.acp.ComposerAttachment
import dev.pegg.intellij.acp.SessionConfigOption
import dev.pegg.intellij.acp.SessionUpdate
import dev.pegg.intellij.acp.ToolCallContent
import dev.pegg.intellij.acp.Diff
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.encodeToJsonElement
import kotlinx.serialization.json.put
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ChatStateOpsTest {

    private fun update(
        sessionUpdate: String,
        block: (SessionUpdate.() -> SessionUpdate)? = null,
    ): SessionUpdate {
        val base = SessionUpdate(sessionUpdate = sessionUpdate)
        return block?.invoke(base) ?: base
    }

    private fun text(value: String): JsonObject =
        JsonObject(mapOf("type" to JsonPrimitive("text"), "text" to JsonPrimitive(value)))

    private fun meta(vararg pairs: Pair<String, kotlinx.serialization.json.JsonElement>): JsonObject =
        buildJsonObject { pairs.forEach { (key, value) -> put(key, value) } }

    private fun toolContent(vararg contents: ToolCallContent): JsonArray =
        JsonArray(contents.map { ACP_JSON.encodeToJsonElement(ToolCallContent.serializer(), it) })

    private fun attachmentsMeta(vararg attachments: AttachmentInfo): JsonObject =
        meta(
            "com.pegg/attachments" to ACP_JSON.encodeToJsonElement(
                kotlinx.serialization.builtins.ListSerializer(AttachmentInfo.serializer()),
                attachments.toList(),
            ),
        )

    @Test
    fun `appends streamed assistant chunks to one message`() {
        var state = ChatStateOps.initialState()
        state = ChatStateOps.applyUpdate(state, update("agent_message_chunk") {
            copy(messageId = "m1", content = text("Hello"))
        })
        state = ChatStateOps.applyUpdate(state, update("agent_message_chunk") {
            copy(messageId = "m1", content = text(" world"))
        })
        assertEquals(1, state.items.size)
        assertEquals("Hello world", (state.items[0] as ChatItem.Assistant).text)
    }

    @Test
    fun `upserts tool calls and captures diffs`() {
        var state = ChatStateOps.initialState()
        state = ChatStateOps.applyUpdate(state, update("tool_call") {
            copy(toolCallId = "t1", title = "edit: main.go", kind = "edit", status = "pending")
        })
        state = ChatStateOps.applyUpdate(state, update("tool_call_update") {
            copy(
                toolCallId = "t1",
                status = "completed",
                content = toolContent(
                    ToolCallContent(
                        type = "diff",
                        diff = Diff("main.go", oldText = "a", newText = "b"),
                    ),
                ),
            )
        })
        assertEquals(1, state.items.size)
        val item = state.items[0] as ChatItem.Tool
        assertEquals("completed", item.tool.status)
        assertEquals("b", item.tool.diff?.newText)
    }

    @Test
    fun `collects delivered files from tool attachments`() {
        var state = ChatStateOps.initialState()
        state = ChatStateOps.applyUpdate(state, update("tool_call") {
            copy(toolCallId = "t1", name = "sendfile", title = "sendfile:1 file", status = "pending")
        })
        state = ChatStateOps.applyUpdate(state, update("tool_call_update") {
            copy(
                toolCallId = "t1",
                name = "sendfile",
                status = "completed",
                meta = attachmentsMeta(
                    AttachmentInfo(
                        name = "report.pdf",
                        mediaType = "application/pdf",
                        type = "file",
                        size = 1536,
                        path = "/tmp/report.pdf",
                    ),
                ),
            )
        })
        val item = state.items.last() as ChatItem.Tool
        val files = assertNotNull(item.tool.files)
        assertEquals(1, files.size)
        assertEquals("report.pdf", files[0].name)
        assertEquals(1536L, files[0].size)
        assertEquals("/tmp/report.pdf", files[0].path)
    }

    @Test
    fun `keeps text before and after a tool call in separate segments`() {
        var state = ChatStateOps.initialState()
        state = ChatStateOps.applyUpdate(state, update("agent_message_chunk") {
            copy(messageId = "before", content = text("before"))
        })
        state = ChatStateOps.applyUpdate(state, update("tool_call") {
            copy(toolCallId = "t1", title = "bash: ls", kind = "other", status = "pending")
        })
        state = ChatStateOps.applyUpdate(state, update("agent_message_chunk") {
            copy(messageId = "after", content = text("after"))
        })
        assertEquals(listOf("assistant", "tool", "assistant"), state.items.map { it.kindName() })
        assertEquals("execute", (state.items[1] as ChatItem.Tool).tool.kind)
    }

    @Test
    fun `merges consecutive assistant chunks with different message ids`() {
        var state = ChatStateOps.initialState()
        state = ChatStateOps.applyUpdate(state, update("agent_message_chunk") {
            copy(messageId = "a1", meta = meta("com.pegg/agentName" to JsonPrimitive("orchestrator")), content = text("One "))
        })
        state = ChatStateOps.applyUpdate(state, update("agent_message_chunk") {
            copy(messageId = "a2", meta = meta("com.pegg/agentName" to JsonPrimitive("orchestrator")), content = text("response"))
        })
        assertEquals(1, state.items.size)
        assertEquals("One response", (state.items[0] as ChatItem.Assistant).text)
    }

    @Test
    fun `merges consecutive reasoning segments into one block`() {
        var state = ChatStateOps.initialState()
        state = ChatStateOps.applyUpdate(state, update("agent_thought_chunk") {
            copy(messageId = "r1", meta = meta("com.pegg/agentName" to JsonPrimitive("orchestrator")), content = text("I should"))
        })
        state = ChatStateOps.applyUpdate(state, update("agent_thought_chunk") {
            copy(messageId = "r2", meta = meta("com.pegg/agentName" to JsonPrimitive("orchestrator")), content = text(" inspect the repo"))
        })
        assertEquals(1, state.items.size)
        assertEquals("I should inspect the repo", (state.items[0] as ChatItem.Reasoning).text)
    }

    @Test
    fun `tracks plan usage mode model and commands`() {
        var state = ChatStateOps.initialState()
        state = ChatStateOps.applyUpdate(state, update("plan") {
            copy(entries = listOf(dev.pegg.intellij.acp.PlanEntry("one", status = "pending")))
        })
        state = ChatStateOps.applyUpdate(state, update("usage_update") {
            copy(used = 10, size = 100, cost = dev.pegg.intellij.acp.Cost(0.1, "USD"))
        })
        state = ChatStateOps.applyUpdate(state, update("current_mode_update") { copy(currentModeId = "plan") })
        state = ChatStateOps.applyUpdate(state, update("config_option_update") {
            copy(
                configOptions = listOf(
                    SessionConfigOption(id = "model", currentValue = JsonPrimitive("gpt")),
                ),
            )
        })
        state = ChatStateOps.applyUpdate(state, update("available_commands_update") {
            copy(availableCommands = listOf(dev.pegg.intellij.acp.SlashCommand("init")))
        })
        assertEquals("one", state.plan[0].content)
        assertEquals(0.1, state.usage.cost)
        assertEquals("plan", state.mode)
        assertEquals("gpt", state.model)
        assertEquals("init", state.commands[0].name)
    }

    @Test
    fun `handles errors and notices`() {
        var state = ChatStateOps.initialState()
        state = ChatStateOps.applyUpdate(state, update("agent_thought_chunk") {
            copy(messageId = "r1", content = text("thinking"))
        })
        state = ChatStateOps.applyUpdate(state, update("notice") {
            copy(severity = "error", title = "boom")
        })
        state = ChatStateOps.addNotice(state, "careful", "warn")
        assertEquals(listOf("reasoning", "error", "notice"), state.items.map { it.kindName() })
    }

    @Test
    fun `manages user messages attachments and clearing`() {
        var state = ChatStateOps.appendUser(ChatStateOps.initialState(), "hi")
        state = ChatStateOps.addAttachment(
            state,
            ComposerAttachment("a1", "a.txt", "text/plain", text = "x"),
        )
        assertEquals(1, state.composerAttachments.size)
        state = ChatStateOps.removeAttachment(state, "a1")
        assertEquals(0, state.composerAttachments.size)
        state = ChatStateOps.applyUpdate(state, update("usage_update") { copy(used = 10, size = 100) })
        state = ChatStateOps.clearConversation(state)
        assertEquals(0, state.items.size)
        assertEquals(0, state.usage.used)
    }

    @Test
    fun `counts only real user messages for history`() {
        var state = ChatStateOps.initialState()
        state = ChatStateOps.appendUser(state, "one")
        state = ChatStateOps.addNotice(state, "a notice")
        state = ChatStateOps.appendUser(state, "two")
        val users = state.items.filterIsInstance<ChatItem.User>()
        assertEquals(0, ChatStateOps.userMessageOrdinal(state, users[0].id))
        assertEquals(1, ChatStateOps.userMessageOrdinal(state, users[1].id))
        assertNull(ChatStateOps.userMessageOrdinal(state, "missing"))
    }

    @Test
    fun `tracks subagent lifecycle and task prompt`() {
        var state = ChatStateOps.initialState()
        state = ChatStateOps.applyUpdate(state, update("subagent") {
            copy(
                status = "running",
                meta = meta(
                    "com.pegg/agentId" to JsonPrimitive("a1"),
                    "com.pegg/agentName" to JsonPrimitive("explorer"),
                ),
            )
        })
        state = ChatStateOps.applyUpdate(state, update("subagent") {
            copy(
                status = "running",
                title = "Find auth",
                meta = meta(
                    "com.pegg/agentId" to JsonPrimitive("a1"),
                    "com.pegg/agentName" to JsonPrimitive("explorer"),
                ),
            )
        })
        state = ChatStateOps.applyUpdate(state, update("subagent") {
            copy(
                status = "finished",
                rawOutput = JsonPrimitive("found"),
                meta = meta(
                    "com.pegg/agentId" to JsonPrimitive("a1"),
                    "com.pegg/agentName" to JsonPrimitive("explorer"),
                ),
            )
        })
        val card = state.items.filterIsInstance<ChatItem.Subagent>().single()
        assertEquals("finished", card.status)
        assertEquals("Find auth", card.task)
        assertEquals("found", card.output)
        val prompt = state.items.filterIsInstance<ChatItem.User>().single()
        assertEquals("Find auth", prompt.text)
        assertEquals("a1", prompt.agentId)
        assertNull(ChatStateOps.userMessageOrdinal(state, prompt.id))
    }

    @Test
    fun `hides orchestrator task tool calls`() {
        val byName = ChatStateOps.applyUpdate(ChatStateOps.initialState(), update("tool_call") {
            copy(toolCallId = "t1", name = "task", title = "task: explorer", kind = "think", status = "pending")
        })
        assertTrue(byName.items.isEmpty())

        val byTitle = ChatStateOps.applyUpdate(ChatStateOps.initialState(), update("tool_call") {
            copy(toolCallId = "t2", title = "task: explorer", kind = "think", status = "pending")
        })
        assertTrue(byTitle.items.isEmpty())
    }

    @Test
    fun `records tool name and owner`() {
        val state = ChatStateOps.applyUpdate(ChatStateOps.initialState(), update("tool_call") {
            copy(
                toolCallId = "t1",
                name = "read",
                title = "read: a.ts",
                kind = "read",
                status = "pending",
                meta = meta(
                    "com.pegg/agentId" to JsonPrimitive("a1"),
                    "com.pegg/agentName" to JsonPrimitive("explorer"),
                ),
            )
        })
        val item = state.items.single() as ChatItem.Tool
        assertEquals("read", item.tool.name)
        assertEquals("a1", item.tool.agentId)
        assertEquals("explorer", item.tool.agentName)
    }

    @Test
    fun `keeps the stored id of a replayed user message`() {
        val state = ChatStateOps.applyUpdate(ChatStateOps.initialState(), update("user_message_chunk") {
            copy(messageId = "stored-1", content = text("replayed"))
        })
        val item = state.items.single() as ChatItem.User
        assertEquals("stored-1", item.messageId)
        assertNotNull(item.text)
        assertTrue(item.text.isNotEmpty())
    }

    @Test
    fun `renders attachment-only user messages and merges their text`() {
        val attachment = AttachmentInfo(name = "report.pdf", mediaType = "application/pdf", type = "file")
        var state = ChatStateOps.applyUpdate(ChatStateOps.initialState(), update("user_message_chunk") {
            copy(messageId = "m1", content = text(""), meta = attachmentsMeta(attachment))
        })
        var item = state.items.single() as ChatItem.User
        assertEquals("", item.text)
        assertEquals(1, item.attachments.size)
        assertEquals("report.pdf", item.attachments.single().name)

        state = ChatStateOps.applyUpdate(state, update("user_message_chunk") {
            copy(messageId = "m1", content = text("see?"))
        })
        item = state.items.single() as ChatItem.User
        assertEquals("see?", item.text)
        assertEquals(1, item.attachments.size)
    }

    @Test
    fun `appended user messages carry their attachments`() {
        val attachment = AttachmentInfo(name = "shot.png", mediaType = "image/png", type = "image")
        val state = ChatStateOps.appendUser(ChatStateOps.initialState(), "look", listOf(attachment))
        val item = state.items.single() as ChatItem.User
        assertEquals("look", item.text)
        assertEquals("shot.png", item.attachments.single().name)
    }

    private fun ChatItem.kindName(): String = when (this) {
        is ChatItem.User -> "user"
        is ChatItem.Assistant -> "assistant"
        is ChatItem.Reasoning -> "reasoning"
        is ChatItem.Tool -> "tool"
        is ChatItem.Subagent -> "subagent"
        is ChatItem.Notice -> "notice"
        is ChatItem.Error -> "error"
    }
}
