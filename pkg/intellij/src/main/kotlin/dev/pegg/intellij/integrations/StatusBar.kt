package dev.pegg.intellij.integrations

import com.intellij.openapi.Disposable
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.wm.StatusBar
import com.intellij.openapi.wm.StatusBarWidget
import com.intellij.openapi.wm.StatusBarWidgetFactory
import com.intellij.openapi.wm.ToolWindowManager
import dev.pegg.intellij.PeggService
import dev.pegg.intellij.acp.ChatState
import java.awt.Component
import java.awt.event.MouseEvent
import com.intellij.util.Consumer

class PeggStatusBarWidgetFactory : StatusBarWidgetFactory {
    override fun getId(): String = "PeggStatus"
    override fun getDisplayName(): String = "Pegg"
    override fun isAvailable(project: Project): Boolean = true
    override fun createWidget(project: Project): StatusBarWidget = PeggStatusBarWidget(project)
    override fun disposeWidget(widget: StatusBarWidget) {
        (widget as? Disposable)?.dispose()
    }
}

class PeggStatusBarWidget(private val project: Project) : StatusBarWidget, StatusBarWidget.TextPresentation, Disposable {
    private var statusBar: StatusBar? = null
    private val controller get() = PeggService.getInstance(project).controller
    private val listener: (ChatState) -> Unit = {
        ApplicationManager.getApplication().invokeLater { statusBar?.updateWidget(ID()) }
    }

    override fun ID(): String = "PeggStatus"

    override fun install(statusBar: StatusBar) {
        this.statusBar = statusBar
        controller.addStateListener(listener)
    }

    override fun getPresentation(): StatusBarWidget.WidgetPresentation = this

    override fun getText(): String {
        val state = controller.state
        if (!state.connected && state.acpSessionId == null) return "Pegg"
        if (state.running) return "Pegg · working · ${state.mode} · ${state.model ?: "auto"}"
        if (state.error != null) return "Pegg · error"
        val usage = if (state.usage.used > 0) " · ${compact(state.usage.used)} tokens" else ""
        return "Pegg · ${state.mode} · ${state.model ?: "auto"}$usage"
    }

    override fun getTooltipText(): String {
        val state = controller.state
        return buildString {
            append("Session: ${state.title}\n")
            append("Provider: ${state.provider ?: "auto"}\n")
            append("Model: ${state.model ?: "auto"}\n")
            append("Mode: ${state.mode}")
            if (state.usage.cost != null) append("\nCost: $${"%.4f".format(state.usage.cost)}")
            state.error?.let { append("\n\n$it") }
        }
    }

    override fun getAlignment(): Float = Component.CENTER_ALIGNMENT

    override fun getClickConsumer(): Consumer<MouseEvent>? = Consumer {
        ToolWindowManager.getInstance(project).getToolWindow("Pegg")?.activate(null)
    }

    override fun dispose() {
        controller.removeStateListener(listener)
        statusBar = null
    }

    private fun compact(value: Int): String = when {
        value >= 1_000_000 -> "${"%.1f".format(value / 1_000_000.0)}M"
        value >= 1_000 -> "${"%.1f".format(value / 1_000.0)}K"
        else -> value.toString()
    }
}
