// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	govppApiImportPath = "git.fd.io/govpp.git/api" // import path of the govpp API package
	inputFileExt       = ".api.json"               // file extension of the VPP binary API files
	outputFileExt      = ".ba.go"                  // file extension of the Go generated files
)

// context is a structure storing data for code generation
type context struct {
	inputFile  string // input file with VPP API in JSON
	outputFile string // output file with generated Go package

	inputData []byte // contents of the input file

	moduleName  string // name of the source VPP module
	packageName string // name of the Go package being generated

	packageData *Package // parsed package data
}

// getContext returns context details of the code generation task
func getContext(inputFile, outputDir string) (*context, error) {
	if !strings.HasSuffix(inputFile, inputFileExt) {
		return nil, fmt.Errorf("invalid input file name: %q", inputFile)
	}

	ctx := &context{
		inputFile: inputFile,
	}

	// package name
	inputFileName := filepath.Base(inputFile)
	ctx.moduleName = inputFileName[:strings.Index(inputFileName, ".")]

	// alter package names for modules that are reserved keywords in Go
	switch ctx.moduleName {
	case "interface":
		ctx.packageName = "interfaces"
	case "map":
		ctx.packageName = "maps"
	default:
		ctx.packageName = ctx.moduleName
	}

	// output file
	packageDir := filepath.Join(outputDir, ctx.packageName)
	outputFileName := ctx.packageName + outputFileExt
	ctx.outputFile = filepath.Join(packageDir, outputFileName)

	return ctx, nil
}

// generatePackage generates code for the parsed package data and writes it into w
func generatePackage(ctx *context, w *bufio.Writer) error {
	logf("generating package %q", ctx.packageName)

	// generate file header
	generateHeader(ctx, w)
	generateImports(ctx, w)

	if *includeAPIVer {
		const APIVerConstName = "VlAPIVersion"
		fmt.Fprintf(w, "// %s represents version of the binary API module.\n", APIVerConstName)
		fmt.Fprintf(w, "const %s = %v\n", APIVerConstName, ctx.packageData.APIVersion)
		fmt.Fprintln(w)
	}

	// generate services
	if len(ctx.packageData.Services) > 0 {
		generateServices(ctx, w, ctx.packageData.Services)
	}

	// TODO: generate implementation for Services interface

	// generate enums
	if len(ctx.packageData.Enums) > 0 {
		fmt.Fprintf(w, "/* Enums */\n\n")

		for _, enum := range ctx.packageData.Enums {
			generateEnum(ctx, w, &enum)
		}
	}

	// generate aliases
	if len(ctx.packageData.Aliases) > 0 {
		fmt.Fprintf(w, "/* Aliases */\n\n")

		for _, alias := range ctx.packageData.Aliases {
			generateAlias(ctx, w, &alias)
		}
	}

	// generate types
	if len(ctx.packageData.Types) > 0 {
		fmt.Fprintf(w, "/* Types */\n\n")

		for _, typ := range ctx.packageData.Types {
			generateType(ctx, w, &typ)
		}
	}

	// generate unions
	if len(ctx.packageData.Unions) > 0 {
		fmt.Fprintf(w, "/* Unions */\n\n")

		for _, union := range ctx.packageData.Unions {
			generateUnion(ctx, w, &union)
		}
	}

	// generate messages
	if len(ctx.packageData.Messages) > 0 {
		fmt.Fprintf(w, "/* Messages */\n\n")

		for _, msg := range ctx.packageData.Messages {
			generateMessage(ctx, w, &msg)
		}
	}

	// generate message registrations
	fmt.Fprintln(w)
	fmt.Fprintln(w, "func init() {")
	for _, msg := range ctx.packageData.Messages {
		name := camelCaseName(msg.Name)
		fmt.Fprintf(w, "\tapi.RegisterMessage((*%s)(nil), \"%s\")\n", name, ctx.moduleName+"."+name)
	}
	fmt.Fprintln(w, "}")

	// flush the data:
	if err := w.Flush(); err != nil {
		return fmt.Errorf("flushing data to %s failed: %v", ctx.outputFile, err)
	}

	return nil
}

