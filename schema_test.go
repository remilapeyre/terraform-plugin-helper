package helper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_camelCase(t *testing.T) {
	tests := []struct {
		name string
		args string
		want string
	}{
		{
			name: "simple",
			args: "CamelCaseTest",
			want: "camelCaseTest",
		},
		{
			name: "acronym",
			args: "ACLCamelCaseTest",
			want: "aclCamelCaseTest",
		},
		{
			name: "already-done",
			args: "aclCamelCaseTest",
			want: "aclCamelCaseTest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := camelCase(tt.args)
			require.Equal(t, tt.want, got)
		})
	}
}

type aliasedType string

type TestPrimitive struct {
	Bool        bool              `terraform:"bool"`
	Int         int               `terraform:"int"`
	UInt        uint              `terraform:"uint"`
	UInt64      uint64            `terraform:"uint64"`
	String      string            `terraform:"string"`
	Aliased     aliasedType       `terraform:"aliased_type"`
	Time        time.Time         `terraform:"time"`
	Duration    time.Duration     `terraform:"duration"`
	StringSlice []string          `terraform:"string_slice"`
	StringMap   map[string]string `terraform:"string_map"`
}

func Test_generateFile(t *testing.T) {
	tests := []struct {
		name          string
		obj           interface{}
		want, content string
	}{
		{
			name: "",
			obj:  TestPrimitive{},
			want: "TestPrimitive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := File{
				Name:   "test-file",
				Object: tt.obj,
			}
			name, content, err := generateSchemaFile(f, nil)
			require.NoError(t, err)
			require.Equal(t, tt.want, name)
			require.Equal(t, tt.content, content)
		})
	}
}
