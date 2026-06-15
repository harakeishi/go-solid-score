package astutil

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/harakeishi/go-solid-score/model"
)

// ExtractDecls extracts struct, interface, and function declarations from a file.
func ExtractDecls(file *ast.File, fpath string, fset *token.FileSet, info *types.Info, structMap map[string]*model.StructInfo, ifaceMap map[string]*model.InterfaceInfo, pi *model.PackageInfo) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch t := ts.Type.(type) {
				case *ast.StructType:
					si := &model.StructInfo{
						Name: ts.Name.Name,
						File: fpath,
						Line: fset.Position(ts.Pos()).Line,
					}
					if t.Fields != nil {
						for _, field := range t.Fields.List {
							fi := ExtractFieldInfo(field, info)
							if fi.Name == "" && len(field.Names) == 0 {
								si.Embeddings = append(si.Embeddings, fi.TypeName)
							}
							si.Fields = append(si.Fields, fi)
						}
					}
					structMap[ts.Name.Name] = si

				case *ast.InterfaceType:
					ii := &model.InterfaceInfo{
						Name: ts.Name.Name,
						File: fpath,
						Line: fset.Position(ts.Pos()).Line,
					}
					if t.Methods != nil {
						for _, m := range t.Methods.List {
							if len(m.Names) > 0 {
								ii.Methods = append(ii.Methods, m.Names[0].Name)
							} else {
								typeName := ExprToString(m.Type)
								ii.Embeds = append(ii.Embeds, typeName)
							}
						}
					}
					ii.TotalMethods = len(ii.Methods)
					if info != nil {
						obj := info.ObjectOf(ts.Name)
						if obj != nil {
							if named, ok := obj.Type().(*types.Named); ok {
								if iface, ok := named.Underlying().(*types.Interface); ok {
									ii.TotalMethods = iface.NumMethods()
								}
							}
						}
					}
					ifaceMap[ts.Name.Name] = ii
				}
			}

		case *ast.FuncDecl:
			if d.Recv != nil {
				continue
			}
			fi := ExtractFuncInfo(d, fpath, fset, info)
			pi.Functions = append(pi.Functions, fi)
		}
	}
}

// ExtractFieldInfo extracts field information from an AST field.
func ExtractFieldInfo(field *ast.Field, info *types.Info) *model.FieldInfo {
	fi := &model.FieldInfo{
		TypeName: ExprToString(field.Type),
	}
	if len(field.Names) > 0 {
		fi.Name = field.Names[0].Name
	}
	if info != nil && field.Type != nil {
		tv := info.TypeOf(field.Type)
		if tv != nil {
			fi.IsIface = IsInterfaceType(tv)
			fi.IsFunc = IsFuncType(tv)
			fi.IsValue = IsValueType(tv)
		}
	}
	return fi
}

// ExtractMethods attaches methods to their receiver struct.
func ExtractMethods(file *ast.File, fpath string, fset *token.FileSet, info *types.Info, structMap map[string]*model.StructInfo) {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv == nil {
			continue
		}
		recvType := ReceiverTypeName(fd.Recv)
		si, ok := structMap[recvType]
		if !ok {
			continue
		}

		mi := &model.MethodInfo{
			Name:         fd.Name.Name,
			ReceiverType: recvType,
			File:         fpath,
			LineStart:    fset.Position(fd.Pos()).Line,
			LineEnd:      fset.Position(fd.End()).Line,
			IsExported:   fd.Name.IsExported(),
		}

		if fd.Type.Params != nil {
			mi.Params = ExtractParams(fd.Type.Params, info)
		}
		if fd.Type.Results != nil {
			mi.Returns = ExtractReturns(fd.Type.Results, info)
		}

		if fd.Body != nil {
			m := WalkBody(fd.Body, info, fset)
			mi.CyclomaticComplexity = m.Complexity + 1
			mi.AccessedFields = m.AccessedFields
			mi.CalledMethods = m.CalledMethods
			mi.HasPanic = m.HasPanic
			mi.HasUnconditionalPanic = m.HasUnconditionalPanic
			mi.TypeSwitchCount = m.TypeSwitchCount
			mi.TypeAssertCount = m.TypeAssertCount
			mi.ReflectUsageCount = m.ReflectUsageCount
			mi.StmtCount = m.StmtCount
			mi.IsNoop = IsNoopBody(fd.Body)
		} else {
			mi.IsNoop = true
		}

		si.Methods = append(si.Methods, mi)
	}
}

