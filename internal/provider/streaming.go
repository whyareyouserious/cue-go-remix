package provider

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"cue-go-remix/internal/model"
	"github.com/coder/websocket"
)

// SpeechStream owns one microphone OR system-audio channel. Send accepts 16 kHz
// mono PCM. Close submits the remaining audio, waits for final transcripts, and
// returns only after all callbacks have stopped. It must not be called from a
// callback: callbacks execute on the stream's receive goroutine.
type SpeechStream struct {
	conn      *websocket.Conn
	ep        model.Endpoint
	ctx       context.Context
	cancel    context.CancelFunc
	onText    func(string, bool)
	onError   func(error)
	writeMu   sync.Mutex
	mu        sync.Mutex
	closing   bool
	sent      bool
	sealed    bool
	pending   int
	closeOnce sync.Once
	drainOnce sync.Once
	drained   chan struct{}
	done      chan struct{}
	closed    chan struct{}
	closeErr  error
	readErr   error
	interim   string
	committed string
}

// OpenStream starts the selected service's native streaming transcription
// protocol. Custom compatible services use Transcribe unless explicitly exposed
// using one of these provider protocols and a custom Base URL.
func OpenStream(ctx context.Context, ep model.Endpoint, language string, onText func(string, bool), onError func(error)) (*SpeechStream, error) {
	ep.Provider = normalizeProvider(ep.Provider)
	if ep.APIKey == "" {
		return nil, errors.New("请填写流式语音服务的 API Key")
	}
	var address string
	var err error
	var setup any
	switch ep.Provider {
	case "openai":
		address, err = endpointURL(ep, "/realtime")
		if ep.Model == "" {
			ep.Model = "gpt-4o-mini-transcribe"
		}
		transcription := map[string]any{"model": ep.Model}
		if language != "" && language != "auto" {
			transcription["language"] = language
		}
		setup = map[string]any{"type": "session.update", "session": map[string]any{"type": "transcription", "audio": map[string]any{"input": map[string]any{"format": map[string]any{"type": "audio/pcm", "rate": 24000}, "transcription": transcription, "turn_detection": map[string]any{"type": "server_vad", "threshold": 0.5, "prefix_padding_ms": 300, "silence_duration_ms": 600}}}}}
	case "deepgram":
		address, err = endpointURL(ep, "/listen")
		if ep.Model == "" {
			ep.Model = "nova-3"
		}
	case "gemini":
		u, e := baseURL(ep)
		err = e
		if err == nil {
			u.Path = "/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent"
			u.RawQuery = ""
			address = u.String()
		}
		if ep.Model == "" {
			ep.Model = "gemini-3.5-transcribe-live"
		}
		setup = map[string]any{"setup": map[string]any{"model": "models/" + strings.TrimPrefix(ep.Model, "models/"), "generationConfig": map[string]any{"responseModalities": []string{"TEXT"}}, "inputAudioTranscription": map[string]any{}}}
	default:
		return nil, errors.New("当前服务不支持实时流式转写，请使用分段转写")
	}
	if err != nil {
		return nil, err
	}
	u, err := baseURL(model.Endpoint{BaseURL: address})
	if err != nil {
		return nil, err
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	q := u.Query()
	switch ep.Provider {
	case "openai":
		q.Set("intent", "transcription")
	case "deepgram":
		q.Set("model", ep.Model)
		q.Set("encoding", "linear16")
		q.Set("sample_rate", "16000")
		q.Set("channels", "1")
		q.Set("interim_results", "true")
		q.Set("smart_format", "true")
		q.Set("endpointing", "600")
		q.Set("utterance_end_ms", "1000")
		if language != "" && language != "auto" {
			q.Set("language", language)
		} else {
			q.Set("language", "multi")
		}
	}
	u.RawQuery = q.Encode()
	header := http.Header{}
	fake := &http.Request{Header: header}
	authHeaders(fake, ep)
	dialCtx, dialCancel := context.WithTimeout(ctx, 15*time.Second)
	defer dialCancel()
	conn, resp, err := websocket.Dial(dialCtx, u.String(), &websocket.DialOptions{HTTPHeader: header, HTTPClient: httpClient})
	if err != nil {
		if resp != nil {
			return nil, serviceError(resp.StatusCode, nil, ep.APIKey)
		}
		return nil, transportError(dialCtx, err, ep.APIKey)
	}
	conn.SetReadLimit(2 << 20)
	streamCtx, cancel := context.WithCancel(ctx)
	s := &SpeechStream{conn: conn, ep: ep, ctx: streamCtx, cancel: cancel, onText: onText, onError: onError, drained: make(chan struct{}), done: make(chan struct{}), closed: make(chan struct{})}
	if setup != nil {
		data, _ := json.Marshal(setup)
		if err = conn.Write(dialCtx, websocket.MessageText, data); err == nil {
			for {
				_, data, e := conn.Read(dialCtx)
				if e != nil {
					err = e
					break
				}
				var v map[string]any
				if e = json.Unmarshal(data, &v); e != nil {
					err = errors.New("语音服务握手返回了无效 JSON")
					break
				}
				if v["error"] != nil {
					err = serviceError(0, data, ep.APIKey)
					break
				}
				// session.created precedes configuration; audio must wait until updated.
				if ep.Provider == "openai" && stringValue(v["type"]) == "session.updated" {
					break
				}
				if ep.Provider == "gemini" && v["setupComplete"] != nil {
					break
				}
			}
		}
		if err != nil {
			cancel()
			_ = conn.CloseNow()
			return nil, transportError(dialCtx, err, ep.APIKey)
		}
	}
	go s.receive()
	return s, nil
}

// Send serializes writes to preserve the order of PCM blocks.
func (s *SpeechStream) Send(pcm []int16) error {
	if len(pcm) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	closing := s.closing
	if !closing {
		s.sent = true
	}
	s.mu.Unlock()
	if closing {
		return errors.New("语音会话已停止")
	}
	data := pcmBytes(pcm)
	switch s.ep.Provider {
	case "deepgram":
		return s.write(websocket.MessageBinary, data)
	case "openai":
		return s.writeJSON(map[string]any{"type": "input_audio_buffer.append", "audio": base64.StdEncoding.EncodeToString(resample16To24(pcm))})
	case "gemini":
		return s.writeJSON(map[string]any{"realtimeInput": map[string]any{"audio": map[string]any{"data": base64.StdEncoding.EncodeToString(data), "mimeType": "audio/pcm;rate=16000"}}})
	}
	return nil
}

func (s *SpeechStream) write(kind websocket.MessageType, data []byte) error {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	if err := s.conn.Write(ctx, kind, data); err != nil {
		return transportError(ctx, err, s.ep.APIKey)
	}
	return nil
}
func (s *SpeechStream) writeJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.write(websocket.MessageText, data)
}

