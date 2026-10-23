package dev.pegg.intellij.chat

import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.editor.colors.CodeInsightColors
import com.intellij.openapi.editor.impl.DocumentMarkupModel
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.fileEditor.FileEditorManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.roots.ProjectFileIndex
import com.intellij.openapi.roots.ModuleRootManager
import com.intellij.openapi.roots.ProjectRootManager
import com.intellij.openapi.util.Computable
import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.openapi.vfs.VfsUtilCore
import com.intellij.openapi.vfs.VirtualFile
import dev.pegg.intellij.settings.PeggSettings
import java.io.File
import java.util.concurrent.TimeUnit

object ContextBuilder {
    private val excluded = setOf(
        "node_modules", ".git", "dist", "build", "out", "site", ".cache", "coverage",
        ".idea", ".gradle", "target", "vendor", "Pods", "DerivedData", ".venv", "venv",
        "__pycache__", ".next", ".turbo",
    )
    private const val MAX_FILE_SNIPPET = 128 * 1024
    private const val MAX_DIFF = 400_000

    private fun <T> readAction(action: () -> T): T =
        ApplicationManager.getApplication().runReadAction(Computable<T> { action() })

    fun projectRoot(project: Project, file: VirtualFile? = null): VirtualFile? =
        readAction {
            val target = file ?: FileEditorManager.getInstance(project).selectedFiles.firstOrNull()
            if (target != null) {
                val index = ProjectFileIndex.getInstance(project)
                index.getContentRootForFile(target)?.let { return@readAction it }
                val module = index.getModuleForFile(target)
                if (module != null) {
                    return@readAction ModuleRootManager.getInstance(module).contentRoots.firstOrNull()
                }
            }
            ProjectRootManager.getInstance(project).contentRoots.firstOrNull()
                ?: project.basePath?.let { LocalFileSystem.getInstance().findFileByPath(it) }
        }

    fun activeFile(project: Project): VirtualFile? =
        FileEditorManager.getInstance(project).selectedFiles.firstOrNull()

    fun relativePath(project: Project, file: VirtualFile): String {
        val root = projectRoot(project, file) ?: return file.path
        return VfsUtilCore.getRelativePath(file, root, '/') ?: file.name
    }

    fun activeContext(project: Project, editor: Editor? = null): String =
        readAction {
            val settings = PeggSettings.getInstance().state
            val active = editor ?: FileEditorManager.getInstance(project).selectedTextEditor
                ?: return@readAction ""
            val file = FileDocumentManager.getInstance().getFile(active.document)
                ?: return@readAction ""
            val parts = mutableListOf<String>()
            if (settings.includeActiveFile) parts.add("Active file: ${file.path}")
            val selection = active.selectionModel
            if (settings.includeSelection && selection.hasSelection()) {
                val text = selection.selectedText ?: ""
                val start = active.document.getLineNumber(selection.selectionStart) + 1
                val end = active.document.getLineNumber(selection.selectionEnd) + 1
                parts.add("Selected lines $start-$end:\n```${languageFor(file)}\n$text\n```")
            }
            if (parts.isEmpty()) "" else "<context>\n${parts.joinToString("\n")}\n</context>"
        }

    fun fileSnippet(file: VirtualFile): String =
        readAction {
            if (file.isDirectory) return@readAction ""
            if (file.length > MAX_FILE_SNIPPET) return@readAction ""
            val bytes = runCatching { file.contentsToByteArray() }.getOrNull()
                ?: return@readAction ""
            if (Attachments.isBinary(bytes)) return@readAction ""
            val text = String(bytes, Charsets.UTF_8)
            "```${languageFor(file)}\n$text\n```"
        }

