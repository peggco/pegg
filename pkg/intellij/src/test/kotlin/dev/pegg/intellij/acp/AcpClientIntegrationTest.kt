package dev.pegg.intellij.acp

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import org.junit.jupiter.api.Assumptions.assumeTrue
import java.io.File
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertTrue

class AcpClientIntegrationTest {

    private fun binary(): File {
        val configured = System.getenv("PEGG_TEST_BINARY")
        if (!configured.isNullOrBlank()) return File(configured)
        return File("../../bin/pegg").absoluteFile
    }

    @Test
    fun `initializes and manages a real ACP session`() {
        val binary = binary()
        assumeTrue(binary.isFile, "Pegg binary not found at ${binary.absolutePath}")

        val cwd = Files.createTempDirectory("pegg-acp-test-").toFile()
        val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
        var created: String? = null
        val client = AcpClient(
            cwd = cwd,
            binary = binary.absolutePath,
            environment = emptyMap(),
            version = "test",
            scope = scope,
            onNotification = { _, _ -> },
            onServerRequest = { method, _ ->
                when (method) {
                    "session/request_permission" -> buildJsonObject {
                        put("outcome", buildJsonObject {
                            put("outcome", "selected")
                            put("optionId", "reject_once")
                        })
                    }
                    "pegg/ask" -> buildJsonObject {
                        put("answers", buildJsonObject { })
                    }
                    else -> throw IllegalStateException("Unsupported method: $method")
                }
            },
            onExit = { },
        )

        try {
            client.start()
            runBlocking {
                val init = client.initialize()
                assertTrue(init.protocolVersion >= 1)

                val newSession = ACP_JSON.decodeFromJsonElement(
                    SessionNewResult.serializer(),
                    client.request(
                        "session/new",
                        buildJsonObject { put("cwd", cwd.path) },
                        60_000,
                    ),
                )
                assertTrue(newSession.sessionId.isNotBlank())
                created = newSession.peggSessionId

                val models = ACP_JSON.decodeFromJsonElement(
                    ModelListResult.serializer(),
                    client.request("pegg/models", buildJsonObject { }, 60_000),
                )
                assertTrue(models.models.size >= 0)

                client.request(
                    "session/set_mode",
                    buildJsonObject {
                        put("sessionId", newSession.sessionId)
                        put("modeId", "plan")
                    },
                    30_000,
                )
                client.request(
                    "session/rename",
                    buildJsonObject {
                        put("sessionId", newSession.sessionId)
                        put("title", "ACP IntelliJ test")
                    },
                    30_000,
                )
                val sessions = ACP_JSON.decodeFromJsonElement(
                    SessionListResult.serializer(),
                    client.request(
                        "session/list",
                        buildJsonObject {
                            put("cwd", cwd.path)
                            put("all", true)
                            put("limit", 10)
                        },
                        60_000,
                    ),
                )
                assertTrue(sessions.sessions.any { it.id == newSession.peggSessionId })
            }
        } finally {
            runBlocking {
                created?.let {
                    runCatching {
                        client.request(
                            "session/delete",
                            buildJsonObject { put("sessionId", it) },
                            30_000,
                        )
                    }
                }
            }
            client.close()
            scope.cancel()
            cwd.deleteRecursively()
        }
    }
}
