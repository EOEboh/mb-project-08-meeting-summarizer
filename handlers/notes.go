package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/EOEboh/mb-project-08-meeting-summarizer/ai"
	"github.com/EOEboh/mb-project-08-meeting-summarizer/whisper"
	"github.com/yuin/goldmark"
)

var tmpl = template.Must(template.ParseFiles("templates/index.html"))

// ── Limits and validation ─────────────────────────────────────────────────────

const (
	// maxAudioSize caps uploads at 50 MB.
	// A 30-minute mono MP3 at 64kbps is roughly 14 MB.
	// A 60-minute stereo MP3 at 128kbps is roughly 57 MB.
	// The cap keeps transcription times reasonable (under 5 minutes).
	maxAudioSize = 50 << 20

	// minTranscriptWords rejects transcripts that are too sparse to summarise.
	// A recording with fewer than 20 recognisable words is likely silent,
	// music-only, or too noisy for Whisper to parse usefully.
	minTranscriptWords = 20
)

// allowedExtensions lists the audio formats whisper.cpp accepts.
// We use the file extension as the primary validation gate because
// http.DetectContentType is unreliable for audio: M4A is reported as
// video/mp4, OGG as application/ogg, and WebM as video/webm — none of
// which immediately communicate "audio" to the validator. The extension
// check is fast, communicates a clear error to the user, and is sufficient
// for the threat model of a local demo tool.
var allowedExtensions = map[string]string{
	".wav":  "WAV",
	".mp3":  "MP3",
	".m4a":  "M4A",
	".ogg":  "OGG",
	".webm": "WebM",
	".mp4":  "MP4",
	".flac": "FLAC",
}

// ── Data types ────────────────────────────────────────────────────────────────

