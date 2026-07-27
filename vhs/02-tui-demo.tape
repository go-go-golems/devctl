# devctl TUI demo
# Shows the interactive terminal UI

Output "02-tui-demo.gif"

Set Shell "bash"
Set FontSize 15
Set Width 1400
Set Height 900
Set Padding 20
Set Theme "Dracula"
Set TypingSpeed 50ms

# Start fresh
Hide
Type "cd /tmp/devctl-demo-repo && devctl down 2>/dev/null; clear" Enter
Sleep 500ms
Show

Type "# The TUI is an interactive dashboard for your dev environment" Enter
Sleep 1s
Type "devctl tui --alt-screen=false" Enter
Sleep 3s

# Show help
Type "?"
Sleep 3s
Type "?"
Sleep 1s

# Start services with 'u'
Type "u"
Sleep 6s

# Navigate to second service
Type "j"
Sleep 1s

# Open logs for the web service
Enter
Sleep 3s

# Toggle follow mode
Type "f"
Sleep 2s

# Pause and resume visible ingestion
Type "p"
Sleep 1s
Type "p"
Sleep 1s

# Go back to Overview
Escape
Sleep 1s

# Navigate to first service
Type "k"
Sleep 1s

# Switch directly to Runs
Type "3"
Sleep 3s

# Open the command palette and refresh the snapshot
Type ":"
Sleep 1s
Enter
Sleep 2s

# Back to Overview
Type "1"
Sleep 2s

# Stop services with confirmation
Type "d"
Sleep 1s
Type "y"
Sleep 3s

# Quit
Type "q"
Sleep 1s

Type "" Enter
Type "# Press ? for help, Tab to switch views, q to quit" Enter
Sleep 2s
