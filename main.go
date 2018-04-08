package main

import (
	"bytes"
	"flag"
	"io/ioutil"

	"github.com/Chyroc/github-contribution/internal"
)

func init() {
	flag.StringVar(&internal.DefaultGithub.Token, "t", "", "token of github")
	flag.BoolVar(&internal.Debug, "v", false, "verbose")
	flag.Parse()
}

func main() {
	var buf bytes.Buffer
	buf.Write([]byte(`# contribute
list: 开源贡献 / 个人项目

`))

	b, err := internal.DefaultGithub.Run()
	if err != nil {
		panic(err)
	}
	buf.Write(b)

	err = ioutil.WriteFile("README.md", buf.Bytes(), 0644)
	if err != nil {
		panic(err)
	}
}
