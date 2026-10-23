package dev.pegg.intellij.ui

import com.intellij.openapi.project.Project
import com.intellij.openapi.util.Disposer
import com.intellij.openapi.wm.ToolWindow
import com.intellij.openapi.wm.ToolWindowFactory
import com.intellij.ui.content.ContentFactory
import dev.pegg.intellij.PeggService
import dev.pegg.intellij.actions.PeggNewSessionAction
import dev.pegg.intellij.actions.PeggSettingsAction
import dev.pegg.intellij.actions.PeggShowLogsAction
import dev.pegg.intellij.actions.PeggStopAction

class PeggToolWindowFactory : ToolWindowFactory {
    override fun createToolWindowContent(project: Project, toolWindow: ToolWindow) {
        val service = PeggService.getInstance(project)
        val chat = ChatWebView(project, service)

        val factory = ContentFactory.getInstance()
        val chatContent = factory.createContent(chat.component, "Pegg", false)
        val sessions = SessionsPanel(
            project = project,
            service = service,
            openChat = { toolWindow.contentManager.setSelectedContent(chatContent) },
        )
        val sessionsContent = factory.createContent(sessions.component, "Sessions", false)

        toolWindow.contentManager.addContent(chatContent)
        toolWindow.contentManager.addContent(sessionsContent)
        toolWindow.contentManager.setSelectedContent(chatContent)
        toolWindow.setToHideOnEmptyContent(false)
        toolWindow.setTitleActions(
            listOf(
                PeggNewSessionAction(),
                PeggStopAction(),
                PeggSettingsAction(),
                PeggShowLogsAction(),
            ),
        )

        service.webView = chat
        service.sessionsPanel = sessions
        service.controller.setChatVisible {
            toolWindow.isVisible && toolWindow.contentManager.selectedContent === chatContent
        }

        Disposer.register(service, chat)
        Disposer.register(service, sessions)

        service.activate()
    }
}
