package svc

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
)

type EcrClient struct {
	*ecr.ECR
}

func (ec *EcrClient) FetchImageWithTag(repo, tag string) (*ecr.Image, error) {
	input := &ecr.BatchGetImageInput{
		ImageIds: []*ecr.ImageIdentifier{
			{
				ImageTag: aws.String(tag),
			},
		},
		RepositoryName: aws.String(repo),
		AcceptedMediaTypes: []*string{
			aws.String("application/vnd.docker.distribution.manifest.v1+json"),
			aws.String("application/vnd.docker.distribution.manifest.v2+json"),
			aws.String("application/vnd.oci.image.manifest.v1+json"),
		},
	}
	result, err := ec.BatchGetImage(input)
	if err != nil {
		return nil, err
	}
	if len(result.Images) == 0 {
		return nil, fmt.Errorf("Not Found Image repo: %s, tag: %s", repo, tag)
	}
	return result.Images[0], nil
}
