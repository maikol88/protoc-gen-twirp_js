package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"regexp"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	"github.com/maikol88/protoc-gen-twirp_js/internal/gen"
	"github.com/maikol88/protoc-gen-twirp_js/internal/gen/stringutils"
	"github.com/maikol88/protoc-gen-twirp_js/internal/gen/typemap"
)

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println(gen.Version)
		os.Exit(0)
	}

	g := newGenerator()
	gen.Main(g)
}

func newGenerator() *generator {
	return &generator{output: new(bytes.Buffer)}
}

type generator struct {
	reg    *typemap.Registry
	output *bytes.Buffer
}

func (g *generator) Generate(in *plugin.CodeGeneratorRequest) *plugin.CodeGeneratorResponse {
	genFiles := gen.FilesToGenerate(in)
	g.reg = typemap.New(in.ProtoFile)

	resp := new(plugin.CodeGeneratorResponse)
	for _, f := range genFiles {
		respFile := g.generateFile(f)
		if respFile != nil {
			resp.File = append(resp.File, respFile)
		}
	}

	return resp
}
func (g *generator) generateFile(file *descriptor.FileDescriptorProto) *plugin.CodeGeneratorResponse_File {
	g.P(`/**`)
	g.P(" * Code generated by protoc-gen-twirp_js ", gen.Version, ", DO NOT EDIT.")
	g.P(" * source: ", file.GetName())
	g.P(" */")
	g.P(`// import our twirp js library dependency`)
	g.P(`var createClient = require("twirp");`)
	g.P(`// import our protobuf definitions`)
	match, _ := regexp.FindString("[^\/]+$", baseFileName(file))
	g.P(`var pb = require(`, strconv.Quote("./"+ match +".js"), `);`)
	g.P(`Object.assign(module.exports, pb);`)
	g.P()
	for _, service := range file.Service {
		g.generateProtobufClient(file, service)
	}
	resp := new(plugin.CodeGeneratorResponse_File)
	resp.Name = proto.String(baseFileName(file) + "_twirp.js")
	resp.Content = proto.String(g.output.String())
	g.output.Reset()

	return resp
}

func (g *generator) generateProtobufClient(file *descriptor.FileDescriptorProto, service *descriptor.ServiceDescriptorProto) {
	cName := clientName(service)
	comments, err := g.reg.ServiceComments(file, service)
	g.P(`/**`)
	if err == nil && comments.Leading != "" {
		g.printComments(comments, ` * `)
	} else {
		g.P(` * Creates a new `, cName)
	}
	g.P(` */`)
	g.P(`module.exports.create`, clientName(service), ` = function(baseurl, extraHeaders, useJSON) {`)
	g.P(`    var rpc = createClient(baseurl, `, strconv.Quote(fullServiceName(file, service)), `, `, strconv.Quote(gen.Version), `,  useJSON, extraHeaders === undefined ? {} : extraHeaders);`)
	g.P(`    return {`)
	// for each method...
	l := len(service.Method)
	for i, method := range service.Method {
		methName := methodName(method)
		jsMethodName := strings.ToLower(methName[0:1]) + methName[1:]
		_, outputName := methodTypesNames(method)
		// we need field definitions for each field
		// then we don't have to rely on

		comments, err := g.reg.MethodComments(file, service, method)
		if err == nil && comments.Leading != "" {
			g.P(`        /**`)
			g.printComments(comments, `         * `)
			g.P(`         */`)
		}
		trailingComma := ","
		if i == l-1 {
			trailingComma = ""
		}
		g.P(
			`        `,
			jsMethodName,
			`: function(data) { return rpc(`,
			strconv.Quote(methName),
			`, data, pb.`,
			outputName,
			`); }`,
			trailingComma,
		)
	}
	g.P(`    }`)
	g.P(`}`)
	g.P()
}

func (g *generator) P(args ...string) {
	for _, v := range args {
		g.output.WriteString(v)
	}
	g.output.WriteByte('\n')
}

func (g *generator) printComments(comments typemap.DefinitionComments, prefix string) {
	text := strings.TrimSuffix(comments.Leading, "\n")
	for _, line := range strings.Split(text, "\n") {
		g.P(prefix, strings.TrimPrefix(line, " "))
	}
}

func serviceName(service *descriptor.ServiceDescriptorProto) string {
	return stringutils.CamelCase(service.GetName())
}

func clientName(service *descriptor.ServiceDescriptorProto) string {
	return serviceName(service) + "Client"
}

func fullServiceName(file *descriptor.FileDescriptorProto, service *descriptor.ServiceDescriptorProto) string {
	name := serviceName(service)
	if pkg := file.GetPackage(); pkg != "" {
		name = pkg + "." + name
	}
	return name
}

// lowerCamelCase
func methodName(method *descriptor.MethodDescriptorProto) string {
	return stringutils.CamelCase(method.GetName())
}

func methodTypesNames(meth *descriptor.MethodDescriptorProto) (string, string) {
	in := strings.Split(meth.GetInputType(), ".")
	out := strings.Split(meth.GetOutputType(), ".")
	return in[len(in)-1], out[len(out)-1]
}

func baseFileName(f *descriptor.FileDescriptorProto) string {
	name := *f.Name
	if ext := path.Ext(name); ext == ".proto" || ext == ".protodevel" {
		name = name[:len(name)-len(ext)]
	}
	return name + "_pb"
}