func (s *SpeechStream) receive() {
	defer close(s.done)
	if s.ep.Provider == "deepgram" {
		defer s.flushCommitted()
	}
	keepAliveDone := make(chan struct{})
	if s.ep.Provider == "deepgram" {
		go func() {
			defer close(keepAliveDone)
			tick := time.NewTicker(3 * time.Second)
			defer tick.Stop()
			for {
				select {
				case <-s.ctx.Done():
					return
				case <-tick.C:
					s.writeMu.Lock()
					s.mu.Lock()
					closing := s.closing
					s.mu.Unlock()
					if !closing {
						_ = s.writeJSON(map[string]any{"type": "KeepAlive"})
					}
					s.writeMu.Unlock()
				}
			}
		}()
	} else {
		close(keepAliveDone)
	}
	defer func() { s.cancel(); <-keepAliveDone }()
	for {
		_, data, err := s.conn.Read(s.ctx)
		if err != nil {
			s.mu.Lock()
			closing := s.closing
			if !closing {
				s.readErr = transportError(s.ctx, err, s.ep.APIKey)
			} else if s.ctx.Err() == nil {
				select {
				case <-s.drained:
				default:
					s.readErr = errors.New("实时连接在最终转写完成前关闭，尾句可能不完整")
				}
			}
			s.mu.Unlock()
			if !closing && s.ctx.Err() == nil && s.onError != nil {
				s.onError(fmt.Errorf("实时转写连接已中断，请重新开始会话：%s", sanitize(err.Error(), s.ep.APIKey)))
			}
			return
		}
		var v map[string]any
		if err = json.Unmarshal(data, &v); err != nil {
			continue
		}
		if err = s.handle(v, data); err != nil {
			s.mu.Lock()
			closing := s.closing
			s.readErr = err
			s.mu.Unlock()
			if !closing && s.onError != nil {
				s.onError(err)
			}
			return
		}
	}
}