// generateHeader writes generated package header into w
func generateHeader(ctx *context, w io.Writer) {
	fmt.Fprintln(w, "// Code generated by GoVPP binapi-generator. DO NOT EDIT.")
	fmt.Fprintf(w, "//  source: %s\n", ctx.inputFile)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "/*")
	fmt.Fprintf(w, " Package %s is a generated from VPP binary API module '%s'.\n", ctx.packageName, ctx.moduleName)
	fmt.Fprintln(w)
	fmt.Fprintln(w, " It contains following objects:")
	var printObjNum = func(obj string, num int) {
		if num > 0 {
			if num > 1 {
				if strings.HasSuffix(obj, "s") {
					obj += "es"
				} else {
					obj += "s"
				}
			}
			fmt.Fprintf(w, "\t%3d %s\n", num, obj)
		}
	}
	printObjNum("message", len(ctx.packageData.Messages))
	printObjNum("type", len(ctx.packageData.Types))
	printObjNum("alias", len(ctx.packageData.Aliases))
	printObjNum("enum", len(ctx.packageData.Enums))
	printObjNum("union", len(ctx.packageData.Unions))
	printObjNum("service", len(ctx.packageData.Services))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "*/")
	fmt.Fprintf(w, "package %s\n", ctx.packageName)
	fmt.Fprintln(w)
}

// generateImports writes generated package imports into w
func generateImports(ctx *context, w io.Writer) {
	fmt.Fprintf(w, "import \"%s\"\n", govppApiImportPath)
	fmt.Fprintf(w, "import \"%s\"\n", "github.com/lunixbochs/struc")
	fmt.Fprintf(w, "import \"%s\"\n", "bytes")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "// Reference imports to suppress errors if they are not otherwise used.\n")
	fmt.Fprintf(w, "var _ = api.RegisterMessage\n")
	fmt.Fprintf(w, "var _ = struc.Pack\n")
	fmt.Fprintf(w, "var _ = bytes.NewBuffer\n")
	fmt.Fprintln(w)
}

// generateComment writes generated comment for the object into w
func generateComment(ctx *context, w io.Writer, goName string, vppName string, objKind string) {
	if objKind == "service" {
		fmt.Fprintf(w, "// %s represents VPP binary API services:\n", goName)
	} else {
		fmt.Fprintf(w, "// %s represents VPP binary API %s '%s':\n", goName, objKind, vppName)
	}

	var isNotSpace = func(r rune) bool {
		return !unicode.IsSpace(r)
	}

	// print out the source of the generated object
	mapType := false
	objFound := false
	objTitle := fmt.Sprintf(`"%s",`, vppName)
	switch objKind {
	case "alias", "service":
		objTitle = fmt.Sprintf(`"%s": {`, vppName)
		mapType = true
	}

	inputBuff := bytes.NewBuffer(ctx.inputData)
	inputLine := 0

	var trimIndent string
	var indent int
	for {
		line, err := inputBuff.ReadString('\n')
		if err != nil {
			break
		}
		inputLine++

		noSpaceAt := strings.IndexFunc(line, isNotSpace)
		if !objFound {
			indent = strings.Index(line, objTitle)
			if indent == -1 {
				continue
			}
			trimIndent = line[:indent]
			// If no other non-whitespace character then we are at the message header.
			if trimmed := strings.TrimSpace(line); trimmed == objTitle {
				objFound = true
				fmt.Fprintln(w, "//")
			}
		} else if noSpaceAt < indent {
			break // end of the definition in JSON for array types
		} else if objFound && mapType && noSpaceAt <= indent {
			fmt.Fprintf(w, "//\t%s", strings.TrimPrefix(line, trimIndent))
			break // end of the definition in JSON for map types (aliases, services)
		}
		fmt.Fprintf(w, "//\t%s", strings.TrimPrefix(line, trimIndent))
	}

	fmt.Fprintln(w, "//")
}

// generateServices writes generated code for the Services interface into w
func generateServices(ctx *context, w *bufio.Writer, services []Service) {
	// generate services comment
	generateComment(ctx, w, "Services", "services", "service")

	// generate interface
	fmt.Fprintf(w, "type %s interface {\n", "Services")
	for _, svc := range ctx.packageData.Services {
		generateService(ctx, w, &svc)
	}
	fmt.Fprintln(w, "}")

	fmt.Fprintln(w)
}

// generateService writes generated code for the service into w
func generateService(ctx *context, w io.Writer, svc *Service) {
	reqTyp := camelCaseName(svc.RequestType)

	// method name is same as parameter type name by default
	method := svc.MethodName()
	params := fmt.Sprintf("*%s", reqTyp)
	returns := "error"
	if replyType := camelCaseName(svc.ReplyType); replyType != "" {
		repTyp := fmt.Sprintf("*%s", replyType)
		if svc.Stream {
			repTyp = fmt.Sprintf("[]%s", repTyp)
		}
		returns = fmt.Sprintf("(%s, error)", repTyp)
	}

	fmt.Fprintf(w, "\t%s(%s) %s\n", method, params, returns)
}

