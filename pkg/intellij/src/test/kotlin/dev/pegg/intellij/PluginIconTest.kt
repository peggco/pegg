package dev.pegg.intellij

import com.intellij.ui.svg.SvgKt
import java.io.File
import kotlin.test.Test
import kotlin.test.assertTrue

class PluginIconTest {
    @Test
    fun `plugin icon svg is loadable by the platform`() {
        val bytes = File("src/main/resources/META-INF/pluginIcon.svg").readBytes()
        val images = SvgKt.loadWithSizes(listOf(40, 80), bytes, 1f)
        assertTrue(images.isNotEmpty())
        assertTrue(images.all { it != null })
    }
}
