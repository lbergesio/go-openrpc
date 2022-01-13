package parse

import (
	"fmt"
	"path"
	"reflect"

	"github.com/go-openapi/spec"
	"github.com/gregdhill/go-openrpc/types"
	"github.com/gregdhill/go-openrpc/util"
)

const (
	params = "Params"
	result = "Result"
)

func persistTitleAndDesc(prev, next spec.Schema) spec.Schema {
	next.Title = util.FirstOf(next.Title, prev.Title)
	next.Description = util.FirstOf(next.Description, prev.Description)
	return next
}

func persistFields(prev, next spec.Schema) spec.Schema {
	next.Title = util.FirstOf(path.Base(prev.Ref.String()), next.Title, prev.Title)
	next.Description = util.FirstOf(next.Description, prev.Description)
	if next.Items == nil {
		next.Items = prev.Items
	}
	return next
}

func resolveSchema(gwmsgs *types.GwMsgSpec1, sch spec.Schema) spec.Schema {
	doc, _, _ := sch.Ref.GetPointer().Get(gwmsgs)

	if s, ok := doc.(spec.Schema); ok {
		sch = persistFields(sch, s)
	} else if cd, ok := doc.(*types.ContentDescriptor); ok {
		sch = persistFields(sch, cd.Schema)
	}

	if sch.Ref.GetURL() != nil {
		return resolveSchema(gwmsgs, sch)
	}
	return sch
}

func getConcreteType(in string) string {
	switch in {
	case reflect.Bool.String(), "boolean":
		return reflect.Bool.String()
	case reflect.Uint.String(), "uint":
		return reflect.Uint.String()
	case reflect.Int.String(), "integer":
		return reflect.Int.String()
	default:
		return in
	}
}

func getObjectType(gwmsgs *types.GwMsgSpec1, sch spec.Schema) string {
	sch = resolveSchema(gwmsgs, sch)

	if len(sch.Properties) > 0 || len(sch.Type) < 1 {
		return util.CamelCase(sch.Title)
	}

	return getConcreteType(sch.Type[0])
}

func dealWithOwnObjectTypes(fieldName string, sch spec.Schema) types.BasicType {
	if sch.Title == "uint" {
		return types.BasicType{sch.Description, fieldName, getConcreteType(sch.Title)}
	}
	return types.BasicType{sch.Description, sch.Title, getConcreteType(sch.Type[0])}
}

func dereference(gwmsgs *types.GwMsgSpec1, name string, sch spec.Schema, om *types.ObjectMap) {
	// resolve all pointers
	fieldName := sch.Title
	sch = resolveSchema(gwmsgs, sch)

	if len(sch.Properties) > 0 {
		for key, value := range sch.Properties {
			value.Title = key
			dereference(gwmsgs, sch.Title, value, om)
		}
		om.Set(name, types.BasicType{sch.Description, fieldName, getObjectType(gwmsgs, sch)})
		return
	} else if len(sch.OneOf) > 0 {
		next := sch.OneOf[0]
		dereference(gwmsgs, sch.Title, next, om)
		om.Set(name, types.BasicType{sch.Description, fieldName, getObjectType(gwmsgs, resolveSchema(gwmsgs, next))})
		return
	} else if sch.Items != nil {
		if sch.Items.Schema != nil {
			dereference(gwmsgs, sch.Title, *sch.Items.Schema, om)
			//dereference(gwmsgs, name, persistTitleAndDesc(sch, *sch.Items.Schema), om)
			om.Set(name, types.BasicType{sch.Description, sch.Title, fmt.Sprintf("[]%s", getObjectType(gwmsgs, persistTitleAndDesc(sch, *sch.Items.Schema)))})
		} else if len(sch.Items.Schemas) > 0 {
			om.Set(name, types.BasicType{sch.Description, fieldName, "[]string"})
		}
		return
	}

	if len(sch.Type) == 0 {
		return
	}

	om.Set(name, dealWithOwnObjectTypes(fieldName, sch))
	return
}

// GetTypes constructs all possible type definitions from the spec
func GetTypes(gwmsgs *types.GwMsgSpec1, om *types.ObjectMap) {
	for _, m := range gwmsgs.Messages {
		name := fmt.Sprintf("%s%s", util.CamelCase(m.Name), params)
		for _, param := range m.Params {
			sch := param.Schema
			sch.Title = util.FirstOf(sch.Title, param.Name)
			sch.Description = util.FirstOf(sch.Description, param.Description)
			dereference(gwmsgs, name, sch, om)
		}
		if m.Result != nil {
			name = fmt.Sprintf("%s%s", util.CamelCase(m.Name), result)
			res := m.Result
			sch := res.Schema
			sch.Title = util.FirstOf(sch.Title, res.Name)
			sch.Description = util.FirstOf(sch.Description, res.Description)
			dereference(gwmsgs, name, sch, om)
		}
	}
}
