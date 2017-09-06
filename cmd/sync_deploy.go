package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"

	yaml "gopkg.in/yaml.v2"

	"github.com/atsushi-ishibashi/influencer/svc"
	"github.com/atsushi-ishibashi/influencer/util"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/urfave/cli"
)

func NewSyncDeployCommand(out, errOut io.Writer) cli.Command {
	return cli.Command{
		Name:  "sync-deploy",
		Usage: "Run tasks and update service synchronously",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "path",
				Usage: "path to yaml deploy config file",
			},
			cli.BoolFlag{
				Name:  "dry-run",
				Usage: "dry-run. output diff in pretty",
			},
		},
		Action: func(c *cli.Context) error {
			if err := util.ConfigAWS(c); err != nil {
				return util.ErrorRed(err.Error())
			}
			sd, err := newSyncDeploy(c)
			if err != nil {
				return util.ErrorRed(err.Error())
			}
			if err = sd.validateECRImage(); err != nil {
				return util.ErrorRed(err.Error())
			}
			for _, dt := range sd.deployTasks {
				ltd, err := sd.ecsCli.FetchLatestTaskDefinition(dt.taskDefinition)
				if err != nil {
					return util.ErrorRed(err.Error())
				}
				ntd, err := sd.createNewTaskDefinition(ltd, dt.image)
				if err != nil {
					return util.ErrorRed(err.Error())
				}
				sd.printWorkFlow(dt, ltd, ntd)
				if c.Bool("dry-run") == false {
					err = sd.execute(dt, ltd, ntd)
					if err != nil {
						return util.ErrorRed(err.Error())
					}
				}
			}
			return nil
		},
	}
}

type deployTask struct {
	taskDefinition string
	image          *containerImage
	cluster        string
	service        string
}

type syncDeploy struct {
	deployTasks []*deployTask
	ecsCli      *svc.EcsClient
	ecrCli      *svc.EcrClient
}

func newSyncDeploy(c *cli.Context) (*syncDeploy, error) {
	sd := &syncDeploy{}
	//path flag
	if c.String("path") != "" {
		if err := sd.parseYaml(c.String("path")); err != nil {
			return nil, err
		}
	}
	awsregion := os.Getenv("AWS_DEFAULT_REGION")
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	sd.ecsCli = &svc.EcsClient{ECS: ecs.New(sess, &aws.Config{
		Region: aws.String(awsregion),
	})}
	sd.ecrCli = &svc.EcrClient{ECR: ecr.New(sess, &aws.Config{
		Region: aws.String(awsregion),
	})}
	return sd, nil
}

func (sd *syncDeploy) execute(dt *deployTask, ltd, ntd *ecs.TaskDefinition) error {
	regiTaskDef, err := sd.ecsCli.RegisterTaskDefinition(ntd)
	if err != nil {
		return err
	}
	util.PrintlnGreen("\tExecuting...")
	util.PrintlnGreen(fmt.Sprintf("\tRegistered task definition: %s:%d...", *regiTaskDef.Family, *regiTaskDef.Revision))
	if dt.service == "" {
		util.PrintlnGreen(fmt.Sprintf("\tRunning task of %s:%d on cluster %s...", *regiTaskDef.Family, *regiTaskDef.Revision, dt.cluster))
		rtRes, err := sd.ecsCli.InvokeTask(dt.cluster, regiTaskDef)
		if err != nil {
			return err
		}
		if len(rtRes.Failures) > 0 {
			return fmt.Errorf("%s", rtRes.Failures)
		}
		taskARNs := make([]*string, len(rtRes.Tasks))
		for _, v := range rtRes.Tasks {
			taskARNs = append(taskARNs, v.TaskArn)
		}
		util.PrintlnGreen(fmt.Sprintf("\tWaiting until %s finish...", dt.taskDefinition))
		// FIXME: WaitUntilTasksStop stopping...
		// if err := sd.ecsCli.WaitUntilTasksStop(taskARNs); err != nil {
		// 	return err
		// }
		util.PrintlnGreen(fmt.Sprintf("\t%s finished!!!", dt.taskDefinition))
	} else {
		curSer, err := sd.ecsCli.FetchService(dt.cluster, dt.service)
		if err != nil {
			return err
		}
		util.PrintlnGreen(fmt.Sprintf("\tUpdating service %s...", dt.service))
		_, err = sd.ecsCli.UpdateServiceWithTaskDef(curSer, regiTaskDef)
		if err != nil {
			return err
		}
		util.PrintlnGreen(fmt.Sprintf("\tWaiting until updating %s finish...", dt.service))
		if err := sd.ecsCli.WaitUntilServiceUpdate(dt.cluster, dt.service); err != nil {
			return err
		}
		util.PrintlnGreen(fmt.Sprintf("\tupdating %s finished!!!", dt.service))
	}
	util.PrintlnGreen("\tFinished!!!")
	return nil
}

