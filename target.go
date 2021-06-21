package hayden

type Target struct {
	URL    string `json:"url"`
	Text   string `json:"text"`
	Invert bool   `json:"invert"`
	Hook   string `json:"hook,omitempty"`
	Period int    `json:"period,omitempty"`
}
