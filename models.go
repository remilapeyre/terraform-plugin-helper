package helper

import (
	"bytes"
	"fmt"
	"go/format"
	"reflect"
	"text/template"
)

type Content struct {
	Imports []string
	Models  []modelDefinition
}

type modelDefinition struct {
	Name        string
	PrivateName string
	PkgName     string
	Fields      []field
}

type field struct {
	Name       string
	Type       string
	Annotation string
	Setter     string
	Getter     string
}

func GenerateModels(f File, known []string) (string, error) {
	models := []modelDefinition{
		getModelDefinition(f.Object),
	}

	for _, obj := range f.DependsOn {
		models = append(models, getModelDefinition(obj))
	}

	tmpl, err := template.New("models").Parse(modelsTemplate)
	if err != nil {
		return "", err
	}

	var result bytes.Buffer
	err = tmpl.Execute(&result, Content{
		Imports: f.Imports,
		Models:  models,
	})
	if err != nil {
		return "", err
	}

	p, err := format.Source(result.Bytes())
	if err != nil {
		return "", err
	}

	return string(p), nil
}

func getModelDefinition(obj interface{}) modelDefinition {
	val := reflect.ValueOf(obj)

	def := modelDefinition{
		Name:        val.Type().Name(),
		PrivateName: camelCase(val.Type().Name()),
		PkgName:     val.Type().String(),
		Fields:      []field{},
	}

	for i := 0; i < val.NumField(); i++ {
		f := val.Type().Field(i)
		name := f.Tag.Get("terraform")
		if name == "" {
			continue
		}

		typ := getType(f.Type)
		def.Fields = append(def.Fields, field{
			Name:       f.Name,
			Type:       typ,
			Annotation: fmt.Sprintf(`tfsdk:"%s"`, name),
			Setter:     setter(f.Type, "obj."+f.Name, typ, "value."+f.Name),
			Getter:     getter(f.Type, "target."+f.Name, typ, "model."+f.Name),
		})
	}

	return def
}

func getType(typ reflect.Type) string {

	switch typ.String() {
	case "time.Duration", "time.Time", "[]uint8":
		return "types.String"

	case "[]string":
		return "types.List"

	case "map[string]string":
		return "types.Map"

	}

	switch kind := typ.Kind(); kind {
	case reflect.Bool:
		return "types.Bool"

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		return "types.Number"

	case reflect.String:
		return "types.String"

	case reflect.Struct:
		return "*" + typ.Name()

	case reflect.Pointer:
		return "*" + typ.Elem().Name()
	}

	return ""
}

func setter(typ reflect.Type, target, tftyp, name string) string {
	switch typ.String() {
	case "time.Duration", "time.Time":
		return fmt.Sprintf("%s = %sValue(%s.String())", target, tftyp, name)
	case "[]uint8":
		return fmt.Sprintf("%s = %sValue(string(%s))", target, tftyp, name)

	}

	switch kind := typ.Kind(); kind {
	case reflect.Bool:
		return fmt.Sprintf("%s = %sValue(%s)", target, tftyp, name)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		return "types.Number"

	case reflect.String:
		return fmt.Sprintf("%s = %sValue(%s)", target, tftyp, name)

	case reflect.Struct:
		return fmt.Sprintf("%s = New%s(&%s)", target, typ.Name(), name)

	case reflect.Pointer:
		return fmt.Sprintf("%s = New%s(%s)", target, typ.Elem().Name(), name)
	}

	return fmt.Sprintf("%s = %sValue(%s)", target, tftyp, name)
}

func getter(typ reflect.Type, target, tftyp, name string) string {
	switch typ.String() {
	case "bool":
		return fmt.Sprintf(`
			if !%s.IsNull() {
				%s = %s.ValueBool()
			}`, name, target, name)

	case "time.Duration":
		return fmt.Sprintf(`
			if !%s.IsNull() {
				if dur, err := time.ParseDuration(%s.ValueString()); err == nil {
					%s = dur
				} else {
					diags.Append(diag.NewErrorDiagnostic("Failed to convert %s", err.Error()))
					return diags
				}
			}
			`, name, name, target, target)

	case "time.Time":
		return fmt.Sprintf(`
			if !%s.IsNull() {
				%s = %s.ValueString()
			}`, name, target, name)

	case "[]uint8":
		return fmt.Sprintf(`
			if !%s.IsNull() {
				%s = []byte(%s.ValueString())
			}`, name, target, name)

	}

	switch kind := typ.Kind(); kind {
	case reflect.Struct:
		return fmt.Sprintf(`
		if %s != nil {
			diags.Append(%sDecode(ctx, *%s, &%s)...)
		}`, name, camelCase(typ.Name()), name, target)

	case reflect.Pointer:
		return fmt.Sprintf(`
		if %s != nil {
			diags.Append(%sDecode(ctx, *%s, %s)...)
		}`, name, camelCase(typ.Elem().Name()), name, target)

	}

	return fmt.Sprintf(`
		if !%s.IsNull() {
			%s = %s.ValueString()
		}`, name, target, name)
}

const modelsTemplate = `package models

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
{{ range $import := .Imports -}}
	"{{$import}}"
{{ end }}
)

type Getter interface {
	Get(ctx context.Context, target interface{}) diag.Diagnostics
}

{{range $model := .Models}}
type {{$model.Name}} struct {
	{{range $field := $model.Fields}}
	{{$field.Name}} {{$field.Type}} ` + "`{{$field.Annotation}}`" + `
	{{- end -}}

}

func New{{$model.Name}}(value *{{$model.PkgName}}) *{{$model.Name}} {
	if value == nil {
		return nil
	}

	obj := {{$model.Name}}{}

	{{range $field := $model.Fields}}
	{{$field.Setter}}
	{{- end }}

	return &obj
}

func {{$model.Name}}Decode(ctx context.Context, conf Getter, target *{{$model.PkgName}}) diag.Diagnostics {
	var model {{$model.Name}}

	diags := conf.Get(ctx, &model)
	if diags.HasError() {
		return diags
	}

	return {{$model.PrivateName}}Decode(ctx, model, target)
}

func {{$model.PrivateName}}Decode(ctx context.Context, model {{$model.Name}}, target *{{$model.PkgName}}) (diags diag.Diagnostics) {
	{{range $field := $model.Fields }}
	{{- $field.Getter -}}
	{{- end }}

	return diags
}

{{end}}

`
