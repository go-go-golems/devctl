# devctl TUI screenshots
# Captures static PNG screenshots for README documentation

# No GIF output - we only want screenshots
# Output "screenshot-tui.gif"

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 800
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

# Start fresh - ensure clean state
Hide
Type "cd /tmp/devctl-demo-repo && devctl down 2>/dev/null; clear" Enter
Sleep 500ms
Show

# Start the TUI
Type "devctl tui --alt-screen=false" Enter
Sleep 3s

# Start services to show real data
Type "u"
Sleep 5s

# Screenshot the Overview view
Screenshot ../docs/screenshots/devctl-tui-overview.png
Sleep 500ms

# Switch to Logs
Type "2"
Sleep 1s

# Screenshot Logs
Screenshot ../docs/screenshots/devctl-tui-logs.png
Sleep 500ms

# Switch to Runs
Type "3"
Sleep 1s

# Screenshot Runs
Screenshot ../docs/screenshots/devctl-tui-runs.png
Sleep 500ms

# Clean up - stop services and quit
Type "1"
Sleep 500ms
Type "d"
Sleep 500ms
Type "y"
Sleep 2s
Type "q"
Sleep 500ms
