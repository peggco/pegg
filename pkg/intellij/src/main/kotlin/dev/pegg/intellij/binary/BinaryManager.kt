package dev.pegg.intellij.binary

import com.intellij.ide.plugins.PluginManagerCore
import com.intellij.ide.trustedProjects.TrustedProjects
import com.intellij.openapi.application.PathManager
import com.intellij.openapi.extensions.PluginId
import com.intellij.openapi.project.Project
import dev.pegg.intellij.chat.ContextBuilder
import dev.pegg.intellij.log.PeggLog
import dev.pegg.intellij.settings.PeggSettings
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.apache.commons.compress.archivers.tar.TarArchiveInputStream
import org.apache.commons.compress.compressors.gzip.GzipCompressorInputStream
import java.io.File
import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.time.Duration
import java.util.concurrent.TimeUnit
import java.util.zip.ZipInputStream

typealias BinaryProgress = (String, Int) -> Unit

class BinaryManager(private val project: Project) {
    @Volatile private var cached: String? = null
    @Volatile private var version: String? = null

    private val client: HttpClient = HttpClient.newBuilder()
        .followRedirects(HttpClient.Redirect.ALWAYS)
        .connectTimeout(Duration.ofSeconds(30))
        .build()

    fun currentVersion(): String? = version

    fun invalidate() {
        cached = null
        version = null
    }

    suspend fun ensure(force: Boolean = false, progress: BinaryProgress? = null): String {
        val settings = PeggSettings.getInstance().state
        if (force) cached = null

        if (settings.executablePath.isNotBlank()) {
            val probe = verify(settings.executablePath)
            if (!probe.ok) {
                throw IllegalStateException(
                    "pegg.executablePath does not point to a working binary: ${settings.executablePath}" +
                        (probe.error?.let { " ($it)" } ?: ""),
                )
            }
            return use(settings.executablePath, probe.version)
        }

        if (!force) {
            findExisting()?.let { return use(it.path, it.version) }
        }

        if (!settings.autoDownload) {
            throw IllegalStateException(
                "Pegg binary not found. Install Pegg, set the Pegg executable path, or enable binary auto-download.",
            )
        }

        return download(progress)
    }

    fun diagnose(): List<String> {
        val settings = PeggSettings.getInstance().state
        val target = detectTarget()
        val lines = mutableListOf<String>()
        lines.add("platform: ${System.getProperty("os.name")}/${System.getProperty("os.arch")}")
        lines.add("target: ${target.os}/${target.arch}")
        lines.add("plugin version: ${pluginVersion()}")
        lines.add("binary version setting: ${settings.binaryVersion}")
        lines.add("download base url: ${downloadBase()}")
        lines.add("probe working directory: ${probeCwd().path}")
        lines.add("search directories: ${searchDirs().joinToString(", ").ifBlank { "(none)" }}")
        lines.add("")
        lines.add("candidates:")
        var resolved: String? = null
        var resolvedVersion: String? = null
        val candidates = if (settings.executablePath.isNotBlank()) {
            listOf(settings.executablePath)
        } else {
            candidates()
        }
        for (candidate in candidates) {
            val probe = verify(candidate)
            if (probe.ok) {
                lines.add("  OK   $candidate (${probe.version})")
                if (resolved == null) {
                    resolved = candidate
                    resolvedVersion = probe.version
                }
            } else {
                lines.add("  FAIL $candidate — ${probe.error ?: "unknown error"}")
            }
        }
        lines.add("")
        if (resolved != null) {
            lines.add("resolved: $resolved${resolvedVersion?.let { " ($it)" } ?: ""}")
        } else {
            lines.add("resolved: none")
        }
        return lines
    }

    private fun use(path: String, version: String?): String {
        cached = path
        this.version = version ?: probe(path).version
        return path
    }

    private fun findExisting(): Probe? {
        val current = cached
        if (current != null) {
            val probe = verify(current)
            if (probe.ok) return probe.copy(path = current)
            cached = null
        }
        for (candidate in candidates()) {
            val probe = verify(candidate)
            if (probe.ok) return probe.copy(path = candidate)
        }
        return null
    }

    private fun candidates(): List<String> {
        val target = detectTarget()
        val settings = PeggSettings.getInstance().state
        val out = LinkedHashSet<String>()
        val root = ContextBuilder.projectRoot(project)
        if (root != null && settings.allowWorkspaceBinary && isTrusted()) {
            out.add(File(File(root.path, "bin"), target.binary).path)
        }
        for (dir in searchDirs()) out.add(File(dir, target.binary).path)
        out.add(cachedPath(resolveTag(), target))
        return out.toList()
    }

