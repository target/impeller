package commandbuilder

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

const (
	ArgTypeRaw        = 1
	ArgTypeShortParam = 2
	ArgTypeLongParam  = 3
)

type CommandBuilder struct {
	Name  string
	Parts []Arg
}

func (cb *CommandBuilder) SafeString() string {
	s := []string{cb.Name}
	for _, arg := range cb.Parts {
		s = append(s, arg.SafeString())
	}
	return strings.Join(s, " ")
}

func (cb *CommandBuilder) Command() *exec.Cmd {
	args := []string{}
	for _, arg := range cb.Parts {
		args = append(args, arg.UnsafeParts()...)
	}

	return exec.Command(cb.Name, args...)
}

func (cb *CommandBuilder) Add(args ...Arg) {
	cb.Parts = append(cb.Parts, args...)
}

func (cb *CommandBuilder) Run() error {
	log.Printf("RUNNING: %s", cb.SafeString())
	cmd := cb.Command()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type Arg struct {
	Type  uint
	Name  string
	Value string

	ValueSecret bool
}

func (a Arg) SafeString() string {
	switch a.Type {
	case ArgTypeRaw:
		return a.SafeValue()
	case ArgTypeShortParam:
		return fmt.Sprintf("-%s %s", a.Name, a.SafeValue())
	case ArgTypeLongParam:
		return fmt.Sprintf("--%s %s", a.Name, a.SafeValue())
	default:
		return a.SafeValue()
	}
}

func (a Arg) SafeValue() string {
	if a.ValueSecret {
		return "[SECRET]"
	}
	return a.Value
}

func (a Arg) UnsafeParts() []string {
	switch a.Type {
	case ArgTypeRaw:
		return []string{a.Value}
	case ArgTypeShortParam:
		return []string{fmt.Sprintf("-%s", a.Name), a.Value}
	case ArgTypeLongParam:
		return []string{fmt.Sprintf("--%s", a.Name), a.Value}
	default:
		return []string{a.Value}
	}
}
