package dev.pegg.intellij.actions

import com.intellij.codeInsight.intention.IntentionAction
import com.intellij.icons.AllIcons
import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.fileEditor.FileEditorManager
import com.intellij.openapi.progress.ProgressIndicator
import com.intellij.openapi.progress.ProgressManager
import com.intellij.openapi.progress.Task
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.Messages
import com.intellij.openapi.ui.popup.JBPopupFactory
import com.intellij.openapi.util.TextRange
import com.intellij.openapi.vfs.VirtualFile
import com.intellij.openapi.wm.ToolWindowManager
import com.intellij.psi.PsiFile
import dev.pegg.intellij.PeggService
import dev.pegg.intellij.acp.SessionListItem
import dev.pegg.intellij.chat.Attachments
import dev.pegg.intellij.chat.ContextBuilder
import dev.pegg.intellij.log.PeggLog
import dev.pegg.intellij.settings.PeggSettings
import dev.pegg.intellij.ui.IdeActions
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import java.io.File
import javax.swing.Icon

data class ModelChoice(
    val provider: String,
    val modelId: String,
    val label: String,
) {
    override fun toString(): String = label
}

private data class SessionChoice(val id: String, val label: String) {
    override fun toString(): String = label
}

object PeggActions {
    fun openChat(project: Project) {
        ToolWindowManager.getInstance(project).getToolWindow("Pegg")?.activate(null)
    }

    fun runSelection(project: Project, editor: Editor, instruction: String) {
        val file = FileDocumentManager.getInstance().getFile(editor.document) ?: return
        val info = selectionInfo(editor)
        if (info.text.isBlank()) return
        val context = buildString {
            append("File: ").append(ContextBuilder.relativePath(project, file))
            append("\n\nLines ").append(info.startLine).append('-').append(info.endLine).append(":\n")
            append("```").append(ContextBuilder.languageFor(file)).append('\n')
            append(info.text).append("\n```")
        }
        send(project, "$instruction\n\n$context", emptyList())
    }

    fun runFile(project: Project, file: VirtualFile, instruction: String, attach: Boolean = false) {
        val context = buildString {
            append("File: ").append(ContextBuilder.relativePath(project, file)).append("\n\n")
            append(ContextBuilder.fileSnippet(file))
        }
        val attachments = if (attach) {
            listOfNotNull(Attachments.makeAttachment(project, file))
        } else {
            emptyList()
        }
        send(project, "$instruction\n\n$context", attachments)
    }

    fun addSelectionToChat(project: Project, editor: Editor) {
        val file = FileDocumentManager.getInstance().getFile(editor.document) ?: return
        val info = selectionInfo(editor)
        if (info.text.isBlank()) return
        val label = "${ContextBuilder.relativePath(project, file)}:${info.startLine}-${info.endLine}"
        PeggService.getInstance(project).controller.addComposerAttachment(
            Attachments.makeTextAttachment(label, info.text, file.path),
        )
        openChat(project)
    }

    fun addFileToChat(project: Project, file: VirtualFile) {
        Attachments.makeAttachment(project, file)?.let {
            PeggService.getInstance(project).controller.addComposerAttachment(it)
        }
        openChat(project)
    }

    fun explainFile(project: Project, file: VirtualFile) {
        val rel = ContextBuilder.relativePath(project, file)
        val attachment = Attachments.makeAttachment(project, file)
        send(
            project,
            "Explain $rel. Cover its purpose, structure, important functions, and anything surprising.",
            listOfNotNull(attachment),
        )
    }

    fun reviewFile(project: Project, file: VirtualFile) {
        val service = PeggService.getInstance(project)
        service.scope.launch {
            val rel = ContextBuilder.relativePath(project, file)
            val diff = ContextBuilder.gitDiff(project, rel)
            val body = diff.ifBlank { ContextBuilder.fileSnippet(file) }
            send(
                project,
                "/review\n\nReview $rel for bugs, regressions, security issues, performance problems, and missing tests. Report findings as file:line bullets.\n\n$body",
                emptyList(),
            )
        }
    }

