package internal

import (
	"io/ioutil"
	"encoding/json"
)

var (
	Debug  = false
	Config = new(config)
)

type config struct {
	SideProject []struct {
		Name         string `json:"name"`
		Introduction string `json:"introduction"`
		URL          string `json:"url"`
	} `json:"side_project"`
}

func LoadConfig(path string) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, Config)
}
