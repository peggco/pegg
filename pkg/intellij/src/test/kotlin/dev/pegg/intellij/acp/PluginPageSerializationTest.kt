package dev.pegg.intellij.acp

import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull

class PluginPageSerializationTest {

    @Test
    fun `decodes the ACP plugin page shape`() {
        val json = """
            {
              "pages": [{
                "id": "kanban",
                "title": "Kanban",
                "owner": "kanban",
                "metrics": [{"label": "Cards", "value": "3"}],
                "sections": [{
                  "title": "Board",
                  "kind": "board",
                  "items": [{
                    "id": "card-1",
                    "title": "Ship it",
                    "column": "backlog",
                    "action": "card.update",
                    "fields": [{"id": "column", "value": "backlog"}],
                    "meta": {"runAction": "card.run"}
                  }]
                }],
                "actions": [{"id": "card.add", "label": "Add card"}],
                "settings": [{"id": "port", "value": "7878"}],
                "saveAction": "settings.save"
              }]
            }
        """.trimIndent()

        val result = ACP_JSON.decodeFromString(PluginPagesResult.serializer(), json)
        assertEquals(1, result.pages.size)
        val page = result.pages.single()
        assertEquals("kanban", page.owner)
        assertEquals("kanban", page.id)
        assertEquals("settings.save", page.saveAction)
        assertEquals("3", page.metrics.single().value)
        val item = page.sections.single().items.single()
        assertEquals("backlog", item.column)
        assertEquals("card.update", item.action)
        assertEquals("card.run", item.meta["runAction"])
        assertEquals("column", item.fields.single().id)
        assertEquals("7878", page.settings.single().value)
    }

    @Test
    fun `encodes an action result with a replacement page`() {
        val page = PluginPage(id = "kanban", title = "Kanban updated", owner = "kanban")
        val json = ACP_JSON.encodeToString(PluginPageResult.serializer(), PluginPageResult(output = "moved", page = page))
        val obj = ACP_JSON.parseToJsonElement(json).jsonObject
        assertEquals("moved", obj["output"]?.jsonPrimitive?.content)
        val encoded = assertNotNull(obj["page"]).jsonObject
        assertEquals("Kanban updated", encoded["title"]?.jsonPrimitive?.content)
        assertEquals("kanban", encoded["owner"]?.jsonPrimitive?.content)
    }

    @Test
    fun `reports the plugin pages capability`() {
        val caps = ACP_JSON.decodeFromString(PeggCapabilities.serializer(), """{"pluginPages":true}""")
        assertEquals(true, caps.pluginPages)
    }
}
