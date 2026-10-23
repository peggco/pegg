package dev.pegg.intellij.settings

import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.fileChooser.FileChooser
import com.intellij.openapi.fileChooser.FileChooserDescriptorFactory
import com.intellij.openapi.options.Configurable
import com.intellij.openapi.options.ConfigurationException
import com.intellij.openapi.progress.ProgressIndicator
import com.intellij.openapi.progress.ProgressManager
import com.intellij.openapi.progress.Task
import com.intellij.openapi.project.ProjectManager
import com.intellij.openapi.ui.Messages
import com.intellij.ui.TitledSeparator
import com.intellij.ui.components.JBCheckBox
import com.intellij.ui.components.JBScrollPane
import com.intellij.ui.components.JBTextField
import com.intellij.ui.table.JBTable
import com.intellij.util.ui.FormBuilder
import com.intellij.util.ui.JBUI
import dev.pegg.intellij.PeggService
import dev.pegg.intellij.log.PeggLog
import dev.pegg.intellij.ui.IdeActions
import kotlinx.coroutines.runBlocking
import java.awt.BorderLayout
import java.awt.FlowLayout
import javax.swing.DefaultComboBoxModel
import javax.swing.JButton
import javax.swing.JComboBox
import javax.swing.JComponent
import javax.swing.JPanel
import javax.swing.table.DefaultTableModel

class PeggConfigurable : Configurable {
    private var panel: JPanel? = null
    private val executablePath = JBTextField()
    private val autoDownload = JBCheckBox("Download a matching Pegg release when no binary is found")
    private val allowWorkspaceBinary = JBCheckBox("Allow <project>/bin/pegg in trusted projects")
    private val binaryVersion = JBTextField()
    private val downloadBaseUrl = JBTextField()
    private val provider = JBTextField()
    private val model = JBTextField()
    private val defaultMode = JComboBox(arrayOf("auto", "plan", "build"))
    private val includeActiveFile = JBCheckBox("Include the active file path in prompts")
    private val includeSelection = JBCheckBox("Include the active selection in prompts")
    private val maxFileBytes = JBTextField()
    private val sendOnEnter = JComboBox(arrayOf("enter", "ctrl+enter"))
    private val notifyOnComplete = JBCheckBox("Notify when a run finishes while the IDE is unfocused")
    private val logLevel = JComboBox(arrayOf("error", "warn", "info", "debug"))
    private val envModel = DefaultTableModel(arrayOf("Key", "Value"), 0)
    private val envTable = JBTable(envModel)

    override fun getDisplayName(): String = "Pegg"

    override fun createComponent(): JComponent {
        reset()
        val browse = JButton("Browse…").apply {
            addActionListener {
                val descriptor = FileChooserDescriptorFactory.createSingleFileNoJarsDescriptor()
                val file = FileChooser.chooseFile(descriptor, null, null) ?: return@addActionListener
                if (!file.isDirectory) executablePath.text = file.path
            }
        }
        val binaryRow = JPanel(BorderLayout(6, 0)).apply {
            add(executablePath, BorderLayout.CENTER)
            add(browse, BorderLayout.EAST)
        }

        envTable.fillsViewportHeight = true
        envTable.rowHeight = 22
        val envScroll = JBScrollPane(envTable).apply {
            preferredSize = JBUI.size(520, 110)
            border = JBUI.Borders.customLine(javax.swing.UIManager.getColor("Component.borderColor") ?: java.awt.Color.GRAY)
        }
        val envButtons = JPanel(FlowLayout(FlowLayout.LEFT, 4, 0)).apply {
            add(JButton("Add").apply {
                addActionListener { envModel.addRow(arrayOf("", "")) }
            })
            add(JButton("Remove").apply {
                addActionListener {
                    val row = envTable.selectedRow
                    if (row >= 0) envModel.removeRow(row)
                }
            })
        }
        val envPanel = JPanel(BorderLayout()).apply {
            add(envScroll, BorderLayout.CENTER)
            add(envButtons, BorderLayout.SOUTH)
        }

        val actions = JPanel(FlowLayout(FlowLayout.LEFT, 6, 0)).apply {
            add(JButton("Install or Update Binary").apply { addActionListener { installBinary() } })
            add(JButton("Diagnose Binary").apply { addActionListener { diagnoseBinary() } })
            add(JButton("Show Logs").apply { addActionListener { showLogs() } })
        }

        val builder = FormBuilder.createFormBuilder()
            .addComponent(TitledSeparator("Binary"))
            .addLabeledComponent("Executable path", binaryRow)
            .addComponent(autoDownload)
            .addComponent(allowWorkspaceBinary)
            .addLabeledComponent("Release tag", binaryVersion)
            .addLabeledComponent("Download base URL", downloadBaseUrl)
            .addComponent(actions)
            .addComponentFillVertically(JPanel(), 0)
            .addComponent(TitledSeparator("Agent"))
            .addLabeledComponent("Preferred provider", provider)
            .addLabeledComponent("Preferred model", model)
            .addLabeledComponent("Default mode", defaultMode)
            .addComponent(includeActiveFile)
            .addComponent(includeSelection)
            .addLabeledComponent("Max attachment bytes", maxFileBytes)
            .addComponent(TitledSeparator("Chat"))
            .addLabeledComponent("Send on", sendOnEnter)
            .addComponent(notifyOnComplete)
            .addComponent(TitledSeparator("Environment"))
            .addLabeledComponent("Environment variables", envPanel)
            .addComponent(TitledSeparator("Logging"))
            .addLabeledComponent("Log level", logLevel)

        return builder.panel.apply {
            border = JBUI.Borders.empty(8, 12)
            panel = this
        }
    }

