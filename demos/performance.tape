# VHS Performance Demo: Python vs Go
# Run from demos/: cd demos && vhs performance.tape
# Requires: llm-support.py symlink and llm-support in PATH

Output performance.gif
Set Width 800
Set Height 350
Set TypingSpeed 20ms

# Python
Type "echo 'Python:' && time python3 llm-support.py detect ~/Documents/GitHub/llm-interface"
Enter
Sleep 1s

# Go
Type "echo 'Go:' && time llm-support detect --path ~/Documents/GitHub/llm-interface"
Enter
Sleep 1s