    private fun verify(path: String): Probe {
        if (path.isBlank()) return Probe(path, false, error = "empty path")
        val file = File(path)
        if (!file.isFile) return Probe(path, false, error = "not a file")
        if (!System.getProperty("os.name").lowercase().startsWith("windows") && !Files.isExecutable(file.toPath())) {
            return Probe(path, false, error = "not executable")
        }
        val probe = probe(path)
        return if (probe.version != null) Probe(path, true, probe.version) else {
            Probe(path, false, error = probe.error)
        }
    }

    private fun probe(path: String): Probe {
        return try {
            val process = ProcessBuilder(path, "version").redirectErrorStream(true).start()
            val text = process.inputStream.bufferedReader(Charsets.UTF_8).use { it.readText() }
            if (!process.waitFor(15, TimeUnit.SECONDS)) {
                process.destroyForcibly()
                return Probe(path, false, error = "version check timed out")
            }
            val match = Regex("pegg version\\s+([^\\s]+)", RegexOption.IGNORE_CASE).find(text)
            if (match != null) {
                Probe(path, true, match.groupValues[1])
            } else {
                Probe(path, false, error = "unexpected output: ${text.lineSequence().firstOrNull { it.isNotBlank() } ?: "empty"}")
            }
        } catch (e: Throwable) {
            Probe(path, false, error = e.message ?: e.javaClass.simpleName)
        }
    }

    private suspend fun download(progress: BinaryProgress?): String {
        val tag = resolveTag()
        val target = detectTarget()
        val dir = cachedDir(tag)
        val finalPath = File(dir, target.binary)

        val existing = verify(finalPath.path)
        if (existing.ok) return use(finalPath.path, existing.version)

        dir.mkdirs()
        val archive = File(dir, target.asset)
        val checksumsFile = File(dir, "checksums.txt")
        archive.delete()
        checksumsFile.delete()
        finalPath.delete()

        try {
            progress?.invoke("Resolving Pegg release…", 0)
            val base = downloadBase()
            progress?.invoke("Downloading ${target.asset}…", 5)
            downloadFileAny(assetUrls(base, tag, target.asset), archive) { received, total ->
                if (total > 0) {
                    val pct = (5 + (received * 65 / total)).toInt().coerceIn(5, 70)
                    progress?.invoke(
                        "Downloading ${target.asset}… ${received * 100 / total}%",
                        pct,
                    )
                }
            }
            progress?.invoke("Downloading checksums…", 72)
            downloadFileAny(assetUrls(base, tag, "checksums.txt"), checksumsFile, null)

            progress?.invoke("Verifying checksum…", 78)
            verifyChecksum(archive, parseChecksums(checksumsFile.readText()), target.asset)

            progress?.invoke("Extracting Pegg…", 85)
            val extractDir = File(dir, "extract-${System.nanoTime()}")
            extractDir.mkdirs()
            extractArchive(archive, extractDir, target.asset)
            val binary = locateBinary(extractDir, target.binary)
                ?: throw IllegalStateException("The ${target.asset} archive did not contain ${target.binary}")

            finalPath.delete()
            runCatching {
                Files.move(binary.toPath(), finalPath.toPath(), StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE)
            }.onFailure {
                Files.copy(binary.toPath(), finalPath.toPath(), StandardCopyOption.REPLACE_EXISTING)
            }
            if (!System.getProperty("os.name").lowercase().startsWith("windows")) {
                finalPath.setExecutable(true, false)
            }
            archive.delete()
            checksumsFile.delete()
            extractDir.deleteRecursively()
            File(dir, "install.json").writeText(
                """{"tag":"$tag","asset":"${target.asset}","installedAt":"${java.time.Instant.now()}"}""",
            )

            val probe = verify(finalPath.path)
            if (!probe.ok) {
                throw IllegalStateException("Downloaded Pegg binary failed its version check: ${probe.error}")
            }
            progress?.invoke("Pegg ${probe.version} installed.", 100)
            PeggLog.info("Installed Pegg binary", finalPath.path, probe.version)
            return use(finalPath.path, probe.version)
        } catch (e: Throwable) {
            archive.delete()
            runCatching { finalPath.delete() }
            throw e
        }
    }

    private suspend fun downloadFileAny(
        urls: List<String>,
        dest: File,
        progress: ((Long, Long) -> Unit)?,
    ) {
        var last: Throwable? = null
        for (url in urls) {
            try {
                downloadFile(url, dest, progress)
                return
            } catch (e: Throwable) {
                last = e
            }
        }
        throw (last ?: IllegalStateException("Download failed"))
    }

