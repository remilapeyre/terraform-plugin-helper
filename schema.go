package helper

import (
	"fmt"
	"go/format"

	"os"
	"reflect"
	"regexp"
	"strings"
)

func GenerateFiles(files []File) error {
	var known []string
	for _, f := range files {
		for _, obj := range f.DependsOn {
			known = append(known, reflect.ValueOf(obj).Type().Name())
		}
	}

	var schema []string

	for _, f := range files {
		schemaName, content, err := generateSchemaFile(f, known)
		if err != nil {
			return err
		}
		err = write(f.Name+".go", content)
		if err != nil {
			return err
		}

		schema = append(schema, fmt.Sprintf("%q: {Attributes: %s},", f.Name, schemaName))
	}

	return write("schema.go", fmt.Sprintf(`package schema

import "github.com/hashicorp/terraform-plugin-framework/tfsdk"


var (
	Schema = map[string]tfsdk.Schema{
		%s
	}
)
`, strings.Join(schema, "\n")))
}

func write(name, content string) error {
	p, err := format.Source([]byte(content))
	if err != nil {
		return err
	}

	w, err := os.Create(fmt.Sprintf("./internal/schema/%s", name))
	if err != nil {
		return err

	}
	defer w.Close()

	w.Write(p)
	return nil
}

func generateSchemaFile(f File, known []string) (string, string, error) {
	var b strings.Builder
	b.WriteString(`package schema

import (
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
`)

	name, err := generateSchema(f.Name, &b, reflect.ValueOf(f.Object), true, known)
	if err != nil {
		return "", "", err
	}

	for _, obj := range f.DependsOn {
		generateSchema(f.Name, &b, reflect.ValueOf(obj), false, known)
	}

	b.WriteString(")")

	p, err := format.Source([]byte(b.String()))

	return name, string(p), err
}

func generateSchema(filename string, b *strings.Builder, obj reflect.Value, exported bool, known knownSchema) (string, error) {
	name := obj.Type().Name()
	if !exported {
		name = camelCase(name)
	}

	b.WriteString(fmt.Sprintf("%s = map[string]tfsdk.Attribute{\n", name))

	var found bool
	for i := 0; i < obj.NumField(); i++ {
		field := obj.Type().Field(i)
		name := field.Tag.Get("terraform")
		if name == "" {
			continue
		}
		found = true

		b.WriteString(fmt.Sprintf("%q: {\n", name))

		err := generateType(filename, b, field.Type, known)
		if err != nil {
			return "", err
		}

		b.WriteString("Optional: true,\n")
		b.WriteString("},\n")
	}

	b.WriteString("}\n\n")

	if !found {
		return "", fmt.Errorf("%s: no attribute found for %s", filename, obj.Type().String())
	}

	return name, nil
}

func generateType(filename string, b *strings.Builder, typ reflect.Type, known knownSchema) error {
	if known.Contains(typ.Name()) {
		b.WriteString(fmt.Sprintf("Attributes: tfsdk.SingleNestedAttributes(%s),\n", camelCase(typ.Name())))
		return nil

	} else if typ.Kind() == reflect.Ptr && known.Contains(typ.Elem().Name()) {
		b.WriteString(fmt.Sprintf("Attributes: tfsdk.SingleNestedAttributes(%s),\n", camelCase(typ.Elem().Name())))
		return nil

	} else if typ.Kind() == reflect.Slice {
		if known.Contains(typ.Elem().Name()) {
			b.WriteString(fmt.Sprintf("Attributes: tfsdk.ListNestedAttributes(%s),\n", camelCase(typ.Elem().Name())))
			return nil
		} else if typ.Elem().Kind() == reflect.Ptr && known.Contains(typ.Elem().Elem().Name()) {
			b.WriteString(fmt.Sprintf("Attributes: tfsdk.ListNestedAttributes(%s),\n", camelCase(typ.Elem().Elem().Name())))
			return nil
		}
	}

	switch typ.String() {
	case "time.Duration", "time.Time":
		b.WriteString("Type: types.StringType,\n")
		return nil
	case "[]string":
		b.WriteString("Type: types.ListType{\nElemType: types.StringType,\n},\n")
		return nil
	case "map[string]string":
		b.WriteString("Type: types.MapType{\nElemType: types.StringType,\n},\n")
		return nil
	}

	switch kind := typ.Kind(); kind {
	case reflect.Bool:
		b.WriteString("Type: types.BoolType,\n")

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		b.WriteString("Type: types.NumberType,\n")

	case reflect.String:
		b.WriteString("Type: types.StringType,\n")

	default:
		return fmt.Errorf("%s: missing type for %q", filename, typ)
	}

	return nil
}

type File struct {
	Name      string
	Imports   []string
	Object    interface{}
	DependsOn []interface{}
}

type knownSchema []string

func (k knownSchema) Contains(typ string) bool {
	for _, v := range k {
		if typ == v {
			return true
		}
	}

	return false
}

var r = regexp.MustCompile("^([A-Z]+)?([A-Z])(.*)$")

func camelCase(s string) string {
	matches := r.FindStringSubmatch(s)
	if matches == nil {
		return s
	} else if matches[1] == "" {
		return strings.ToLower(matches[2]) + matches[3]
	}
	return strings.ToLower(matches[1]) + matches[2] + matches[3]
}
