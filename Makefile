.PHONY: run build tidy clean setup setup-whisper start-whisper help

## run: start the Go web server (requires whisper server + ollama already running)
run:
	go run main.go

## build: compile to ./bin/app
build:
	@mkdir -p bin
	go build -o bin/app .
	@echo "✅ Built → bin/app"

## tidy: download Go dependencies
tidy:
	go mod tidy

## clean: remove build artifacts
clean:
	rm -rf bin/

## setup: download Go deps and confirm Ollama model
setup:
	@echo "Downloading Go dependencies (goldmark)..."
	go mod tidy
	@echo "Confirming llama3.2:3b..."
	ollama pull llama3.2:3b
	@echo ""
	@echo "⚠ You also need whisper.cpp running. See: make setup-whisper"
	@echo ""
	@echo "To start everything:"
	@echo "  Terminal 1: ollama serve"
	@echo "  Terminal 2: make start-whisper   (from your whisper.cpp directory)"
	@echo "  Terminal 3: make run"

## setup-whisper: print whisper.cpp setup instructions
setup-whisper:
	@echo ""
	@echo "── whisper.cpp Setup ────────────────────────────────────────────────"
	@echo ""
	@echo "1. Clone whisper.cpp:"
	@echo "   git clone https://github.com/ggerganov/whisper.cpp"
	@echo "   cd whisper.cpp"
	@echo ""
	@echo "2. Build it:"
	@echo "   make"
	@echo ""
	@echo "3. Download the base model (~150 MB):"
	@echo "   bash models/download-ggml-model.sh base.en"
	@echo ""
	@echo "4. Start the server:"
	@echo "   ./server -m models/ggml-base.en.bin --port 8888"
	@echo ""
	@echo "The server will be available at http://localhost:8888"
	@echo "Set WHISPER_URL env var to override."
	@echo ""

## start-whisper: start the whisper.cpp server (run from your whisper.cpp directory)
## Update WHISPER_DIR to your local whisper.cpp path
start-whisper:
	@echo "Run this from your whisper.cpp directory:"
	@echo "  ./server -m models/ggml-base.en.bin --port 8888"

## help: list all commands
help:
	@grep -E '^##' Makefile | sed 's/## //'