func (s *SpeechStream) emit(text string, final bool) {
	if s.onText != nil {
		s.onText(text, final)
	}
}
func (s *SpeechStream) drain() { s.drainOnce.Do(func() { close(s.drained) }) }
func (s *SpeechStream) handle(v map[string]any, raw []byte) error {
	if v["error"] != nil || stringValue(v["type"]) == "Error" {
		code := stringValue(object(v["error"])["code"])
		s.mu.Lock()
		closing := s.closing
		if closing && code == "input_audio_buffer_commit_empty" {
			s.sealed = true
			if s.pending == 0 {
				s.drain()
			}
			s.mu.Unlock()
			return nil
		}
		s.mu.Unlock()
		return serviceError(0, raw, s.ep.APIKey)
	}
	switch s.ep.Provider {
	case "openai":
		switch stringValue(v["type"]) {
		case "input_audio_buffer.committed":
			s.mu.Lock()
			s.pending++
			if s.closing {
				s.sealed = true
			}
			s.mu.Unlock()
		case "conversation.item.input_audio_transcription.delta":
			s.interim += stringValue(v["delta"])
			s.emit(s.interim, false)
		case "conversation.item.input_audio_transcription.completed":
			text := strings.TrimSpace(stringValue(v["transcript"]))
			s.interim = ""
			if text != "" {
				s.emit(text, true)
			}
			s.emit("", false)
			s.mu.Lock()
			if s.pending > 0 {
				s.pending--
			}
			if s.closing && s.sealed && s.pending == 0 {
				s.drain()
			}
			s.mu.Unlock()
		case "conversation.item.input_audio_transcription.failed":
			return serviceError(0, raw, s.ep.APIKey)
		}
	case "deepgram":
		switch stringValue(v["type"]) {
		case "Results":
			alts := list(object(v["channel"])["alternatives"])
			if len(alts) == 0 {
				return nil
			}
			text := strings.TrimSpace(stringValue(object(alts[0])["transcript"]))
			if boolValue(v["is_final"]) {
				s.committed = strings.TrimSpace(s.committed + " " + text)
				if boolValue(v["speech_final"]) {
					s.flushCommitted()
				} else {
					s.emit(s.committed, false)
				}
			} else {
				s.emit(strings.TrimSpace(s.committed+" "+text), false)
			}
		case "UtteranceEnd":
			s.flushCommitted()
		case "Metadata":
			s.flushCommitted()
			s.mu.Lock()
			closing := s.closing
			s.mu.Unlock()
			if closing {
				s.drain()
			}
		}
	case "gemini":
		content := object(v["serverContent"])
		if part := object(content["interimInputTranscription"]); part != nil {
			s.interim = stringValue(part["text"])
			s.emit(s.interim, false)
		}
		if part := object(content["inputTranscription"]); part != nil {
			text := strings.TrimSpace(stringValue(part["text"]))
			s.interim = ""
			if text != "" {
				s.emit(text, true)
			}
			s.emit("", false)
			s.mu.Lock()
			closing := s.closing
			s.mu.Unlock()
			if closing {
				s.drain()
			}
		}
		if boolValue(content["turnComplete"]) {
			s.mu.Lock()
			closing := s.closing
			s.mu.Unlock()
			if closing {
				s.drain()
			}
		}
	}
	return nil
}

func (s *SpeechStream) flushCommitted() {
	text := strings.TrimSpace(s.committed)
	s.committed = ""
	if text != "" {
		s.emit(text, true)
	}
	s.emit("", false)
}

func (s *SpeechStream) Close() error {
	s.closeOnce.Do(func() {
		defer close(s.closed)
		s.writeMu.Lock()
		s.mu.Lock()
		s.closing = true
		sent := s.sent
		s.mu.Unlock()
		if sent {
			switch s.ep.Provider {
			case "openai":
				s.closeErr = s.writeJSON(map[string]any{"type": "input_audio_buffer.commit"})
			case "deepgram":
				s.closeErr = s.writeJSON(map[string]any{"type": "CloseStream"})
			case "gemini":
				s.closeErr = s.writeJSON(map[string]any{"realtimeInput": map[string]any{"audioStreamEnd": true}})
			}
		}
		s.writeMu.Unlock()
		if sent && s.closeErr == nil {
			timer := time.NewTimer(8 * time.Second)
			select {
			case <-s.drained:
			case <-s.done:
			case <-s.ctx.Done():
			case <-timer.C:
				s.closeErr = errors.New("等待最终转写超时，最后一段语音可能尚未完成")
			}
			timer.Stop()
		}
		s.cancel()
		_ = s.conn.CloseNow()
		<-s.done
		s.mu.Lock()
		if s.closeErr == nil {
			s.closeErr = s.readErr
		}
		s.mu.Unlock()
	})
	<-s.closed
	return s.closeErr
}

func pcmBytes(pcm []int16) []byte {
	data := make([]byte, len(pcm)*2)
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(v))
	}
	return data
}
func resample16To24(pcm []int16) []byte {
	if len(pcm) == 0 {
		return nil
	}
	n := len(pcm) * 3 / 2
	data := make([]byte, n*2)
	for i := 0; i < n; i++ {
		position := float64(i) * 2 / 3
		left := int(position)
		right := left + 1
		if right >= len(pcm) {
			right = len(pcm) - 1
		}
		sample := float64(pcm[left]) + (float64(pcm[right])-float64(pcm[left]))*(position-float64(left))
		binary.LittleEndian.PutUint16(data[i*2:], uint16(int16(sample)))
	}
	return data
}
