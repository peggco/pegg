package dev.pegg.intellij.binary

import java.io.File
import java.security.MessageDigest

fun parseChecksums(text: String): Map<String, String> {
    val out = LinkedHashMap<String, String>()
    for (raw in text.lineSequence()) {
        val line = raw.trim()
        if (line.isEmpty()) continue
        val match = Regex("^([0-9a-fA-F]{64})\\s+\\*?(.+)$").find(line) ?: continue
        val name = File(match.groupValues[2].trim()).name
        out[name] = match.groupValues[1].lowercase()
    }
    return out
}

fun sha256(file: File): String {
    val digest = MessageDigest.getInstance("SHA-256")
    file.inputStream().use { input ->
        val buffer = ByteArray(8192)
        while (true) {
            val read = input.read(buffer)
            if (read < 0) break
            digest.update(buffer, 0, read)
        }
    }
    return digest.digest().joinToString("") { "%02x".format(it) }
}

fun verifyChecksum(file: File, checksums: Map<String, String>, asset: String) {
    val expected = checksums[asset] ?: throw IllegalStateException("No checksum entry found for $asset")
    val actual = sha256(file)
    if (actual != expected) {
        throw IllegalStateException("Checksum mismatch for $asset\nExpected: $expected\nActual:   $actual")
    }
}
