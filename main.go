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
	config := flag.String("c", "", "config path")
	flag.Parse()

	if *config != "" {
		err := internal.LoadConfig(*config)
		if err != nil {
			panic(err)
		}
	}
}

func main() {
	var buf bytes.Buffer
	buf.Write([]byte(`# contribute
list: 开源贡献 / 个人项目
> 使用工具github-contribution制作(https://github.com/Chyroc/github-contribution)

`))

	b, err := internal.DefaultGithub.Run()
	if err != nil {
		panic(err)
	}
	buf.Write(b)

	b, err = internal.DefaultSideProject.Run()
	if err != nil {
		panic(err)
	}
	buf.Write(b)

	err = ioutil.WriteFile("README.md", buf.Bytes(), 0644)
	if err != nil {
		panic(err)
	}

}
