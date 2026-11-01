package dev.pegg.intellij.ui

import com.intellij.ide.ui.LafManagerListener
import com.intellij.openapi.Disposable
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.fileChooser.FileChooser
import com.intellij.openapi.fileChooser.FileChooserDescriptorFactory
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.Messages
import com.intellij.openapi.ui.popup.JBPopupFactory
import com.intellij.ui.components.JBLabel
import com.intellij.ui.components.JBPanel
import com.intellij.ui.jcef.JBCefApp
import com.intellij.ui.jcef.JBCefBrowser
import com.intellij.ui.jcef.JBCefBrowserBase
import com.intellij.ui.jcef.JBCefClient
import com.intellij.ui.jcef.JBCefJSQuery
import com.intellij.util.ui.JBUI
import com.intellij.util.ui.UIUtil
import dev.pegg.intellij.PeggService
import dev.pegg.intellij.acp.ACP_JSON
import dev.pegg.intellij.acp.ChatSettings
import dev.pegg.intellij.acp.ChatState
import dev.pegg.intellij.acp.ModelEntry
import dev.pegg.intellij.chat.Attachments
import dev.pegg.intellij.chat.ContextBuilder
import dev.pegg.intellij.log.PeggLog
import dev.pegg.intellij.settings.PeggSettings
import kotlinx.coroutines.launch
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.encodeToJsonElement
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put
import org.cef.CefSettings
import org.cef.browser.CefBrowser
import org.cef.browser.CefFrame
import org.cef.handler.CefDisplayHandlerAdapter
import org.cef.handler.CefLoadHandler
import org.cef.handler.CefLoadHandlerAdapter
import java.awt.BorderLayout
import java.util.concurrent.atomic.AtomicLong
import javax.swing.JComponent
import javax.swing.JLabel
import javax.swing.Timer

