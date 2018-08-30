package internal

import (
	"bytes"
	"encoding/json"
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

var perCount = 100
var debug = false
var DefaultGithub = new(GithubContribution)

type GithubContribution struct {
	Token    string
	username string
}

type PR struct {
	TotalCount int          `json:"total_count"`
	PRs        []*PRContent `json:"items"`
}

type PRContent struct {
	RepoURL           string    `json:"-"`
	RepoName          string    `json:"-"`
	BelongSelf        bool      `json:"-"`
	RepoStarCount     int       `json:"-"`
	URL               string    `json:"html_url"`
	RepoAPIURL        string    `json:"repository_url"`
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
	if t := strings.Compare(p.prs[i].RepoName, p.prs[j].RepoName); t != 0 {
		return t > 0
	}
	return p.prs[i].CreatedAt.Sub(p.prs[j].CreatedAt) > 0
}

func (p PRContents) Swap(i, j int) {
	p.prs[i], p.prs[j] = p.prs[j], p.prs[i]
}

func printVerbose(format string, a ...interface{}) {
	if Debug {
		fmt.Printf(format, a...)
	}
}

func (g *GithubContribution) RequestGithubApi(method, path string, query map[string]string, body io.Reader) ([]byte, error) {
	if g.Token == "" {
		return nil, fmt.Errorf("Token of github cannot empty")
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

	req.Header.Add("Authorization", "Token "+g.Token)

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

	printVerbose("Login to GitHub as [%s] ...\n\n", r.Login)
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

	for _, v := range r.PRs {
		v.RepoName = strings.TrimPrefix(v.RepoAPIURL, "https://api.github.com/repos/")
		v.RepoURL = strings.Replace(v.RepoAPIURL, "https://api.github.com/repos/", "https://github.com/", -1)
		v.BelongSelf = strings.Contains(v.RepoAPIURL, "https://api.github.com/repos/"+g.username+"/")
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
		if !v.BelongSelf {
			prs = append(prs, v)
		}
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

func (g *GithubContribution) GetRepoStar(p *PRContent) (int, error) {
	printVerbose("get repo(%s) star count\n", p.RepoName)
	body, err := g.RequestGithubApi("GET", "repos/"+p.RepoName, nil, nil)
	if err != nil {
		return -1, err
	}

	var r struct {
		StarCount int `json:"stargazers_count"`
	}
	err = json.Unmarshal(body, &r)
	if err != nil {
		return -1, err
	}

	return r.StarCount, nil
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
			defer sw.Done()

			printVerbose("start goroutine %d\n", page)
			prs, err := g.FetchPRContent(page, perCount)
			if err != nil {
				panic(err)
			}

			for _, v := range prs {
				prChan <- v
			}
		}(page)

	}
	sw.Wait()

	var prs []*PRContent
	var repoStarMap = make(map[string]int)
	var sw2 sync.WaitGroup

	for {
		select {
		case v := <-prChan:
			if _, ok := repoStarMap[v.RepoName]; !ok {
				// get star
				sw2.Add(1)
				go func() {
					defer sw2.Done()
					starCount, err := g.GetRepoStar(v)
					if err != nil {
						panic(err)
					}
					repoStarMap[v.RepoName] = starCount
				}()
			}
			prs = append(prs, v)
		default:
			goto Re
		}
	}

Re:
	sw2.Wait()

	for _, v := range prs {
		v.RepoStarCount = repoStarMap[v.RepoName]
	}

	return prs, nil
}

func (g *GithubContribution) ParsePRContents(prs []*PRContent) ([]byte, error) {
	pp := PRContents{prs}
	sort.Sort(pp)

	var exist = make(map[string]bool)
	var buf bytes.Buffer

	var newPrs []*PRContent
	for _, v := range pp.prs {
		ignored := false
		for _, ignore := range Config.GithubProject.Ignore {
			if strings.Contains(strings.ToLower(v.RepoName), ignore) || strings.Contains(strings.ToLower(v.Title), ignore) {
				ignored = true
				break
			}
		}
		if ignored {
			continue
		}

		newPrs = append(newPrs, v)
	}
	buf.Write([]byte(fmt.Sprintf(`## 开源项目贡献统计(%d merged)`, len(newPrs)) + "\n\n"))

	for _, v := range newPrs {
		ignored := false
		for _, ignore := range Config.GithubProject.Ignore {
			if v.RepoName == ignore {
				ignored = true
				break
			}
		}
		if ignored {
			continue
		}

		if !exist[v.RepoName] {
			exist[v.RepoName] = true
			buf.Write([]byte(fmt.Sprintf("* [**%s**(★%d)](%s)\n", v.RepoName, v.RepoStarCount, v.RepoURL)))
		}

		buf.Write([]byte(fmt.Sprintf("  * [%s](%s)\n", v.Title, v.URL)))
	}

	return buf.Bytes(), nil
}

func (g *GithubContribution) Run() ([]byte, error) {
	prs, err := g.FetchAllPRs()
	if err != nil {
		return nil, err
	}

	return g.ParsePRContents(prs)
}
