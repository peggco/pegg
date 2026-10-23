package dev.pegg.intellij.ui

import com.intellij.openapi.application.PathManager
import dev.pegg.intellij.BuildInfo
import java.io.File

object WebViewAssets {
    private val files = listOf("index.html", "bootstrap.js", "webview.js", "webview.css")

    @Synchronized
    fun ensure(): File {
        val version = BuildInfo.VERSION
        val dir = File(PathManager.getTempPath(), "pegg-webview/$version")
        dir.mkdirs()
        for (name in files) {
            val target = File(dir, name)
            val stream = WebViewAssets::class.java.getResourceAsStream("/$name")
                ?: throw IllegalStateException("Missing bundled webview asset: $name")
            stream.use { input -> target.outputStream().use { output -> input.copyTo(output) } }
        }
        return File(dir, "index.html")
    }
}
