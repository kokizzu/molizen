package generator

import (
	"fmt"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/sanposhiho/mock/mockgen/model"
)

const (
	molizenActorImportPath  = "github.com/sanposhiho/molizen/actor"
	molizenFutureImportPath = "github.com/sanposhiho/molizen/future"
)

func (g *Generator) Generate(pkg *model.Package, outputPkgName string, outputPkgPath string) error {
	g.GenerateHeader()

	if err := g.GenerateImport(pkg, outputPkgName, outputPkgPath); err != nil {
		return fmt.Errorf("generate import: %w", err)
	}

	for _, intf := range pkg.Interfaces {
		originalName := intf.Name
		actorName := intf.Name + "Actor"

		g.GenerateActorStruct(actorName, originalName, pkg.Name)
		g.GenerateNewFunction(actorName, originalName, pkg.Name)
		g.GenerateMockMethods(actorName, intf, outputPkgPath)
	}

	return nil
}

func (g *Generator) GenerateHeader() {
	if g.CopyrightHeader != "" {
		lines := strings.Split(g.CopyrightHeader, "\n")
		for _, line := range lines {
			g.p("// %s", line)
		}
		g.p("")
	}

	g.p("// Code generated by Molizen. DO NOT EDIT.")
	g.p("")

	return
}

func (g *Generator) GenerateImport(pkg *model.Package, outputPkgName string, outputPackagePath string) error {
	im := pkg.Imports()

	im[pkg.PkgPath] = true
	im[molizenActorImportPath] = true
	im[molizenFutureImportPath] = true
	im["sync"] = true
	im["context"] = true

	// Sort keys to make import alias generation predictable
	sortedPaths := make([]string, len(im))
	x := 0
	for pth := range im {
		sortedPaths[x] = pth
		x++
	}
	sort.Strings(sortedPaths)

	packagesName, err := createPackageMap(sortedPaths)
	if err != nil {
		return fmt.Errorf("create package map: %w", err)
	}

	g.packageMap = make(map[string]string, len(im))
	localNames := make(map[string]bool, len(im))
	for _, pth := range sortedPaths {
		base, ok := packagesName[pth]
		if !ok {
			base = Sanitize(path.Base(pth))
		}

		// Local names for an imported package can usually be the basename of the import path.
		// A couple of situations don't permit that, such as duplicate local names
		// (e.g. importing "html/template" and "text/template"), or where the basename is
		// a keyword (e.g. "foo/case").
		// try base0, base1, ...
		pkgName := base
		i := 0
		for localNames[pkgName] || token.Lookup(pkgName).IsKeyword() {
			pkgName = base + strconv.Itoa(i)
			i++
		}

		// Avoid importing package if source pkg == output pkg
		if pth == pkg.PkgPath && outputPackagePath == pkg.PkgPath {
			continue
		}

		g.packageMap[pth] = pkgName
		localNames[pkgName] = true
	}

	g.p("// Package %v is a generated Molizen package.", outputPkgName)

	g.p("package %v", outputPkgName)
	g.p("")
	g.p("import (")
	g.in()
	for pkgPath, pkgName := range g.packageMap {
		if pkgPath == outputPackagePath {
			continue
		}
		g.p("%v %q", pkgName, pkgPath)
	}
	for _, pkgPath := range pkg.DotImports {
		g.p(". %q", pkgPath)
	}
	g.out()
	g.p(")")

	return nil
}

func (g *Generator) GenerateActorStruct(actorName, originalName, originalPkgName string) {
	g.p("")
	g.p("// %v is a actor of %v interface.", actorName, originalName)
	g.p("type %v struct {", actorName)
	g.in()
	g.p("lock     sync.Mutex")
	g.p("internal %v.%v", originalPkgName, originalName)
	g.out()
	g.p("}")
	g.p("")
}

func (g *Generator) GenerateMockMethods(mockType string, intf *model.Interface, outputPkgPath string) {
	sort.Slice(intf.Methods, func(i, j int) bool { return intf.Methods[i].Name < intf.Methods[j].Name })

	for _, m := range intf.Methods {
		g.p("")
		g.GenerateMethod(mockType, m, outputPkgPath)
		g.p("")
	}
}

func (g *Generator) GenerateNewFunction(actorName, originalName, originalPkgName string) {
	g.p("")
	g.p("func New(internal %v.%v) *%v {", originalPkgName, originalName, actorName)
	g.in()
	g.p("return &%v{", actorName)
	g.p("internal: internal,")
	g.in()
	g.out()
	g.p("}")
	g.out()
	g.p("}")
	g.p("")
}

func (g *Generator) GenerateMethod(mockType string, m *model.Method, outputPkgPath string) {
	receiverName := "a"
	argNames := g.getArgNames(m)
	argString := makeArgString(argNames, g.getArgTypes(m, outputPkgPath))
	resultType := m.Name + "Result"

	g.p("// %v is the result type for %v.", resultType, m.Name)
	g.p("type %v struct {", m.Name+"Result")
	g.in()

	rets := make([]string, len(m.Out))
	resultVars := make([]string, len(m.Out))
	for i, p := range m.Out {
		resultVars[i] = "ret" + strconv.Itoa(i)
		g.p("%v %v", "Ret"+strconv.Itoa(i), p.Type.String(g.packageMap, outputPkgPath))
		rets[i] = p.Type.String(g.packageMap, outputPkgPath)
	}
	g.out()
	g.p("}")
	g.p("")
	retString := strings.Join(rets, ", ")
	if len(rets) > 1 {
		retString = "(" + retString + ")"
	}
	if retString != "" {
		retString = " " + retString
	}

	g.p("// %v actor base method.", m.Name)
	g.p("func (%v *%v) %v(%v) future.Future[%v] {", receiverName, mockType, m.Name, argString, resultType)
	g.in()
	g.p("ctx.UnlockParent()")
	g.p("newctx := ctx.NewChildContext(a, %v.lock.Lock, %v.lock.Unlock)", receiverName, receiverName)
	g.p("")
	g.p("f := future.New[%v]()", resultType)
	g.p("go func(){")
	g.in()
	g.p("%s.lock.Lock()", receiverName)
	g.p("defer %s.lock.Unlock()", receiverName)
	g.p("")
	if len(resultVars) > 0 {
		g.p("%v := %s.internal.%v(newctx, %v)", strings.Join(resultVars, ", "), receiverName, m.Name, strings.Join(argNames[1:], ", "))
	} else {
		g.p("%s.internal.%v(newctx, %v)", receiverName, m.Name, strings.Join(argNames[1:], ", "))
	}
	g.p("")
	g.p("ret := %v{", resultType)
	g.in()
	for i, _ := range m.Out {
		g.p("%v: %v,", "Ret"+strconv.Itoa(i), "ret"+strconv.Itoa(i))
	}
	g.out()
	g.p("}")
	g.p("")
	g.p("ctx.LockParent()")
	g.p("")
	g.p("f.Send(ret)")
	g.out()
	g.p("}()")
	g.p("")
	g.p("return f")
	g.out()
	g.p("}")
	g.p("")

	return
}
