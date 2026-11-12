package model

type Status struct {
	Platform   string   `json:"platform"`
	Listening  bool     `json:"listening"`
	Busy       bool     `json:"busy"`
	DataDir    string   `json:"dataDir"`
	Transcript []Turn   `json:"transcript"`
	Slides     []Slide  `json:"slides"`
	Warnings   []string `json:"warnings"`
}
