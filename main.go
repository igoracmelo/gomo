package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
)

type param struct {
	name string
	kind string
}

const tmpl = `package {{ .Package }}

{{- if .Imports }}
import (
	{{- range .Imports }}
	{{ if .Alias }}{{ .Alias }}{{ end }} {{ .Path }}
	{{ end }}
)
{{ end -}}

type {{ .Name }}Mock struct {
    {{- range .Methods }}
    {{ .Name }}Func func({{ joinParams .Params }}) {{ if .Returns }}({{ joinParams .Returns }}){{ end }}
    {{- end }}
}

var _ {{ .Name }} = &{{ .Name }}Mock{}

{{ range .Methods }}
func (m *{{ $.Name }}Mock) {{ .Name }}({{ joinParams .Params }}){{ if .Returns }} ({{ joinParams .Returns }}){{ end }} {
    {{ if .Returns }}return {{ end }}m.{{ .Name }}Func({{ joinParamNames .Params }})
}
{{ end }}
`

func main() {
	var relPath string
	var target string
	flag.StringVar(&relPath, "f", "", "path to go file")
	flag.StringVar(&target, "i", "", "interface to mock")
	flag.Parse()

	if relPath == "" || target == "" {
		flag.Usage()
		os.Exit(1)
	}

	fset := token.NewFileSet()
	src, err := os.ReadFile(relPath)
	if err != nil {
		panic(err)
	}
	astFile, err := parser.ParseFile(fset, "", src, parser.AllErrors)
	if err != nil {
		panic(err)
	}

	type methodInfo struct {
		Name    string
		Params  []param
		Returns []param
	}

	type Info struct {
		Package  string
		Name     string
		MockName string
		Imports  []struct {
			Path  string
			Alias string
		}
		Methods []methodInfo
	}
	info := Info{}

	info.Package = astFile.Name.Name

	extractImport := func(info *Info, imp *ast.ImportSpec) {
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
			println(alias)
		}
		println(imp.Path.Value)

		info.Imports = append(info.Imports, struct {
			Path  string
			Alias string
		}{
			Path:  imp.Path.Value,
			Alias: alias,
		})
	}

	ast.Inspect(astFile, func(n ast.Node) bool {
		imp, ok := n.(*ast.ImportSpec)
		if ok {
			extractImport(&info, imp)
			return true
		}

		typ, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		if typ.Name.Name != target {
			return true
		}
		info.Name = typ.Name.Name
		info.MockName = info.Name + "Mock"
		// fmt.Println("type", typ.Name.Name, "interface {")

		iface, ok := typ.Type.(*ast.InterfaceType)
		if !ok {
			return true
		}
		for _, meth := range iface.Methods.List {
			if len(meth.Names) == 0 {
				continue
			}
			methName := meth.Names[0].Name
			params := []param{}
			returns := []param{}

			// interfaceInfo.methods = append(interfaceInfo.methods, methodInfo{
			// 	name: meth.Names[0].Name,
			// })
			fn, ok := meth.Type.(*ast.FuncType)
			if !ok {
				continue
			}
			for i, p := range fn.Params.List {
				var name string
				if len(p.Names) != 0 {
					name = p.Names[0].Name
				}
				name = argName(name, i)
				kind := string(src[p.Type.Pos()-1 : p.Type.End()-1])
				params = append(params, param{
					name: name,
					kind: kind,
				})
			}

			if fn.Results != nil {
				for i, r := range fn.Results.List {
					var name string
					if len(r.Names) != 0 {
						name = r.Names[0].Name
					}
					name = returnName(name, i)
					kind := string(src[r.Type.Pos()-1 : r.Type.End()-1])
					returns = append(returns, param{
						name: name,
						kind: kind,
					})
				}
			}

			info.Methods = append(info.Methods, methodInfo{
				Name:    methName,
				Params:  params,
				Returns: returns,
			})
		}

		// 			if ; ok {
		// 				for _, param := range fn.Params.List {
		// 					param.(*ast.)
		// 					for _, pname := range param.Names {
		// 						fmt.Println(pname.Name)
		// 					}
		// 				}
		// 			}
		// 		}
		// 	}
		// }
		return true
	})

	t, err := template.New("").Funcs(template.FuncMap{
		"joinParams":     joinParams,
		"joinParamNames": joinParamNames,
	}).Parse(tmpl)
	if err != nil {
		panic(err)
	}

	name := filepath.Base(relPath)
	dir := filepath.Dir(relPath)
	name = strings.Replace(name, ".go", "_mock.go", 1)
	filename := filepath.Join(dir, name)

	buf := &bytes.Buffer{}
	err = t.Execute(buf, info)
	if err != nil {
		panic(err)
	}

	b, err := imports.Process(name, buf.Bytes(), nil)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(filename, b, 0666)
	if err != nil {
		panic(err)
	}
}

func argName(name string, index int) string {
	if name == "" {
		return fmt.Sprintf("a%d", index)
	}
	return name
}

func returnName(name string, index int) string {
	if name == "" {
		return fmt.Sprintf("r%d", index)
	}
	return name
}

func joinParams(params []param) string {
	parts := make([]string, len(params))
	for i, param := range params {
		parts[i] = fmt.Sprintf("%s %s", param.name, param.kind)
	}
	return strings.Join(parts, ", ")
}

func joinParamNames(params []param) string {
	parts := make([]string, len(params))
	for i, param := range params {
		parts[i] = param.name
	}
	return strings.Join(parts, ", ")
}