    fun reviewChanges(project: Project) {
        val service = PeggService.getInstance(project)
        service.scope.launch {
            val diff = ContextBuilder.gitDiff(project)
            if (diff.isBlank()) {
                ApplicationManager.getApplication().invokeLater {
                    Messages.showInfoMessage(project, "No changes to review.", "Pegg")
                }
                return@launch
            }
            send(
                project,
                "/review\n\nReview these changes for bugs, regressions, security issues, and missing tests. Report findings as file:line bullets.\n\n$diff",
                emptyList(),
            )
        }
    }

    fun fixDiagnostics(project: Project, file: VirtualFile) {
        val service = PeggService.getInstance(project)
        service.scope.launch {
            val diagnostics = ContextBuilder.diagnostics(project, file)
            if (diagnostics.isEmpty()) {
                ApplicationManager.getApplication().invokeLater {
                    Messages.showInfoMessage(project, "No errors or warnings in this file.", "Pegg")
                }
                return@launch
            }
            runFile(
                project,
                file,
                "Fix all diagnostics in this file. Make focused changes, preserve behavior, and verify with the relevant checks.\n\nDiagnostics:\n${diagnostics.joinToString("\n")}",
            )
        }
    }

    fun explainDiagnostics(project: Project, file: VirtualFile) {
        val service = PeggService.getInstance(project)
        service.scope.launch {
            val diagnostics = ContextBuilder.diagnostics(project, file)
            if (diagnostics.isEmpty()) {
                ApplicationManager.getApplication().invokeLater {
                    Messages.showInfoMessage(project, "No errors or warnings in this file.", "Pegg")
                }
                return@launch
            }
            runFile(
                project,
                file,
                "Explain these diagnostics and suggest concrete fixes.\n\nDiagnostics:\n${diagnostics.joinToString("\n")}",
            )
        }
    }

    fun askSelection(project: Project, editor: Editor, title: String) {
        val question = Messages.showInputDialog(
            project,
            "What do you want to know about the selected code?",
            title,
            null,
            "",
            null,
        )?.trim() ?: return
        if (question.isEmpty()) return
        runSelection(project, editor, question)
    }

    fun editSelection(project: Project, editor: Editor) {
        val instruction = Messages.showInputDialog(
            project,
            "Describe the change you want.",
            "Edit Selection with Pegg",
            null,
            "",
            null,
        )?.trim() ?: return
        if (instruction.isEmpty()) return
        runSelection(
            project,
            editor,
            "Edit the selected code according to this instruction. Make the change in place, keep the diff focused, and verify it.\n\nInstruction: $instruction",
        )
    }

    fun askFile(project: Project, file: VirtualFile) {
        val rel = ContextBuilder.relativePath(project, file)
        val question = Messages.showInputDialog(
            project,
            "What do you want to know about $rel?",
            "Ask Pegg About File",
            null,
            "",
            null,
        )?.trim() ?: return
        if (question.isEmpty()) return
        runFile(project, file, question)
    }

    fun editFile(project: Project, file: VirtualFile) {
        val rel = ContextBuilder.relativePath(project, file)
        val instruction = Messages.showInputDialog(
            project,
            "Describe the change for $rel.",
            "Edit File with Pegg",
            null,
            "",
            null,
        )?.trim() ?: return
        if (instruction.isEmpty()) return
        runFile(
            project,
            file,
            "Edit $rel according to this instruction. Keep the diff focused and verify it.\n\nInstruction: $instruction",
        )
    }

    fun send(project: Project, prompt: String, attachments: List<dev.pegg.intellij.acp.ComposerAttachment>) {
        val service = PeggService.getInstance(project)
        ApplicationManager.getApplication().invokeLater { openChat(project) }
        service.scope.launch {
            runCatching {
                service.controller.sendPrompt(prompt, attachments = attachments, includeActiveContext = false)
            }.onFailure { service.showError("Pegg request failed", it) }
        }
    }

    data class SelectionInfo(val text: String, val startLine: Int, val endLine: Int)

    fun selectionInfo(editor: Editor): SelectionInfo {
        val document = editor.document
        val sm = editor.selectionModel
        return if (sm.hasSelection()) {
            SelectionInfo(
                text = sm.selectedText ?: "",
                startLine = document.getLineNumber(sm.selectionStart) + 1,
                endLine = document.getLineNumber(sm.selectionEnd) + 1,
            )
        } else {
            val line = document.getLineNumber(editor.caretModel.offset).coerceIn(0, document.lineCount - 1)
            SelectionInfo(
                text = document.getText(TextRange(document.getLineStartOffset(line), document.getLineEndOffset(line))),
                startLine = line + 1,
                endLine = line + 1,
            )
        }
    }
}

