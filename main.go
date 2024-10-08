package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"

	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"

	"github.com/golang/protobuf/proto"

	"github.com/golang/glog"

	"github.com/grpc-ecosystem/grpc-gateway/codegenerator"

	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway/descriptor"
)

func print(v ...interface{}) {
	os.Stderr.WriteString(fmt.Sprintf("\n%+v\n", v))
}

var (
	headerTemplate = template.Must(template.New("header").Parse(`// Code generated by protoc-gen-xorm. DO NOT EDIT.
// source: {{.GetName}}
package {{.GoPkg.Name}}

import (
	"strings"
	"database/sql/driver"
)

var _ = strings.Trim
var _ = driver.Bool
`))

	messageTemplate = template.Must(template.New("message").Parse(`
// FromDB implements xorm.Conversion.FromDB
func (x *{{.EnumName}}) FromDB(bytes []byte) error {
	values := {{.EnumName}}_value
	key := string(bytes)

	value := int32(0)
	if v, ok := values[key]; ok {
		value = v
	} else if v, ok := values["{{.Prefix}}"+"_"+key]; ok {
		value = v
	}

	*x = {{.EnumName}}(value)
	return nil
}

// ToDB implements xorm.Conversion.ToDB
func (x *{{.EnumName}}) ToDB() ([]byte, error) {
	name := {{.EnumName}}_name[int32(*x)]
	return []byte(strings.TrimPrefix(name, "{{.Prefix}}"+"_")), nil
}

// Value when parser where args
func (x {{.EnumName}}) Value() (driver.Value, error) {
	return {{.EnumName}}_name[int32(x)], nil
}
`))

	importPrefix = flag.String("import_prefix", "", "prefix to be added to go package paths for imported proto files")
)

func main() {
	flag.Parse()
	defer glog.Flush()

	reg := descriptor.NewRegistry()

	req, err := codegenerator.ParseRequest(os.Stdin)
	if err != nil {
		glog.Fatal(err)
	}

	reg.SetPrefix(*importPrefix)

	if err := reg.Load(req); err != nil {
		emitError(err)
		return
	}

	var targets []*descriptor.File
	for _, target := range req.FileToGenerate {
		f, err := reg.LookupFile(target)
		if err != nil {
			glog.Fatal("LookupFile error ", err)
		}
		targets = append(targets, f)
	}

	var files []*plugin.CodeGeneratorResponse_File

	for _, file := range targets {
		w := bytes.NewBuffer(nil)

		if err := headerTemplate.Execute(w, file); err != nil {
			glog.Fatal("Execute headerTemplate error ", err)
		}

		for _, e := range file.GetEnumType() {
			if err := messageTemplate.Execute(w, map[string]string{
				"EnumName": strcase.ToCamel(e.GetName()),
				"Prefix":   strcase.ToScreamingSnake(e.GetName()),
			}); err != nil {
				glog.Fatal(err)
			}
		}

		for _, msg := range file.Messages {
			if msg.Options != nil && msg.Options.GetMapEntry() {
				continue
			}

			name := *msg.Name
			if len(msg.Outers) > 0 {
				name = strings.Join(msg.Outers, "_") + "_" + name
			}

			for _, e := range msg.GetEnumType() {
				if err := messageTemplate.Execute(w, map[string]string{
					"EnumName": name + "_" + strcase.ToCamel(e.GetName()),
					"Prefix":   strcase.ToScreamingSnake(e.GetName()),
				}); err != nil {
					glog.Fatal("Execute messageTemplate error ", err)
				}
			}
		}

		name := file.GetName()
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		output := fmt.Sprintf("%s.pb.xorm.go", base)
		files = append(files, &plugin.CodeGeneratorResponse_File{
			Name:    proto.String(output),
			Content: proto.String(w.String()),
		})
	}

	emitFiles(files)
}

func emitFiles(out []*plugin.CodeGeneratorResponse_File) {
	emitResp(&plugin.CodeGeneratorResponse{File: out})
}

func emitError(err error) {
	emitResp(&plugin.CodeGeneratorResponse{Error: proto.String(err.Error())})
}

func emitResp(resp *plugin.CodeGeneratorResponse) {
	buf, err := proto.Marshal(resp)
	if err != nil {
		glog.Fatal(err)
	}
	if _, err := os.Stdout.Write(buf); err != nil {
		glog.Fatal(err)
	}
}