// ExtractFuncInfo extracts information from a package-level function.
func ExtractFuncInfo(fd *ast.FuncDecl, fpath string, fset *token.FileSet, info *types.Info) *model.FuncInfo {
	fi := &model.FuncInfo{
		Name:       fd.Name.Name,
		File:       fpath,
		LineStart:  fset.Position(fd.Pos()).Line,
		LineEnd:    fset.Position(fd.End()).Line,
		IsExported: fd.Name.IsExported(),
	}

	if fd.Type.Params != nil {
		fi.Params = ExtractParams(fd.Type.Params, info)
	}
	if fd.Type.Results != nil {
		fi.Returns = ExtractReturns(fd.Type.Results, info)
	}

	if fd.Body != nil {
		m := WalkBody(fd.Body, info, fset)
		fi.CyclomaticComplexity = m.Complexity + 1
		fi.TypeSwitchCount = m.TypeSwitchCount
		fi.TypeAssertCount = m.TypeAssertCount
		fi.ReflectUsageCount = m.ReflectUsageCount
		fi.StmtCount = m.StmtCount
	}

	return fi
}

// ExtractParams extracts parameter info from a field list.
func ExtractParams(fl *ast.FieldList, info *types.Info) []*model.ParamInfo {
	var params []*model.ParamInfo
	for _, f := range fl.List {
		typeName := ExprToString(f.Type)
		isIface := false
		isFunc := false
		isValue := false
		if info != nil && f.Type != nil {
			tv := info.TypeOf(f.Type)
			if tv != nil {
				isIface = IsInterfaceType(tv)
				isFunc = IsFuncType(tv)
				isValue = IsValueType(tv)
			}
		}
		if len(f.Names) == 0 {
			params = append(params, &model.ParamInfo{TypeName: typeName, IsIface: isIface, IsFunc: isFunc, IsValue: isValue})
		}
		for _, name := range f.Names {
			params = append(params, &model.ParamInfo{
				Name:     name.Name,
				TypeName: typeName,
				IsIface:  isIface,
				IsFunc:   isFunc,
				IsValue:  isValue,
			})
		}
	}
	return params
}

// ExtractReturns extracts return type info from a field list.
func ExtractReturns(fl *ast.FieldList, info *types.Info) []*model.ReturnInfo {
	var returns []*model.ReturnInfo
	for _, f := range fl.List {
		typeName := ExprToString(f.Type)
		isIface := false
		if info != nil && f.Type != nil {
			tv := info.TypeOf(f.Type)
			if tv != nil {
				isIface = IsInterfaceType(tv)
			}
		}
		count := len(f.Names)
		if count == 0 {
			count = 1
		}
		for range count {
			returns = append(returns, &model.ReturnInfo{
				TypeName: typeName,
				IsIface:  isIface,
			})
		}
	}
	return returns
}

// CountImplementors counts how many structs implement each interface in the package.
func CountImplementors(pi *model.PackageInfo, scope *types.Package) {
	if scope == nil {
		return
	}
	for _, iface := range pi.Interfaces {
		obj := scope.Scope().Lookup(iface.Name)
		if obj == nil {
			continue
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		ifaceType, ok := named.Underlying().(*types.Interface)
		if !ok {
			continue
		}
		count := 0
		for _, s := range pi.Structs {
			sObj := scope.Scope().Lookup(s.Name)
			if sObj == nil {
				continue
			}
			sType := sObj.Type()
			if types.Implements(sType, ifaceType) || types.Implements(types.NewPointer(sType), ifaceType) {
				count++
			}
		}
		iface.Implementors = count
	}
}