    override fun isModified(): Boolean {
        val current = PeggSettings.getInstance().state
        return current.executablePath != executablePath.text.trim() ||
            current.autoDownload != autoDownload.isSelected ||
            current.allowWorkspaceBinary != allowWorkspaceBinary.isSelected ||
            current.binaryVersion != binaryVersion.text.trim() ||
            current.downloadBaseUrl != downloadBaseUrl.text.trim() ||
            current.provider != provider.text.trim() ||
            current.model != model.text.trim() ||
            current.defaultMode != (defaultMode.selectedItem as? String ?: "auto") ||
            current.includeActiveFile != includeActiveFile.isSelected ||
            current.includeSelection != includeSelection.isSelected ||
            current.maxFileBytes != maxFileBytes.text.trim().toIntOrNull() ||
            current.sendOnEnter != (sendOnEnter.selectedItem as? String ?: "enter") ||
            current.notifyOnComplete != notifyOnComplete.isSelected ||
            current.logLevel != (logLevel.selectedItem as? String ?: "info") ||
            current.env != readEnv()
    }

    override fun apply() {
        val bytes = maxFileBytes.text.trim().toIntOrNull()
            ?: throw ConfigurationException("Max attachment bytes must be a number.")
        if (bytes < 1024) throw ConfigurationException("Max attachment bytes must be at least 1024.")
        val settings = PeggSettings.getInstance()
        val old = settings.state.copy(env = LinkedHashMap(settings.state.env))
        val state = settings.state
        state.executablePath = executablePath.text.trim()
        state.autoDownload = autoDownload.isSelected
        state.allowWorkspaceBinary = allowWorkspaceBinary.isSelected
        state.binaryVersion = binaryVersion.text.trim().ifBlank { "match-extension" }
        state.downloadBaseUrl = downloadBaseUrl.text.trim().ifBlank {
            "https://github.com/peggco/pegg/releases/download"
        }
        state.provider = provider.text.trim()
        state.model = model.text.trim()
        state.defaultMode = defaultMode.selectedItem as? String ?: "auto"
        state.includeActiveFile = includeActiveFile.isSelected
        state.includeSelection = includeSelection.isSelected
        state.maxFileBytes = bytes
        state.sendOnEnter = sendOnEnter.selectedItem as? String ?: "enter"
        state.notifyOnComplete = notifyOnComplete.isSelected
        state.logLevel = logLevel.selectedItem as? String ?: "info"
        state.env = readEnv()
        settings.notifyChanged()

        val restartNeeded = old.executablePath != state.executablePath ||
            old.autoDownload != state.autoDownload ||
            old.allowWorkspaceBinary != state.allowWorkspaceBinary ||
            old.binaryVersion != state.binaryVersion ||
            old.downloadBaseUrl != state.downloadBaseUrl ||
            old.env != state.env
        if (restartNeeded) {
            val answer = Messages.showYesNoDialog(
                "Pegg binary or environment settings changed. Restart the Pegg server now?",
                "Pegg",
                Messages.getQuestionIcon(),
            )
            if (answer == Messages.YES) {
                ProjectManager.getInstance().openProjects.forEach { project ->
                    val service = PeggService.getInstance(project)
                    if (service.controller.state.connected || service.controller.state.acpSessionId != null) {
                        runCatching { service.restart() }
                    }
                }
            }
        }
    }