abstract class PeggAction(
    text: String,
    description: String? = null,
    icon: Icon? = null,
) : AnAction(text, description, icon) {

    protected fun project(event: AnActionEvent): Project? = event.project

    protected fun service(event: AnActionEvent): PeggService? =
        event.project?.let { PeggService.getInstance(it) }

    protected fun editor(event: AnActionEvent): Editor? =
        event.getData(CommonDataKeys.EDITOR)
            ?: event.project?.let { FileEditorManager.getInstance(it).selectedTextEditor }

    protected fun file(event: AnActionEvent): VirtualFile? =
        event.getData(CommonDataKeys.VIRTUAL_FILE)
            ?: event.getData(CommonDataKeys.VIRTUAL_FILE_ARRAY)?.firstOrNull()
            ?: editor(event)?.let { FileDocumentManager.getInstance().getFile(it.document) }
}

class PeggExplainSelectionAction : PeggAction("Explain Selection") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val editor = editor(event) ?: return
        PeggActions.runSelection(project, editor, "Explain this code. Focus on intent, data flow, and any risks.")
    }
}

class PeggEditSelectionAction : PeggAction("Edit Selection…") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val editor = editor(event) ?: return
        PeggActions.editSelection(project, editor)
    }
}

class PeggAskSelectionAction : PeggAction("Ask About Selection…") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val editor = editor(event) ?: return
        PeggActions.askSelection(project, editor, "Ask Pegg About Selection")
    }
}

class PeggFixSelectionAction : PeggAction("Fix Selection") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val editor = editor(event) ?: return
        PeggActions.runSelection(
            project,
            editor,
            "Find bugs or correctness issues in this code, then fix them. Keep the change focused and explain what you changed.",
        )
    }
}

class PeggFindBugsAction : PeggAction("Find Bugs in Selection") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val editor = editor(event) ?: return
        PeggActions.runSelection(
            project,
            editor,
            "Find bugs and correctness issues in this code. Report each finding as file:line with severity, impact, and a concrete fix.",
        )
    }
}

class PeggReviewSelectionAction : PeggAction("Review Selection") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val editor = editor(event) ?: return
        PeggActions.runSelection(
            project,
            editor,
            "Review this code for bugs, security issues, performance problems, and missing tests. Report findings as file:line bullets.",
        )
    }
}

class PeggSecurityReviewAction : PeggAction("Security Review") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val editor = editor(event) ?: return
        PeggActions.runSelection(
            project,
            editor,
            "Review this code for security vulnerabilities. Report severity, file:line, exploit path, and a concrete fix.",
        )
    }
}

class PeggRefactorAction : PeggAction("Refactor Selection") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val editor = editor(event) ?: return
        PeggActions.runSelection(
            project,
            editor,
            "Refactor this code for clarity and maintainability without changing behavior. Preserve public APIs unless necessary.",
        )
    }
}

class PeggOptimizeAction : PeggAction("Optimize Selection") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val editor = editor(event) ?: return
        PeggActions.runSelection(
            project,
            editor,
            "Optimize this code for performance and clarity without changing behavior. Explain the tradeoffs briefly.",
        )
    }
}

class PeggDocumentSelectionAction : PeggAction("Document Selection") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val editor = editor(event) ?: return
        PeggActions.runSelection(
            project,
            editor,
            "Add or improve documentation comments for this code. Match the project's style and do not change behavior.",
        )
    }
}

class PeggGenerateTestsAction : PeggAction("Generate Tests") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val editor = editor(event) ?: return
        PeggActions.runSelection(
            project,
            editor,
            "Write thorough tests for this code using the project conventions. Add the tests to the appropriate test file and run them.",
        )
    }
}

class PeggAddSelectionToChatAction : PeggAction("Add Selection to Pegg Chat") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val editor = editor(event) ?: return
        PeggActions.addSelectionToChat(project, editor)
    }
}

class PeggExplainFileAction : PeggAction("Explain File") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val file = file(event) ?: return
        PeggActions.explainFile(project, file)
    }
}

class PeggEditFileAction : PeggAction("Edit File…") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val file = file(event) ?: return
        PeggActions.editFile(project, file)
    }
}

