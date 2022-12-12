package helper

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

const expected = `
type hashicupsProviderModel struct {
    Host     types.String 'tfsdk:"host"'
    Username types.String 'tfsdk:"username"'
    Password types.String 'tfsdk:"password"'
}

`

func Test_generateModels(t *testing.T) {
	f := File{
		Name:      "test",
		Object:    TestPrimitive{},
		DependsOn: []interface{}{},
	}
	got, err := GenerateModels(f, nil)
	require.NoError(t, err)
	require.Equal(t, "", got)
}

func Test_generateModelsConfig(t *testing.T) {
	f := File{
		Imports: []string{"github.com/hashicorp/consul/api"},
		Name:    "test",
		Object:  api.Config{},
		DependsOn: []interface{}{
			api.HttpBasicAuth{},
			api.TLSConfig{},
		},
	}
	got, err := GenerateModels(f, nil)
	require.NoError(t, err)
	fmt.Print(got)
	// require.Equal(t, "", got)
}
