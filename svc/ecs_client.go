package svc

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
)

type EcsClient struct {
	*ecs.ECS
}

func (ec *EcsClient) FetchTaskDefinition(taskDefName string) (*ecs.TaskDefinition, error) {
	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefName),
	}
	result, err := ec.DescribeTaskDefinition(input)
	if err != nil {
		return nil, err
	}
	return result.TaskDefinition, nil
}

func (ec *EcsClient) FetchLatestTaskDefinition(familyName string) (*ecs.TaskDefinition, error) {
	input := &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: aws.String(familyName),
		MaxResults:   aws.Int64(1),
		Sort:         aws.String("DESC"),
	}
	result, err := ec.ListTaskDefinitions(input)
	if err != nil {
		return nil, err
	}
	if len(result.TaskDefinitionArns) == 0 {
		return nil, fmt.Errorf("Not found task definitions of %s (family-prefix)", familyName)
	}
	descInput := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: result.TaskDefinitionArns[0],
	}
	descResult, err := ec.DescribeTaskDefinition(descInput)
	if err != nil {
		return nil, err
	}
	return descResult.TaskDefinition, nil
}

func (ec *EcsClient) FetchService(cluster, service string) (*ecs.Service, error) {
	input := &ecs.DescribeServicesInput{
		Cluster: aws.String(cluster),
		Services: []*string{
			aws.String(service),
		},
	}
	result, err := ec.DescribeServices(input)
	if err != nil {
		return nil, err
	}
	if len(result.Services) == 0 {
		return nil, fmt.Errorf("Not Found Service: %s", service)
	}
	return result.Services[0], nil
}

func (ec *EcsClient) RegisterTaskDefinition(taskDef *ecs.TaskDefinition) (*ecs.TaskDefinition, error) {
	input := &ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions: taskDef.ContainerDefinitions,
		Family:               taskDef.Family,
		NetworkMode:          taskDef.NetworkMode,
		PlacementConstraints: taskDef.PlacementConstraints,
		TaskRoleArn:          taskDef.TaskRoleArn,
		Volumes:              taskDef.Volumes,
	}
	res, err := ec.ECS.RegisterTaskDefinition(input)
	if err != nil {
		return nil, err
	}
	return res.TaskDefinition, nil
}

func (ec *EcsClient) UpdateServiceWithTaskDef(service *ecs.Service, taskDef *ecs.TaskDefinition) (*ecs.Service, error) {
	input := &ecs.UpdateServiceInput{
		Cluster:                 service.ClusterArn,
		DeploymentConfiguration: service.DeploymentConfiguration,
		DesiredCount:            service.DesiredCount,
		Service:                 service.ServiceName,
		TaskDefinition:          taskDef.TaskDefinitionArn,
	}

	result, err := ec.UpdateService(input)
	if err != nil {
		return nil, err
	}

	return result.Service, nil
}

func (ec *EcsClient) WaitUntilTasksStop(taskARNs []*string) error {
	input := &ecs.DescribeTasksInput{
		Tasks: taskARNs,
	}
	return ec.WaitUntilTasksStopped(input)
}

func (ec *EcsClient) WaitUntilServiceUpdate(cluster, service string) error {
	input := &ecs.DescribeServicesInput{
		Cluster: aws.String(cluster),
		Services: []*string{
			aws.String(service),
		},
	}
	return ec.WaitUntilServicesStable(input)
}

func (ec *EcsClient) InvokeTask(cluster string, taskDef *ecs.TaskDefinition) (*ecs.RunTaskOutput, error) {
	input := &ecs.RunTaskInput{
		Cluster:        aws.String(cluster),
		TaskDefinition: taskDef.TaskDefinitionArn,
	}
	return ec.RunTask(input)
}

func (ec *EcsClient) WatchTasks(taskARNs []*string) (*ecs.DescribeTasksOutput, error) {
	input := &ecs.DescribeTasksInput{
		Tasks: taskARNs,
	}
	return ec.DescribeTasks(input)
}
