package dev.pegg.intellij.git

object BranchName {
    private const val MAX_SLUG = 40

    fun slugify(title: String): String {
        val plain = title
            .replace(Regex("\\[[^\\]]*]"), " ")
            .replace(Regex("\\([^)]*\\)"), " ")
            .lowercase()
            .replace(Regex("[^a-z0-9]+"), "-")
            .replace(Regex("^-+|-+$"), "")
        if (plain.length <= MAX_SLUG) return plain
        val cut = plain.take(MAX_SLUG)
        val lastDash = cut.lastIndexOf('-')
        return (if (lastDash > 20) cut.take(lastDash) else cut).trimEnd('-')
    }

    fun render(
        template: String,
        number: String?,
        title: String,
        id: String?,
        provider: String?,
    ): String {
        val slug = slugify(title)
        val rendered = template
            .replace("{number}", number.orEmpty())
            .replace("{iid}", number.orEmpty())
            .replace("{slug}", slug)
            .replace("{title}", slug)
            .replace("{id}", id?.trim()?.lowercase().orEmpty())
            .replace("{provider}", provider?.trim()?.lowercase().orEmpty())
            .replace(Regex("[^A-Za-z0-9/_.-]+"), "-")
            .replace(Regex("-+"), "-")
            .replace(Regex("^[-/]+|[-/]+$"), "")
            .replace(Regex("/{2,}"), "/")
        if (rendered.isEmpty() || rendered == "-" || rendered == ".") return slug.ifEmpty { number.orEmpty() }
        return rendered
    }

    fun forCard(template: String, card: KanbanCard): String {
        val issue = card.issue
        val number = (issue?.number ?: issue?.iid)?.toString()
        val rendered = render(template, number, card.title, card.id, issue?.provider)
        return rendered.ifEmpty { slugify(card.title).ifEmpty { number ?: "work" } }
    }
}
