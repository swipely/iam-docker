package mock

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/service/sts"
)

// STSClient implements github.com/swipely/iam-docker/src/iam.STSClient.
type STSClient struct {
	AssumableRoles map[string]*sts.Credentials
}

// NewSTSClient returns a mock STSClient.
func NewSTSClient() *STSClient {
	return &STSClient{
		AssumableRoles: make(map[string]*sts.Credentials),
	}
}

// AssumeRole uses the mock's AssumableRoles to try to assume a new IAM role.
func (mock *STSClient) AssumeRole(input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	if input == nil {
		return nil, errors.New("No AssumeRoleInput given")
	} else if input.RoleArn == nil {
		return nil, errors.New("No RoleArn given")
	}
	credential, hasKey := mock.AssumableRoles[*input.RoleArn]
	if !hasKey {
		return nil, fmt.Errorf("Cannot assume role: %s", *input.RoleArn)
	}
	output := &sts.AssumeRoleOutput{Credentials: credential}
	return output, nil
}
