package dev.pegg.intellij.ui

import com.intellij.openapi.editor.colors.EditorColorsManager
import com.intellij.ui.JBColor
import com.intellij.util.ui.UIUtil
import java.awt.Color

object ThemeCss {
    fun variables(): Map<String, String> {
        val scheme = EditorColorsManager.getInstance().globalScheme
        val labelFont = UIUtil.getLabelFont()
        val editorFont = scheme.editorFontName
        val labelFamily = labelFont.family.replace("'", "\\'")
        val editorFamily = editorFont.replace("'", "\\'")
        val uiFontStack = "'$labelFamily', 'JetBrains Sans', 'Inter', 'Segoe UI', 'Ubuntu', 'Noto Sans', 'DejaVu Sans', sans-serif"
        val editorFontStack = "'$editorFamily', 'JetBrains Mono', 'SFMono-Regular', 'Consolas', 'Liberation Mono', 'DejaVu Sans Mono', monospace"
        val panel = named("Panel.background", JBColor(Color(0xF2F2F2), Color(0x2B2D30)))
        val foreground = named("Label.foreground", JBColor(Color(0x1F1F1F), Color(0xDFE1E5)))
        val border = named("Component.borderColor", JBColor(Color(0xD1D1D1), Color(0x43454A)))
        val input = named("TextField.background", JBColor(Color(0xFFFFFF), Color(0x1E1F22)))
        val inputFg = named("TextField.foreground", foreground)
        val selection = named("List.selectionBackground", JBColor(Color(0x2F65CA), Color(0x2F65CA)))
        val selectionFg = named("List.selectionForeground", JBColor(Color.WHITE, Color.WHITE))
        val focus = named("Component.focusColor", JBColor(Color(0x2F65CA), Color(0x3574F0)))
        val link = named("Link.activeForeground", JBColor(Color(0x2F65CA), Color(0x589DF6)))
        val button = named("Button.startBackground", JBColor(Color(0x2F65CA), Color(0x365880)))
        val buttonFg = named("Button.foreground", JBColor(Color.WHITE, Color.WHITE))
        val buttonHover = named("Button.hoverStartBackground", JBColor(Color(0x3A78D8), Color(0x4A6B8C)))
        val error = named("Component.errorFocusColor", JBColor(Color(0xC7222D), Color(0xFF5261)))
        val warning = named("Component.warningFocusColor", JBColor(Color(0xE2A03F), Color(0xE2A03F)))
        val success = JBColor(Color(0x59A869), Color(0x499C54))
        val editorBg = scheme.defaultBackground
        val codeBg = named("Editor.background", editorBg)
        val description = UIUtil.getLabelDisabledForeground()
        val section = named("SidePanel.background", panel)

        return linkedMapOf(
            "--vscode-foreground" to hex(foreground),
            "--vscode-sideBar-background" to hex(panel),
            "--vscode-editor-background" to hex(editorBg),
            "--vscode-descriptionForeground" to hex(description),
            "--vscode-panel-border" to hex(border),
            "--vscode-sideBarSectionHeader-border" to hex(border),
            "--vscode-sideBarSectionHeader-background" to hex(section),
            "--vscode-input-background" to hex(input),
            "--vscode-input-foreground" to hex(inputFg),
            "--vscode-input-placeholderForeground" to hex(description),
            "--vscode-input-border" to hex(border),
            "--vscode-focusBorder" to hex(focus),
            "--vscode-button-background" to hex(button),
            "--vscode-button-foreground" to hex(buttonFg),
            "--vscode-button-hoverBackground" to hex(buttonHover),
            "--vscode-button-secondaryBackground" to hex(panel),
            "--vscode-button-secondaryForeground" to hex(foreground),
            "--vscode-list-activeSelectionBackground" to hex(selection),
            "--vscode-list-activeSelectionForeground" to hex(selectionFg),
            "--vscode-textLink-foreground" to hex(link),
            "--vscode-textCodeBlock-background" to hex(codeBg),
            "--vscode-editorWidget-background" to hex(panel),
            "--vscode-editorSuggestWidget-background" to hex(panel),
            "--vscode-editorSuggestWidget-foreground" to hex(foreground),
            "--vscode-editorSuggestWidget-border" to hex(border),
            "--vscode-editorSuggestWidget-selectedBackground" to hex(selection),
            "--vscode-editorSuggestWidget-selectedForeground" to hex(selectionFg),
            "--vscode-badge-background" to hex(selection),
            "--vscode-badge-foreground" to hex(selectionFg),
            "--vscode-textBlockQuote-border" to hex(border),
            "--vscode-textBlockQuote-foreground" to hex(description),
            "--vscode-testing-iconPassed" to hex(success),
            "--vscode-testing-iconFailed" to hex(error),
            "--vscode-editorWarning-foreground" to hex(warning),
            "--vscode-progressBar-background" to hex(link),
            "--vscode-terminal-ansiCyan" to hex(JBColor(Color(0x2AA198), Color(0x56B6C2))),
            "--vscode-terminal-ansiGreen" to hex(JBColor(Color(0x3A9A4A), Color(0x73C991))),
            "--vscode-terminal-ansiBlue" to hex(JBColor(Color(0x2F65CA), Color(0x589DF6))),
            "--vscode-icon-foreground" to hex(foreground),
            "--vscode-font-family" to uiFontStack,
            "--vscode-font-size" to "${labelFont.size}px",
            "--vscode-editor-font-family" to editorFontStack,
            "--vscode-editor-font-size" to "${scheme.editorFontSize}px",
            "--vscode-symbolIcon-keywordForeground" to hex(JBColor(Color(0x0033B3), Color(0xCF8E6D))),
            "--vscode-symbolIcon-functionForeground" to hex(JBColor(Color(0x00627A), Color(0x56A8F5))),
            "--vscode-symbolIcon-classForeground" to hex(JBColor(Color(0x000000), Color(0xA9B7C6))),
            "--vscode-symbolIcon-variableForeground" to hex(JBColor(Color(0x660E00), Color(0xC77DBB))),
            "--vscode-symbolIcon-constantForeground" to hex(JBColor(Color(0x0000C0), Color(0x4FC1FF))),
            "--vscode-debugTokenExpression-string" to hex(JBColor(Color(0x067D17), Color(0x6AAB73))),
            "--vscode-debugTokenExpression-number" to hex(JBColor(Color(0x1750EB), Color(0x2AACB8))),
            "--vscode-notificationsInfoIcon-foreground" to hex(link),
            "--vscode-scrollbarSlider-background" to hex(named("ScrollBar.thumb", JBColor(Color(0xC4C4C4), Color(0x5A5D63)))),
            "--vscode-scrollbarSlider-hoverBackground" to hex(named("ScrollBar.hoverThumbColor", JBColor(Color(0xA8A8A8), Color(0x7A7D83)))),
        )
    }

    private fun named(key: String, fallback: Color): Color = JBColor.namedColor(key, fallback)

    private fun hex(color: Color): String = "#%02x%02x%02x".format(color.red, color.green, color.blue)
}
