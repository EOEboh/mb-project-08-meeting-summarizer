# Project 08: Meeting Notes Summarizer

> **Bootcamp Day 8** · Tier 3: Complex · Est. build time: 75 min

Upload a meeting recording and get structured notes: a summary, decisions made, action items with interactive checkboxes, and next steps. Download everything as a markdown file.

---

## What You Will Build

A meeting notes tool where:

- The user uploads an audio file (WAV, MP3, M4A, OGG, FLAC, WebM)
- **Step 1** — Go calls the `whisper-cli` binary as a subprocess to transcribe the audio to text
- **Step 2** — llama3.2:3b receives the transcript and returns a structured four-section summary
- The result renders as four cards: Meeting Summary, Decisions Made, Action Items, Next Steps
- Action items render as interactive checkboxes (`- [ ]` markdown rendered by goldmark)
- A collapsible panel shows the full raw transcript
- A Download button exports the summary as `meeting-notes.md`

---

## Key Concepts Introduced

| Concept | What It Teaches |
|---|---|
| Two-service AI pipeline | Routing to the right model for the right modality: Whisper for audio, Ollama for language |
| `exec.Command` subprocess | Calling an external binary from Go and capturing its output |
| `os.CreateTemp` + `defer os.Remove` | Writing a short-lived file safely and guaranteeing cleanup on all return paths |
| `godotenv` | Loading a `.env` file at Go startup without manual `source` or `export` commands |
| `exec.LookPath` | Checking whether a binary exists on the system PATH before trying to run it |
| Audio file validation | Extension-based validation and why `http.DetectContentType` is unreliable for audio |
| Four-section structured parsing | The same `parseSections` pattern from Project 03 applied to a richer meeting schema |
| GitHub task list rendering | goldmark converts `- [ ]` to `<input type="checkbox">` for interactive action items |

---

## Prerequisites

- Go 1.22+
- Ollama installed with `llama3.2:3b` pulled
- `whisper-cli` binary installed (setup below, per OS)
- A whisper ggml model file downloaded (setup below)

---

## Part 1: Install whisper.cpp

### macOS

```bash
brew install whisper-cpp
```

> **Note:** Despite the package being called `whisper-cpp`, Homebrew installs a binary named `whisper-cli`. This is correct and expected.

Confirm the binary is there:

```bash
which whisper-cli
# Should print: /opt/homebrew/bin/whisper-cli
```

---

### Linux (Ubuntu / Debian)

```bash
sudo apt update && sudo apt install -y cmake build-essential git

cd ~
git clone https://github.com/ggerganov/whisper.cpp
cd whisper.cpp
cmake -B build && cmake --build build --config Release
```

Add the binary to your PATH (add this line to `~/.bashrc` to make it permanent):

```bash
export PATH="$PATH:$HOME/whisper.cpp/build/bin"
source ~/.bashrc
```

Confirm:

```bash
which whisper-cli
```

---

### Windows (MSYS2)