class PeggAskFileAction : PeggAction("Ask About File…") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val file = file(event) ?: return
        PeggActions.askFile(project, file)
    }
}

class PeggReviewFileAction : PeggAction("Review File") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val file = file(event) ?: return
        PeggActions.reviewFile(project, file)
    }
}

class PeggDocumentFileAction : PeggAction("Document File") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val file = file(event) ?: return
        PeggActions.runFile(
            project,
            file,
            "Add or improve documentation comments across this file. Match the project's style and do not change behavior.",
        )
    }
}

class PeggGenerateTestsForFileAction : PeggAction("Generate Tests for File") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val file = file(event) ?: return
        PeggActions.runFile(
            project,
            file,
            "Write thorough tests for this file using the project conventions. Add the tests to the appropriate test file and run them.",
        )
    }
}

class PeggFixDiagnosticsAction : PeggAction("Fix All Diagnostics in File") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val file = file(event) ?: return
        PeggActions.fixDiagnostics(project, file)
    }
}

class PeggAddFileToChatAction : PeggAction("Add File to Pegg Chat") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val file = file(event) ?: return
        PeggActions.addFileToChat(project, file)
    }
}

class PeggOpenChatAction : PeggAction("Open Pegg Chat", "Open the Pegg tool window", AllIcons.Actions.Show) {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        PeggActions.openChat(project)
        service(event)?.activate()
    }
}

class PeggNewSessionAction : PeggAction("New Session", "Start a new Pegg session", AllIcons.General.Add) {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val service = service(event) ?: return
        val toolWindow = ToolWindowManager.getInstance(project).getToolWindow("Pegg")
        if (toolWindow != null) {
            toolWindow.activate {
                service.scope.launch {
                    runCatching { service.controller.newSession() }
                        .onFailure { service.showError("Could not start a session", it) }
                    service.refreshSessions()
                }
            }
        } else {
            service.scope.launch {
                runCatching { service.controller.newSession() }
                    .onFailure { service.showError("Could not start a session", it) }
                service.refreshSessions()
            }
        }
    }
}

class PeggStopAction : PeggAction("Stop Agent", "Cancel the running Pegg agent", AllIcons.Actions.Suspend) {
    override fun actionPerformed(event: AnActionEvent) {
        val service = service(event) ?: return
        service.scope.launch { runCatching { service.controller.cancel() } }
    }
}

class PeggSelectModeAction : PeggAction("Select Mode", "Choose Auto, Plan, or Build") {
    override fun actionPerformed(event: AnActionEvent) {
        val service = service(event) ?: return
        val modes = listOf("auto", "plan", "build")
        JBPopupFactory.getInstance()
            .createPopupChooserBuilder(modes)
            .setTitle("Pegg mode")
            .setItemChosenCallback { mode ->
                service.scope.launch {
                    runCatching { service.controller.setMode(mode) }
                        .onFailure { service.showError("Could not change mode", it) }
                }
            }
            .createPopup()
            .showInFocusCenter()
    }
}

class PeggSelectModelAction : PeggAction("Select Model", "Choose a provider and model") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val service = service(event) ?: return
        service.scope.launch {
            runCatching { service.controller.models() }
                .onSuccess { models ->
                    if (models.isEmpty()) {
                        ApplicationManager.getApplication().invokeLater {
                            Messages.showInfoMessage(
                                project,
                                "No Pegg models are configured. Run Pegg: Provider Login first.",
                                "Pegg",
                            )
                        }
                        return@onSuccess
                    }
                    ApplicationManager.getApplication().invokeLater {
                        val choices = models.map {
                            ModelChoice(
                                it.provider,
                                it.model.id,
                                "${it.model.name ?: it.model.id} · ${it.provider}",
                            )
                        }
                        JBPopupFactory.getInstance()
                            .createPopupChooserBuilder(choices)
                            .setTitle("Pegg model")
                            .setItemChosenCallback { entry ->
                                service.scope.launch {
                                    runCatching { service.controller.setModel(entry.provider, entry.modelId) }
                                        .onFailure { service.showError("Could not change model", it) }
                                }
                            }
                            .createPopup()
                            .showInFocusCenter()
                    }
                }
                .onFailure { service.showError("Could not list models", it) }
        }
    }
}

