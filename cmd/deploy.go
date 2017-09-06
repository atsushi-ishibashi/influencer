package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/atsushi-ishibashi/influencer/svc"
	"github.com/atsushi-ishibashi/influencer/util"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/urfave/cli"
)

func NewPlanCommand(out, errOut io.Writer) cli.Command {
	return cli.Command{
		Name:  "deploy",
		Usage: "Update task definition by image in args and update service with the task definition",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "cluster",
				Usage: "cluster name",
			},
			cli.StringFlag{
				Name:  "service",
				Usage: "service name",
			},
			cli.StringSliceFlag{
				Name:  "image",
				Usage: "image repo:tag, more than 1",
			},
			cli.BoolFlag{
				Name:  "dry-run",
				Usage: "dry-run. output diff in pretty",
			},
		},
		Action: func(c *cli.Context) error {
			if err := util.ConfigAWS(c); err != nil {
				return err
			}
			p, err := newPlan(c)
			if err != nil {
				return err
			}
			if err = p.validateECRImage(); err != nil {
				return fmt.Errorf("\x1b[31m%s\x1b[0m", err)
			}
			if c.Bool("dry-run") {
				if err = p.printDiff(); err != nil {
					return err
				}
			} else {
				if err = p.execute(); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

type plan struct {
	cluster string
	service string
	images  []containerImage
	ecsCli  *svc.EcsClient
	ecrCli  *svc.EcrClient
}

func newPlan(c *cli.Context) (plan, error) {
	p := plan{images: make([]containerImage, 0)}
	if c.String("cluster") == "" {
		return p, errors.New("\x1b[31m--cluster is required\x1b[0m")
	}
	if c.String("service") == "" {
		return p, errors.New("\x1b[31m--service is required\x1b[0m")
	}
	if len(c.StringSlice("image")) == 0 {
		return p, errors.New("\x1b[31m--image is required\x1b[0m")
	}
	p.cluster = c.String("cluster")
	p.service = c.String("service")
	for _, v := range c.StringSlice("image") {
		ci, err := toContainerImage(v)
		if err != nil {
			return p, err
		}
		p.images = append(p.images, ci)
	}
	awsregion := os.Getenv("AWS_DEFAULT_REGION")
	sess, err := session.NewSession()
	if err != nil {
		return p, err
	}
	p.ecsCli = &svc.EcsClient{ECS: ecs.New(sess, &aws.Config{
		Region: aws.String(awsregion),
	})}
	p.ecrCli = &svc.EcrClient{ECR: ecr.New(sess, &aws.Config{
		Region: aws.String(awsregion),
	})}
	return p, nil
}

func (p *plan) execute() error {
	serv, err := p.fetchService()
	if err != nil {
		return err
	}
	taskDef, err := p.fetchTaskDefinition(*serv.TaskDefinition)
	if err != nil {
		return err
	}
	newTaskDef, changed, err := p.createNewTaskDefinition(taskDef)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Println("\x1b[31m" + "There is no difference from current task definition..." + "\x1b[0m")
		util.PdiffTaskDef(newTaskDef.String(), taskDef.String())
		return nil
	}
	regiTaskDef, err := p.registerTaskDefinition(newTaskDef)
	if err != nil {
		return err
	}
	fmt.Println("\x1b[32m" + "Registered New Task Definition..." + "\x1b[0m")
	util.PdiffTaskDef(regiTaskDef.String(), taskDef.String())
	newServ, err := p.ecsCli.UpdateServiceWithTaskDef(serv, regiTaskDef)
	if err != nil {
		return err
	}
	fmt.Printf("\x1b[32mUpdate Service... cluster arn: %s, service name: %s, task definition: %s, task count: %d\x1b[0m\n", *newServ.ClusterArn, *newServ.ServiceName, *newServ.TaskDefinition, *newServ.DesiredCount)
	return nil
}

func (p *plan) printDiff() error {
	serv, err := p.fetchService()
	if err != nil {
		return err
	}
	taskDef, err := p.fetchTaskDefinition(*serv.TaskDefinition)
	if err != nil {
		return err
	}
	newTaskDef, _, err := p.createNewTaskDefinition(taskDef)
	if err != nil {
		return err
	}
	util.PdiffTaskDef(newTaskDef.String(), taskDef.String())
	return nil
}

func (p *plan) fetchTaskDefinition(taskDefName string) (*ecs.TaskDefinition, error) {
	return p.ecsCli.FetchTaskDefinition(taskDefName)
}

func (p *plan) fetchService() (*ecs.Service, error) {
	return p.ecsCli.FetchService(p.cluster, p.service)
}

func (p *plan) registerTaskDefinition(taskDef *ecs.TaskDefinition) (*ecs.TaskDefinition, error) {
	return p.ecsCli.RegisterTaskDefinition(taskDef)
}

func (p *plan) createNewTaskDefinition(taskDef *ecs.TaskDefinition) (*ecs.TaskDefinition, bool, error) {
	newTaskDef := *taskDef
	changed := false
	var containers []*ecs.ContainerDefinition
	for _, c := range taskDef.ContainerDefinitions {
		cc := *c
		if img, ok := p.searchImage(*c.Name); ok {
			dimg, err := p.ecrCli.FetchImageWithTag(img.name, img.tag)
			if err != nil {
				// TODO: DockerHubなどのイメージ対応
				return nil, changed, err
			}
			cc.Image = aws.String(fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:%s", *dimg.RegistryId, os.Getenv("AWS_DEFAULT_REGION"), *dimg.RepositoryName, *dimg.ImageId.ImageTag))
			if *cc.Image != *c.Image {
				changed = true
			}
		}
		containers = append(containers, &cc)
	}
	newTaskDef.ContainerDefinitions = containers
	return &newTaskDef, changed, nil
}

func (p *plan) validateECRImage() error {
	for _, v := range p.images {
		_, err := p.ecrCli.FetchImageWithTag(v.name, v.tag)
		if err != nil {
			return fmt.Errorf("Not Found ECR Image %s:%s\n", v.name, v.tag)
		}
	}
	return nil
}

func (p *plan) searchImage(imageName string) (containerImage, bool) {
	for _, v := range p.images {
		if v.name == imageName {
			return v, true
		}
	}
	return containerImage{}, false
}
