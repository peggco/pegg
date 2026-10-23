package dev.pegg.intellij.ui

import com.intellij.icons.AllIcons
import com.intellij.openapi.Disposable
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.Messages
import com.intellij.ui.CollectionListModel
import com.intellij.ui.ColoredListCellRenderer
import com.intellij.ui.SearchTextField
import com.intellij.ui.SimpleTextAttributes
import com.intellij.ui.components.JBList
import com.intellij.ui.components.JBScrollPane
import com.intellij.util.ui.JBUI
import dev.pegg.intellij.PeggService
import dev.pegg.intellij.acp.SessionListItem
import kotlinx.coroutines.launch
import java.awt.BorderLayout
import java.awt.FlowLayout
import java.awt.event.KeyAdapter
import java.awt.event.KeyEvent
import java.awt.event.MouseAdapter
import java.awt.event.MouseEvent
import javax.swing.event.DocumentEvent
import javax.swing.event.DocumentListener
import javax.swing.Icon
import javax.swing.JButton
import javax.swing.JComponent
import javax.swing.JMenuItem
import javax.swing.JPanel
import javax.swing.JPopupMenu
import javax.swing.ListSelectionModel
import javax.swing.ScrollPaneConstants

class SessionsPanel(
    private val project: Project,
    private val service: PeggService,
    private val openChat: () -> Unit,
) : Disposable {

    private val controller get() = service.controller
    private val model = CollectionListModel<SessionRow>()
    private val list = JBList(model)
    private val search = SearchTextField()
    private var allRows: List<SessionRow> = emptyList()
    private var disposed = false

    private val sessionsListener: () -> Unit = { refresh() }
    private val runListener: (dev.pegg.intellij.acp.ChatState) -> Unit = { refresh() }

    val component: JComponent

    init {
        list.selectionMode = ListSelectionModel.SINGLE_SELECTION
        list.cellRenderer = SessionRenderer()
        list.setEmptyText("No Pegg sessions yet. Start a chat.")
        list.addMouseListener(object : MouseAdapter() {
            override fun mouseClicked(event: MouseEvent) {
                if (event.clickCount == 2 && event.button == MouseEvent.BUTTON1) resumeSelected()
                if (event.isPopupTrigger) showPopup(event)
            }

            override fun mousePressed(event: MouseEvent) {
                if (event.isPopupTrigger) showPopup(event)
            }
        })
        list.addKeyListener(object : KeyAdapter() {
            override fun keyPressed(event: KeyEvent) {
                when (event.keyCode) {
                    KeyEvent.VK_ENTER -> resumeSelected()
                    KeyEvent.VK_F2 -> renameSelected()
                    KeyEvent.VK_DELETE -> deleteSelected()
                }
            }
        })
        search.addDocumentListener(object : DocumentListener {
            override fun insertUpdate(e: DocumentEvent) = applyFilter()
            override fun removeUpdate(e: DocumentEvent) = applyFilter()
            override fun changedUpdate(e: DocumentEvent) = applyFilter()
        })
        search.textEditor.toolTipText = "Search Pegg sessions"

        val toolbar = JPanel(FlowLayout(FlowLayout.RIGHT, 2, 0))
        toolbar.add(iconButton(AllIcons.General.Add, "New session") {
            service.scope.launch {
                runCatching { controller.newSession() }
                    .onFailure { service.showError("Could not start a session", it) }
                ApplicationManager.getApplication().invokeLater { openChat() }
            }
        })
        toolbar.add(iconButton(AllIcons.Actions.Refresh, "Refresh sessions") { refresh() })

        val header = JPanel(BorderLayout(6, 0))
        header.border = JBUI.Borders.empty(6, 8, 6, 8)
        header.add(search, BorderLayout.CENTER)
        header.add(toolbar, BorderLayout.EAST)

        val scroll = JBScrollPane(
            list,
            ScrollPaneConstants.VERTICAL_SCROLLBAR_AS_NEEDED,
            ScrollPaneConstants.HORIZONTAL_SCROLLBAR_NEVER,
        )
        scroll.border = JBUI.Borders.empty()

        val panel = JPanel(BorderLayout())
        panel.add(header, BorderLayout.NORTH)
        panel.add(scroll, BorderLayout.CENTER)
        component = panel

        controller.addSessionsListener(sessionsListener)
        controller.addRunFinishedListener(runListener)
        refresh()
    }

    fun refresh() {
        if (disposed) return
        service.scope.launch {
            val sessions = runCatching { controller.listSessions(true) }.getOrDefault(emptyList())
            ApplicationManager.getApplication().invokeLater {
                if (!disposed) update(sessions)
            }
        }
    }

    private fun update(sessions: List<SessionListItem>) {
        val current = controller.state.peggSessionId
        allRows = sessions.map { SessionRow.from(it, it.id == current) }
        applyFilter()
    }

    private fun applyFilter() {
        val query = search.text.trim().lowercase()
        val filtered = if (query.isBlank()) {
            allRows
        } else {
            allRows.filter {
                it.title.lowercase().contains(query) ||
                    it.id.lowercase().contains(query) ||
                    (it.model ?: "").lowercase().contains(query) ||
                    (it.mode ?: "").lowercase().contains(query)
            }
        }
        model.replaceAll(filtered)
        if (model.size > 0 && list.selectedIndex < 0) list.selectedIndex = 0
    }

    private fun selected(): SessionRow? = list.selectedValue

    private fun resumeSelected() {
        val row = selected() ?: return
        service.scope.launch {
            runCatching { controller.resumeSession(row.id) }
                .onSuccess { ApplicationManager.getApplication().invokeLater { openChat() } }
                .onFailure { service.showError("Could not resume session", it) }
        }
    }

    private fun renameSelected() {
        val row = selected() ?: return
        val title = Messages.showInputDialog(
            project,
            "New title",
            "Rename Pegg Session",
            null,
            row.title,
            null,
        )?.trim() ?: return
        if (title.isEmpty()) return
        service.scope.launch {
            runCatching { controller.renameSession(row.id, title) }
                .onFailure { service.showError("Could not rename session", it) }
        }
    }

    private fun deleteSelected() {
        val row = selected() ?: return
        val answer = Messages.showYesNoDialog(
            project,
            "Delete this Pegg session permanently?\n\n${row.title}",
            "Delete Pegg Session",
            Messages.getWarningIcon(),
        )
        if (answer != Messages.YES) return
        service.scope.launch {
            runCatching { controller.deleteSession(row.id) }
                .onFailure { service.showError("Could not delete session", it) }
        }
    }

    private fun showPopup(event: MouseEvent) {
        val index = list.locationToIndex(event.point)
        if (index >= 0) list.selectedIndex = index
        val popup = JPopupMenu()
        popup.add(menuItem("Resume Session", AllIcons.Actions.Execute) { resumeSelected() })
        popup.add(menuItem("Rename Session…", AllIcons.Actions.Edit) { renameSelected() })
        popup.add(menuItem("Delete Session…", AllIcons.Actions.DeleteTag) { deleteSelected() })
        popup.addSeparator()
        popup.add(menuItem("Refresh", AllIcons.Actions.Refresh) { refresh() })
        popup.show(list, event.x, event.y)
    }

    private fun menuItem(text: String, icon: Icon?, action: () -> Unit): JMenuItem =
        JMenuItem(text, icon).apply { addActionListener { action() } }

    private fun iconButton(icon: Icon, tooltip: String, action: () -> Unit): JButton =
        JButton(icon).apply {
            toolTipText = tooltip
            isBorderPainted = false
            isContentAreaFilled = false
            isFocusable = false
            addActionListener { action() }
        }

    override fun dispose() {
        disposed = true
        controller.removeSessionsListener(sessionsListener)
        controller.removeRunFinishedListener(runListener)
    }

    private class SessionRenderer : ColoredListCellRenderer<SessionRow>() {
        override fun customizeCellRenderer(
            list: javax.swing.JList<out SessionRow>,
            value: SessionRow,
            index: Int,
            selected: Boolean,
            hasFocus: Boolean,
        ) {
            icon = if (value.isCurrent) AllIcons.General.InspectionsOK else AllIcons.Vcs.History
            append(
                value.title.ifBlank { "Untitled session" },
                SimpleTextAttributes.REGULAR_BOLD_ATTRIBUTES,
            )
            val detail = listOfNotNull(value.model, value.mode).filter { it.isNotBlank() }.joinToString(" · ")
            if (detail.isNotBlank()) {
                append("  $detail", SimpleTextAttributes.GRAYED_SMALL_ATTRIBUTES)
            }
            toolTipText = buildString {
                append("<html><b>").append(escape(value.title)).append("</b><br/>")
                append("Project: ").append(escape(value.projectDir ?: "unknown")).append("<br/>")
                append("Model: ").append(escape(listOfNotNull(value.provider, value.model).joinToString("/")))
                append("<br/>Mode: ").append(escape(value.mode ?: "auto"))
                if (value.updatedAt != null) append("<br/>Updated: ").append(escape(value.updatedAt))
                if (value.isCurrent) append("<br/><b>Current session</b>")
                append("</html>")
            }
        }

        private fun escape(value: String): String =
            value.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
    }

    data class SessionRow(
        val id: String,
        val title: String,
        val isCurrent: Boolean,
        val provider: String?,
        val model: String?,
        val mode: String?,
        val projectDir: String?,
        val updatedAt: String?,
    ) {
        companion object {
            fun from(item: SessionListItem, current: Boolean) = SessionRow(
                id = item.id,
                title = item.title.ifBlank { "Untitled session" },
                isCurrent = current,
                provider = item.provider,
                model = item.model,
                mode = item.mode,
                projectDir = item.projectDir,
                updatedAt = item.updatedAt,
            )
        }
    }
}
