package customcommands

import (
	"errors"

	"github.com/deviceplane/agent/pkg/agent/variables"
	"github.com/deviceplane/agent/pkg/models"
)

var (
	ErrCustomCommandsAreDisabled = errors.New("custom commands are disabled on this device")
)

type Validator struct {
	variables variables.Interface
}

func NewValidator(variables variables.Interface) *Validator {
	return &Validator{
		variables: variables,
	}
}

func (i *Validator) Validate(s models.Service) error {
	if i.variables.GetDisableCustomCommands() {
		if len(s.Command) != 0 ||
			len(s.Entrypoint) != 0 {
			return ErrCustomCommandsAreDisabled
		}
	}
	return nil
}

func (i *Validator) Name() string { return "DisableCustomCommandsValidator" }
