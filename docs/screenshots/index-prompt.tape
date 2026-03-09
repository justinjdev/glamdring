# VHS tape for capturing the index build prompt screenshot.
# Usage:
#   go build -o /tmp/glamdring-demo ./cmd/glamdring
#   vhs docs/screenshots/index-prompt.tape

Set Shell zsh
Set FontFamily "Monaco"
Set FontSize 12
Set LetterSpacing 0
Set LineHeight 1.2
Set Padding 0
Set Width 900
Set Height 600

Hide
Type "stty -echo && /tmp/glamdring-demo --demo-index-prompt --demo-theme glamdring"
Enter
Sleep 2s
Show
Sleep 1s
Screenshot docs/screenshots/index-prompt.png

Hide
Type "/quit"
Enter
Sleep 500ms