class PeggAiActionsAction : PeggAction("AI Actions…", "Selection or file actions", AllIcons.Actions.IntentionBulb) {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val editor = editor(event)
        val file = file(event)
        val selected = editor?.selectionModel?.hasSelection() == true
        val items = if (selected && editor != null) {
            listOf(
                ActionItem("Explain selection") { PeggActions.runSelection(project, editor, "Explain this code. Focus on intent, data flow, and any risks.") },
                ActionItem("Edit selection…") { PeggActions.editSelection(project, editor) },
                ActionItem("Ask about selection…") { PeggActions.askSelection(project, editor, "Ask Pegg About Selection") },
                ActionItem("Find bugs") { PeggActions.runSelection(project, editor, "Find bugs and correctness issues in this code. Report file:line, severity, impact, and a fix.") },
                ActionItem("Security review") { PeggActions.runSelection(project, editor, "Review this code for security vulnerabilities. Report severity, file:line, exploit path, and a fix.") },
                ActionItem("Refactor") { PeggActions.runSelection(project, editor, "Refactor this code for clarity without changing behavior.") },
                ActionItem("Optimize") { PeggActions.runSelection(project, editor, "Optimize this code for performance and clarity without changing behavior.") },
                ActionItem("Document") { PeggActions.runSelection(project, editor, "Add or improve documentation comments for this code.") },
                ActionItem("Generate tests") { PeggActions.runSelection(project, editor, "Write thorough tests for this code using project conventions and run them.") },
                ActionItem("Add selection to chat") { PeggActions.addSelectionToChat(project, editor) },
            )
        } else if (file != null) {
            listOf(
                ActionItem("Explain file") { PeggActions.explainFile(project, file) },
                ActionItem("Edit file…") { PeggActions.editFile(project, file) },
                ActionItem("Ask about file…") { PeggActions.askFile(project, file) },
                ActionItem("Review file") { PeggActions.reviewFile(project, file) },
                ActionItem("Document file") { PeggActions.runFile(project, file, "Add or improve documentation comments across this file.") },
                ActionItem("Generate tests for file") { PeggActions.runFile(project, file, "Write thorough tests for this file using project conventions and run them.") },
                ActionItem("Fix all diagnostics") { PeggActions.fixDiagnostics(project, file) },
                ActionItem("Add file to chat") { PeggActions.addFileToChat(project, file) },
            )
        } else {
            emptyList()
        }
        if (items.isEmpty()) return
        JBPopupFactory.getInstance()
            .createPopupChooserBuilder(items)
            .setTitle("Pegg Actions")
            .setItemChosenCallback { it.run() }
            .createPopup()
            .showInFocusCenter()
    }

    private data class ActionItem(val title: String, val run: () -> Unit) {
        override fun toString(): String = title
    }
}

class PeggReviewChangesAction : PeggAction("Review Changes", "Review the working-tree diff") {
    override fun actionPerformed(event: AnActionEvent) {
        project(event)?.let { PeggActions.reviewChanges(it) }
    }
}

class PeggReviewCurrentFileAction : PeggAction("Review Current File Changes") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val file = file(event) ?: return
        PeggActions.reviewFile(project, file)
    }
}

class PeggResumeSessionAction : PeggAction("Resume Session", "Load a previous Pegg session") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val service = service(event) ?: return
        service.scope.launch {
            val sessions = runCatching { service.controller.listSessions(true) }.getOrDefault(emptyList())
            ApplicationManager.getApplication().invokeLater { showSessionPicker(project, service, sessions, "Resume Pegg Session") { id ->
                service.scope.launch {
                    runCatching { service.controller.resumeSession(id) }
                        .onFailure { service.showError("Could not resume session", it) }
                    PeggActions.openChat(project)
                }
            } }
        }
    }
}

class PeggRenameSessionAction : PeggAction("Rename Session", "Rename a Pegg session") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val service = service(event) ?: return
        service.scope.launch {
            val sessions = runCatching { service.controller.listSessions(true) }.getOrDefault(emptyList())
            ApplicationManager.getApplication().invokeLater { showSessionPicker(project, service, sessions, "Rename Pegg Session") { id ->
                val current = sessions.firstOrNull { it.id == id }?.title ?: ""
                val title = Messages.showInputDialog(project, "New title", "Rename Pegg Session", null, current, null)
                    ?.trim() ?: return@showSessionPicker
                if (title.isEmpty()) return@showSessionPicker
                service.scope.launch {
                    runCatching { service.controller.renameSession(id, title) }
                        .onFailure { service.showError("Could not rename session", it) }
                }
            } }
        }
    }
}

