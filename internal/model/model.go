package model

// Endpoint describes an independently configured chat or transcription service.
type Endpoint struct {
	Provider   string `json:"provider"`
	BaseURL    string `json:"baseUrl"`
	APIKey     string `json:"apiKey"`
	HasKey     bool   `json:"hasKey"`
	ClearKey   bool   `json:"clearKey,omitempty"`
	Model      string `json:"model"`
	SmartModel string `json:"smartModel"`
	Stream     bool   `json:"stream"`
}

type Settings struct {
	Chat              Endpoint `json:"chat"`
	Speech            Endpoint `json:"speech"`
	Smart             bool     `json:"smart"`
	Language          string   `json:"language"`
	AIRules           string   `json:"aiRules"`
	Background        string   `json:"background"`
	MeetingAudio      bool     `json:"meetingAudio"`
	Opacity           float64  `json:"opacity"`
	ContentProtection bool     `json:"contentProtection"`
	SlidesEnabled     bool     `json:"slidesEnabled"`
	SlidesInterval    int      `json:"slidesInterval"`
	WhisperModel      string   `json:"whisperModel"`
	WhisperPath       string   `json:"whisperPath"`
	WhisperThreads    int      `json:"whisperThreads"`
	Onboarded         bool     `json:"onboarded"`
	WindowX           int      `json:"windowX"`
	WindowY           int      `json:"windowY"`
	WindowPositioned  bool     `json:"windowPositioned"`
}

type Turn struct {
	ID      string `json:"id"`
	Channel string `json:"channel"`
	Text    string `json:"text"`
	Time    int64  `json:"time"`
}
type Meeting struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	StartedAt  int64  `json:"startedAt"`
	EndedAt    int64  `json:"endedAt"`
	Turns      []Turn `json:"turns"`
	Notes      string `json:"notes"`
	NotesTurns int    `json:"notesTurns"`
}
type Slide struct {
	ID   string `json:"id"`
	Time int64  `json:"time"`
	Text string `json:"text"`
}
type AskRequest struct {
	Mode string `json:"mode"`
	Text string `json:"text"`
}
type Status struct {
	Platform   string   `json:"platform"`
	Listening  bool     `json:"listening"`
	Busy       bool     `json:"busy"`
	DataDir    string   `json:"dataDir"`
	Transcript []Turn   `json:"transcript"`
	Slides     []Slide  `json:"slides"`
	Warnings   []string `json:"warnings"`
}

// Event is the single frontend event envelope, emitted as cue:event.
type Event struct {
	Type    string `json:"type"`
	ID      string `json:"id,omitempty"`
	Text    string `json:"text,omitempty"`
	Channel string `json:"channel,omitempty"`
	Data    any    `json:"data,omitempty"`
}
type ModelInfo struct {
	ID          string `json:"id"`
	Bytes       int64  `json:"bytes"`
	SHA256      string `json:"sha256"`
	Installed   bool   `json:"installed"`
	Downloading bool   `json:"downloading"`
	Downloaded  int64  `json:"downloaded"`
}
