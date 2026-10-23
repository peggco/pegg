package dev.pegg.intellij.chat

import dev.pegg.intellij.acp.ACP_JSON
import dev.pegg.intellij.acp.AskQuestion
import dev.pegg.intellij.acp.ChatItem
import dev.pegg.intellij.acp.ChatState
import dev.pegg.intellij.acp.PendingInteraction
import dev.pegg.intellij.acp.PermissionOption
import dev.pegg.intellij.acp.ToolState
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class ChatStateSerializationTest {

    @Test
    fun `always includes arrays required by the shared webview`() {
        val json = ACP_JSON.encodeToString(ChatState.serializer(), ChatState())
        val obj = ACP_JSON.parseToJsonElement(json).jsonObject
        assertTrue(obj.containsKey("items"))
        assertTrue(obj.containsKey("plan"))
        assertTrue(obj.containsKey("pending"))
        assertTrue(obj.containsKey("composerAttachments"))
        assertTrue(obj.containsKey("commands"))
        assertTrue(obj["items"] is kotlinx.serialization.json.JsonArray)
    }

    @Test
    fun `serializes the wire shape consumed by the shared webview`() {
        val state = ChatState(
            acpSessionId = "acp-1",
            peggSessionId = "pegg-1",
            title = "Test",
            mode = "build",
            connected = true,
            running = true,
            items = mutableListOf(
                ChatItem.User("user-1", "hello", 1),
                ChatItem.Assistant("a-1", "hi", 2, agentName = "orchestrator"),
                ChatItem.Tool(
                    "t-1",
                    ToolState(
                        id = "t-1",
                        name = "bash",
                        title = "bash: ls",
                        kind = "execute",
                        status = "completed",
                        output = kotlinx.serialization.json.JsonPrimitive("ok"),
                    ),
                    3,
                ),
                ChatItem.Subagent("subagent-a1", "a1", "explorer", "finished", "find", "found", null, 4),
                ChatItem.Reasoning("r-1", "thinking", 5),
                ChatItem.Notice("n-1", "note", "warn", 6),
                ChatItem.Error("e-1", "boom", 7),
            ),
            pending = mutableListOf(
                PendingInteraction.Permission(
                    "p-1",
                    "Allow?",
                    listOf(PermissionOption("allow_once", "Allow", "allow_once")),
                ),
                PendingInteraction.Question(
                    "q-1",
                    "explorer",
                    listOf(AskQuestion("x", "Which?", "select", listOf("a", "b"))),
                    linkedMapOf("x" to "a"),
                ),
            ),
            usage = dev.pegg.intellij.acp.UsageState(10, 100, 0.01),
        )

        val json = ACP_JSON.encodeToString(ChatState.serializer(), state)
        val obj = ACP_JSON.parseToJsonElement(json).jsonObject
        assertEquals("acp-1", obj["acpSessionId"]?.jsonPrimitive?.content)
        assertEquals("pegg-1", obj["peggSessionId"]?.jsonPrimitive?.content)
        assertEquals("build", obj["mode"]?.jsonPrimitive?.content)
        assertEquals(10, obj["usage"]?.jsonObject?.get("used")?.jsonPrimitive?.content?.toInt())

        val items = obj["items"]?.jsonArray ?: error("missing items")
        assertEquals(
            listOf("user", "assistant", "tool", "subagent", "reasoning", "notice", "error"),
            items.map { it.jsonObject["kind"]?.jsonPrimitive?.content },
        )
        val tool = items[2].jsonObject
        assertTrue(tool.containsKey("tool"))
        assertEquals("bash", tool["tool"]?.jsonObject?.get("name")?.jsonPrimitive?.content)

        val pending = obj["pending"]?.jsonArray ?: error("missing pending")
        assertEquals("permission", pending[0].jsonObject["kind"]?.jsonPrimitive?.content)
        assertEquals("question", pending[1].jsonObject["kind"]?.jsonPrimitive?.content)
        assertEquals(
            "allow_once",
            pending[0].jsonObject["options"]?.jsonArray?.first()?.jsonObject?.get("optionId")?.jsonPrimitive?.content,
        )
    }
}