// generateEnum writes generated code for the enum into w
func generateEnum(ctx *context, w io.Writer, enum *Enum) {
	name := camelCaseName(enum.Name)
	typ := binapiTypes[enum.Type]

	logf(" writing enum %q (%s) with %d entries", enum.Name, name, len(enum.Entries))

	// generate enum comment
	generateComment(ctx, w, name, enum.Name, "enum")

	// generate enum definition
	fmt.Fprintf(w, "type %s %s\n", name, typ)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "const (")

	// generate enum entries
	for _, entry := range enum.Entries {
		fmt.Fprintf(w, "\t%s %s = %v\n", entry.Name, name, entry.Value)
	}

	fmt.Fprintln(w, ")")

	fmt.Fprintln(w)
}

// generateAlias writes generated code for the alias into w
func generateAlias(ctx *context, w io.Writer, alias *Alias) {
	name := camelCaseName(alias.Name)

	logf(" writing type %q (%s), length: %d", alias.Name, name, alias.Length)

	// generate struct comment
	generateComment(ctx, w, name, alias.Name, "alias")

	// generate struct definition
	fmt.Fprintf(w, "type %s ", name)

	if alias.Length > 0 {
		fmt.Fprintf(w, "[%d]", alias.Length)
	}

	dataType := convertToGoType(ctx, alias.Type)
	fmt.Fprintf(w, "%s\n", dataType)

	fmt.Fprintln(w)
}

// generateUnion writes generated code for the union into w
func generateUnion(ctx *context, w io.Writer, union *Union) {
	name := camelCaseName(union.Name)

	logf(" writing union %q (%s) with %d fields", union.Name, name, len(union.Fields))

	// generate struct comment
	generateComment(ctx, w, name, union.Name, "union")

	// generate struct definition
	fmt.Fprintln(w, "type", name, "struct {")

	// maximum size for union
	maxSize := getUnionSize(ctx, union)

	// generate data field
	fieldName := "Union_data"
	fmt.Fprintf(w, "\t%s [%d]byte\n", fieldName, maxSize)

	// generate end of the struct
	fmt.Fprintln(w, "}")

	// generate name getter
	generateTypeNameGetter(w, name, union.Name)

	// generate CRC getter
	generateCrcGetter(w, name, union.CRC)

	// generate getters for fields
	for _, field := range union.Fields {
		fieldName := camelCaseName(field.Name)
		fieldType := convertToGoType(ctx, field.Type)
		generateUnionGetterSetter(w, name, fieldName, fieldType)
	}

	// generate union methods
	//generateUnionMethods(w, name)

	fmt.Fprintln(w)
}

// generateUnionMethods generates methods that implement struc.Custom
// interface to allow having Union_data field unexported
// TODO: do more testing when unions are actually used in some messages
func generateUnionMethods(w io.Writer, structName string) {
	// generate struc.Custom implementation for union
	fmt.Fprintf(w, `
func (u *%[1]s) Pack(p []byte, opt *struc.Options) (int, error) {
	var b = new(bytes.Buffer)
	if err := struc.PackWithOptions(b, u.union_data, opt); err != nil {
		return 0, err
	}
	copy(p, b.Bytes())
	return b.Len(), nil
}
func (u *%[1]s) Unpack(r io.Reader, length int, opt *struc.Options) error {
	return struc.UnpackWithOptions(r, u.union_data[:], opt)
}
func (u *%[1]s) Size(opt *struc.Options) int {
	return len(u.union_data)
}
func (u *%[1]s) String() string {
	return string(u.union_data[:])
}
`, structName)
}

func generateUnionGetterSetter(w io.Writer, structName string, getterField, getterStruct string) {
	fmt.Fprintf(w, `
func (u *%[1]s) Set%[2]s(a %[3]s) {
	var b = new(bytes.Buffer)
	if err := struc.Pack(b, &a); err != nil {
		return
	}
	copy(u.Union_data[:], b.Bytes())
}
func (u *%[1]s) Get%[2]s() (a %[3]s) {
	var b = bytes.NewReader(u.Union_data[:])
	struc.Unpack(b, &a)
	return
}
`, structName, getterField, getterStruct)
}

// generateType writes generated code for the type into w
func generateType(ctx *context, w io.Writer, typ *Type) {
	name := camelCaseName(typ.Name)

	logf(" writing type %q (%s) with %d fields", typ.Name, name, len(typ.Fields))

	// generate struct comment
	generateComment(ctx, w, name, typ.Name, "type")

	// generate struct definition
	fmt.Fprintf(w, "type %s struct {\n", name)

	// generate struct fields
	for i, field := range typ.Fields {
		// skip internal fields
		switch strings.ToLower(field.Name) {
		case "crc", "_vl_msg_id":
			continue
		}

		generateField(ctx, w, typ.Fields, i)
	}

	// generate end of the struct
	fmt.Fprintln(w, "}")

	// generate name getter
	generateTypeNameGetter(w, name, typ.Name)

	// generate CRC getter
	generateCrcGetter(w, name, typ.CRC)

	fmt.Fprintln(w)
}

