package generate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"github.com/go-openapi/spec"
	"github.com/gobuffalo/packr/v2"
	"github.com/gregdhill/go-openrpc/types"
	"github.com/gregdhill/go-openrpc/util"
	"github.com/imdario/mergo"
)

const (
	params    = "Params"
	result    = "Result"
	goExt     = "go"
	goTmplExt = goExt + "tmpl"
)

var ProgramName = "CHANGME"

func schemaAsJSONPretty(s spec.Schema) string {
	b, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		return ""
	}
	b = bytes.ReplaceAll(b, []byte("{"), []byte(""))
	b = bytes.ReplaceAll(b, []byte("}"), []byte(""))
	b = bytes.ReplaceAll(b, []byte(`"`), []byte(""))
	b = bytes.ReplaceAll(b, []byte(`$ref: #`), []byte(""))

	// Remove empty JSON $refs
	reg := regexp.MustCompile(`^\s*\$ref.*$/mg`)
	ss := reg.ReplaceAllString(string(b), "")

	return ss
}

func getConstraints(p spec.SchemaProps) string {
	s := ""
	fields := []string{"Format", "Maximum", "Minimum", "Pattern", "Enum"}
	e := reflect.ValueOf(&p).Elem()
	for i, f := range fields {
		fval := e.FieldByName(f).Interface()
		if f == "Enum" {
			if reflect.ValueOf(fval).Len() > 0 {
				s += fmt.Sprintf("%s:\"%v\"", util.LowerFirst(f), fval)
			}
			continue
		} else if fval == reflect.Zero(reflect.TypeOf(fval)).Interface() {
			continue
		}
		if i != 0 && s != "" {
			s += "; "
		}
		if f == "Pattern" {
			if val, ok := fval.(string); ok {
				val := strings.Replace(val, "|", "\\|", -1)
				s += fmt.Sprintf("%s:\"%v\"", util.LowerFirst(f), val)
			}
		} else if reflect.ValueOf(fval).Kind() == reflect.Ptr {
			val := reflect.Indirect(reflect.ValueOf(fval))
			s += fmt.Sprintf("%s:\"%v\"", util.LowerFirst(f), val)
		} else {
			s += fmt.Sprintf("%s:\"%v\"", util.LowerFirst(f), fval)
		}
	}
	return s
}

func messageExampleRequestAsJSONPretty(message string, ex types.ExamplePairing) string {
	params := make(map[string]interface{})
	for _, p := range ex.Params {
		params[p.Name] = p.Value
	}

	r := types.RequestJson{
		Id:      1,
		Message: message,
		Params:  params,
	}

	j, err := json.MarshalIndent(r, "", "    ")
	if err != nil {
		return ""
	}
	return string(j)
}

func messageExampleResponseAsJSONPretty(message string, ex types.ExamplePairing) string {
	r := types.ResponseJson{
		Id:      1,
		Message: message,
		Result:  ex.Result.Value,
	}

	j, err := json.MarshalIndent(r, "", "    ")
	if err != nil {
		return ""
	}
	return string(j)
}

func maybeLookupComponentsContentDescriptor(cmpnts *types.Components, cd *types.ContentDescriptor) (rootCD *types.ContentDescriptor, err error) {
	rootCD = cd
	if cd == nil || cmpnts == nil {
		return
	}
	if strings.Contains(cd.Schema.Ref.String(), "contentDescriptors") {
		r := filepath.Base(cd.Schema.Ref.String())
		rootCD = cmpnts.ContentDescriptors[r]
		return
	}
	return
}

func schemaHazRef(sch spec.Schema) bool {
	return sch.Ref.String() != ""
}

