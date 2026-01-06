# ElastiCat Demo
Output docs/demo.mp4

# Terminal settings
Set FontSize 18
Set Width 1600
Set Height 1000

# Launch the TUI
Type "bin/elasticat ui"
Enter
Sleep 500ms

# Scroll through logs
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "j"
Sleep 500ms

# View log detail
Enter
Sleep 2s

# Go back
Escape
Sleep 1s

# Switch to metrics
Type "m"
Sleep 2s

# Scroll through metrics
Type "j"
Type "j"
Type "j"
Type "j"
Sleep 500ms

# View metric detail (chart + values)
Enter
Sleep 2s

# Scroll through metric values
Type "j"
Sleep 300ms
Type "j"
Sleep 500ms

# Go back to metrics list
Escape
Sleep 1s

# Switch to traces  
Type "m"
Sleep 2s

# Scroll through trace transactions
Type "j"
Sleep 300ms
Type "j"
Sleep 500ms

# Select a transaction to see traces
Enter
Sleep 2s

# Scroll through traces for this transaction
Type "j"
Sleep 300ms
Type "j"
Sleep 500ms

# View trace detail (spans)
Enter
Sleep 3s

# Scroll through spans
Type "j"
Sleep 300ms
Type "j"
Sleep 500ms

# Go back to trace list
Escape
Sleep 1s

# Go back to transaction names
Escape
Sleep 1s

# Go back to logs
Type "m"
Sleep 1s

# Show perspectives (filter by service)
Type "p"
Sleep 2s

# Select a service
Enter
Sleep 2s

# Show search
Type "/"
Sleep 500ms
Type "error"
Enter
Sleep 2s