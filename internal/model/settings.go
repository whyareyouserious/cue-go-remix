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
