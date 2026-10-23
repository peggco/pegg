package dev.pegg.intellij.ui

import com.intellij.diff.DiffContentFactory
import com.intellij.diff.DiffManager
import com.intellij.diff.requests.SimpleDiffRequest
import com.intellij.execution.ExecutionManager
import com.intellij.execution.executors.DefaultRunExecutor
import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.process.OSProcessHandler
import com.intellij.execution.ui.RunContentDescriptor
import com.intellij.ide.BrowserUtil
import com.intellij.ide.actions.ShowFilePathAction
import com.intellij.openapi.fileEditor.FileEditorManager
import com.intellij.openapi.fileEditor.OpenFileDescriptor
import com.intellij.openapi.ide.CopyPasteManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.DialogWrapper
import com.intellij.openapi.ui.Messages
import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.ui.JBColor
import com.intellij.ui.components.JBScrollPane
import com.intellij.ui.components.JBTextArea
import com.intellij.util.ui.JBUI
import com.intellij.execution.filters.TextConsoleBuilderFactory
import dev.pegg.intellij.log.PeggLog
import dev.pegg.intellij.settings.PeggConfigurable
import java.awt.BorderLayout
import java.awt.Dimension
import java.awt.datatransfer.StringSelection
import java.io.File
import javax.swing.JButton
import javax.swing.JPanel

object IdeActions {
    fun openFile(project: Project, path: String, line: Int) {
        if (path.isBlank()) return
        val target = File(path).let { if (it.isAbsolute) it else File(project.basePath ?: "", path) }
        val file = LocalFileSystem.getInstance().refreshAndFindFileByIoFile(target)
        if (file == null || !file.isValid) {
            Messages.showWarningDialog(project, "File not found: $path", "Pegg")
            return
        }
        OpenFileDescriptor(project, file, (line - 1).coerceAtLeast(0), 0).navigate(true)
    }

    fun openDiff(project: Project, path: String, oldText: String, newText: String) {
        val factory = DiffContentFactory.getInstance()
        val request = SimpleDiffRequest(
            "Pegg: ${File(path).name}",
            factory.create(oldText),
            factory.create(newText),
            "Before",
            "After",
        )
        DiffManager.getInstance().showDiff(project, request)
    }

    fun openExternal(url: String) {
        if (Regex("^https?://", RegexOption.IGNORE_CASE).containsMatchIn(url)) BrowserUtil.browse(url)
    }

    fun copy(text: String) {
        CopyPasteManager.getInstance().setContents(StringSelection(text))
    }

    fun openSettings(project: Project) {
        com.intellij.openapi.options.ShowSettingsUtil.getInstance()
            .showSettingsDialog(project, PeggConfigurable::class.java)
    }

    fun showLogs(project: Project) {
        val dialog = object : DialogWrapper(project) {
            private val area = JBTextArea(PeggLog.lines().ifBlank { "No log entries yet." })

            init {
                title = "Pegg Log"
                area.isEditable = false
                area.lineWrap = true
                area.background = JBColor.PanelBackground
                init()
            }

            override fun createCenterPanel(): JPanel {
                val panel = JPanel(BorderLayout())
                panel.preferredSize = Dimension(900, 560)
                panel.add(JBScrollPane(area), BorderLayout.CENTER)
                val copy = JButton("Copy").apply {
                    addActionListener { copy(area.text) }
                }
                val bottom = JPanel(BorderLayout())
                bottom.border = JBUI.Borders.empty(8, 0, 0, 0)
                bottom.add(copy, BorderLayout.EAST)
                panel.add(bottom, BorderLayout.SOUTH)
                return panel
            }
        }
        dialog.show()
    }

    fun runTool(project: Project, title: String, binary: String, args: List<String>) {
        val command = GeneralCommandLine(listOf(binary) + args)
        project.basePath?.let { command.withWorkDirectory(it) }
        val handler = OSProcessHandler(command)
        val console = TextConsoleBuilderFactory.getInstance().createBuilder(project).console
        console.attachToProcess(handler)
        val descriptor = RunContentDescriptor(console, handler, console.component, title)
        ExecutionManager.getInstance(project).getContentManager().showRunContent(DefaultRunExecutor.getRunExecutorInstance(), descriptor)
        handler.startNotify()
    }

    fun openConfigFile(project: Project, path: String) {
        val file = File(path)
        if (!file.exists()) {
            file.parentFile?.mkdirs()
            file.writeText("{}\n")
        }
        val virtual = LocalFileSystem.getInstance().refreshAndFindFileByIoFile(file) ?: return
        FileEditorManager.getInstance(project).openFile(virtual, true)
    }

    fun revealProjectConfig(project: Project) {
        val root = project.basePath ?: return
        val dir = File(root, ".pegg")
        dir.mkdirs()
        ShowFilePathAction.openFile(dir)
    }
}
