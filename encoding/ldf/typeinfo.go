package ldf

import (
	"reflect"
	"strings"
	"sync"
)

type typeInfo struct {
	fields []fieldInfo
}

type fieldInfo struct {
	name      string
	ignore    bool
	omitEmpty bool
	raw       bool
}

var tinfoMap sync.Map

// Pretty much just nabbed this from the native encoding/xml package
func getTypeInfo(t reflect.Type) *typeInfo {
	if info, ok := tinfoMap.Load(t); ok {
		return info.(*typeInfo)
	}

	info := &typeInfo{}

	if t.Kind() == reflect.Struct {
		n := t.NumField()
		for i := 0; i < n; i++ {
			f := t.Field(i)
			tag := f.Tag.Get("ldf")

			name, options, _ := strings.Cut(tag, ",")
			if len(name) == 0 {
				name = f.Name
			}

			fieldInfo := fieldInfo{
				name:   name,
				ignore: !f.IsExported() || tag == "-",
			}
			if !fieldInfo.ignore {
				for len(options) > 0 {
					var option string
					option, options, _ = strings.Cut(options, ",")

					switch option {
					case "omitempty":
						fieldInfo.omitEmpty = true
					case "raw":
						fieldInfo.raw = true
					}
				}
			}

			info.fields = append(info.fields, fieldInfo)
		}
	}

	tinfoMap.Store(t, info)
	return info
}
