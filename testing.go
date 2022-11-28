package helper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"
)

const (
	testRecordEnvName = "TF_TEST_RECORD"
)

// TestSteps returns a list of resource.TestStep based on the content of the
// given directory.
func TestSteps(t *testing.T, dir string) []resource.TestStep {
	record, err := parseBool(os.Getenv(testRecordEnvName))
	require.NoError(t, err)

	fileInfo, err := os.ReadDir(dir)
	require.NoError(t, err)

	var files []string
	for _, fi := range fileInfo {
		if filepath.Ext(fi.Name()) == ".tf" {
			files = append(files, dir+fi.Name())
		}
	}

	require.NotZero(t, len(files), "no test file found in %s", dir)

	sort.Strings(files)

	var steps []resource.TestStep
	for _, path := range files {
		rawConfig, err := os.ReadFile(path)
		require.NoError(t, err)

		config := string(rawConfig)
		expectError := parseErrorMessage(t, config)

		path = strings.TrimSuffix(path, ".tf") + ".json"

		if !record {
			steps = append(steps, resource.TestStep{
				Config:      config,
				ExpectError: expectError,
				Check:       checkFunction(t, path),
			})

		} else {
			if expectError != nil {
				continue
			}

			steps = append(steps,
				[]resource.TestStep{
					{
						Config: config,
						Check:  recordCheckFunction(path),
					},
					{
						Destroy: true,
						Config:  config,
						Check:   recordCheckFunction(path),
					},
					{
						Config: config,
						Check:  diffAndRecordCheckFunction(path),
					},
				}...,
			)
		}
	}

	return steps
}

func parseBool(s string) (bool, error) {
	if s == "" {
		return false, nil
	}

	return strconv.ParseBool(s)
}

type state map[string]map[string]string

func readSavedState(path string) (state, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	state := state{}
	json.Unmarshal(content, &state)

	return state, nil
}

func checkFunction(t *testing.T, path string) func(*terraform.State) error {
	state, err := readSavedState(path)
	require.NoError(t, err)

	testFuncs := []resource.TestCheckFunc{}
	for name, attrs := range state {
		for attr, value := range attrs {
			var f resource.TestCheckFunc

			switch value {
			case "set()":
				f = resource.TestCheckResourceAttrSet(name, attr)
			default:
				f = resource.TestCheckResourceAttr(name, attr, value)
			}

			testFuncs = append(testFuncs, f)
		}
	}

	return resource.ComposeAggregateTestCheckFunc(testFuncs...)
}

func diffAndRecordCheckFunction(path string) func(s *terraform.State) error {
	return func(s *terraform.State) error {
		old, err := readSavedState(path)
		if err != nil {
			return err
		}

		new := state{}
		resources := s.RootModule().Resources
		for name, resource := range resources {
			oldResource := old[name]

			attrs := map[string]string{}
			for k, v := range resource.Primary.Attributes {
				if k == "%" {
					continue
				}

				if oldResource[k] == v {
					attrs[k] = v
				} else {
					attrs[k] = "set()"
				}
			}

			new[name] = attrs
		}

		return writeState(path, new)
	}
}

func recordCheckFunction(path string) func(s *terraform.State) error {
	return func(s *terraform.State) error {
		state := state{}

		resources := s.RootModule().Resources
		for name, resource := range resources {
			attrs := map[string]string{}
			for k, v := range resource.Primary.Attributes {
				if k == "%" {
					continue
				}
				attrs[k] = v
			}

			state[name] = attrs
		}

		return writeState(path, state)
	}
}

func writeState(path string, state state) error {
	content, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, content, 0644)
}

func getAnnotation(name, config string) string {
	lines := strings.Split(config, "\n")

	var re = regexp.MustCompile(`^\s*#\s*` + name + `:\s*(.*?)\s*$`)
	for _, line := range lines {
		line = strings.TrimSpace(line)

		match := re.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		return match[1]
	}

	return ""
}

func parseErrorMessage(t *testing.T, config string) *regexp.Regexp {
	str := getAnnotation("Error", config)
	if str == "" {
		return nil
	}

	re, err := regexp.Compile(str)
	require.NoError(t, err)

	return re
}
