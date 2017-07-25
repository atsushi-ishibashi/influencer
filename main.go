package main

import (
	"os"

	"github.com/atsushi-ishibashi/influencer/cmd"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "awsconf",
			Usage: "~/.aws/credentialsから環境変数をセット(プロセスの間のみ)",
		},
		cli.StringFlag{
			Name:  "awsregion",
			Usage: "AWS_DEFAULT_REGIONにセット(プロセスの間のみ)",
			Value: "ap-northeast-1",
		},
	}

	planCommand := cmd.NewPlanCommand(os.Stdout, os.Stderr)

	app.Commands = []cli.Command{
		planCommand,
	}
	app.Run(os.Args)
}