    override fun reset() {
        val state = PeggSettings.getInstance().state
        executablePath.text = state.executablePath
        autoDownload.isSelected = state.autoDownload
        allowWorkspaceBinary.isSelected = state.allowWorkspaceBinary
        binaryVersion.text = state.binaryVersion
        downloadBaseUrl.text = state.downloadBaseUrl
        provider.text = state.provider
        model.text = state.model
        defaultMode.model = DefaultComboBoxModel(arrayOf("auto", "plan", "build"))
        defaultMode.selectedItem = state.defaultMode
        includeActiveFile.isSelected = state.includeActiveFile
        includeSelection.isSelected = state.includeSelection
        maxFileBytes.text = state.maxFileBytes.toString()
        sendOnEnter.model = DefaultComboBoxModel(arrayOf("enter", "ctrl+enter"))
        sendOnEnter.selectedItem = state.sendOnEnter
        notifyOnComplete.isSelected = state.notifyOnComplete
        logLevel.model = DefaultComboBoxModel(arrayOf("error", "warn", "info", "debug"))
        logLevel.selectedItem = state.logLevel
        envModel.rowCount = 0
        state.env.toSortedMap().forEach { (key, value) -> envModel.addRow(arrayOf(key, value)) }
    }

    override fun disposeUIResources() {
        panel = null
    }

    private fun readEnv(): MutableMap<String, String> {
        val out = linkedMapOf<String, String>()
        for (row in 0 until envModel.rowCount) {
            val key = envModel.getValueAt(row, 0)?.toString()?.trim() ?: ""
            val value = envModel.getValueAt(row, 1)?.toString() ?: ""
            if (key.isNotBlank()) out[key] = value
        }
        return out
    }

    private fun installBinary() {
        val project = ProjectManager.getInstance().openProjects.firstOrNull()
        if (project == null) {
            Messages.showInfoMessage("Open a project first.", "Pegg")
            return
        }
        val service = PeggService.getInstance(project)
        ProgressManager.getInstance().run(object : Task.Backgroundable(project, "Installing Pegg", true) {
            override fun run(indicator: ProgressIndicator) {
                try {
                    val path = runBlocking {
                        service.binary.ensure(force = true) { message, percent ->
                            indicator.text = message
                            indicator.fraction = (percent / 100.0).coerceIn(0.0, 1.0)
                        }
                    }
                    service.binary.invalidate()
                    if (service.controller.state.connected) service.restart()
                    ApplicationManager.getApplication().invokeLater {
                        Messages.showInfoMessage("Pegg binary ready:\n$path", "Pegg")
                    }
                } catch (e: Throwable) {
                    PeggLog.error("Binary install failed: ${e.message}", e)
                    ApplicationManager.getApplication().invokeLater {
                        Messages.showErrorDialog(e.message ?: "Install failed", "Pegg")
                    }
                }
            }
        })
    }

    private fun diagnoseBinary() {
        val project = ProjectManager.getInstance().openProjects.firstOrNull() ?: return
        val service = PeggService.getInstance(project)
        ProgressManager.getInstance().run(object : Task.Backgroundable(project, "Diagnosing Pegg", true) {
            override fun run(indicator: ProgressIndicator) {
                val lines = service.binary.diagnose()
                PeggLog.info("Pegg binary diagnostics:\n${lines.joinToString("\n")}")
                ApplicationManager.getApplication().invokeLater {
                    Messages.showInfoMessage(project, lines.joinToString("\n"), "Pegg Binary Diagnostics")
                }
            }
        })
    }

    private fun showLogs() {
        val project = ProjectManager.getInstance().openProjects.firstOrNull() ?: return
        IdeActions.showLogs(project)
    }
}
