Output docs/tui-demo.gif

Set Shell "zsh"
Set FontSize 14
Set Width 1280
Set Height 760
Set Padding 16
Set TypingSpeed 40ms
Set Theme "Catppuccin Mocha"

Type "go run ./cmd/codepresent --demo --tui"
Enter
Sleep 2.5s

# Step 2: Loading & Validating a Story
Right
Sleep 2s

# Scroll code pane down and back up to showcase scrolling + connector tracking
Down@120ms 12
Sleep 900ms
Up@120ms 12
Sleep 900ms

# Step 3: Two-Way Editor Sync
Right
Sleep 2.5s

# Toggle Play/Pause and jump back to Step 1
Space
Sleep 1.2s
Type "1"
Sleep 1.2s

# Quit cleanly
Type "q"
Sleep 400ms
