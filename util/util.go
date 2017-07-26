package util

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/urfave/cli"
)

const (
	accessKeyID     = "AWS_ACCESS_KEY_ID"
	secretAccessKey = "AWS_SECRET_ACCESS_KEY"
	defaultRegion   = "AWS_DEFAULT_REGION"
)

func ConfigAWS(c *cli.Context) error {
	region := c.GlobalString("awsregion")
	os.Setenv(defaultRegion, region)
	name := c.GlobalString("awsconf")
	if name == "" {
		return nil
	}
	cred := credentials.NewSharedCredentials("", name)
	credValue, err := cred.Get()
	if err != nil {
		return err
	}
	fmt.Printf("\x1b[32mAWS Profile Name: %s, Region: %s\x1b[0m\n", name, region)
	os.Setenv(accessKeyID, credValue.AccessKeyID)
	os.Setenv(secretAccessKey, credValue.SecretAccessKey)
	return nil
}

func PdiffTaskDef(target, previous string) {
	targetStrSlice := strings.Split(target, "\n")
	previousStrSlice := strings.Split(previous, "\n")
	shorter := len(targetStrSlice) < len(previousStrSlice)
	var buff bytes.Buffer
	var minNum int
	if shorter {
		minNum = len(targetStrSlice)
	} else {
		minNum = len(previousStrSlice)
	}
	for i := 0; i < minNum; i++ {
		tv := targetStrSlice[i]
		pv := previousStrSlice[i]
		if tv != pv {
			_, _ = buff.WriteString("\x1b[32m")
			_, _ = buff.WriteString("+" + tv)
			_, _ = buff.WriteString("\x1b[0m\n")
			_, _ = buff.WriteString("\x1b[31m")
			_, _ = buff.WriteString("-" + pv)
			_, _ = buff.WriteString("\x1b[0m\n")
		} else {
			_, _ = buff.WriteString(pv + "\n")
		}
	}
	if shorter {
		for i := minNum; i < len(previousStrSlice); i++ {
			_, _ = buff.WriteString("\x1b[31m")
			_, _ = buff.WriteString("-" + previousStrSlice[i])
			_, _ = buff.WriteString("\x1b[0m\n")
		}
	} else {
		for i := minNum; i < len(targetStrSlice); i++ {
			_, _ = buff.WriteString("\x1b[32m")
			_, _ = buff.WriteString("-" + targetStrSlice[i])
			_, _ = buff.WriteString("\x1b[0m\n")
		}
	}
	fmt.Println(buff.String())
}
