package commandbuilder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandBuilderSafeString(t *testing.T) {
	cb := CommandBuilder{Name: "testbin"}
	cb.Add(Arg{Type: ArgTypeRaw, Value: "notsecret", ValueSecret: false})
	cb.Add(Arg{Type: ArgTypeRaw, Value: "rawparam", ValueSecret: true})
	cb.Add(Arg{Type: ArgTypeShortParam, Name: "s", Value: "shortparam", ValueSecret: true})
	cb.Add(Arg{Type: ArgTypeLongParam, Name: "long", Value: "longparam", ValueSecret: true})
	assert.Equal(t, "testbin notsecret [SECRET] -s [SECRET] --long [SECRET]", cb.SafeString())

	cb = CommandBuilder{Name: "testbin"}
	cb.Add(Arg{Type: 99999, Value: "rawparam", ValueSecret: true})
	assert.Equal(t, "testbin [SECRET]", cb.SafeString())

	cb = CommandBuilder{Name: "helm"}
	cb.Add(Arg{Type: ArgTypeLongParam, Name: "set", Value: "servers[0].zones[0].zone=12.12.12.0/24 12.0.0.0/4 cluster.local"})
	assert.Equal(t, "helm --set 'servers[0].zones[0].zone=12.12.12.0/24 12.0.0.0/4 cluster.local'", cb.SafeString())
}

func TestShellQuote(t *testing.T) {
	assert.Equal(t, "simple", ShellQuote("simple"))
	assert.Equal(t, "'needs spaces'", ShellQuote("needs spaces"))
	assert.Equal(t, "'contains='", ShellQuote("contains="))
	assert.Equal(t, "'it'\"'\"'s quoted'", ShellQuote("it's quoted"))
}

func TestCommandBuilderCommand(t *testing.T) {
	cb := CommandBuilder{Name: "testbin"}
	cb.Add(Arg{Type: ArgTypeRaw, Value: "rawparam", ValueSecret: true})
	cb.Add(Arg{Type: ArgTypeShortParam, Name: "s", Value: "shortparam", ValueSecret: true})
	cb.Add(Arg{Type: ArgTypeLongParam, Name: "long", Value: "longparam", ValueSecret: true})
	cmd := cb.Command()
	assert.Equal(t, "testbin", cmd.Path)
	assert.Equal(t, []string{"testbin", "rawparam", "-s", "shortparam", "--long", "longparam"}, cmd.Args)

	cb = CommandBuilder{Name: "testbin"}
	cb.Add(Arg{Type: 99999, Value: "rawparam", ValueSecret: true})
	cmd = cb.Command()
	assert.Equal(t, "testbin", cmd.Path)
	assert.Equal(t, []string{"testbin", "rawparam"}, cmd.Args)
}
