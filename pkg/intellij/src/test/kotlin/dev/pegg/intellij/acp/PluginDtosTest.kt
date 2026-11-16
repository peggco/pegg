package dev.pegg.intellij.acp

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class PluginDtosTest {
    @Test
    fun parsesPluginListPayload() {
        val result = ACP_JSON.decodeFromString(
            PluginListResult.serializer(),
            """
            {
              "plugins": [
                {
                  "name": "kanban",
                  "version": "1.2.3",
                  "source": "peggco/pegg-kanban",
                  "enabled": true,
                  "loaded": false,
                  "lastError": "missing token",
                  "envKeys": ["KANBAN_TOKEN"]
                }
              ],
              "pluginsEnabled": true
            }
            """.trimIndent(),
        )

        assertTrue(result.pluginsEnabled)
        assertEquals(1, result.plugins.size)
        val plugin = result.plugins.single()
        assertEquals("kanban", plugin.name)
        assertEquals("1.2.3", plugin.version)
        assertEquals("peggco/pegg-kanban", plugin.source)
        assertTrue(plugin.enabled)
        assertFalse(plugin.loaded)
        assertEquals("missing token", plugin.lastError)
        assertEquals(listOf("KANBAN_TOKEN"), plugin.envKeys)
    }

    @Test
    fun parsesPluginCatalogPayloadWithDefaults() {
        val result = ACP_JSON.decodeFromString(
            PluginCatalogResult.serializer(),
            """
            {
              "items": [
                {"id": "peggco/pegg-browser", "name": "browser", "source": "peggco/pegg-browser"}
              ]
            }
            """.trimIndent(),
        )

        assertNull(result.nextCursor)
        val item = result.items.single()
        assertEquals("peggco/pegg-browser", item.id)
        assertFalse(item.official)
        assertFalse(item.updateAvailable)
        assertNull(item.installedVersion)
        assertEquals(emptyList(), item.requiredEnv)
    }

    @Test
    fun parsesPluginMutationPayload() {
        val result = ACP_JSON.decodeFromString(
            PluginMutationResult.serializer(),
            """
            {
              "name": "kanban",
              "restartRequired": true,
              "warnings": ["catalog may be stale"],
              "installedVersion": "1.3.0"
            }
            """.trimIndent(),
        )

        assertEquals("kanban", result.name)
        assertTrue(result.restartRequired)
        assertEquals(listOf("catalog may be stale"), result.warnings)
        assertEquals("1.3.0", result.installedVersion)
    }

    @Test
    fun decodesPluginsCapability() {
        val capabilities = ACP_JSON.decodeFromString(
            PeggCapabilities.serializer(),
            """{"plugins":true,"models":true}""",
        )

        assertTrue(capabilities.plugins == true)
        assertTrue(capabilities.models == true)
        assertNull(capabilities.ask)
    }
}