1. Install [MSYS2](https://www.msys2.org) and open the **MSYS2 UCRT64** terminal.

2. Install build tools:

```bash
pacman -S mingw-w64-ucrt-x86_64-cmake mingw-w64-ucrt-x86_64-gcc git curl
```

3. Clone and build:

```bash
cd ~
git clone https://github.com/ggerganov/whisper.cpp
cd whisper.cpp
cmake -B build && cmake --build build --config Release
export PATH="$PATH:$(pwd)/build/bin"
```

Add the `export PATH` line to `~/.bashrc` to make it permanent.

---

## Part 2: Download the model

This step is the same on all platforms. Run it once and the model lives at `~/whisper-models/ggml-base.en.bin`.

```bash
mkdir -p ~/whisper-models
curl -L -o ~/whisper-models/ggml-base.en.bin \
  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en.bin"
```

The download is approximately 150 MB.

Verify transcription works before continuing:

```bash
# Download an 11-second sample clip
curl -L -o /tmp/jfk.wav \
  "https://github.com/ggerganov/whisper.cpp/raw/master/samples/jfk.wav"

# Transcribe it
whisper-cli -m ~/whisper-models/ggml-base.en.bin -f /tmp/jfk.wav
# Should print: "And so my fellow Americans..."
```

If that works, whisper is correctly installed.

---

## Part 3: Set up the Go project

```bash
git clone https://github.com/EOEboh/mb-project-08-meeting-summarizer
cd mb-project-08-meeting-summarizer
```

Copy the environment file and fill in your actual paths:

```bash
cp .env.example .env
```

Open `.env` in your editor. The two values you must set correctly:

```bash
# Find your actual username
echo $USER
```

```env
# macOS example (replace "yourname" with your actual username from echo $USER):
WHISPER_MODEL=/Users/yourname/whisper-models/ggml-base.en.bin
WHISPER_BIN=whisper-cli

# Linux example:
WHISPER_MODEL=/home/yourname/whisper-models/ggml-base.en.bin
WHISPER_BIN=whisper-cli

# Windows (MSYS2) example:
WHISPER_MODEL=C:/Users/yourname/whisper-models/ggml-base.en.bin
WHISPER_BIN=whisper-cli
```

> **Important:** The app reads `.env` automatically at startup via `godotenv`. You do not need to run `source .env` or `export` anything manually.

Install Go dependencies:

```bash
make tidy
```

---

## Part 4: Verify everything before running

```bash
make check
```

You should see all three sections showing green:

```
=== Whisper binary ===
  whisper-cli: /opt/homebrew/bin/whisper-cli

=== Whisper model ===
  Found (default): /Users/yourname/whisper-models/ggml-base.en.bin

=== Ollama ===
  Running at localhost:11434
```

If anything shows NOT FOUND, fix it before continuing. Do not skip this step.

---

## Part 5: Run

> **macOS users:** Ollama runs as a background service and starts automatically on login. If `make check` shows Ollama running, you do not need to run `ollama serve`. If it shows NOT running, start it with `ollama serve`.

```bash
# If Ollama is not already running:
ollama serve   # keep this terminal open

# Start the Go server (in a new terminal):
make run
# → http://localhost:8080
```

If you see `bind: address already in use` on port 8080, another process is using it:

```bash
kill $(lsof -t -i :8080)
make run
```

---

## Common Issues

| Error | Cause | Fix |
|---|---|---|
| `whisper model not found at /Users/yourname/...` | Wrong username in WHISPER_MODEL | Run `echo $USER`, update .env with the correct username |
| `whisper binary "whisper-cli" not found` | whisper-cli not on PATH | macOS: `brew install whisper-cpp`. Linux: add build/bin to PATH |
| `address already in use` on port 8080 | Another process on port 8080 | `kill $(lsof -t -i :8080)` then `make run` |
| `ollama serve` error: address in use | Ollama already running | Good — skip that step, it is already running |
| Empty transcript | Silent or very short audio | Use a real recording with clear speech |

---

## Project Structure

```
mb-project-08-meeting-summarizer/
├── main.go              loads .env via godotenv, registers 3 routes
├── go.mod               godotenv + goldmark
├── .env.example         copy to .env and fill in your paths
├── Makefile             make run, make check, make setup
├── whisper/
│   └── client.go        calls whisper-cli as a subprocess via exec.Command
├── ai/
│   └── ollama.go        calls Ollama via HTTP as in every other project
├── handlers/
│   └── notes.go         two-service pipeline, section parsing, export
├── templates/
│   └── index.html       "index" + "result" named templates
└── static/
    └── style.css
```

---

## Architecture

```
Browser                Go Server              whisper-cli         Ollama
  |                        |                  (subprocess)        (port 11434)
  | POST /summarize         |                       |                 |
  | audio file              |                       |                 |
  |-----------------------> |                       |                 |
  |                         | validate ext + size   |                 |
  |                         | os.CreateTemp(audio)  |                 |
  |                         |                       |                 |
  |                         | exec.Command(         |                 |
  |                         |  whisper-cli -m model |                 |
  |                         |  -f /tmp/audio.mp3)   |                 |
  |                         |---------------------->|                 |
  |                         |                  transcribes           |
  |                         | reads .txt output     |                 |
  |                         |<----------------------|                 |
  |                         | defer os.Remove(temp files)            |
  |                         |                                        |
  |                         | ai.Chat(transcript + prompt)           |
  |                         |--------------------------------------> |
  |                         | structured markdown                    |
  |                         |<-------------------------------------- |
  |                         |                                        |
  |                         | parseSections + mdToHTML               |
  |                         | ExecuteTemplate("result")              |
  | text/html fragment      |                                        |
  |<----------------------- |                                        |
```

---

## Key Design Decisions

**Why subprocess instead of HTTP server?**
Homebrew's `whisper-cpp` package installs a CLI binary, not a server. Using `exec.Command` works with that binary directly, with no additional server process to start or manage.

**Why `godotenv`?**
Go's `os.Getenv()` only reads variables that are already exported in the shell session. It does not read `.env` files automatically. `godotenv.Load()` in `main.go` bridges this gap so students can set paths in `.env` without any shell-level exports.

**Why extension-based validation for audio?**
`http.DetectContentType` is reliable for images but not for audio: M4A reports as `video/mp4`, OGG as `application/ogg`. Extension checking is simpler and gives users a clearer error message.

**Why `defer os.Remove` immediately after `os.CreateTemp`?**
`defer` runs on every return path — success, validation failure, transcription error. Placing it immediately after the `CreateTemp` call guarantees the temp file is deleted regardless of how the function exits.

---

## Makefile Commands

```bash
make run     # start the Go server
make check   # verify whisper, model, and Ollama are all accessible
make setup   # download Go deps, pull Ollama model, run check
make tidy    # download Go dependencies only
make build   # compile to bin/app
make help
```

---

## Off-Day Extensions

| # | Extension | What It Builds Toward |
|---|---|---|
| E1 | Show live progress with SSE: "Transcribing... Summarising..." | Combining subprocess with streaming |
| E2 | Save summaries to SQLite with timestamps (reuse Project 05 db/ pattern) | Persisting AI pipeline output |
| E3 | Parse VTT output (`--output-vtt`) for timestamps | Structured transcript processing |
| E4 | Accept a YouTube URL via yt-dlp subprocess | Chaining multiple subprocess calls |
| E5 | Follow-up questions about the transcript | Multi-turn conversation with context |

---

## What Is Next

**Project 09: AI Agent with Tool Use** : ReAct pattern from scratch with no frameworks. The agent reasons, selects a tool (web search, calculator, etc.), executes it, and uses the result in its next reasoning step. First project where the AI drives the control flow rather than the Go code.