    fun languageFor(file: VirtualFile): String = when (file.extension?.lowercase()) {
        "ts" -> "typescript"
        "tsx" -> "typescriptreact"
        "js" -> "javascript"
        "jsx" -> "javascriptreact"
        "go" -> "go"
        "py" -> "python"
        "rs" -> "rust"
        "java" -> "java"
        "kt" -> "kotlin"
        "json" -> "json"
        "md" -> "markdown"
        "css" -> "css"
        "html" -> "html"
        "sh" -> "shellscript"
        "yaml", "yml" -> "yaml"
        "toml" -> "toml"
        "php" -> "php"
        "c" -> "c"
        "h" -> "c"
        "cpp", "cc", "cxx" -> "cpp"
        "cs" -> "csharp"
        "rb" -> "ruby"
        "sql" -> "sql"
        else -> "plaintext"
    }

    fun listFiles(project: Project): List<String> = readAction {
        val out = ArrayList<String>(4096)
        val index = ProjectFileIndex.getInstance(project)
        index.iterateContent { file ->
            if (out.size >= 5000) return@iterateContent false
            if (file.isDirectory || !file.isValid) return@iterateContent true
            val path = relativePath(project, file)
            if (path.isBlank() || path.split('/').any { it in excluded }) {
                return@iterateContent true
            }
            out.add(path)
            true
        }
        out.sortWith(String.CASE_INSENSITIVE_ORDER)
        out
    }

    fun diagnostics(project: Project, file: VirtualFile): List<String> {
        return runCatching {
            readAction {
                val document = FileDocumentManager.getInstance().getDocument(file)
                    ?: return@readAction emptyList()
                val markup = DocumentMarkupModel.forDocument(document, project, false)
                    ?: return@readAction emptyList()
                markup.allHighlighters.asSequence()
                    .filter { it.isValid }
                    .mapNotNull { highlighter ->
                        val severity = when (highlighter.textAttributesKey) {
                            CodeInsightColors.ERRORS_ATTRIBUTES,
                            CodeInsightColors.GENERIC_SERVER_ERROR_OR_WARNING,
                            CodeInsightColors.RUNTIME_ERROR -> "error"
                            CodeInsightColors.WARNINGS_ATTRIBUTES -> "warning"
                            else -> return@mapNotNull null
                        }
                        val description = highlighter.gutterIconRenderer?.tooltipText
                            ?.takeIf { it.isNotBlank() }
                            ?: highlighter.errorStripeTooltip?.toString()
                            ?: "problem"
                        val line = document.getLineNumber(highlighter.startOffset) + 1
                        "- $severity line $line: $description"
                    }
                    .take(100)
                    .toList()
            }
        }.getOrElse { emptyList() }
    }

    fun gitDiff(project: Project, path: String? = null, staged: Boolean = false): String {
        val root = projectRoot(project)?.path ?: return ""
        val args = mutableListOf(
            "git", "--no-pager", "diff", "--no-color", "--src-prefix=a/", "--dst-prefix=b/",
        )
        if (staged) args.add("--cached")
        if (!path.isNullOrBlank()) {
            args.add("--")
            args.add(path)
        }
        return runCatching {
            val process = ProcessBuilder(args)
                .directory(File(root))
                .redirectErrorStream(true)
                .start()
            val text = process.inputStream.bufferedReader(Charsets.UTF_8).use { it.readText() }.trim()
            if (!process.waitFor(30, TimeUnit.SECONDS)) process.destroyForcibly()
            if (text.length > MAX_DIFF) "$text\n\n[diff truncated]" else text
        }.getOrDefault("")
    }

    fun currentLineText(editor: Editor): String {
        val document = editor.document
        val line = document.getLineNumber(editor.caretModel.offset).coerceIn(0, document.lineCount - 1)
        return document.getText(
            com.intellij.openapi.util.TextRange(
                document.getLineStartOffset(line),
                document.getLineEndOffset(line),
            ),
        )
    }

    fun selectionOrLine(editor: Editor): String {
        val selection = editor.selectionModel
        return if (selection.hasSelection()) selection.selectedText ?: "" else currentLineText(editor)
    }
}
