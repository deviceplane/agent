package validator

import (
	"github.com/deviceplane/agent/pkg/models"
)

type Validator interface {
	Validate(models.Service) error
	Name() string
}
