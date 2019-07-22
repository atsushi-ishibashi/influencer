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
			Name:  "awscredentialsfile",
			Usage: "awsのcredentialsファイル（デフォルトは~/.aws/credential）",
		},
		cli.StringFlag{
			Name:  "awsconf",
			Usage: "awscredentialsfileから環境変数をセット(プロセスの間のみ)",
		},
		cli.StringFlag{
			Name:  "awsregion",
			Usage: "AWS_DEFAULT_REGIONにセット(プロセスの間のみ)",
			Value: "ap-northeast-1",
		},
	}

	planCommand := cmd.NewPlanCommand(os.Stdout, os.Stderr)
	syncDeployCommand := cmd.NewSyncDeployCommand(os.Stdout, os.Stderr)

	app.Commands = []cli.Command{
		planCommand,
		syncDeployCommand,
	}
	app.Run(os.Args)
}