class PeggDeleteSessionAction : PeggAction("Delete Session", "Delete a Pegg session") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val service = service(event) ?: return
        service.scope.launch {
            val sessions = runCatching { service.controller.listSessions(true) }.getOrDefault(emptyList())
            ApplicationManager.getApplication().invokeLater { showSessionPicker(project, service, sessions, "Delete Pegg Session") { id ->
                val title = sessions.firstOrNull { it.id == id }?.title ?: id
                if (Messages.showYesNoDialog(
                        project,
                        "Delete this Pegg session permanently?\n\n$title",
                        "Delete Pegg Session",
                        Messages.getWarningIcon(),
                    ) != Messages.YES
                ) return@showSessionPicker
                service.scope.launch {
                    runCatching { service.controller.deleteSession(id) }
                        .onFailure { service.showError("Could not delete session", it) }
                }
            } }
        }
    }
}

class PeggRefreshSessionsAction : PeggAction("Refresh Sessions", "Refresh the sessions list") {
    override fun actionPerformed(event: AnActionEvent) {
        service(event)?.refreshSessions()
    }
}

private fun showSessionPicker(
    project: Project,
    service: PeggService,
    sessions: List<SessionListItem>,
    title: String,
    onChosen: (String) -> Unit,
) {
    if (sessions.isEmpty()) {
        Messages.showInfoMessage(project, "No Pegg sessions yet.", "Pegg")
        return
    }
    val choices = sessions.map { SessionChoice(it.id, it.title.ifBlank { it.id }) }
    JBPopupFactory.getInstance()
        .createPopupChooserBuilder(choices)
        .setTitle(title)
        .setItemChosenCallback { onChosen(it.id) }
        .createPopup()
        .showInFocusCenter()
}

class PeggLoginAction : PeggAction("Provider Login", "Run pegg login") {
    override fun actionPerformed(event: AnActionEvent) = runBinary(event, listOf("login"), "Pegg Login")
}

class PeggDoctorAction : PeggAction("Run Doctor", "Run pegg doctor") {
    override fun actionPerformed(event: AnActionEvent) = runBinary(event, listOf("doctor"), "Pegg Doctor")
}

class PeggUpdateAction : PeggAction("Check for Updates", "Run pegg update") {
    override fun actionPerformed(event: AnActionEvent) = runBinary(event, listOf("update"), "Pegg Update")
}

class PeggTeamAction : PeggAction("Connect Team", "Run pegg team") {
    override fun actionPerformed(event: AnActionEvent) = runBinary(event, listOf("team"), "Pegg Team")
}

private fun runBinary(event: AnActionEvent, args: List<String>, title: String) {
    val project = event.project ?: return
    val service = PeggService.getInstance(project)
    service.scope.launch {
        runCatching {
            val path = service.binary.ensure()
            ApplicationManager.getApplication().invokeLater {
                IdeActions.runTool(project, title, path, args)
            }
        }.onFailure { service.showError("Could not run $title", it) }
    }
}

class PeggOpenGlobalConfigAction : PeggAction("Open Global Config", "Open ~/.pegg/pegg.json") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        IdeActions.openConfigFile(project, File(System.getProperty("user.home"), ".pegg/pegg.json").path)
    }
}

class PeggOpenProjectConfigAction : PeggAction("Open Project .pegg Folder") {
    override fun actionPerformed(event: AnActionEvent) {
        project(event)?.let { IdeActions.revealProjectConfig(it) }
    }
}

class PeggShowLogsAction : PeggAction("Show Logs", "Show the Pegg log", AllIcons.Debugger.Console) {
    override fun actionPerformed(event: AnActionEvent) {
        project(event)?.let { IdeActions.showLogs(it) }
    }
}

class PeggSettingsAction : PeggAction("Settings…", "Open Pegg settings", AllIcons.General.Settings) {
    override fun actionPerformed(event: AnActionEvent) {
        project(event)?.let { IdeActions.openSettings(it) }
    }
}

