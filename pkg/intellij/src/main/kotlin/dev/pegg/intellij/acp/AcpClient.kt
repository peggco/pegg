package dev.pegg.intellij.acp

import dev.pegg.intellij.log.PeggLog
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put
import java.io.File
import java.util.concurrent.CompletableFuture
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.ExecutionException
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import java.util.concurrent.atomic.AtomicLong

private const val MAX_FRAME = 64 * 1024 * 1024

class AcpClient(
    private val cwd: File,
    private val binary: String,
    private val environment: Map<String, String>,
    private val version: String,
    private val scope: CoroutineScope,
    private val onNotification: (String, JsonElement?) -> Unit,
    private val onServerRequest: suspend (String, JsonElement?) -> JsonElement,
    private val onExit: (Int?) -> Unit,
) : AutoCloseable {

    private var process: Process? = null
    private var writerJob: Job? = null
    private val outgoing = Channel<JsonObject>(Channel.UNLIMITED)
    private val notifications = Channel<Pair<String, JsonElement?>>(Channel.UNLIMITED)
    private val pending = ConcurrentHashMap<String, CompletableFuture<JsonElement>>()
    private val nextId = AtomicLong(1)

    @Volatile
    private var closed = false

    @Synchronized
    fun start() {
        if (process != null) return
        val builder = ProcessBuilder(binary, "serve", "--acp", "--stdio")
        builder.directory(cwd)
        builder.environment().putAll(environment)
        val child = builder.start()
        process = child
        PeggLog.info("Starting ACP server", binary, cwd.absolutePath)
        writerJob = scope.launch(Dispatchers.IO) { writerLoop(child) }
        scope.launch(Dispatchers.IO) {
            for ((method, params) in notifications) {
                runCatching { onNotification(method, params) }
                    .onFailure { PeggLog.warn("ACP notification handler failed: ${it.message}") }
            }
        }
        scope.launch(Dispatchers.IO) { readLoop(child) }
        scope.launch(Dispatchers.IO) { stderrLoop(child) }
    }

    suspend fun initialize(): InitializeResult {
        val params = buildJsonObject {
            put("protocolVersion", 1)
            put("clientInfo", buildJsonObject {
                put("name", "pegg-intellij")
                put("title", "Pegg for JetBrains")
                put("version", version)
            })
            put("clientCapabilities", buildJsonObject {
                put("terminal", true)
                put("fs", buildJsonObject {
                    put("readTextFile", true)
                    put("writeTextFile", false)
                })
            })
        }
        val result = request("initialize", params, 60_000)
        return ACP_JSON.decodeFromJsonElement(InitializeResult.serializer(), result)
    }

    suspend fun request(method: String, params: JsonElement? = null, timeoutMs: Long = 60_000): JsonElement {
        val child = process
        if (child == null || closed) throw IllegalStateException("Pegg server is not running")
        val id = "pegg-${nextId.getAndIncrement()}"
        val future = CompletableFuture<JsonElement>()
        pending[id] = future
        val message = buildJsonObject {
            put("jsonrpc", "2.0")
            put("id", id)
            put("method", method)
            if (params != null) put("params", params)
        }
        outgoing.send(message)
        try {
            return withContext(Dispatchers.IO) { future.get(timeoutMs, TimeUnit.MILLISECONDS) }
        } catch (e: TimeoutException) {
            pending.remove(id)
            throw IllegalStateException("Pegg request timed out: $method")
        } catch (e: ExecutionException) {
            pending.remove(id)
            throw (e.cause ?: e)
        } catch (e: InterruptedException) {
            pending.remove(id)
            Thread.currentThread().interrupt()
            throw IllegalStateException("Pegg request interrupted: $method")
        } finally {
            pending.remove(id)
        }
    }

    fun notify(method: String, params: JsonElement? = null) {
        if (closed) return
        val message = buildJsonObject {
            put("jsonrpc", "2.0")
            put("method", method)
            if (params != null) put("params", params)
        }
        outgoing.trySend(message)
    }

    fun cancelPrompt(sessionId: String) {
        notify("session/cancel", buildJsonObject { put("sessionId", sessionId) })
    }

    override fun close() {
        if (closed) return
        closed = true
        outgoing.close()
        notifications.close()
        writerJob?.cancel()
        failAll(IllegalStateException("Pegg server stopped"))
        val child = process
        process = null
        if (child == null) return
        runCatching { child.outputStream.close() }
        runCatching { child.destroy() }
        Thread {
            runCatching {
                if (!child.waitFor(2, TimeUnit.SECONDS)) child.destroyForcibly()
            }
        }.apply {
            isDaemon = true
            name = "pegg-acp-reaper"
            start()
        }
    }

    private suspend fun writerLoop(child: Process) {
        try {
            val stream = child.outputStream
            for (message in outgoing) {
                val text = ACP_JSON.encodeToString(JsonObject.serializer(), message) + "\n"
                if (text.length > MAX_FRAME) {
                    PeggLog.warn("Dropping oversized ACP frame")
                    continue
                }
                stream.write(text.toByteArray(Charsets.UTF_8))
                stream.flush()
            }
        } catch (e: Throwable) {
            if (!closed) PeggLog.warn("ACP writer stopped: ${e.message}")
        }
    }

    private fun readLoop(child: Process) {
        try {
            child.inputStream.bufferedReader(Charsets.UTF_8).useLines { lines ->
                for (line in lines) {
                    if (line.isBlank()) continue
                    if (line.length > MAX_FRAME) {
                        PeggLog.warn("Dropping oversized ACP frame")
                        continue
                    }
                    handleLine(line)
                }
            }
        } catch (e: Throwable) {
            if (!closed) PeggLog.warn("ACP reader stopped: ${e.message}")
        } finally {
            val code = runCatching { child.exitValue() }.getOrNull()
            handleExit(code)
        }
    }

    private fun stderrLoop(child: Process) {
        try {
            child.errorStream.bufferedReader(Charsets.UTF_8).useLines { lines ->
                for (line in lines) if (line.isNotBlank()) PeggLog.warn("pegg stderr: $line")
            }
        } catch (_: Throwable) {
            // Process closed.
        }
    }

    private fun handleLine(line: String) {
        val message = runCatching {
            ACP_JSON.parseToJsonElement(line).jsonObject
        }.getOrElse {
            PeggLog.warn("Invalid JSON from Pegg: ${line.take(300)}")
            return
        }
        val method = message["method"]?.jsonPrimitive?.contentOrNull
        val id = message["id"]

        if (method != null) {
            if (id != null && id !is JsonNull) {
                scope.launch {
                    val response = runCatching { onServerRequest(method, message["params"]) }
                    response.onSuccess { result -> outgoing.trySend(responseMessage(id, result, null)) }
                        .onFailure { error ->
                            outgoing.trySend(
                                responseMessage(
                                    id,
                                    null,
                                    error.message ?: error.javaClass.simpleName,
                                ),
                            )
                        }
                }
            } else {
                notifications.trySend(method to message["params"])
            }
            return
        }

        if (id == null || id is JsonNull) return
        val key = idKey(id)
        val future = pending.remove(key) ?: return
        val error = message["error"]
        if (error != null && error !is JsonNull) {
            val text = runCatching {
                error.jsonObject["message"]?.jsonPrimitive?.contentOrNull
            }.getOrNull() ?: "Pegg request failed"
            future.completeExceptionally(IllegalStateException(text))
        } else {
            future.complete(message["result"] ?: JsonNull)
        }
    }

    private fun responseMessage(id: JsonElement, result: JsonElement?, error: String?): JsonObject =
        buildJsonObject {
            put("jsonrpc", "2.0")
            put("id", id)
            if (error == null) {
                put("result", result ?: JsonNull)
            } else {
                put("error", buildJsonObject {
                    put("code", -32603)
                    put("message", error)
                })
            }
        }

    private fun idKey(id: JsonElement): String {
        val primitive = runCatching { id.jsonPrimitive }.getOrNull()
        return primitive?.contentOrNull ?: id.toString()
    }

    private fun handleExit(code: Int?) {
        if (closed) return
        closed = true
        outgoing.close()
        notifications.close()
        failAll(IllegalStateException("Pegg server exited (${code ?: "unknown"})"))
        runCatching { onExit(code) }
    }

    private fun failAll(error: Throwable) {
        for ((id, future) in pending) {
            future.completeExceptionally(error)
            pending.remove(id)
        }
    }
}
