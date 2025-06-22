package main

import (
	"log"
	"shared/types"
	"strings"

	"golang.org/x/net/html"
)

type Analyzer struct {
	htmlVersion  string
	title        string
	headings     map[string]int
	links        []string
	hasLoginForm bool
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

func (a *Analyzer) AnalyzeHTML(content string) (types.AnalyzeResult, error) {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		log.Printf("Failed to parse HTML: %v", err)
		return types.AnalyzeResult{}, err
	}

	a.htmlVersion = a.detectHtmlVersion(content)

	a.dfs(doc)

	return types.AnalyzeResult{
		HtmlVersion:  a.htmlVersion,
		PageTitle:    a.title,
		Headings:     a.headings,
		Links:        a.links,
		HasLoginForm: a.hasLoginForm,
	}, nil
}

func (a *Analyzer) detectHtmlVersion(content string) string {
	content = strings.ToLower(content)

	if strings.Contains(content, "<!doctype html>") {
		return "HTML5"
	}

	return "No !doctype"
}

func (a *Analyzer) dfs(n *html.Node) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "title":
			a.title = strings.TrimSpace(n.FirstChild.Data)
		case "h1", "h2", "h3", "h4", "h5", "h6":
			a.headings[n.Data]++
		case "a":
			for _, attr := range n.Attr {
				if attr.Key == "href" && attr.Val != "" {
					a.links = append(a.links, attr.Val)
				}
			}
		case "form":
			if a.isLoginForm(n) {
				a.hasLoginForm = true
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		a.dfs(c)
	}
}

func (a *Analyzer) isLoginForm(formNode *html.Node) bool {
	hasPasswordField := false
	hasUsernameField := false

	a.dfsFormInputs(formNode, &hasPasswordField, &hasUsernameField)

	return hasPasswordField && hasUsernameField
}

func (a *Analyzer) dfsFormInputs(n *html.Node, hasPassword, hasEmail *bool) {
	if n.Type == html.ElementNode && n.Data == "input" {
		inputType := ""

		for _, attr := range n.Attr {
			if attr.Key == "type" {
				inputType = strings.ToLower(attr.Val)
			}
		}

		if inputType == "password" {
			*hasPassword = true
		}

		if inputType == "email" {
			*hasEmail = true
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		a.dfsFormInputs(c, hasPassword, hasEmail)
		if *hasPassword && *hasEmail {
			break
		}
	}
}
