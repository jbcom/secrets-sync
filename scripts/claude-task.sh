#!/bin/bash
# Helper script to run Claude Code CLI tasks in background
# Usage: ./scripts/claude-task.sh "your prompt here" [log_suffix]

PROMPT="$1"
LOG_SUFFIX="${2:-task}"
LOG_FILE="/tmp/claude-${LOG_SUFFIX}-$(date +%s).log"

if [ -z "$PROMPT" ]; then
    echo "Usage: $0 'prompt' [log_suffix]"
    exit 1
fi

if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "Error: ANTHROPIC_API_KEY not set"
    exit 1
fi

echo "Starting Claude task..."
echo "Log file: $LOG_FILE"

# Run Claude - pipe prompt to stdin
(echo "$PROMPT" | claude -p --verbose --model sonnet --tools "Bash,Read,Write,Edit" > "$LOG_FILE" 2>&1) &

PID=$!
echo "Background PID: $PID"
echo "Monitor with: tail -f $LOG_FILE"
