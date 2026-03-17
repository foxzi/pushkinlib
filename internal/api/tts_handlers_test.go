package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockTTSServer creates an httptest server that simulates the TTS backend.
func mockTTSServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"model_id": "v5_ru",
				"speakers": []string{"aidar", "baya", "kseniya", "xenia", "eugene"},
			},
		})
	})

	mux.HandleFunc("/v1/audio/speech", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		input, _ := req["input"].(string)
		if input == "" {
			http.Error(w, `{"error":"input is required"}`, http.StatusBadRequest)
			return
		}

		// Return fake audio data
		w.Header().Set("Content-Type", "audio/ogg")
		w.Header().Set("Content-Length", "11")
		w.Write([]byte("fake-audio!"))
	})

	return httptest.NewServer(mux)
}

// --- GetTTSStatus tests ---

func TestGetTTSStatus_NotConfigured(t *testing.T) {
	h := setupTestHandlers(t)
	// tts is empty TTSConfig by default

	req := httptest.NewRequest("GET", "/api/v1/tts/status", nil)
	w := httptest.NewRecorder()
	h.GetTTSStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if result["available"] != false {
		t.Errorf("expected available=false, got %v", result["available"])
	}
	if result["reason"] != "TTS server not configured" {
		t.Errorf("unexpected reason: %v", result["reason"])
	}
}

func TestGetTTSStatus_ServerAvailable(t *testing.T) {
	ts := mockTTSServer(t)
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "")

	req := httptest.NewRequest("GET", "/api/v1/tts/status", nil)
	w := httptest.NewRecorder()
	h.GetTTSStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if result["available"] != true {
		t.Errorf("expected available=true, got %v", result["available"])
	}
	if result["server_url"] != ts.URL {
		t.Errorf("expected server_url=%s, got %v", ts.URL, result["server_url"])
	}
}

func TestGetTTSStatus_ServerUnreachable(t *testing.T) {
	h := setupTestHandlers(t)
	h.SetTTSConfig("http://127.0.0.1:1", "") // port 1 — unreachable

	req := httptest.NewRequest("GET", "/api/v1/tts/status", nil)
	w := httptest.NewRecorder()
	h.GetTTSStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if result["available"] != false {
		t.Errorf("expected available=false, got %v", result["available"])
	}
	reason, _ := result["reason"].(string)
	if reason == "" {
		t.Error("expected non-empty reason for unreachable server")
	}
}

// --- GetTTSVoices tests ---

func TestGetTTSVoices_NotConfigured(t *testing.T) {
	h := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/tts/voices", nil)
	w := httptest.NewRecorder()
	h.GetTTSVoices(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestGetTTSVoices_Success(t *testing.T) {
	ts := mockTTSServer(t)
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "")

	req := httptest.NewRequest("GET", "/api/v1/tts/voices", nil)
	w := httptest.NewRecorder()
	h.GetTTSVoices(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var models []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &models); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0]["model_id"] != "v5_ru" {
		t.Errorf("expected model_id=v5_ru, got %v", models[0]["model_id"])
	}
	speakers, ok := models[0]["speakers"].([]interface{})
	if !ok {
		t.Fatalf("speakers is not an array: %T", models[0]["speakers"])
	}
	if len(speakers) != 5 {
		t.Errorf("expected 5 speakers, got %d", len(speakers))
	}
}

func TestGetTTSVoices_WithAPIKey(t *testing.T) {
	var receivedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "test-secret-key")

	req := httptest.NewRequest("GET", "/api/v1/tts/voices", nil)
	w := httptest.NewRecorder()
	h.GetTTSVoices(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if receivedAuth != "Bearer test-secret-key" {
		t.Errorf("expected Authorization 'Bearer test-secret-key', got '%s'", receivedAuth)
	}
}

