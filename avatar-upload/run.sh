#!/bin/bash
# Simple script to run the avatar upload example

cd "$(dirname "$0")"

echo "🚀 Starting Avatar Upload Example..."
echo "📍 Server will run at http://localhost:8082"
echo ""

# Set PORT and run
PORT=8082 go run main.go
