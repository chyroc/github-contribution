package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var g = new(GithubContribution)
var perCount = 100

type GithubContribution struct {
	token    string
	username string
}

type PR struct {
	TotalCount int          `json:"total_count"`
	PRs        []*PRContent `json:"items"`
}

type PRContent struct {
	URL               string    `json:"html_url"`
	RepoURL           string    `json:"repository_url"`
	Title             string    `json:"title"`
	CreatedAt         time.Time `json:"created_at"`
	AuthorAssociation string    `json:"author_association"`
	User              struct {
		Username string `json:"login"`
	} `json:"user"`
}

type PRGroupBy struct {
	RepoName string       `json:"repo_name"`
	RepoStar int          `json:"repo_star"`
	RepoURL  string       `json:"repo_url"`
	PR       []*PRContent `json:"pr"`
}

type PRContents struct {
	prs []*PRContent
}

func (p PRContents) Len() int {
	return len(p.prs)
}

func (p PRContents) Less(i, j int) bool {
	if t := strings.Compare(p.prs[i].RepoURL, p.prs[j].RepoURL); t != 0 {
		return t > 0
	}
	return p.prs[i].CreatedAt.Sub(p.prs[j].CreatedAt) > 0
}

func (p PRContents) Swap(i, j int) {
	p.prs[i], p.prs[j] = p.prs[j], p.prs[i]
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

func (g *GithubContribution) FetchPR(page, perPage int) (*PR, error) {
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
		"per_page": strconv.Itoa(perPage),
		"page":     strconv.Itoa(page),
	}

	body, err := g.RequestGithubApi("GET", "search/issues", query, nil)
	if err != nil {
		return nil, err
	}

	var r = new(PR)
	if err = json.Unmarshal(body, &r); err != nil {
		return nil, err
	}

	return r, nil
}

func (g *GithubContribution) FetchPRContent(page, perPage int) ([]*PRContent, error) {
	r, err := g.FetchPR(page, perPage)
	if err != nil {
		return nil, err
	}

	var prs []*PRContent
	for _, v := range r.PRs {
		if strings.Contains(v.RepoURL, "https://api.github.com/repos/"+g.username+"/") {
			continue
		}

		v.RepoURL = strings.Replace(v.RepoURL, "https://api.github.com/repos/", "https://github.com/", -1)
		prs = append(prs, v)
	}
	return prs, nil
}

func (g *GithubContribution) FetchPRCount(page, perPage int) (int, error) {
	r, err := g.FetchPR(page, perPage)
	if err != nil {
		return 0, err
	}

	return r.TotalCount, nil
}

func (g *GithubContribution) FetchRepoInfo(page, perPage int) (int, error) {
	return 0, fmt.Errorf("err")
}

func (g *GithubContribution) FetchAllPRs() ([]*PRContent, error) {
	count, err := g.FetchPRCount(1, 1)
	if err != nil {
		return nil, err
	}

	var prChan = make(chan *PRContent, count)

	var sw sync.WaitGroup
	for page := 1; page*perCount < count; page++ {
		sw.Add(1)
		go func(page int) {
			fmt.Printf("start goroutine %d\n", page)
			prs, err := g.FetchPRContent(page, perCount)
			if err != nil {
				panic(err)
			}

			for _, v := range prs {
				prChan <- v
			}

			sw.Done()
		}(page)

	}
	sw.Wait()

	var prs []*PRContent
	for {
		select {
		case v := <-prChan:
			prs = append(prs, v)
		default:
			return prs, nil
		}
	}

	return prs, nil
}

func (g *GithubContribution) ParsePRContents(prs []*PRContent) ([]byte, error) {
	pp := PRContents{prs}
	sort.Sort(pp)

	var exist = make(map[string]bool)
	var buf bytes.Buffer
	buf.Write([]byte("# 开源项目贡献统计\n\n"))
	buf.Write([]byte(fmt.Sprintf("## Contributions(%d merged)\n\n\n", len(prs))))

	for _, v := range pp.prs {
		fmt.Printf("%s\n", v.URL)
		if !exist[v.RepoURL] {
			exist[v.RepoURL] = true
			buf.Write([]byte(fmt.Sprintf("* [**%s**(★%d)](%s)\n", strings.TrimPrefix(v.RepoURL, "https://github.com/"), 10, v.RepoURL)))
		}

		buf.Write([]byte(fmt.Sprintf("  * [%s](%s)\n", v.Title, v.URL)))
	}

	return buf.Bytes(), nil
}

func (g *GithubContribution) Run() error {
	prs, err := g.FetchAllPRs()
	if err != nil {
		return err
	}

	body, err := g.ParsePRContents(prs)
	if err != nil {
		return err
	}

	fmt.Printf("%s", body)
	return nil
}

func main() {
	err := g.Run()
	if err != nil {
		panic(err)
	}
}