func derefSchemaRecurse(cts *types.Components, sch spec.Schema) spec.Schema {
	if schemaHazRef(sch) {
		sch = getSchemaFromRef(cts, sch.Ref)
		sch = derefSchemaRecurse(cts, sch)
	}
	for i := range sch.OneOf {
		got := derefSchemaRecurse(cts, sch.OneOf[i])
		if err := mergo.Merge(&got, sch.OneOf[i]); err != nil {
			panic(err.Error())
		}
		got.Schema = ""
		sch.OneOf[i] = got
	}
	for i := range sch.AnyOf {
		got := derefSchemaRecurse(cts, sch.AnyOf[i])
		if err := mergo.Merge(&got, sch.AnyOf[i]); err != nil {
			panic(err.Error())
		}
		got.Schema = ""
		sch.AnyOf[i] = got
	}
	for i := range sch.AllOf {
		got := derefSchemaRecurse(cts, sch.AllOf[i])
		if err := mergo.Merge(&got, sch.AllOf[i]); err != nil {
			panic(err.Error())
		}
		got.Schema = ""
		sch.AllOf[i] = got
	}
	for k, _ := range sch.Properties {
		got := derefSchemaRecurse(cts, sch.Properties[k])
		if err := mergo.Merge(&got, sch.Properties[k]); err != nil {
			panic(err.Error())
		}
		got.Schema = ""
		sch.Properties[k] = got
	}
	for k, _ := range sch.PatternProperties {
		got := derefSchemaRecurse(cts, sch.PatternProperties[k])
		if err := mergo.Merge(&got, sch.PatternProperties[k]); err != nil {
			panic(err.Error())
		}
		got.Schema = ""
		sch.PatternProperties[k] = got
	}
	if sch.Items == nil {
		return sch
	}
	if sch.Items.Len() > 1 {
		for i := range sch.Items.Schemas {
			got := derefSchemaRecurse(cts, sch.Items.Schemas[i])
			if err := mergo.Merge(&got, sch.Items.Schemas[i]); err != nil {
				panic(err.Error())
			}
			got.Schema = ""
			sch.Items.Schemas[i] = got
		}
	} else {
		// Is schema
		got := derefSchemaRecurse(cts, *sch.Items.Schema)
		if err := mergo.Merge(&got, sch.Items.Schema); err != nil {
			panic(err.Error())
		}
		got.Schema = ""
		sch.Items.Schema = &got
	}

	return sch
}

func getSchemaFromRef(cmpnts *types.Components, ref spec.Ref) (sch spec.Schema) {
	if cmpnts == nil || ref.String() == "" {
		return
	}
	r := filepath.Base(ref.String())
	sch = cmpnts.Schemas[r] // Trust parser
	return
}

func maybeMessageParams(message types.Message) string {
	if len(message.Params) > 0 {
		return fmt.Sprintf("%s%s", util.CamelCase(message.Name), params)
	}
	return ""
}

func maybeMessageResult(message types.Message) string {
	if message.Result != nil {
		return fmt.Sprintf("%s%s", util.CamelCase(message.Name), result)
	}
	return ""
}

func maybeMessageComment(message types.Message) string {
	if comment := util.FirstOf(message.Description, message.Summary); comment != "" {
		return fmt.Sprintf("// %s", comment)
	}
	return ""
}

func maybeFieldComment(desc string) string {
	if desc != "" {
		return fmt.Sprintf("// %s", desc)
	}
	return ""
}

func getProgramName() string {
	return ProgramName
}

type object struct {
	Name   string
	Fields *types.FieldMap
}

func getNestedParam(parentName string, parentSch spec.Schema) string {
	s := ""
	for name, sch := range parentSch.Properties {
		props := sch.SchemaProps
		ptype := ""
		if len(props.Type) > 0 {
			ptype = props.Type[0]
		}
		connector := parentName
		if !util.IsRequired(parentSch.Required, name) {
			connector += "?"
		}
		connector += "." + name
		s += "\n" + fmt.Sprintf("| %s | %s |%s | %s |", connector, ptype, getConstraints(props), sch.Description)
		s += getNestedParam(connector, sch)
	}
	if parentSch.Items == nil {
		return s
	}
	if parentSch.Items.Len() > 1 {
		for _, sv := range parentSch.Items.Schemas {
			connector := parentName + "[]"
			if sv.Items == nil {
				props := sv.SchemaProps
				ptype := ""
				if len(props.Type) > 0 {
					ptype = props.Type[0]
				}
				s += "\n" + fmt.Sprintf("| %s | %s |%s | %s |", connector, ptype, getConstraints(props), sv.Description)
			}
			s += getNestedParam(connector, sv)
		}
	} else {
		connector := parentName + "[]"
		if parentSch.Items.Schema.Items == nil {
			csch := *parentSch.Items.Schema
			props := csch.SchemaProps
			ptype := ""
			if len(props.Type) > 0 {
				ptype = props.Type[0]
			}
			s += "\n" + fmt.Sprintf("| %s | %s |%s | %s |", connector, ptype, getConstraints(props), csch.Description)
		}
		s += getNestedParam(connector, *parentSch.Items.Schema)
	}
	return s
}

