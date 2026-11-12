package model

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
