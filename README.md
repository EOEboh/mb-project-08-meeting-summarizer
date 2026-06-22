# Project 08: Meeting Notes Summarizer

> **Bootcamp Day 19** · Tier 3: Complex · Est. build time: 75 min

Upload a meeting recording and get structured notes: a summary, decisions made, action items with interactive checkboxes, and next steps. Download everything as a markdown file.

---

## What You Will Build

A meeting notes tool where:

- The user uploads an audio file (WAV, MP3, M4A, OGG, FLAC, WebM)
- **Step 1** — whisper.cpp transcribes the audio to text (a separate local AI service)
- **Step 2** — llama3.2:3b receives the transcript and returns a structured four-section summary
- The result renders as four cards: Meeting Summary, Decisions Made, Action Items, Next Steps
- Action items render as interactive checkboxes (goldmark converts `- [ ]` syntax to HTML checkboxes)
- A collapsible section shows the raw Whisper transcript for verification
- A Download button exports the full summary as `meeting-notes.md`

---

## Key Concepts Introduced

| Concept | What It Teaches |
|---|---|
| Two-service AI pipeline | Routing to the right model for the right modality: Whisper (speech-to-text) then Ollama (language reasoning) |
| `whisper/client.go` package | A second AI service client alongside `ai/ollama.go` — same HTTP pattern, different API |
| Audio file validation | Extension-based validation and why `http.DetectContentType` is unreliable for audio |
| Long HTTP client timeout | `http.Client{Timeout: 10 * time.Minute}` for operations that legitimately take minutes |
| Structured four-section parsing | Same `parseSections` pattern from Project 03, applied to a richer output schema |
| GitHub task list rendering | goldmark converts `- [ ]` to `<input type="checkbox">` for interactive action items |
| Three running services | First project requiring orchestration of three local processes simultaneously |

---

## Prerequisites

- Go 1.22+
- Ollama running with `llama3.2:3b` pulled
- whisper.cpp compiled and its server running on port 8888

---

## Setup

### 1. whisper.cpp (do this once)

```bash
# Clone and build
git clone https://github.com/ggerganov/whisper.cpp
cd whisper.cpp
make

# Download the base.en model (~150 MB)
bash models/download-ggml-model.sh base.en

# Start the server on port 8888
./server -m models/ggml-base.en.bin --port 8888
```

### 2. Go dependencies

```bash
git clone https://github.com/EOEboh/mb-project-08-meeting-summarizer
cd mb-project-08-meeting-summarizer
make setup
```

### 3. Run (three terminals)

```bash
# Terminal 1
ollama serve

# Terminal 2 (from your whisper.cpp directory)
./server -m models/ggml-base.en.bin --port 8888

# Terminal 3 (from this project)
make run
# → http://localhost:8080
```

---

## Project Structure

```
mb-project-08-meeting-summarizer/
├── main.go                   Three routes: /, /summarize, /export
├── go.mod                    goldmark only (whisper.cpp is a separate process)
├── Makefile                  make run, make setup, make setup-whisper
├── .env.example
├── whisper/
│   └── client.go             HTTP client for the whisper.cpp /inference endpoint
├── ai/
│   └── ollama.go             Chat() unchanged from scaffold
├── handlers/
│   └── notes.go              Two-service pipeline, section parsing, export
├── templates/
│   └── index.html            "index" + "result" named templates
└── static/
    └── style.css
```

---

## Architecture

```
Browser                   Go Server                 Whisper Server       Ollama
  |                           |                     (port 8888)          (port 11434)
  |  POST /summarize          |                           |                   |
  |  multipart: audio file    |                           |                   |
  |-------------------------> |                           |                   |
  |                           | validate (ext, size)      |                   |
  |                           | io.ReadAll(file)          |                   |
  |                           |                           |                   |
  |                           |  POST /inference          |                   |
  |                           |  (multipart: audio)       |                   |
  |                           |-------------------------> |                   |
  |                           |                           | ggml-base.en      |
  |                           |  {"text": "..."}          |                   |
  |                           | <------------------------ |                   |
  |                           |                           |                   |
  |                           | validate transcript       |                   |
  |                           | (min 20 words)            |                   |
  |                           |                                               |
  |                           |  POST /api/chat                               |
  |                           |  (summarySystemPrompt + transcript)           |
  |                           |---------------------------------------------> |
  |                           |                           llama3.2:3b         |
  |                           |  structured markdown                          |
  |                           | <--------------------------------------------- |
  |                           |                                               |
  |                           | parseSections(raw)                            |
  |                           | mdToHTML() x 4 sections                       |
  |                           | ExecuteTemplate("result", data)               |
  |  text/html fragment       |                                               |
  | <------------------------ |                                               |
  |  4 section cards          |                                               |
  |  transcript accordion     |                                               |
```

---

## Key Design Decisions

### Why whisper.cpp as a separate server, not a Go library?

whisper.cpp has Go bindings but they require CGO — a C compiler must be present at build time. Every other project in this bootcamp builds with pure Go. The server mode keeps the stack CGO-free: whisper.cpp runs as its own process and Go calls it via HTTP, the same way Go calls Ollama.

### Why extension-based validation for audio?

`http.DetectContentType` is reliable for images (JPEG, PNG, WEBP all have clear magic bytes). Audio is messier: M4A reports as `video/mp4`, OGG as `application/ogg`, WebM as `video/webm`. None of these communicate "audio" unambiguously to a validator. Extension checking gives users a clear, accurate error message and is the right tool when byte sniffing is unreliable for the input type.

### Why a 10-minute HTTP client timeout?

`http.DefaultClient` has no timeout, which means a hanging whisper.cpp process would hang the Go handler indefinitely. A 10-minute cap is generous enough for a 30-minute meeting recording on typical hardware (transcription usually takes 10-20% of audio duration with the base model) while still preventing indefinite hangs.

### Why `- [ ]` for action items?

The GitHub task list format is standard markdown that goldmark converts to real `<input type="checkbox">` elements. JavaScript in the result template enables the checkboxes and applies a strikethrough class on check. This turns the action items from a static list into a usable checklist without any server-side state management.

---

## Makefile Commands

```bash
make run           # start the Go server (whisper + ollama must be running)
make setup         # download Go deps and confirm Ollama model
make setup-whisper # print whisper.cpp build and setup instructions
make build         # compile to bin/app
make help
```

---

## Off-Day Extensions

| # | Extension | What It Builds Toward |
|---|---|---|
| E1 | Show a live progress bar: "Transcribing... Summarising..." using SSE | Combining audio pipeline with streaming (Project 01 pattern) |
| E2 | Save summaries to SQLite with timestamps (reuse Project 05's db/ pattern) | Persisting AI pipeline output |
| E3 | Add a speaker detection mode: ask Whisper for VTT output and parse speaker turns | Structured transcript processing |
| E4 | Support YouTube URLs: fetch audio via yt-dlp subprocess, feed to pipeline | Subprocess and external tool integration |
| E5 | Let users ask follow-up questions about the transcript | Multi-turn conversation with persistent context |

---

## What Is Next

**Project 09: AI Agent with Tool Use** : ReAct pattern from scratch with no frameworks. The agent reasons, selects a tool (web search, calculator, etc.), executes it, and uses the result in its next reasoning step. First project where the AI drives the control flow rather than the Go code.