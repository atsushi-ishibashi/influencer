package cmd

import (
	"fmt"
	"strings"
)

type containerImage struct {
	name string
	tag  string
}

func toContainerImage(s string) (containerImage, error) {
	ci := containerImage{}
	sl := strings.Split(s, ":")
	if len(sl) != 2 {
		return ci, fmt.Errorf("image path is invalid: %s", s)
	}
	ci.name = sl[0]
	ci.tag = sl[1]
	return ci, nil
}

func (c *containerImage) String() string {
	return fmt.Sprintf("%s:%s", c.name, c.tag)
}