class ChatWebView(
    private val project: Project,
    private val service: PeggService,
) : Disposable {

    private val controller get() = service.controller
    @Volatile private var browser: JBCefBrowser? = null
    @Volatile private var query: JBCefJSQuery? = null
    @Volatile private var disposed = false

    private val stateTimer = Timer(24) { postStateNow() }.apply { isRepeats = false }
    private val stateVersion = AtomicLong()
    private val stateListener: (ChatState) -> Unit = { scheduleState() }
    private val composerListener: (String) -> Unit = { postComposerText(it) }
    private val lafListener = LafManagerListener { applyTheme() }
    private val lafConnection = ApplicationManager.getApplication().messageBus.connect(this)
    private val settingsListener: () -> Unit = { scheduleState() }

    val component: JComponent

    init {
        component = createComponent()
        controller.addStateListener(stateListener)
        controller.addComposerTextListener(composerListener)
        lafConnection.subscribe(LafManagerListener.TOPIC, lafListener)
        PeggSettings.getInstance().addListener(settingsListener)
    }

    fun scheduleState() {
        if (disposed) return
        ApplicationManager.getApplication().invokeLater {
            if (!disposed) stateTimer.restart()
        }
    }

    fun postStateNow() {
        if (disposed) return
        val version = stateVersion.incrementAndGet()
        val state = controller.snapshotState()
        val settings = PeggSettings.getInstance().state
        service.scope.launch {
            val payload = buildJsonObject {
                put("type", "state")
                put("state", ACP_JSON.encodeToJsonElement(ChatState.serializer(), state))
                put(
                    "settings",
                    ACP_JSON.encodeToJsonElement(
                        ChatSettings.serializer(),
                        ChatSettings(settings.sendOnEnter, settings.defaultMode),
                    ),
                )
            }
            if (!disposed && stateVersion.get() == version) postJson(payload.toString())
        }
    }

    fun postFiles() {
        service.scope.launch {
            val files = ContextBuilder.listFiles(project)
            val payload = buildJsonObject {
                put("type", "files")
                put("files", JsonArray(files.map { JsonPrimitive(it) }))
            }
            postJson(payload.toString())
        }
    }

    fun postComposerText(text: String) {
        val payload = buildJsonObject {
            put("type", "composerText")
            put("text", text)
        }
        postJson(payload.toString())
    }

    fun applyTheme() {
        val obj = JsonObject(ThemeCss.variables().mapValues { JsonPrimitive(it.value) })
        val scheme = if (com.intellij.ui.JBColor.isBright()) "light" else "dark"
        execute("document.documentElement.style.colorScheme = '$scheme'; if (window.__peggTheme) window.__peggTheme(${obj});")
    }

    private fun createComponent(): JComponent {
        if (!JBCefApp.isSupported()) return fallback()
        return runCatching {
            val client = JBCefApp.getInstance().createClient()
            client.setProperty(JBCefClient.Properties.JS_QUERY_POOL_SIZE, 1)
            val created = JBCefBrowser.createBuilder().setClient(client).setCreateImmediately(true).build()
            browser = created
            val jsQuery = JBCefJSQuery.create(created as JBCefBrowserBase)
            query = jsQuery
            jsQuery.addHandler { request ->
                runCatching { handleMessage(request) }
                    .onFailure { PeggLog.warn("Webview message failed: ${it.message}") }
                null
            }
            created.jbCefClient.addLoadHandler(
                object : CefLoadHandlerAdapter() {
                    override fun onLoadStart(browser: CefBrowser?, frame: CefFrame?, transitionType: org.cef.network.CefRequest.TransitionType?) {
                        if (frame?.isMain == true) PeggLog.info("Pegg webview load started")
                    }

                    override fun onLoadEnd(browser: CefBrowser?, frame: CefFrame?, httpStatusCode: Int) {
                        if (frame?.isMain == true) {
                            PeggLog.info("Pegg webview load finished", "status=$httpStatusCode")
                            installBridge()
                            applyTheme()
                            postStateNow()
                        }
                    }

                    override fun onLoadError(
                        browser: CefBrowser?,
                        frame: CefFrame?,
                        errorCode: CefLoadHandler.ErrorCode?,
                        errorText: String?,
                        failedUrl: String?,
                    ) {
                        PeggLog.error("Pegg webview load failed: $errorCode $errorText $failedUrl")
                    }
                },
                created.cefBrowser,
            )
            created.jbCefClient.addDisplayHandler(
                object : CefDisplayHandlerAdapter() {
                    override fun onConsoleMessage(
                        browser: CefBrowser?,
                        level: CefSettings.LogSeverity?,
                        message: String?,
                        source: String?,
                        line: Int,
                    ): Boolean {
                        if (level == CefSettings.LogSeverity.LOGSEVERITY_ERROR) {
                            PeggLog.warn("Pegg webview console $message ($source:$line)")
                        } else {
                            PeggLog.debug("Pegg webview console $message ($source:$line)")
                        }
                        return false
                    }
                },
                created.cefBrowser,
            )
            created.component.background = UIUtil.getPanelBackground()
            val url = WebViewAssets.ensure().toURI().toURL().toString()
            PeggLog.info("Loading Pegg webview", url)
            created.loadURL(url)
            created.component
        }.getOrElse {
            PeggLog.error("Could not create the Pegg webview", it)
            fallback(it.message)
        }
    }

    private fun installBridge() {
        val jsQuery = query ?: return
        execute(
            "window.cefQuery = function(q) { ${jsQuery.inject("q.request")} };" +
                "if (window.__peggDrainOutbox) window.__peggDrainOutbox();",
        )
    }

    private fun execute(js: String) {
        val current = browser ?: return
        ApplicationManager.getApplication().invokeLater {
            if (disposed) return@invokeLater
            runCatching { current.cefBrowser.executeJavaScript(js, current.cefBrowser.url ?: "about:blank", 0) }
                .onFailure { PeggLog.warn("Webview JS failed: ${it.message}") }
        }
    }

    private fun postJson(json: String) {
        val literal = JsonPrimitive(json).toString()
        execute("window.postMessage(JSON.parse($literal), '*');")
    }

    private fun handleMessage(request: String) {
        val obj = runCatching { ACP_JSON.parseToJsonElement(request).jsonObject }.getOrNull() ?: return
        when (val type = obj["type"]?.jsonPrimitive?.contentOrNull) {
            "ready" -> {
                postStateNow()
                postFiles()
            }
            "listFiles" -> postFiles()
            "send" -> {
                val text = obj["text"]?.jsonPrimitive?.contentOrNull ?: ""
                val attachments = controller.state.composerAttachments.toList()
                ApplicationManager.getApplication().invokeLater {
                    val context = runCatching { ContextBuilder.activeContext(project) }.getOrDefault("")
                    service.scope.launch {
                        runCatching { controller.sendPrompt(text, context = context, attachments = attachments) }
                            .onFailure { showError(it) }
                    }
                }
            }
            "stop" -> service.scope.launch { runCatching { controller.cancel() } }
            "forkMessage" -> {
                val id = obj["id"]?.jsonPrimitive?.contentOrNull ?: return
                service.scope.launch { runCatching { controller.forkMessage(id) }.onFailure { showError(it) } }
            }
            "revertMessage" -> {
                val id = obj["id"]?.jsonPrimitive?.contentOrNull ?: return
                service.scope.launch { runCatching { controller.revertMessage(id) }.onFailure { showError(it) } }
            }
            "newSession" -> service.scope.launch { runCatching { controller.newSession() }.onFailure { showError(it) } }
            "loadOlderMessages" -> service.scope.launch {
                runCatching { controller.loadOlderMessages() }.onFailure { showError(it) }
            }
            "setMode" -> {
                val mode = obj["mode"]?.jsonPrimitive?.contentOrNull ?: return
                service.scope.launch { runCatching { controller.setMode(mode) }.onFailure { showError(it) } }
            }
            "pickMode" -> ApplicationManager.getApplication().invokeLater { showModePicker() }
            "pickModel" -> service.scope.launch {
                runCatching { controller.models() }
                    .onSuccess { postModels(it) }
                    .onFailure {
                        postModels(emptyList())
                        showError(it)
                    }
            }
            "setModel" -> {
                val modelProvider = obj["provider"]?.jsonPrimitive?.contentOrNull ?: return
                val modelId = obj["model"]?.jsonPrimitive?.contentOrNull ?: return
                service.scope.launch {
                    runCatching { controller.setModel(modelProvider, modelId) }
                        .onFailure { showError(it) }
                }
            }
            "pickFiles" -> ApplicationManager.getApplication().invokeLater { chooseFiles() }
            "removeAttachment" -> {
                val id = obj["id"]?.jsonPrimitive?.contentOrNull ?: return
                controller.removeComposerAttachment(id)
            }
            "attachPaths" -> {
                val paths = (obj["paths"] as? JsonArray)
                    ?.mapNotNull { it.jsonPrimitive.contentOrNull }
                    ?: emptyList()
                for (path in paths) {
                    runCatching { Attachments.makeAttachment(project, path) }
                        .getOrNull()
                        ?.let { controller.addComposerAttachment(it) }
                }
            }
            "addAttachmentData" -> {
                val name = obj["name"]?.jsonPrimitive?.contentOrNull ?: "attachment"
                val mediaType = obj["mediaType"]?.jsonPrimitive?.contentOrNull ?: "application/octet-stream"
                val data = obj["data"]?.jsonPrimitive?.contentOrNull ?: ""
                val size = obj["size"]?.jsonPrimitive?.contentOrNull?.toLongOrNull() ?: 0L
                controller.addComposerAttachment(Attachments.makeDataAttachment(name, mediaType, data, size))
            }
            "permissionResponse" -> {
                val id = obj["id"]?.jsonPrimitive?.contentOrNull ?: return
                val optionId = obj["optionId"]?.jsonPrimitive?.contentOrNull ?: return
                controller.respondPermission(id, optionId)
            }
            "questionResponse" -> {
                val id = obj["id"]?.jsonPrimitive?.contentOrNull ?: return
                val answers = (obj["answers"] as? JsonObject)
                    ?.mapValues { it.value.jsonPrimitive.contentOrNull ?: "" }
                    ?: emptyMap()
                controller.respondQuestion(id, answers)
            }
            "startVoice" -> service.scope.launch {
                runCatching { controller.startVoice() }
                    .onSuccess { postRecording(true) }
                    .onFailure {
                        postRecording(false)
                        postVoiceError(it.message ?: "Voice input is not available")
                    }
            }
            "stopVoice" -> {
                postRecording(false)
                service.scope.launch {
                    runCatching { controller.stopVoice() }
                        .onSuccess { postTranscript(it) }
                        .onFailure { postVoiceError(it.message ?: "Voice input failed") }
                }
            }
            "transcribe" -> {
                val audio = obj["audio"]?.jsonPrimitive?.contentOrNull ?: return
                val mediaType = obj["mediaType"]?.jsonPrimitive?.contentOrNull ?: "audio/webm"
                val fileName = obj["fileName"]?.jsonPrimitive?.contentOrNull
                service.scope.launch {
                    runCatching { controller.transcribe(audio, mediaType, fileName) }
                        .onSuccess { postTranscript(it) }
                        .onFailure { postVoiceError(it.message ?: "Transcription failed") }
                }
            }
            "openFile" -> {
                val path = obj["path"]?.jsonPrimitive?.contentOrNull ?: return
                val line = obj["line"]?.jsonPrimitive?.contentOrNull?.toIntOrNull() ?: 1
                ApplicationManager.getApplication().invokeLater { IdeActions.openFile(project, path, line) }
            }
            "openDiff" -> {
                val path = obj["path"]?.jsonPrimitive?.contentOrNull ?: return
                val oldText = obj["oldText"]?.jsonPrimitive?.contentOrNull ?: ""
                val newText = obj["newText"]?.jsonPrimitive?.contentOrNull ?: ""
                ApplicationManager.getApplication().invokeLater {
                    IdeActions.openDiff(project, path, oldText, newText)
                }
            }
            "openExternal" -> {
                val url = obj["url"]?.jsonPrimitive?.contentOrNull ?: return
                ApplicationManager.getApplication().invokeLater { IdeActions.openExternal(url) }
            }
            "openSettings" -> ApplicationManager.getApplication().invokeLater {
                IdeActions.openSettings(project)
            }
            "showLogs" -> ApplicationManager.getApplication().invokeLater {
                IdeActions.showLogs(project)
            }
            "restart" -> service.scope.launch { runCatching { controller.restart() }.onFailure { showError(it) } }
            "copyText" -> {
                val text = obj["text"]?.jsonPrimitive?.contentOrNull ?: return
                ApplicationManager.getApplication().invokeLater { IdeActions.copy(text) }
            }
            else -> PeggLog.debug("Unknown webview message: $type")
        }
    }

    private fun chooseFiles() {
        val descriptor = FileChooserDescriptorFactory.createMultipleFilesNoJarsDescriptor()
        val files = FileChooser.chooseFiles(descriptor, project, null)
        for (file in files) {
            runCatching { Attachments.makeAttachment(project, file) }
                .getOrNull()
                ?.let { controller.addComposerAttachment(it) }
        }
    }

    private fun showModePicker() {
        val modes = listOf("Auto", "Plan", "Build")
        JBPopupFactory.getInstance()
            .createPopupChooserBuilder(modes)
            .setTitle("Pegg mode")
            .setNamerForFiltering { it }
            .setItemChosenCallback { chosen ->
                service.scope.launch {
                    runCatching { controller.setMode(chosen.lowercase()) }.onFailure { showError(it) }
                }
            }
            .createPopup()
            .showInCenterOf(component)
    }

    private fun postModels(models: List<ModelEntry>) {
        val array = JsonArray(models.map { entry ->
            buildJsonObject {
                put("provider", entry.provider)
                put("model", buildJsonObject {
                    put("id", entry.model.id)
                    entry.model.name?.takeIf { it.isNotBlank() }?.let { put("name", it) }
                })
            }
        })
        postJson(buildJsonObject {
            put("type", "models")
            put("models", array)
        }.toString())
    }

    private fun postRecording(recording: Boolean) {
        postJson(buildJsonObject {
            put("type", "recording")
            put("recording", recording)
        }.toString())
    }

    private fun postVoiceError(message: String) {
        postJson(buildJsonObject {
            put("type", "voiceError")
            put("error", message)
        }.toString())
    }

    private fun postTranscript(text: String) {
        postJson(buildJsonObject {
            put("type", "transcript")
            put("text", text)
        }.toString())
    }

    private fun showError(error: Throwable) {
        PeggLog.error("Webview action failed: ${error.message}", error)
        ApplicationManager.getApplication().invokeLater {
            Messages.showErrorDialog(project, error.message ?: error.javaClass.simpleName, "Pegg")
        }
    }

    private fun fallback(reason: String? = null): JComponent {
        val panel = JBPanel<JBPanel<*>>(BorderLayout())
        panel.border = JBUI.Borders.empty(16)
        val text = buildString {
            append("<html><b>JCEF is not available in this IDE.</b><br/><br/>")
            append("Pegg's chat UI uses the IDE's embedded browser. Enable JCEF in the IDE runtime")
            append(" (Help → Find Action → Choose Boot Java Runtime) and restart the IDE.")
            if (!reason.isNullOrBlank()) append("<br/><br/>${reason.replace("<", "&lt;")}")
            append("</html>")
        }
        panel.add(JBLabel(text), BorderLayout.NORTH)
        panel.add(JLabel(""), BorderLayout.CENTER)
        return panel
    }

    override fun dispose() {
        disposed = true
        stateTimer.stop()
        controller.removeStateListener(stateListener)
        controller.removeComposerTextListener(composerListener)
        runCatching { lafConnection.disconnect() }
        runCatching { PeggSettings.getInstance().removeListener(settingsListener) }
        runCatching { browser?.dispose() }
        browser = null
        query = null
    }
}
