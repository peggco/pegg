package dev.pegg.intellij

import com.intellij.notification.NotificationGroupManager
import com.intellij.notification.NotificationType
import com.intellij.notification.Notifications
import com.intellij.openapi.Disposable
import com.intellij.openapi.components.Service
import com.intellij.openapi.project.Project
import dev.pegg.intellij.binary.BinaryManager
import dev.pegg.intellij.chat.ChatController
import dev.pegg.intellij.log.PeggLog
import dev.pegg.intellij.ui.ChatWebView
import dev.pegg.intellij.ui.SessionsPanel
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch

@Service(Service.Level.PROJECT)
class PeggService(private val project: Project) : Disposable {
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    val binary = BinaryManager(project)
    val controller = ChatController(project, binary, scope)

    @Volatile
    var webView: ChatWebView? = null

    @Volatile
    var sessionsPanel: SessionsPanel? = null

    @Volatile
    var chatVisible: Boolean = false

    init {
        controller.setChatVisible { chatVisible }
    }

    fun activate() {
        scope.launch {
            runCatching { controller.start() }
                .onFailure { showError("Could not start Pegg", it) }
        }
    }

    fun restart() {
        scope.launch {
            runCatching { controller.restart() }
                .onFailure { showError("Could not restart Pegg", it) }
        }
    }

    fun refreshSessions() {
        sessionsPanel?.refresh()
    }

    fun showError(title: String, error: Throwable) {
        PeggLog.error("$title: ${error.message}", error)
        val notification = NotificationGroupManager.getInstance()
            .getNotificationGroup("Pegg")
            .createNotification(title, error.message ?: error.javaClass.simpleName, NotificationType.ERROR)
        Notifications.Bus.notify(notification, project)
    }

    override fun dispose() {
        webView = null
        sessionsPanel = null
        controller.dispose()
        scope.cancel()
    }

    companion object {
        fun getInstance(project: Project): PeggService = project.getService(PeggService::class.java)
    }
}