// summaryResult is the data passed to the "result" template.
type summaryResult struct {
	Error        string
	FileName     string
	WordCount    int // words in the transcript
	ElapsedSec   float64
	Transcript   string        // raw whisper output, shown in the collapsible section
	Overview     template.HTML // goldmark-rendered ## Meeting Summary section
	Decisions    template.HTML // goldmark-rendered ## Decisions Made section
	ActionItems  template.HTML // goldmark-rendered ## Action Items section
	NextSteps    template.HTML // goldmark-rendered ## Next Steps section
	FullMarkdown string        // complete AI output, embedded for export
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// Index serves the audio upload page.
func Index(w http.ResponseWriter, r *http.Request) {
	if err := tmpl.ExecuteTemplate(w, "index", nil); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// Summarize is the two-service audio pipeline handler.
//
// ── What is new in Project 08 ─────────────────────────────────────────────────
//
// Every previous project sent text or images directly to an AI model.
// This project introduces a preprocessing step: audio cannot be sent to
// Ollama (a text/vision LLM). Before the LLM is involved at all, the audio
// must be converted to text by a dedicated speech-to-text model.
//
// The pipeline has two service calls, each to a different AI system:
//
//	STEP 1 — Transcription (whisper.cpp server):
//	  Audio bytes → POST /inference → plain-text transcript
//	  Whisper is a separate service from Ollama. It is a speech-to-text
//	  specialist; it does not reason, summarise, or generate — it only
//	  converts spoken audio to written words.
//
//	STEP 2 — Summarisation (Ollama / llama3.2:3b):
//	  Transcript text → POST /api/chat → structured markdown summary
//	  The LLM receives the transcript as a standard user message. From
//	  Ollama's perspective, this is identical to every other project —
//	  just a text prompt. The complexity lives in the preprocessing, not
//	  in the LLM call itself.
//
// This is the "right model for the right modality" principle that defines
// mature AI system design: audio specialists handle audio, language models
// handle language. Trying to do both in one model is possible with some
// multimodal systems but produces worse results than specialised services.
// ─────────────────────────────────────────────────────────────────────────────
func Summarize(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// ── 1. Parse the multipart upload ─────────────────────────────────────
	if err := r.ParseMultipartForm(maxAudioSize); err != nil {
		renderFragment(w, summaryResult{Error: "Could not parse upload: " + err.Error()})
		return
	}

	file, header, err := r.FormFile("audio")
	if err != nil {
		renderFragment(w, summaryResult{Error: "Please select an audio file to upload."})
		return
	}
	defer file.Close()

	// ── 2. Validate: extension and size ───────────────────────────────────
	ext := strings.ToLower(filepath.Ext(header.Filename))
	label, ok := allowedExtensions[ext]
	if !ok {
		renderFragment(w, summaryResult{
			Error: fmt.Sprintf(
				"Unsupported file type (%s). Please upload a WAV, MP3, M4A, OGG, FLAC, WebM, or MP4 audio file.",
				ext,
			),
		})
		return
	}
	_ = label // used by allowedExtensions for user-facing labels; keep for documentation clarity

	if header.Size > maxAudioSize {
		renderFragment(w, summaryResult{
			Error: fmt.Sprintf(
				"File is too large (%s). Maximum size is 50 MB.",
				formatFileSize(header.Size),
			),
		})
		return
	}

	// ── 3. Read audio into memory ──────────────────────────────────────────
	// io.ReadAll is used here because whisper.cpp's server accepts the full
	// audio file as a multipart upload. We need the complete byte slice to
	// build the multipart request body in whisper/client.go.
	// This is the same reasoning as Project 06's image encoding.
	audioBytes := make([]byte, header.Size)
	if _, err := file.Read(audioBytes); err != nil {
		log.Printf("read audio: %v", err)
		renderFragment(w, summaryResult{Error: "Could not read the uploaded file."})
		return
	}

	// ── 4. Step 1: Transcribe via Whisper ─────────────────────────────────
	// The audio never touches Ollama. Whisper converts speech to text and
	// returns a plain string. Everything after this point is text processing.
	log.Printf("transcribing %s (%s)...", header.Filename, formatFileSize(header.Size))
	transcript, err := whisper.Transcribe(audioBytes, header.Filename)
	if err != nil {
		log.Printf("transcription error: %v", err)
		renderFragment(w, summaryResult{
			Error: "Transcription failed: " + err.Error(),
		})
		return
	}

	wordCount := len(strings.Fields(transcript))
	if wordCount < minTranscriptWords {
		renderFragment(w, summaryResult{
			Error: fmt.Sprintf(
				"The transcript is too short (%d words). The recording may be silent, "+
					"too noisy, or too brief to summarise meaningfully.",
				wordCount,
			),
		})
		return
	}

	log.Printf("transcript: %d words, sending to Ollama...", wordCount)

	// ── 5. Step 2: Summarise via Ollama ───────────────────────────────────
	// From Ollama's perspective this is a standard Chat call — the audio
	// pipeline is invisible. The transcript is the user message; the model
	// has no knowledge of where it came from.
	raw, err := ai.Chat(ai.DefaultModel, []ai.Message{
		{Role: "system", Content: summarySystemPrompt()},
		{Role: "user", Content: "Meeting transcript:\n\n" + transcript},
	})
	if err != nil {
		log.Printf("ollama error: %v", err)
		renderFragment(w, summaryResult{
			Error: "AI summarisation failed: " + err.Error(),
		})
		return
	}

	// ── 6. Parse sections and render ──────────────────────────────────────
	overview, decisions, actions, nextSteps := parseSections(raw)
	elapsed := time.Since(start).Seconds()

	renderFragment(w, summaryResult{
		FileName:     header.Filename,
		WordCount:    wordCount,
		ElapsedSec:   elapsed,
		Transcript:   transcript,
		Overview:     mdToHTML(overview),
		Decisions:    mdToHTML(decisions),
		ActionItems:  mdToHTML(actions),
		NextSteps:    mdToHTML(nextSteps),
		FullMarkdown: strings.TrimSpace(raw),
	})
}

// Export returns the full markdown summary as a downloadable file.
// Identical pattern to Project 07: the markdown was assembled during
// Summarize and embedded in the result fragment as a hidden form field.
// No AI calls happen here.
func Export(w http.ResponseWriter, r *http.Request) {
	markdown := r.FormValue("markdown")
	filename := strings.TrimSpace(r.FormValue("filename"))
	if filename == "" {
		filename = "meeting-notes.md"
	}
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	fmt.Fprint(w, markdown)
}

// ── Prompt ────────────────────────────────────────────────────────────────────

// summarySystemPrompt defines the four-section structure the LLM must produce.
//
// The four sections are chosen because they represent what people most
// commonly want to extract from a meeting recording:
//
//	Meeting Summary  — what was the meeting about? (for people who did not attend)
//	Decisions Made   — what was agreed upon? (accountable, revisitable)
//	Action Items     — who needs to do what? (the most immediately useful output)
//	Next Steps       — what comes after this meeting? (continuity)
//
// The "None recorded" fallback is essential: without it, the model invents
// decisions or action items when none were mentioned. Making the empty case
// explicit teaches the model to distinguish absence from presence.
//
// Action items use the GitHub task list syntax (- [ ] ) so goldmark renders
// them as interactive checkboxes in the HTML output.
func summarySystemPrompt() string {
	return `You are an expert meeting facilitator and note-taker.

You will receive a transcript from a meeting. Analyse it carefully and produce a structured summary.

Use this exact markdown structure and nothing else:

## Meeting Summary
Two to three sentences summarising what was discussed and the overall outcome. Be specific and concise.

## Decisions Made
List every concrete decision that was reached during the meeting. Use a bullet list.
If no clear decisions were made, write: None recorded.

## Action Items
List every task, commitment, or follow-up mentioned, with the responsible person if named.
Use GitHub task list syntax so each item renders as a checkbox:
- [ ] @PersonName: What they committed to do
- [ ] Action: Task description (if no specific person was named)
If no action items were mentioned, write: None recorded.

## Next Steps
List any upcoming meetings, deadlines, planned follow-ups, or milestones mentioned.
If none were mentioned, write: None recorded.

Base this entirely on what was said in the transcript. Do not invent content that was not mentioned.`
}

// ── Section parser ────────────────────────────────────────────────────────────

// parseSections splits the LLM's structured response into its four named sections.
//
// The approach is identical to Project 03's parseSections: scan line by line,
// use case-insensitive partial matching to identify ## headings, and accumulate
// lines under each heading into the corresponding section string.
// Fallback: if no headings are found, the entire response goes into Overview.
func parseSections(raw string) (overview, decisions, actions, nextSteps string) {
	type section int
	const (
		sNone section = iota
		sOverview
		sDecisions
		sActions
		sNextSteps
	)

	current := sNone
	var buffers [5]strings.Builder

	for _, line := range strings.Split(raw, "\n") {
		upper := strings.ToUpper(strings.TrimSpace(line))

		switch {
		case strings.Contains(upper, "MEETING SUMMARY") || strings.Contains(upper, "OVERVIEW"):
			current = sOverview
			continue
		case strings.Contains(upper, "DECISION"):
			current = sDecisions
			continue
		case strings.Contains(upper, "ACTION"):
			current = sActions
			continue
		case strings.Contains(upper, "NEXT STEP"):
			current = sNextSteps
			continue
		}

		if current != sNone {
			buffers[current].WriteString(line + "\n")
		}
	}

	overview = strings.TrimSpace(buffers[sOverview].String())
	decisions = strings.TrimSpace(buffers[sDecisions].String())
	actions = strings.TrimSpace(buffers[sActions].String())
	nextSteps = strings.TrimSpace(buffers[sNextSteps].String())

	// Fallback: if no headings were matched, show the full response in Overview
	if overview == "" && decisions == "" && actions == "" && nextSteps == "" {
		overview = strings.TrimSpace(raw)
	}

	return
}

// ── Shared helpers ────────────────────────────────────────────────────────────

func renderFragment(w http.ResponseWriter, data summaryResult) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "result", data); err != nil {
		log.Printf("render fragment: %v", err)
	}
}

// mdToHTML converts markdown to template.HTML using goldmark.
// goldmark's TaskList extension is NOT added here because the standard
// goldmark.New() already handles - [ ] syntax via the default parser
// when the source contains it. The rendered checkboxes are styled in CSS.
func mdToHTML(md string) template.HTML {
	if strings.TrimSpace(md) == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := goldmark.New().Convert([]byte(md), &buf); err != nil {
		return template.HTML("<p>" + template.HTMLEscapeString(md) + "</p>")
	}
	return template.HTML(buf.String())
}

func formatFileSize(size int64) string {
	const mb = 1 << 20
	if size >= mb {
		return fmt.Sprintf("%.1f MB", float64(size)/float64(mb))
	}
	return fmt.Sprintf("%d KB", size/1024)
}