// generateMessage writes generated code for the message into w
func generateMessage(ctx *context, w io.Writer, msg *Message) {
	name := camelCaseName(msg.Name)

	logf(" writing message %q (%s) with %d fields", msg.Name, name, len(msg.Fields))

	// generate struct comment
	generateComment(ctx, w, name, msg.Name, "message")

	// generate struct definition
	fmt.Fprintf(w, "type %s struct {", name)

	msgType := otherMessage
	wasClientIndex := false

	// generate struct fields
	n := 0
	for i, field := range msg.Fields {
		if i == 1 {
			if field.Name == "client_index" {
				// "client_index" as the second member,
				// this might be an event message or a request
				msgType = eventMessage
				wasClientIndex = true
			} else if field.Name == "context" {
				// reply needs "context" as the second member
				msgType = replyMessage
			}
		} else if i == 2 {
			if wasClientIndex && field.Name == "context" {
				// request needs "client_index" as the second member
				// and "context" as the third member
				msgType = requestMessage
			}
		}

		// skip internal fields
		switch strings.ToLower(field.Name) {
		case "crc", "_vl_msg_id":
			continue
		case "client_index", "context":
			if n == 0 {
				continue
			}
		}
		n++
		if n == 1 {
			fmt.Fprintln(w)
		}

		generateField(ctx, w, msg.Fields, i)
	}

	// generate end of the struct
	fmt.Fprintln(w, "}")

	// generate name getter
	generateMessageNameGetter(w, name, msg.Name)

	// generate CRC getter
	generateCrcGetter(w, name, msg.CRC)

	// generate message type getter method
	generateMessageTypeGetter(w, name, msgType)
}

// generateField writes generated code for the field into w
func generateField(ctx *context, w io.Writer, fields []Field, i int) {
	field := fields[i]

	fieldName := strings.TrimPrefix(field.Name, "_")
	fieldName = camelCaseName(fieldName)

	// generate length field for strings
	if field.Type == "string" {
		fmt.Fprintf(w, "\tXXX_%sLen uint32 `struc:\"sizeof=%s\"`\n", fieldName, fieldName)
	}

	dataType := convertToGoType(ctx, field.Type)

	fieldType := dataType
	if field.IsArray() {
		if dataType == "uint8" {
			dataType = "byte"
		}
		fieldType = "[]" + dataType
	}
	fmt.Fprintf(w, "\t%s %s", fieldName, fieldType)

	if field.Length > 0 {
		// fixed size array
		fmt.Fprintf(w, "\t`struc:\"[%d]%s\"`", field.Length, dataType)
	} else {
		for _, f := range fields {
			if f.SizeFrom == field.Name {
				// variable sized array
				sizeOfName := camelCaseName(f.Name)
				fmt.Fprintf(w, "\t`struc:\"sizeof=%s\"`", sizeOfName)
			}
		}
	}

	fmt.Fprintln(w)
}

// generateMessageNameGetter generates getter for original VPP message name into the provider writer
func generateMessageNameGetter(w io.Writer, structName, msgName string) {
	fmt.Fprintf(w, `func (*%s) GetMessageName() string {
	return %q
}
`, structName, msgName)
}

// generateTypeNameGetter generates getter for original VPP type name into the provider writer
func generateTypeNameGetter(w io.Writer, structName, msgName string) {
	fmt.Fprintf(w, `func (*%s) GetTypeName() string {
	return %q
}
`, structName, msgName)
}

// generateCrcGetter generates getter for CRC checksum of the message definition into the provider writer
func generateCrcGetter(w io.Writer, structName, crc string) {
	crc = strings.TrimPrefix(crc, "0x")
	fmt.Fprintf(w, `func (*%s) GetCrcString() string {
	return %q
}
`, structName, crc)
}

// generateMessageTypeGetter generates message factory for the generated message into the provider writer
func generateMessageTypeGetter(w io.Writer, structName string, msgType MessageType) {
	fmt.Fprintln(w, "func (*"+structName+") GetMessageType() api.MessageType {")
	if msgType == requestMessage {
		fmt.Fprintln(w, "\treturn api.RequestMessage")
	} else if msgType == replyMessage {
		fmt.Fprintln(w, "\treturn api.ReplyMessage")
	} else if msgType == eventMessage {
		fmt.Fprintln(w, "\treturn api.EventMessage")
	} else {
		fmt.Fprintln(w, "\treturn api.OtherMessage")
	}
	fmt.Fprintln(w, "}")
}
