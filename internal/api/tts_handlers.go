package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// TTSConfig holds TTS proxy configuration.
type TTSConfig struct {
	ServerURL string
	APIKey    string
}

// TTSEnabled returns true if TTS server URL is configured.
func (c *TTSConfig) TTSEnabled() bool {
	return c.ServerURL != ""
}

// ttsHTTPClient is a shared HTTP client for TTS proxy requests.
var ttsHTTPClient = &http.Client{
	Timeout: 120 * time.Second, // TTS synthesis can be slow for long texts
}

// ttsRequest is the request body for speech synthesis.
type ttsRequest struct {
	Input          string  `json:"input"`
	Voice          string  `json:"voice,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
	Model          string  `json:"model,omitempty"`
	ResponseFormat string  `json:"response_format,omitempty"`
}

// GetTTSStatus checks whether the TTS server is available.
// GET /api/v1/tts/status
func (h *Handlers) GetTTSStatus(w http.ResponseWriter, r *http.Request) {
	if !h.tts.TTSEnabled() {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"available": false,
			"reason":    "TTS server not configured",
		})
		return
	}

	// Probe TTS health endpoint
	resp, err := ttsHTTPClient.Get(h.tts.ServerURL + "/health")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"available": false,
			"reason":    fmt.Sprintf("TTS server unreachable: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	available := resp.StatusCode == http.StatusOK

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"available":  available,
		"server_url": h.tts.ServerURL,
	})
}

// GetTTSVoices returns available voices from the TTS server.
// GET /api/v1/tts/voices
func (h *Handlers) GetTTSVoices(w http.ResponseWriter, r *http.Request) {
	if !h.tts.TTSEnabled() {
		http.Error(w, "TTS server not configured", http.StatusServiceUnavailable)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), "GET", h.tts.ServerURL+"/v1/models", nil)
	if err != nil {
		log.Printf("GetTTSVoices: failed to create request: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if h.tts.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.tts.APIKey)
	}

	resp, err := ttsHTTPClient.Do(req)
	if err != nil {
		log.Printf("GetTTSVoices: TTS server error: %v", err)
		http.Error(w, "TTS server unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	if err != nil {
		log.Printf("GetTTSVoices: failed to read response: %v", err)
		http.Error(w, "Failed to read TTS response", http.StatusBadGateway)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("GetTTSVoices: TTS returned %d: %s", resp.StatusCode, string(body))
		http.Error(w, "TTS server error", resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// SynthesizeSpeech proxies a TTS synthesis request to the TTS server.
// POST /api/v1/tts/speech
func (h *Handlers) SynthesizeSpeech(w http.ResponseWriter, r *http.Request) {
	if !h.tts.TTSEnabled() {
		http.Error(w, "TTS server not configured", http.StatusServiceUnavailable)
		return
	}

	var ttsReq ttsRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&ttsReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(ttsReq.Input) == "" {
		http.Error(w, "Input text is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if ttsReq.Model == "" {
		ttsReq.Model = "v5_ru"
	}
	if ttsReq.Voice == "" {
		ttsReq.Voice = "xenia"
	}
	if ttsReq.ResponseFormat == "" {
		ttsReq.ResponseFormat = "opus"
	}
	if ttsReq.Speed == 0 {
		ttsReq.Speed = 1.0
	}

	// Build request to TTS server (OpenAI-compatible API)
	payload, err := json.Marshal(map[string]interface{}{
		"model":           ttsReq.Model,
		"input":           ttsReq.Input,
		"voice":           ttsReq.Voice,
		"speed":           ttsReq.Speed,
		"response_format": ttsReq.ResponseFormat,
	})
	if err != nil {
		log.Printf("SynthesizeSpeech: failed to marshal request: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), "POST", h.tts.ServerURL+"/v1/audio/speech", bytes.NewReader(payload))
	if err != nil {
		log.Printf("SynthesizeSpeech: failed to create request: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if h.tts.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.tts.APIKey)
	}

	resp, err := ttsHTTPClient.Do(req)
	if err != nil {
		log.Printf("SynthesizeSpeech: TTS server error: %v", err)
		http.Error(w, "TTS server unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		log.Printf("SynthesizeSpeech: TTS returned %d: %s", resp.StatusCode, string(body))

		// Forward specific error codes from TTS
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			http.Error(w, "TTS rate limit exceeded", http.StatusTooManyRequests)
		case http.StatusBadRequest:
			http.Error(w, "Invalid TTS request: "+string(body), http.StatusBadRequest)
		default:
			http.Error(w, "TTS synthesis failed", http.StatusBadGateway)
		}
		return
	}

	// Stream audio response back to client
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "audio/ogg"
	}
	w.Header().Set("Content-Type", contentType)

	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}

	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("SynthesizeSpeech: failed to stream response: %v", err)
	}
}
