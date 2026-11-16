package dev.pegg.intellij.ui

import com.intellij.ide.ui.LafManagerListener
import com.intellij.openapi.Disposable
import com.intellij.openapi.application.ApplicationManager
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
import dev.pegg.intellij.log.PeggLog
import kotlinx.coroutines.launch
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.encodeToJsonElement
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put
import org.cef.browser.CefBrowser
import org.cef.browser.CefFrame
import org.cef.handler.CefLoadHandler
import org.cef.handler.CefLoadHandlerAdapter
import java.awt.BorderLayout
import javax.swing.JComponent
import javax.swing.JLabel

class PluginPagesPanel(
    private val service: PeggService,
) : Disposable {
    private val controller get() = service.controller
    @Volatile private var browser: JBCefBrowser? = null
    @Volatile private var query: JBCefJSQuery? = null
    @Volatile private var disposed = false

    private val lafListener = LafManagerListener { applyTheme() }
    private val lafConnection = ApplicationManager.getApplication().messageBus.connect(this)

    val component: JComponent

    init {
        component = createComponent()
        lafConnection.subscribe(LafManagerListener.TOPIC, lafListener)
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
                    .onFailure { PeggLog.warn("Plugin page message failed: ${it.message}") }
                null
            }
            created.jbCefClient.addLoadHandler(
                object : CefLoadHandlerAdapter() {
                    override fun onLoadEnd(browser: CefBrowser?, frame: CefFrame?, httpStatusCode: Int) {
                        if (frame?.isMain == true) {
                            PeggLog.info("Pegg plugin page loaded", "status=$httpStatusCode")
                            installBridge()
                            applyTheme()
                            sendInit()
                        }
                    }

                    override fun onLoadError(
                        browser: CefBrowser?,
                        frame: CefFrame?,
                        errorCode: CefLoadHandler.ErrorCode?,
                        errorText: String?,
                        failedUrl: String?,
                    ) {
                        PeggLog.error("Pegg plugin page failed: $errorCode $errorText $failedUrl")
                    }
                },
                created.cefBrowser,
            )
            created.component.background = UIUtil.getPanelBackground()
            val url = WebViewAssets.ensurePluginPage().toURI().toURL().toString()
            PeggLog.info("Loading Pegg plugin page", url)
            created.loadURL(url)
            created.component
        }.getOrElse {
            PeggLog.error("Could not create the Pegg plugin page view", it)
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

    private fun applyTheme() {
        val obj = JsonObject(ThemeCss.variables().mapValues { JsonPrimitive(it.value) })
        val scheme = if (com.intellij.ui.JBColor.isBright()) "light" else "dark"
        execute("document.documentElement.style.colorScheme = '$scheme'; if (window.__peggTheme) window.__peggTheme(${obj});")
    }

    private fun execute(js: String) {
        val current = browser ?: return
        ApplicationManager.getApplication().invokeLater {
            if (disposed) return@invokeLater
            runCatching { current.cefBrowser.executeJavaScript(js, current.cefBrowser.url ?: "about:blank", 0) }
                .onFailure { PeggLog.warn("Plugin page JS failed: ${it.message}") }
        }
    }

    private fun postJson(json: String) {
        val literal = JsonPrimitive(json).toString()
        execute("window.postMessage(JSON.parse($literal), '*');")
    }

    private fun sendInit() {
        service.scope.launch {
            try {
                val pages = controller.listPluginPages()
                val first = pages.firstOrNull()
                val page = first?.let { runCatching { controller.getPluginPage(it.owner, it.id) }.getOrDefault(it) }
                postJson(
                    buildJsonObject {
                        put("type", "init")
                        put("pages", ACP_JSON.encodeToJsonElement(pages))
                        if (page != null) put("page", ACP_JSON.encodeToJsonElement(page))
                    }.toString(),
                )
            } catch (error: Throwable) {
                postError(error, null)
            }
        }
    }

    private fun handleMessage(request: String) {
        val obj = runCatching { ACP_JSON.parseToJsonElement(request).jsonObject }.getOrNull() ?: return
        when (obj["type"]?.jsonPrimitive?.contentOrNull) {
            "ready" -> sendInit()
            "list" -> listPages(obj.text("requestId"))
            "get", "refresh" -> getPage(obj)
            "action" -> runAction(obj)
            else -> PeggLog.debug("Unknown plugin page message: $request")
        }
    }

    private fun listPages(requestId: String) {
        service.scope.launch {
            try {
                val pages = controller.listPluginPages()
                postJson(
                    buildJsonObject {
                        put("type", "pages")
                        put("requestId", requestId)
                        put("pages", ACP_JSON.encodeToJsonElement(pages))
                    }.toString(),
                )
            } catch (error: Throwable) {
                postError(error, requestId)
            }
        }
    }

    private fun getPage(obj: JsonObject) {
        val requestId = obj.text("requestId")
        val owner = obj.text("owner")
        val pageId = obj.text("pageId")
        service.scope.launch {
            try {
                val page = controller.getPluginPage(owner, pageId)
                postJson(
                    buildJsonObject {
                        put("type", "page")
                        put("requestId", requestId)
                        put("page", ACP_JSON.encodeToJsonElement(page))
                    }.toString(),
                )
            } catch (error: Throwable) {
                postError(error, requestId)
            }
        }
    }

    private fun runAction(obj: JsonObject) {
        val requestId = obj.text("requestId")
        val owner = obj.text("owner")
        val pageId = obj.text("pageId")
        val actionId = obj.text("actionId")
        val values = (obj["values"] as? JsonObject)?.entries?.associate { (key, value) ->
            key to (value.jsonPrimitive.contentOrNull ?: "")
        } ?: emptyMap()
        service.scope.launch {
            try {
                val result = controller.runPluginPageAction(owner, pageId, actionId, values)
                postJson(
                    buildJsonObject {
                        put("type", "actionResult")
                        put("requestId", requestId)
                        result.output?.let { put("output", it) }
                        result.page?.let { put("page", ACP_JSON.encodeToJsonElement(it)) }
                    }.toString(),
                )
            } catch (error: Throwable) {
                postJson(
                    buildJsonObject {
                        put("type", "actionResult")
                        put("requestId", requestId)
                        put("error", errorText(error))
                    }.toString(),
                )
            }
        }
    }

    private fun postError(error: Throwable, requestId: String?) {
        postJson(
            buildJsonObject {
                put("type", "error")
                if (!requestId.isNullOrBlank()) put("requestId", requestId)
                put("error", errorText(error))
            }.toString(),
        )
    }

    private fun JsonObject.text(key: String): String =
        this[key]?.jsonPrimitive?.contentOrNull ?: ""

    private fun errorText(error: Throwable): String =
        error.message ?: error.javaClass.simpleName

    private fun fallback(reason: String? = null): JComponent {
        val panel = JBPanel<JBPanel<*>>(BorderLayout())
        panel.border = JBUI.Borders.empty(16)
        val text = buildString {
            append("<html><b>JCEF is not available in this IDE.</b><br/><br/>")
            append("Pegg's plugin pages use the IDE's embedded browser. Enable JCEF in the IDE runtime")
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
        runCatching { lafConnection.disconnect() }
    }
}