func TestGetTTSVoices_ServerUnreachable(t *testing.T) {
	h := setupTestHandlers(t)
	h.SetTTSConfig("http://127.0.0.1:1", "")

	req := httptest.NewRequest("GET", "/api/v1/tts/voices", nil)
	w := httptest.NewRecorder()
	h.GetTTSVoices(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestGetTTSVoices_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "")

	req := httptest.NewRequest("GET", "/api/v1/tts/voices", nil)
	w := httptest.NewRecorder()
	h.GetTTSVoices(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- SynthesizeSpeech tests ---

func TestSynthesizeSpeech_NotConfigured(t *testing.T) {
	h := setupTestHandlers(t)

	body := bytes.NewBufferString(`{"input":"Привет мир"}`)
	req := httptest.NewRequest("POST", "/api/v1/tts/speech", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SynthesizeSpeech(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestSynthesizeSpeech_Success(t *testing.T) {
	ts := mockTTSServer(t)
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "")

	body := bytes.NewBufferString(`{"input":"Привет мир","voice":"xenia","speed":1.0}`)
	req := httptest.NewRequest("POST", "/api/v1/tts/speech", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SynthesizeSpeech(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if ct := w.Header().Get("Content-Type"); ct != "audio/ogg" {
		t.Errorf("expected Content-Type audio/ogg, got %s", ct)
	}

	if w.Header().Get("Content-Length") != "11" {
		t.Errorf("expected Content-Length 11, got %s", w.Header().Get("Content-Length"))
	}

	if w.Body.String() != "fake-audio!" {
		t.Errorf("expected 'fake-audio!', got '%s'", w.Body.String())
	}
}

func TestSynthesizeSpeech_Defaults(t *testing.T) {
	var receivedReq map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "audio/ogg")
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "")

	// Send minimal request — only input
	body := bytes.NewBufferString(`{"input":"test"}`)
	req := httptest.NewRequest("POST", "/api/v1/tts/speech", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SynthesizeSpeech(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify defaults were set
	if receivedReq["model"] != "v5_ru" {
		t.Errorf("default model: expected v5_ru, got %v", receivedReq["model"])
	}
	if receivedReq["voice"] != "xenia" {
		t.Errorf("default voice: expected xenia, got %v", receivedReq["voice"])
	}
	if receivedReq["response_format"] != "opus" {
		t.Errorf("default format: expected opus, got %v", receivedReq["response_format"])
	}
	if receivedReq["speed"].(float64) != 1.0 {
		t.Errorf("default speed: expected 1.0, got %v", receivedReq["speed"])
	}
}

func TestSynthesizeSpeech_InvalidBody(t *testing.T) {
	ts := mockTTSServer(t)
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "")

	body := bytes.NewBufferString(`not json`)
	req := httptest.NewRequest("POST", "/api/v1/tts/speech", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SynthesizeSpeech(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSynthesizeSpeech_EmptyInput(t *testing.T) {
	ts := mockTTSServer(t)
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "")

	body := bytes.NewBufferString(`{"input":""}`)
	req := httptest.NewRequest("POST", "/api/v1/tts/speech", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SynthesizeSpeech(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSynthesizeSpeech_WhitespaceOnlyInput(t *testing.T) {
	ts := mockTTSServer(t)
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "")

	body := bytes.NewBufferString(`{"input":"   \n\t  "}`)
	req := httptest.NewRequest("POST", "/api/v1/tts/speech", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SynthesizeSpeech(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for whitespace-only input, got %d", w.Code)
	}
}

func TestSynthesizeSpeech_WithAPIKey(t *testing.T) {
	var receivedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "audio/ogg")
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "my-secret")

	body := bytes.NewBufferString(`{"input":"test"}`)
	req := httptest.NewRequest("POST", "/api/v1/tts/speech", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SynthesizeSpeech(w, req)

	if receivedAuth != "Bearer my-secret" {
		t.Errorf("expected Authorization 'Bearer my-secret', got '%s'", receivedAuth)
	}
}

func TestSynthesizeSpeech_ServerUnreachable(t *testing.T) {
	h := setupTestHandlers(t)
	h.SetTTSConfig("http://127.0.0.1:1", "")

	body := bytes.NewBufferString(`{"input":"test"}`)
	req := httptest.NewRequest("POST", "/api/v1/tts/speech", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SynthesizeSpeech(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestSynthesizeSpeech_TTSBadRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid voice"}`, http.StatusBadRequest)
	}))
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "")

	body := bytes.NewBufferString(`{"input":"test","voice":"nonexistent"}`)
	req := httptest.NewRequest("POST", "/api/v1/tts/speech", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SynthesizeSpeech(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSynthesizeSpeech_TTSRateLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limit", http.StatusTooManyRequests)
	}))
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "")

	body := bytes.NewBufferString(`{"input":"test"}`)
	req := httptest.NewRequest("POST", "/api/v1/tts/speech", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SynthesizeSpeech(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestSynthesizeSpeech_TTSInternalError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "")

	body := bytes.NewBufferString(`{"input":"test"}`)
	req := httptest.NewRequest("POST", "/api/v1/tts/speech", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SynthesizeSpeech(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502 for unknown TTS error, got %d", w.Code)
	}
}

func TestSynthesizeSpeech_StreamsFullResponse(t *testing.T) {
	// Simulate larger audio payload
	audioData := make([]byte, 64*1024) // 64KB
	for i := range audioData {
		audioData[i] = byte(i % 256)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain the request body
		io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write(audioData)
	}))
	defer ts.Close()

	h := setupTestHandlers(t)
	h.SetTTSConfig(ts.URL, "")

	body := bytes.NewBufferString(`{"input":"long text here","response_format":"mp3"}`)
	req := httptest.NewRequest("POST", "/api/v1/tts/speech", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SynthesizeSpeech(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "audio/mpeg" {
		t.Errorf("expected Content-Type audio/mpeg, got %s", ct)
	}

	if w.Body.Len() != len(audioData) {
		t.Errorf("expected %d bytes, got %d", len(audioData), w.Body.Len())
	}
}

// --- TTSConfig tests ---

func TestTTSConfig_Enabled(t *testing.T) {
	cfg := &TTSConfig{ServerURL: "http://localhost:8000"}
	if !cfg.TTSEnabled() {
		t.Error("expected TTSEnabled=true when ServerURL is set")
	}
}

func TestTTSConfig_Disabled(t *testing.T) {
	cfg := &TTSConfig{}
	if cfg.TTSEnabled() {
		t.Error("expected TTSEnabled=false when ServerURL is empty")
	}
}

func TestSetTTSConfig(t *testing.T) {
	h := setupTestHandlers(t)

	if h.tts.TTSEnabled() {
		t.Error("expected TTS disabled by default")
	}

	h.SetTTSConfig("http://localhost:8000", "key123")

	if !h.tts.TTSEnabled() {
		t.Error("expected TTS enabled after SetTTSConfig")
	}
	if h.tts.ServerURL != "http://localhost:8000" {
		t.Errorf("expected ServerURL=http://localhost:8000, got %s", h.tts.ServerURL)
	}
	if h.tts.APIKey != "key123" {
		t.Errorf("expected APIKey=key123, got %s", h.tts.APIKey)
	}
}
