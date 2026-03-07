# VHS tape for capturing theme screenshots.
# Usage: vhs docs/screenshots/themes.tape

Set Shell zsh
Set Width 1200
Set Height 800
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Padding 24

Hide
Type "stty -echo"
Enter
Sleep 500ms
Show

Type "clear && docs/screenshots/preview.sh glamdring"
Enter
Sleep 3s
Screenshot docs/screenshots/theme-glamdring.png

Type "clear && docs/screenshots/preview.sh rivendell"
Enter
Sleep 3s
Screenshot docs/screenshots/theme-rivendell.png

Type "clear && docs/screenshots/preview.sh mithril"
Enter
Sleep 3s
Screenshot docs/screenshots/theme-mithril.png

Type "clear && docs/screenshots/preview.sh lothlorien"
Enter
Sleep 3s
Screenshot docs/screenshots/theme-lothlorien.png

Type "clear && docs/screenshots/preview.sh shire"
Enter
Sleep 3s
Screenshot docs/screenshots/theme-shire.png
Sleep 500ms
