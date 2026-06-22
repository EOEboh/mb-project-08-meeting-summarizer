.PHONY: run build tidy clean setup check help

## run: start the Go web server
run:
	go run main.go

## build: compile to ./bin/app
build:
	@mkdir -p bin
	go build -o bin/app .
	@echo "Built → bin/app"

## tidy: download Go dependencies
tidy:
	go mod tidy

## clean: remove build artifacts
clean:
	rm -rf bin/

## setup: download Go deps, confirm Ollama, and check whisper
setup:
	@echo "=== Downloading Go dependencies ==="
	go mod tidy
	@echo ""
	@echo "=== Confirming llama3.2:3b ==="
	ollama pull llama3.2:3b
	@echo ""
	@$(MAKE) check

## check: verify whisper binary and model are accessible
check:
	@echo "=== Whisper binary ==="
	@which whisper-cpp 2>/dev/null && echo "  whisper-cpp: $$(which whisper-cpp)" || \
	  which whisper-cli 2>/dev/null && echo "  whisper-cli: $$(which whisper-cli)" || \
	  echo "  NOT FOUND — macOS: brew install whisper-cpp | Linux: build from source"
	@echo ""
	@echo "=== Whisper model ==="
	@if [ -n "$$WHISPER_MODEL" ] && [ -f "$$WHISPER_MODEL" ]; then \
	  echo "  Found (from env): $$WHISPER_MODEL"; \
	elif [ -f "$$HOME/whisper-models/ggml-base.en.bin" ]; then \
	  echo "  Found (default): $$HOME/whisper-models/ggml-base.en.bin"; \
	else \
	  echo "  NOT FOUND — download with:"; \
	  echo "    mkdir -p ~/whisper-models"; \
	  echo "    curl -L -o ~/whisper-models/ggml-base.en.bin \\"; \
	  echo "      https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en.bin"; \
	fi
	@echo ""
	@echo "=== Ollama ==="
	@curl -sf http://localhost:11434/api/tags > /dev/null && \
	  echo "  Running at localhost:11434" || \
	  echo "  NOT running — start with: ollama serve"

## help: list all commands
help:
	@grep -E '^##' Makefile | sed 's/## //'