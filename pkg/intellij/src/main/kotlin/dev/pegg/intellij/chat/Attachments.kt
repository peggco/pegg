package dev.pegg.intellij.chat

import com.intellij.openapi.project.Project
import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.openapi.vfs.VirtualFile
import dev.pegg.intellij.acp.ComposerAttachment
import dev.pegg.intellij.acp.ContentBlock
import dev.pegg.intellij.acp.EmbeddedResource
import dev.pegg.intellij.settings.PeggSettings
import java.io.File
import java.util.Base64
import java.util.UUID

object Attachments {
    private const val MAX_MEDIA = 20 * 1024 * 1024

    fun makeAttachment(project: Project, file: VirtualFile): ComposerAttachment? {
        if (file.isDirectory) return null
        val name = file.name
        val mediaType = mediaTypeFor(name)
        val size = file.length
        val max = PeggSettings.getInstance().state.maxFileBytes
        if ((size > max && !mediaType.startsWith("image/")) ||
            (mediaType.startsWith("image/") && size > MAX_MEDIA)
        ) {
            return ComposerAttachment(cryptoId(), name, mediaType, size, file.path)
        }
        return runCatching {
            val bytes = file.contentsToByteArray()
            buildAttachment(name, mediaType, size, file.path, bytes)
        }.getOrNull()
    }

    fun makeAttachment(project: Project, path: String): ComposerAttachment? {
        val file = LocalFileSystem.getInstance().findFileByPath(path)
        if (file != null) return makeAttachment(project, file)
        val disk = File(path)
        if (!disk.isFile) return null
        val name = disk.name
        val mediaType = mediaTypeFor(name)
        val size = disk.length()
        val max = PeggSettings.getInstance().state.maxFileBytes
        if ((size > max && !mediaType.startsWith("image/")) ||
            (mediaType.startsWith("image/") && size > MAX_MEDIA)
        ) {
            return ComposerAttachment(cryptoId(), name, mediaType, size, path)
        }
        return runCatching {
            buildAttachment(name, mediaType, size, path, disk.readBytes())
        }.getOrNull()
    }

    private fun buildAttachment(
        name: String,
        mediaType: String,
        size: Long,
        path: String,
        bytes: ByteArray,
    ): ComposerAttachment {
        val id = cryptoId()
        if (mediaType.startsWith("image/")) {
            return ComposerAttachment(id, name, mediaType, size, path, data = base64(bytes))
        }
        if (isBinary(bytes)) {
            return ComposerAttachment(id, name, mediaType, size, path)
        }
        return ComposerAttachment(id, name, mediaType, size, path, text = String(bytes, Charsets.UTF_8))
    }

    fun makeTextAttachment(name: String, text: String, path: String? = null): ComposerAttachment =
        ComposerAttachment(
            id = cryptoId(),
            name = name,
            mediaType = "text/plain",
            size = text.toByteArray(Charsets.UTF_8).size.toLong(),
            path = path,
            text = text,
        )

    fun makeDataAttachment(name: String, mediaType: String, data: String, size: Long): ComposerAttachment =
        ComposerAttachment(cryptoId(), name, mediaType, size, data = data)

    fun attachmentToBlock(attachment: ComposerAttachment): ContentBlock? {
        val data = attachment.data
        if (!data.isNullOrEmpty()) {
            return when {
                attachment.mediaType.startsWith("image/") ->
                    ContentBlock("image", data = data, mimeType = attachment.mediaType)
                attachment.mediaType.startsWith("audio/") ->
                    ContentBlock("audio", data = data, mimeType = attachment.mediaType)
                attachment.mediaType.startsWith("video/") ->
                    ContentBlock("video", data = data, mimeType = attachment.mediaType)
                else -> ContentBlock("text", text = String(Base64.getDecoder().decode(data), Charsets.UTF_8))
            }
        }
        val text = attachment.text
        if (text != null) {
            return ContentBlock(
                type = "resource",
                resource = EmbeddedResource(
                    uri = attachment.path ?: "untitled:${attachment.name}",
                    mimeType = attachment.mediaType.ifBlank { "text/plain" },
                    text = text,
                ),
            )
        }
        val path = attachment.path ?: return null
        val bytes = runCatching { File(path).readBytes() }.getOrNull() ?: return null
        val media = attachment.mediaType.startsWith("image/") ||
            attachment.mediaType.startsWith("audio/") ||
            attachment.mediaType.startsWith("video/")
        val limit = if (media) {
            MAX_MEDIA
        } else {
            minOf(MAX_MEDIA, PeggSettings.getInstance().state.maxFileBytes)
        }
        if (bytes.size > limit) {
            return ContentBlock(
                type = "text",
                text = "[Attached file is too large to inline: $path (${bytes.size} bytes)]",
            )
        }
        return when {
            attachment.mediaType.startsWith("image/") ->
                ContentBlock("image", data = base64(bytes), mimeType = attachment.mediaType)
            attachment.mediaType.startsWith("audio/") ->
                ContentBlock("audio", data = base64(bytes), mimeType = attachment.mediaType)
            attachment.mediaType.startsWith("video/") ->
                ContentBlock("video", data = base64(bytes), mimeType = attachment.mediaType)
            isBinary(bytes) -> ContentBlock(
                type = "resource",
                resource = EmbeddedResource(
                    uri = File(path).toURI().toString(),
                    mimeType = attachment.mediaType.ifBlank { "application/octet-stream" },
                    blob = base64(bytes),
                ),
            )
            else -> ContentBlock(
                type = "resource",
                resource = EmbeddedResource(
                    uri = File(path).toURI().toString(),
                    mimeType = attachment.mediaType.ifBlank { "text/plain" },
                    text = String(bytes, Charsets.UTF_8),
                ),
            )
        }
    }

    fun isBinary(bytes: ByteArray): Boolean {
        val limit = minOf(bytes.size, 8000)
        for (i in 0 until limit) if (bytes[i].toInt() == 0) return true
        return false
    }

    fun mediaTypeFor(path: String): String {
        val ext = path.substringAfterLast('.', "").lowercase()
        return when (ext) {
            "png" -> "image/png"
            "jpg", "jpeg" -> "image/jpeg"
            "gif" -> "image/gif"
            "webp" -> "image/webp"
            "bmp" -> "image/bmp"
            "svg" -> "image/svg+xml"
            "mp3" -> "audio/mpeg"
            "wav" -> "audio/wav"
            "ogg" -> "audio/ogg"
            "m4a" -> "audio/mp4"
            "mp4" -> "video/mp4"
            "webm" -> "video/webm"
            "mov" -> "video/quicktime"
            "pdf" -> "application/pdf"
            "json" -> "application/json"
            "md" -> "text/markdown"
            else -> "text/plain"
        }
    }

    fun cryptoId(): String = "att-${System.currentTimeMillis()}-${UUID.randomUUID().toString().take(8)}"

    private fun base64(bytes: ByteArray): String = Base64.getEncoder().encodeToString(bytes)
}
