// Package whisper provides a client for the whisper.cpp local transcription server.
//
// whisper.cpp can run in server mode and expose an HTTP endpoint that accepts
// audio files and returns plain text transcriptions. This package wraps that
// HTTP call into a single function: Transcribe.
//
// Why a separate package instead of calling HTTP directly in the handler?
// The same reason ai/ exists: keeping the transport and model concerns out of
// handler code. If you swapped whisper.cpp for faster-whisper, Groq's Whisper
// API, or any other transcription service, only this file changes.
//
// Setup (run this before starting the Go server):
//
//	# Clone and build whisper.cpp
//	git clone https://github.com/ggerganov/whisper.cpp
//	cd whisper.cpp && make
//
//	# Download the base model (~150 MB — good balance of speed and accuracy)
//	bash models/download-ggml-model.sh base.en
//
//	# Start the server on port 8888
//	./server -m models/ggml-base.en.bin --port 8888
package whisper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

// serverURL is the whisper.cpp server address.
// Override with the WHISPER_URL environment variable.
func serverURL() string {
	if u := os.Getenv("WHISPER_URL"); u != "" {
		return u
	}
	return "http://localhost:8888"
}

// transcriptionResponse maps the JSON returned by whisper.cpp /inference.
// The server returns { "text": "transcribed content..." } when
// response_format is set to "json".
type transcriptionResponse struct {
	Text string `json:"text"`
}

// Transcribe sends audioBytes to the whisper.cpp server and returns the
// plain-text transcript.
//
// filename is used as the multipart field filename — whisper.cpp uses the
// extension to detect the audio format, so the name matters. Pass the
// original uploaded filename rather than a generic placeholder.
//
// The HTTP client timeout is set to 10 minutes. A 30-minute meeting
// recording with the base model typically transcribes in 2-4 minutes,
// but giving generous headroom avoids spurious timeouts on slower machines.
func Transcribe(audioBytes []byte, filename string) (string, error) {
	url := serverURL() + "/inference"

	// Build the multipart form body.
	// The whisper.cpp server expects the audio under the "file" field,
	// and "response_format" set to "json" for machine-readable output.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("whisper: create form file: %w", err)
	}
	if _, err := fw.Write(audioBytes); err != nil {
		return "", fmt.Errorf("whisper: write audio bytes: %w", err)
	}

	if err := w.WriteField("response_format", "json"); err != nil {
		return "", fmt.Errorf("whisper: write response_format field: %w", err)
	}
	w.Close()

	// Use a custom HTTP client with a long timeout.
	// http.DefaultClient has no timeout, which is dangerous for production,
	// but a 10-minute cap here prevents indefinite hangs on very large files.
	client := &http.Client{Timeout: 10 * time.Minute}

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return "", fmt.Errorf("whisper: build request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf(
			"whisper: server unreachable at %s — is whisper.cpp running? "+
				"Run: ./server -m models/ggml-base.en.bin --port 8888 | %w",
			serverURL(), err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("whisper: server returned status %d", resp.StatusCode)
	}

	var result transcriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("whisper: decode response: %w", err)
	}

	transcript := strings.TrimSpace(result.Text)
	if transcript == "" {
		return "", fmt.Errorf("whisper: empty transcript — the audio may be silent or the model could not detect speech")
	}

	return transcript, nil
}
