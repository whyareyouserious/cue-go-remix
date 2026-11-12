package model

// Event is the single frontend event envelope, emitted as cue:event.
type Event struct {
	Type    string `json:"type"`
	ID      string `json:"id,omitempty"`
	Text    string `json:"text,omitempty"`
	Channel string `json:"channel,omitempty"`
	Data    any    `json:"data,omitempty"`
}