func getNestedParams(cmpnts *types.Components, cd *types.ContentDescriptor) string {
	cd, err := maybeLookupComponentsContentDescriptor(cmpnts, cd)
	if err != nil {
		return ""
	}
	content := cd.Content
	sch := derefSchemaRecurse(cmpnts, content.Schema)
	props := sch.SchemaProps
	ptype := ""
	if len(props.Type) > 0 {
		ptype = props.Type[0]
	}
	s := fmt.Sprintf("| %s | %s | %s | %s |", content.Name, ptype, getConstraints(props), content.Description)
	s += getNestedParam(content.Name, sch)
	return s
}

func funcMap(gwmsgs *types.GwMsgSpec1) template.FuncMap {
	return template.FuncMap{
		"programName":                        getProgramName,
		"derefSchema":                        derefSchemaRecurse,
		"schemaHasRef":                       schemaHazRef,
		"schemaAsJSONPretty":                 schemaAsJSONPretty,
		"messageExampleRequestAsJSONPretty":  messageExampleRequestAsJSONPretty,
		"messageExampleResponseAsJSONPretty": messageExampleResponseAsJSONPretty,
		"getNestedParams":                    getNestedParams,
		"lookupContentDescriptor":            maybeLookupComponentsContentDescriptor,
		"sanitizeBackticks":                  util.SanitizeBackticks,
		"inspect":                            util.Inpect,
		"slice":                              util.Slice,
		"camelCase":                          util.CamelCase,
		"lowerFirst":                         util.LowerFirst,
		"maybeMessageComment":                maybeMessageComment,
		"maybeMessageParams":                 maybeMessageParams,
		"maybeMessageResult":                 maybeMessageResult,
		"maybeFieldComment":                  maybeFieldComment,
		"getObjects": func(om *types.ObjectMap) []object {
			keys := om.GetKeys()
			objects := make([]object, 0, len(keys))
			for _, k := range keys {
				objects = append(objects, object{k, om.Get(k)})
			}
			return objects
		},
		"getFields": func(fm *types.FieldMap) []types.BasicType {
			keys := fm.GetKeys()
			fields := make([]types.BasicType, 0, len(keys))
			for _, k := range keys {
				fields = append(fields, fm.Get(k))
			}
			return fields
		},
		"indent": func(spaces int, v string) string {
			pad := strings.Repeat(" ", spaces)
			return pad + strings.Replace(v, "\n", "\n"+pad, -1)
		},
	}
}

func WriteFile(box *packr.Box, name, pkg string, gwmsgs *types.GwMsgSpec1) error {
	data, err := box.Find(fmt.Sprintf("%s.%s", name, goTmplExt))
	if err != nil {
		return err
	}

	tmp, err := template.New(name).Funcs(funcMap(gwmsgs)).Parse(string(data))
	if err != nil {
		return err
	}

	tmpl := new(bytes.Buffer)
	err = tmp.Execute(tmpl, gwmsgs)
	if err != nil {
		return err
	}

	fset := new(token.FileSet)
	root, err := parser.ParseFile(fset, "", tmpl.Bytes(), parser.ParseComments)
	if err != nil {
		return err
	}
	ast.SortImports(fset, root)
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}

	root.Name.Name = path.Base(pkg)

	err = os.MkdirAll(pkg, os.ModePerm)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path.Join(pkg, fmt.Sprintf("%s.%s", name, goExt)), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	return cfg.Fprint(file, fset, root)
}

func WriteDocMd(box *packr.Box, name, pkg string, gwmsgs *types.GwMsgSpec1) error {
	data, err := box.Find(fmt.Sprintf("%s.%s", name, goTmplExt))
	if err != nil {
		return err
	}

	tmp, err := template.New(name).Funcs(funcMap(gwmsgs)).Parse(string(data))
	if err != nil {
		return err
	}

	tmpl := new(bytes.Buffer)
	err = tmp.Execute(tmpl, gwmsgs)
	if err != nil {
		return err
	}

	err = os.MkdirAll(pkg, os.ModePerm)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path.Join(pkg, fmt.Sprintf("%s.md", name)), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	//return cfg.Fprint(file, fset, root)
	_, err = file.Write(tmpl.Bytes())
	if err != nil {
		return err
	}
	return nil
}
