#!/bin/bash

# RI Studio Development Start Script

set -e

echo "🚀 Starting RI Studio..."
echo ""

# Check if we're in the right directory
if [ ! -f "README.md" ]; then
    echo "❌ Error: Please run this script from the docs/ri-studio directory"
    exit 1
fi

# Check if node_modules exists
if [ ! -d "web/node_modules" ]; then
    echo "📦 Installing frontend dependencies..."
    cd web && npm install && cd ..
    echo "✅ Dependencies installed"
    echo ""
fi

# Check if frontend is built
if [ ! -d "web/dist" ]; then
    echo "🔨 Building frontend for production..."
    cd web && npm run build && cd ..
    echo "✅ Frontend built"
    echo ""
fi

echo "🎯 Starting RI Studio server..."
echo "   Server will be available at: http://localhost:8080"
echo ""
echo "   For development mode with hot-reload:"
echo "     Terminal 1: cd server && go run main.go"
echo "     Terminal 2: cd web && npm run dev (open http://localhost:3000)"
echo ""

cd server && go run main.go