class PeggRestartServerAction : PeggAction("Restart Server", "Restart the Pegg ACP server") {
    override fun actionPerformed(event: AnActionEvent) {
        service(event)?.restart()
    }
}

class PeggInstallBinaryAction : PeggAction("Install or Update Binary", "Download a verified Pegg binary") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val service = service(event) ?: return
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
                        Messages.showInfoMessage(project, "Pegg binary ready:\n$path", "Pegg")
                    }
                } catch (e: Throwable) {
                    PeggLog.error("Binary install failed: ${e.message}", e)
                    ApplicationManager.getApplication().invokeLater {
                        Messages.showErrorDialog(project, e.message ?: "Install failed", "Pegg")
                    }
                }
            }
        })
    }
}

class PeggDiagnoseBinaryAction : PeggAction("Diagnose Binary", "Show binary resolution diagnostics") {
    override fun actionPerformed(event: AnActionEvent) {
        val project = project(event) ?: return
        val service = service(event) ?: return
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
}

abstract class PeggIntention(private val label: String) : IntentionAction {
    override fun getText(): String = label
    override fun getFamilyName(): String = "Pegg"
    override fun startInWriteAction(): Boolean = false

    override fun isAvailable(project: Project, editor: Editor?, file: PsiFile?): Boolean =
        editor != null && file != null && file.virtualFile != null

    protected fun virtualFile(file: PsiFile?): VirtualFile? = file?.virtualFile
}

class PeggExplainIntention : PeggIntention("Explain with Pegg") {
    override fun invoke(project: Project, editor: Editor?, file: PsiFile?) {
        val vf = virtualFile(file) ?: return
        if (editor != null && editor.selectionModel.hasSelection()) {
            PeggActions.runSelection(project, editor, "Explain this code. Focus on intent, data flow, and any risks.")
        } else {
            PeggActions.explainFile(project, vf)
        }
    }
}

class PeggFixIntention : PeggIntention("Fix with Pegg") {
    override fun invoke(project: Project, editor: Editor?, file: PsiFile?) {
        val vf = virtualFile(file) ?: return
        if (editor != null && editor.selectionModel.hasSelection()) {
            PeggActions.runSelection(
                project,
                editor,
                "Find bugs or correctness issues in this code, then fix them. Keep the change focused.",
            )
        } else {
            PeggActions.fixDiagnostics(project, vf)
        }
    }
}

class PeggReviewIntention : PeggIntention("Review with Pegg") {
    override fun invoke(project: Project, editor: Editor?, file: PsiFile?) {
        val vf = virtualFile(file) ?: return
        if (editor != null && editor.selectionModel.hasSelection()) {
            PeggActions.runSelection(
                project,
                editor,
                "Review this code for bugs, security issues, performance problems, and missing tests. Report findings as file:line bullets.",
            )
        } else {
            PeggActions.reviewFile(project, vf)
        }
    }
}

class PeggGenerateTestsIntention : PeggIntention("Generate tests with Pegg") {
    override fun invoke(project: Project, editor: Editor?, file: PsiFile?) {
        val vf = virtualFile(file) ?: return
        if (editor != null && editor.selectionModel.hasSelection()) {
            PeggActions.runSelection(
                project,
                editor,
                "Write thorough tests for this code using project conventions and run them.",
            )
        } else {
            PeggActions.runFile(
                project,
                vf,
                "Write thorough tests for this file using project conventions and run them.",
            )
        }
    }
}

class PeggAddToChatIntention : PeggIntention("Add to Pegg chat") {
    override fun invoke(project: Project, editor: Editor?, file: PsiFile?) {
        val vf = virtualFile(file) ?: return
        if (editor != null && editor.selectionModel.hasSelection()) {
            PeggActions.addSelectionToChat(project, editor)
        } else {
            PeggActions.addFileToChat(project, vf)
        }
    }
}

class PeggExplainDiagnosticIntention : PeggIntention("Explain diagnostic with Pegg") {
    override fun invoke(project: Project, editor: Editor?, file: PsiFile?) {
        virtualFile(file)?.let { PeggActions.explainDiagnostics(project, it) }
    }
}

class PeggFixDiagnosticIntention : PeggIntention("Fix diagnostic with Pegg") {
    override fun invoke(project: Project, editor: Editor?, file: PsiFile?) {
        virtualFile(file)?.let { PeggActions.fixDiagnostics(project, it) }
    }
}
