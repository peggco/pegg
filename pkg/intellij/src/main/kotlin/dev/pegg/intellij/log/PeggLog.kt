package dev.pegg.intellij.log

import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.diagnostic.Logger
import dev.pegg.intellij.settings.PeggSettings
import java.time.LocalTime
import java.time.format.DateTimeFormatter

object PeggLog {
    private val logger: Logger = Logger.getInstance("dev.pegg.intellij")
    private val ring = ArrayDeque<String>()
    private val formatter = DateTimeFormatter.ofPattern("HH:mm:ss.SSS")

    private val secretPatterns = listOf(
        Regex("(sk-[A-Za-z0-9_-]{12,})") to "sk-***",
        Regex("(Bearer\\s+)[A-Za-z0-9._-]{8,}", RegexOption.IGNORE_CASE) to "$1***",
        Regex("(api[_-]?key[\"'\\s:=]+)[A-Za-z0-9._-]{8,}", RegexOption.IGNORE_CASE) to "$1***",
        Regex("(eyJ[A-Za-z0-9_-]{10,}\\.[A-Za-z0-9_-]{10,}\\.[A-Za-z0-9_-]{10,})") to "***JWT***",
    )

    fun redact(value: String): String {
        var out = value
        for ((pattern, replacement) in secretPatterns) out = pattern.replace(out, replacement)
        return out
    }

    fun debug(message: String, vararg args: Any?) {
        if (level() >= 3) write("DEBUG", message, args)
    }

    fun info(message: String, vararg args: Any?) {
        if (level() >= 2) write("INFO", message, args)
    }

    fun warn(message: String, vararg args: Any?) {
        if (level() >= 1) write("WARN", message, args)
    }

    fun error(message: String, vararg args: Any?) {
        write("ERROR", message, args)
    }

    fun error(message: String, error: Throwable) {
        write("ERROR", "$message ${error.stackTraceToString()}")
    }

    @Synchronized
    fun lines(): String = ring.joinToString("\n")

    @Synchronized
    fun clear() = ring.clear()

    private fun level(): Int {
        if (ApplicationManager.getApplication() == null) return 3
        val configured = runCatching { PeggSettings.getInstance().logLevel() }.getOrDefault("info")
        return when (configured) {
            "error" -> 0
            "warn" -> 1
            "info" -> 2
            else -> 3
        }
    }

    @Synchronized
    private fun write(level: String, message: String, args: Array<out Any?> = emptyArray()) {
        val text = redact(format(message, args))
        val line = "${LocalTime.now().format(formatter)} [$level] $text"
        ring.addLast(line)
        while (ring.size > 3000) ring.removeFirst()
        runCatching {
            when (level) {
                "ERROR" -> logger.error(text)
                "WARN" -> logger.warn(text)
                "DEBUG" -> logger.debug(text)
                else -> logger.info(text)
            }
        }
    }

    private fun format(message: String, args: Array<out Any?>): String {
        if (args.isEmpty()) return message
        val suffix = args.joinToString(" ") { arg ->
            when (arg) {
                is Throwable -> arg.stackTraceToString()
                is String -> arg
                else -> runCatching { arg.toString() }.getOrDefault("null")
            }
        }
        return "$message $suffix"
    }
}
