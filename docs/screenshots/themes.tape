# VHS tape for capturing theme screenshots using the real TUI.
# Usage:
#   go build -o /tmp/glamdring-demo ./cmd/glamdring
#   vhs docs/screenshots/themes.tape

Set Shell zsh
Set FontFamily "Monaco"
Set FontSize 12
Set LetterSpacing 0
Set LineHeight 1.2
Set Padding 0
Set Width 900
Set Height 600

Hide
Type "stty -echo && /tmp/glamdring-demo --demo --demo-theme glamdring"
Enter
Sleep 2s
Show
Sleep 1s
Screenshot docs/screenshots/theme-glamdring.png

Hide
Type "/quit"
Enter
Sleep 500ms
Type "/tmp/glamdring-demo --demo --demo-theme rivendell"
Enter
Sleep 2s
Show
Sleep 1s
Screenshot docs/screenshots/theme-rivendell.png

Hide
Type "/quit"
Enter
Sleep 500ms
Type "/tmp/glamdring-demo --demo --demo-theme mithril"
Enter
Sleep 2s
Show
Sleep 1s
Screenshot docs/screenshots/theme-mithril.png

Hide
Type "/quit"
Enter
Sleep 500ms
Type "/tmp/glamdring-demo --demo --demo-theme lothlorien"
Enter
Sleep 2s
Show
Sleep 1s
Screenshot docs/screenshots/theme-lothlorien.png

Hide
Type "/quit"
Enter
Sleep 500ms
Type "/tmp/glamdring-demo --demo --demo-theme shire"
Enter
Sleep 2s
Show
Sleep 1s
Screenshot docs/screenshots/theme-shire.png

Hide
Type "/quit"
Enter
Sleep 500ms
Type "/tmp/glamdring-demo --demo --demo-theme anduin"
Enter
Sleep 2s
Show
Sleep 1s
Screenshot docs/screenshots/theme-anduin.png

Hide
Type "/quit"
Enter
Sleep 500ms
