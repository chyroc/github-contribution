package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var g = new(GithubContribution)

type GithubContribution struct {
	token    string
	username string
}

type PR struct {
	URL               string    `json:"html_url"`
	RepoURL           string    `json:"repository_url"`
	Title             string    `json:"title"`
	CreatedAt         time.Time `json:"created_at"`
	AuthorAssociation string    `json:"author_association"`
	User              struct {
		Username string `json:"login"`
	} `json:"user"`
}

func init() {
	flag.StringVar(&g.token, "t", "", "token of github")
	flag.Parse()
}

func (g *GithubContribution) RequestGithubApi(method, path string, query map[string]string, body io.Reader) ([]byte, error) {
	if g.token == "" {
		return nil, fmt.Errorf("token of github cannot empty")
	}

	url := "https://api.github.com/" + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if query != nil {
		q := req.URL.Query()
		for k, v := range query {
			q.Add(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	req.Header.Add("Authorization", "token "+g.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(resp.Body)
}

func (g *GithubContribution) GetSelfUsername() (string, error) {
	body, err := g.RequestGithubApi("GET", "user", nil, nil)
	if err != nil {
		return "", err
	}

	var r struct {
		Login string `json:"login"`
	}
	err = json.Unmarshal(body, &r)
	if err != nil {
		return "", err
	}

	g.username = r.Login

	return r.Login, nil
}

func (g *GithubContribution) FetchPRs(page int) ([]*PR, error) {
	if g.username == "" {
		_, err := g.GetSelfUsername()
		if err != nil {
			return nil, err
		}
	}

	query := map[string]string{
		"q":        fmt.Sprintf("author:%s type:pr is:merged", g.username),
		"sort":     "created",
		"order":    "desc",
		"per_page": "100",
		"page":     strconv.Itoa(page),
	}

	body, err := g.RequestGithubApi("GET", "search/issues", query, nil)
	if err != nil {
		return nil, err
	}

	var r struct {
		TotalCount int   `json:"total_count"`
		Items      []*PR `json:"items"`
	}
	if err = json.Unmarshal(body, &r); err != nil {
		return nil, err
	}

	var prs []*PR
	for _, v := range r.Items {
		if strings.Contains(v.RepoURL, "https://api.github.com/repos/"+g.username+"/") {
			continue
		}

		v.RepoURL = strings.Replace(v.RepoURL, "https://api.github.com/repos/", "https://github.com/", -1)
		prs = append(prs, v)
	}
	return prs, nil
}

func main() {
	// get username
	name, err := g.GetSelfUsername()
	if err != nil {
		panic(err)
	}
	fmt.Printf("github name %s\n", name)

	for i := 1; i < 3; i++ {
		prs, err := g.FetchPRs(i)
		if err != nil {
			panic(err)
		}
		for _, v := range prs {
			fmt.Printf("pr %s\n", v)
		}
	}
}
