package internal

import (
	"bytes"
	"fmt"
)

type side struct {
}

var DefaultSideProject = new(side)

func (s *side) Run() ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("\n## 个人项目\n\n")

	for _, v := range Config.SideProject {
		buf.WriteString(fmt.Sprintf(`* [%s](%s): %s`+"\n", v.Name, v.URL, v.Introduction))
	}

	return buf.Bytes(), nil
}