    private suspend fun downloadFile(url: String, dest: File, progress: ((Long, Long) -> Unit)?) =
        withContext(Dispatchers.IO) {
            dest.parentFile?.mkdirs()
            val request = HttpRequest.newBuilder(URI.create(url))
                .header("User-Agent", "pegg-intellij")
                .header("Accept", "application/octet-stream,*/*")
                .timeout(Duration.ofMinutes(5))
                .GET()
                .build()
            val response = client.send(request, HttpResponse.BodyHandlers.ofInputStream())
            if (response.statusCode() !in 200..299) {
                throw IllegalStateException("Download failed (HTTP ${response.statusCode()}) for $url")
            }
            val total = response.headers().firstValueAsLong("content-length").orElse(-1L)
            response.body().use { input ->
                dest.outputStream().use { output ->
                    val buffer = ByteArray(64 * 1024)
                    var received = 0L
                    while (true) {
                        val read = input.read(buffer)
                        if (read < 0) break
                        output.write(buffer, 0, read)
                        received += read
                        progress?.invoke(received, total)
                    }
                }
            }
        }

    private fun extractArchive(archive: File, dest: File, asset: String) {
        if (asset.endsWith(".zip")) extractZip(archive, dest) else extractTarGz(archive, dest)
    }

    private fun extractZip(archive: File, dest: File) {
        val root = dest.canonicalFile
        ZipInputStream(archive.inputStream()).use { zip ->
            while (true) {
                val entry = zip.nextEntry ?: break
                val out = File(root, entry.name).canonicalFile
                if (!out.path.startsWith(root.path + File.separator)) {
                    throw IllegalStateException("Unsafe archive entry: ${entry.name}")
                }
                if (entry.isDirectory) {
                    out.mkdirs()
                } else {
                    out.parentFile.mkdirs()
                    out.outputStream().use { zip.copyTo(it) }
                }
                zip.closeEntry()
            }
        }
    }

    private fun extractTarGz(archive: File, dest: File) {
        val root = dest.canonicalFile
        GzipCompressorInputStream(archive.inputStream().buffered()).use { gzip ->
            TarArchiveInputStream(gzip).use { tar ->
                while (true) {
                    val entry = tar.nextEntry ?: break
                    val out = File(root, entry.name).canonicalFile
                    if (!out.path.startsWith(root.path + File.separator)) {
                        throw IllegalStateException("Unsafe archive entry: ${entry.name}")
                    }
                    if (entry.isDirectory) {
                        out.mkdirs()
                    } else {
                        out.parentFile.mkdirs()
                        out.outputStream().use { tar.copyTo(it) }
                    }
                }
            }
        }
    }

    private fun locateBinary(root: File, name: String): File? =
        root.walkTopDown().firstOrNull { it.isFile && it.name == name }

    private fun downloadBase(): String =
        (System.getenv("PEGG_DOWNLOAD_BASE_URL") ?: PeggSettings.getInstance().state.downloadBaseUrl)
            .trimEnd('/')

    private fun assetUrls(base: String, tag: String, asset: String): List<String> {
        if (tag == "latest") {
            val urls = mutableListOf<String>()
            if (base.endsWith("/download")) {
                urls.add("${base.removeSuffix("/download")}/latest/download/$asset")
            }
            urls.add("$base/latest/$asset")
            return urls
        }
        return listOf("$base/$tag/$asset")
    }

    private fun resolveTag(): String {
        val configured = PeggSettings.getInstance().state.binaryVersion.trim()
        if (configured.isNotEmpty() && configured != "match-extension") {
            if (configured == "latest") return "latest"
            return if (configured.startsWith("v")) configured else "v$configured"
        }
        val version = pluginVersion()
        return if (Regex("^\\d+\\.\\d+\\.\\d+(?:-[0-9A-Za-z.-]+)?$").matches(version)) "v$version" else "latest"
    }

    private fun pluginVersion(): String =
        PluginManagerCore.getPlugin(PluginId.getId("dev.pegg.intellij"))?.version ?: "0.2.0"

    private fun cachedDir(tag: String): File =
        File(PathManager.getConfigPath(), "pegg/bin/${tag.replace(Regex("[^A-Za-z0-9._-]"), "_")}")

    private fun cachedPath(tag: String, target: Target): String =
        File(cachedDir(tag), target.binary).path

    private fun probeCwd(): File =
        ContextBuilder.projectRoot(project)?.path?.let { File(it) } ?: File(System.getProperty("user.home"))

    private fun isTrusted(): Boolean = runCatching { TrustedProjects.isProjectTrusted(project) }.getOrDefault(false)

    data class Probe(
        val path: String,
        val ok: Boolean,
        val version: String? = null,
        val error: String? = null,
    )
}

fun peggHome(): File = File(System.getProperty("user.home"), ".pegg")