func (sd *syncDeploy) printWorkFlow(dt *deployTask, ltd, ntd *ecs.TaskDefinition) {
	if dt.service == "" {
		fmt.Println("Deploy oneshot task:")
	} else {
		fmt.Println("Deploy service task:")
	}
	fmt.Printf("\tcluster: %s\n", dt.cluster)
	if dt.service != "" {
		fmt.Printf("\tservice: %s\n", dt.service)
	}
	fmt.Printf("\ttask definition: %s\n", dt.taskDefinition)
	util.PrintlnRed(fmt.Sprintf("\t\t- %s:%d", *ltd.Family, *ltd.Revision))
	for _, vv := range ltd.ContainerDefinitions {
		util.PrintlnRed(fmt.Sprintf("\t\t- %s", *vv.Image))
	}
	util.PrintlnGreen(fmt.Sprintf("\t\t+ %s:%d", *ntd.Family, *ntd.Revision+1))
	for _, vv := range ntd.ContainerDefinitions {
		util.PrintlnGreen(fmt.Sprintf("\t\t+ %s", *vv.Image))
	}
	fmt.Printf("\tcontainer imager: %s\n", dt.image.String())
}

func (sd *syncDeploy) createNewTaskDefinition(taskDef *ecs.TaskDefinition, container *containerImage) (*ecs.TaskDefinition, error) {
	reg := regexp.MustCompile(fmt.Sprintf(".+dkr.ecr.%s.amazonaws.com/%s", os.Getenv("AWS_DEFAULT_REGION"), container.name))
	newTaskDef := *taskDef
	var containers []*ecs.ContainerDefinition
	for _, c := range taskDef.ContainerDefinitions {
		cc := *c
		if reg.MatchString(*c.Image) {
			dimg, err := sd.ecrCli.FetchImageWithTag(container.name, container.tag)
			if err != nil {
				// TODO: DockerHubなどのイメージ対応
				return nil, err
			}
			cc.Image = aws.String(fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:%s", *dimg.RegistryId, os.Getenv("AWS_DEFAULT_REGION"), *dimg.RepositoryName, *dimg.ImageId.ImageTag))
		}
		containers = append(containers, &cc)
	}
	newTaskDef.ContainerDefinitions = containers
	return &newTaskDef, nil
}

type DeployTaskYamlConfig struct {
	Task    string `yaml:"task"`
	Image   string `yaml:"image"`
	Cluster string `yaml:"cluster"`
	Service string `yaml:"service"`
}

func (sd *syncDeploy) parseYaml(path string) error {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	var ycs []*DeployTaskYamlConfig
	if err = yaml.Unmarshal(buf, &ycs); err != nil {
		return err
	}
	dts := make([]*deployTask, 0)
	for _, v := range ycs {
		dt := &deployTask{}
		if v.Cluster == "" {
			return util.ErrorRed("cluster is required in yaml")
		}
		if v.Task == "" {
			return util.ErrorRed("task is required in yaml")
		}
		dt.cluster = v.Cluster
		dt.service = v.Service
		dt.taskDefinition = v.Task
		img, err := toContainerImage(v.Image)
		if err != nil {
			return util.ErrorRed(fmt.Sprintf("Container name is invalid, %s", v.Image))
		}
		dt.image = &img
		dts = append(dts, dt)
	}
	sd.deployTasks = dts
	return nil
}

func (sd *syncDeploy) validateECRImage() error {
	for _, v := range sd.deployTasks {
		_, err := sd.ecrCli.FetchImageWithTag(v.image.name, v.image.tag)
		if err != nil {
			return fmt.Errorf("Not found ecr image %s:%s", v.image.name, v.image.tag)
		}
	}
	return nil
}
