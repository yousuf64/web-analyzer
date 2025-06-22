package types

type AnalyzeRequest struct {
	Url string `json:"url"`
}

type AnalyzeResult struct {
	HtmlVersion  string         `json:"html_version"`
	PageTitle    string         `json:"page_title"`
	Headings     map[string]int `json:"headings"`
	Links        []string       `json:"links"`
	HasLoginForm bool           `json:"has_login_form"`
}
