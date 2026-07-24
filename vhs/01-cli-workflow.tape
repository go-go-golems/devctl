# devctl CLI workflow demo
# Shows the basic commands: plugins list → plan → up → status → logs → down

Output "01-cli-workflow.gif"

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 800
Set Padding 30
Set Theme "Dracula"
Set TypingSpeed 50ms

# Start fresh
Hide
Type "cd /tmp/devctl-demo-repo && devctl down 2>/dev/null; clear" Enter
Sleep 500ms
Show

# Show we're in a project
Type "# A typical repo with a backend API and frontend" Enter
Sleep 1s
Type "ls -la" Enter
Sleep 2s

Type "clear" Enter
Sleep 500ms

# Show the config
Type "# devctl configuration - defines our plugin" Enter
Sleep 500ms
Type "cat .devctl.yaml" Enter
Sleep 3s

Type "clear" Enter
Sleep 500ms

# List plugins
Type "# Step 1: Verify plugin loads correctly" Enter
Sleep 500ms
Type "devctl plugins list" Enter
Sleep 3s

# Show the plan
Type "" Enter
Type "# Step 2: See what would run (no side effects)" Enter
Sleep 500ms
Type "devctl plan | head -30" Enter
Sleep 4s

# Start everything
Type "" Enter
Type "# Step 3: Start all services" Enter
Sleep 500ms
Type "devctl up" Enter
Sleep 6s

# Check status
Type "" Enter
Type "# Step 4: Check what's running" Enter
Sleep 500ms
Type "devctl status" Enter
Sleep 4s

# Test the API
Type "" Enter
Type "# The API is live - let's test it" Enter
Sleep 500ms
Type "curl -s http://localhost:8080/health" Enter
Sleep 2s

# Show logs
Type "" Enter
Type "# Step 5: View service logs" Enter
Sleep 500ms
Type "devctl logs api | tail -5" Enter
Sleep 3s

# Stop everything
Type "" Enter
Type "# Step 6: Stop all services and clean up" Enter
Sleep 500ms
Type "devctl down" Enter
Sleep 3s

Type "" Enter
Type "# Done! The whole workflow: plugins list → plan → up → status → logs → down" Enter
Sleep 3s
