// Package whisper provides a client for local audio transcription using whisper.cpp.
//
// This package calls the whisper.cpp binary as a subprocess. It works with:
//   - macOS Homebrew install: binary is "whisper-cpp"
//   - Linux/Windows built from source: binary is "whisper-cli"
//
// Configuration (via .env or environment variables):
//
//	WHISPER_BIN   — binary name or full path (auto-detected if not set)
//	WHISPER_MODEL — full path to your .bin model file (auto-detected if not set)
package whisper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// binaryName returns the whisper.cpp binary to run.
// Checks WHISPER_BIN env var first, then auto-detects from PATH.
func binaryName() string {
	if b := os.Getenv("WHISPER_BIN"); b != "" {
		return b
	}
	// Auto-detect: try common names in order.
	// whisper-cpp  = Homebrew on macOS
	// whisper-cli  = built from source (cmake)
	for _, candidate := range []string{"whisper-cpp", "whisper-cli"} {
		if path, err := exec.LookPath(candidate); err == nil && path != "" {
			return candidate
		}
	}
	return "whisper-cpp" // fallback — will produce a clear "not found" error
}

// modelPath returns the path to the ggml model file.
// Checks WHISPER_MODEL env var first, then searches common locations.
func modelPath() string {
	if m := os.Getenv("WHISPER_MODEL"); m != "" {
		return m
	}

	// Auto-detect from common locations so students don't have to set
	// WHISPER_MODEL manually if they followed the README download path.
	home, _ := os.UserHomeDir()
	candidates := []string{
		// Standard download location from README
		filepath.Join(home, "whisper-models", "ggml-base.en.bin"),
		// Homebrew on Apple Silicon
		"/opt/homebrew/share/whisper-cpp/models/ggml-base.en.bin",
		// Homebrew on Intel Mac
		"/usr/local/share/whisper-cpp/models/ggml-base.en.bin",
		// Linux local
		filepath.Join(home, ".local", "share", "whisper-models", "ggml-base.en.bin"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	// Return the README default so the error message is helpful
	return filepath.Join(home, "whisper-models", "ggml-base.en.bin")
}

// Transcribe writes audioBytes to a temporary file, runs the whisper.cpp
// binary against it, reads the generated .txt transcript, and returns
// the plain-text result. Temp files are removed on return regardless of
// success or failure.
func Transcribe(audioBytes []byte, filename string) (string, error) {
	bin := binaryName()
	model := modelPath()

	// Verify binary is available before touching the filesystem.
	if _, err := exec.LookPath(bin); err != nil {
		return "", fmt.Errorf(
			"whisper binary %q not found.\n"+
				"  macOS:   brew install whisper-cpp\n"+
				"  Linux:   build from source, add build/bin to PATH\n"+
				"  Or set WHISPER_BIN in your .env file to the full path",
			bin,
		)
	}

	// Verify the model file exists.
	if _, err := os.Stat(model); err != nil {
		return "", fmt.Errorf(
			"whisper model not found at: %s\n\n"+
				"Download it with:\n"+
				"  mkdir -p ~/whisper-models\n"+
				"  curl -L -o ~/whisper-models/ggml-base.en.bin \\\n"+
				"    https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en.bin\n\n"+
				"Or set WHISPER_MODEL in your .env file to the actual path",
			model,
		)
	}

	// Write uploaded audio to a temp file.
	// whisper.cpp requires a file path — it cannot read from stdin.
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".wav"
	}
	tmpAudio, err := os.CreateTemp("", "whisper-audio-*"+ext)
	if err != nil {
		return "", fmt.Errorf("whisper: create temp file: %w", err)
	}
	defer os.Remove(tmpAudio.Name())

	if _, err := tmpAudio.Write(audioBytes); err != nil {
		tmpAudio.Close()
		return "", fmt.Errorf("whisper: write temp file: %w", err)
	}
	tmpAudio.Close()

	// whisper.cpp writes the transcript to <inputfile>.txt when --output-txt is set.
	transcriptPath := tmpAudio.Name() + ".txt"
	defer os.Remove(transcriptPath)

	// Run whisper.cpp.
	cmd := exec.Command(bin,
		"-m", model,
		"-f", tmpAudio.Name(),
		"--output-txt",
		"--language", "en",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// whisper.cpp sometimes exits non-zero even on success (minor warnings).
		// Only treat it as a real error if the transcript file was not produced.
		if _, statErr := os.Stat(transcriptPath); statErr != nil {
			return "", fmt.Errorf("whisper: transcription failed: %w\n%s", err, output)
		}
	}

	content, err := os.ReadFile(transcriptPath)
	if err != nil {
		return "", fmt.Errorf("whisper: read transcript: %w", err)
	}

	transcript := strings.TrimSpace(string(content))
	if transcript == "" {
		return "", fmt.Errorf(
			"whisper: empty transcript — " +
				"the audio may be silent, too noisy, or too short. " +
				"Try a clearer recording or a larger model",
		)
	}

	return transcript, nil
}
