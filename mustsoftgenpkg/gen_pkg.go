package mustsoftgenpkg

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/yyle88/done"
	"github.com/yyle88/mustdone"
	"github.com/yyle88/mustdone/internal/utils"
	"gitlab.yyle.com/golang/uvyyle.git/utils_golang/utils_golang_ast"
	"gitlab.yyle.com/golang/uvyyle.git/utils_golang/utils_golang_ast/utils_golang_ast_fields"
	"gitlab.yyle.com/golang/uvyyle.git/utils_golang/utils_golang_fmt"
)

func WriteGenFlex(
	t *testing.T,
	pkgRoot string,
	flexibleType mustdone.FlexibleHandlingType,
	pkgPath string,
) {
	WriteOneFlex(
		t,
		pkgRoot,
		pkgRoot,
		flexibleType,
		pkgPath,
		mustdone.GetPkgPath(),
		mustdone.GetPkgName(),
	)
}

func WriteOneFlex(
	t *testing.T,
	absPackagePath string,
	writeToAbsRoot string,
	flexibleType mustdone.FlexibleHandlingType,
	pkgPath string,
	flexPkgPath string,
	flexUseNode string,
) {
	fmt.Println(absPackagePath, writeToAbsRoot)

	names := utils.MustLsFileNames(absPackagePath)
	for _, name := range names {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		absPath := filepath.Join(absPackagePath, name)
		srcData := done.VAE(os.ReadFile(absPath)).Done()

		astFile, err := utils_golang_ast.NewAstXFilepath(absPath)
		done.Done(err)

		packageName := astFile.Name.Name

		astFns := utils_golang_ast.GetFunctions(astFile)

		var sliceFuncCodes []string
		for _, astFunc := range astFns {
			if astFunc.Recv != nil {
				if len(astFunc.Recv.List) > 0 && len(astFunc.Recv.List[0].Names) > 0 {
					t.Log(astFunc.Recv.List[0].Names[0].Name, astFunc.Name.Name)
				}
				continue
			}
			t.Log(astFunc.Name.Name)
			if !utils.C0IsUpperString(astFunc.Name.Name) {
				continue
			}
			results, anonymous := parseResults(srcData, astFunc)
			t.Log(utils.NeatString(results))

			sFuncCode := newFuncCode(srcData, packageName, astFunc, results, anonymous, flexibleType, flexUseNode)
			t.Log(sFuncCode)

			sliceFuncCodes = append(sliceFuncCodes, sFuncCode)
		}

		if len(sliceFuncCodes) > 0 {
			var shortFlexName = strings.ToLower(string(flexibleType))

			newPackageName := packageName + "_" + shortFlexName

			ptx := utils.NewPTX()
			ptx.Println("package" + " " + newPackageName)
			ptx.Println("import(")
			ptx.Println(utils.SetDoubleQuotes(pkgPath))
			ptx.Println(utils.SetDoubleQuotes(flexPkgPath))
			ptx.Println(")")
			ptx.Println(strings.Join(sliceFuncCodes, "\n"))

			newName := strings.Replace(name, ".go", "_"+shortFlexName+".go", 1)
			newPath := filepath.Join(writeToAbsRoot, newPackageName, newName)
			utils.MustWriteToPath(newPath, utils_golang_fmt.MustSource(ptx.String()))
		}
	}
}

func newFuncCode(srcData []byte, packageName string, astFunc *ast.FuncDecl, results []*retType, anonymous bool, flexibleType mustdone.FlexibleHandlingType, flexUseNode string) string {
	var res = "func " + astFunc.Name.Name
	if astFunc.Type.TypeParams != nil {
		res += utils_golang_ast.GetNodeString(srcData, astFunc.Type.TypeParams)
	}
	res += "("
	var argList []string
	if astFunc.Type.Params != nil && len(astFunc.Type.Params.List) > 0 {
		var args []string
		for _, param := range astFunc.Type.Params.List {
			if len(param.Names) == 0 {
				argName := "arg" + strconv.Itoa(len(args))
				args = append(args, argName+" "+utils_golang_ast.GetNodeString(srcData, param.Type))
				argList = append(argList, argName)
			} else {
				args = append(args, utils_golang_ast.GetNodeString(srcData, param))
				for _, name := range param.Names {
					// 检查参数是否是 "..."
					if _, variadic := param.Type.(*ast.Ellipsis); variadic {
						argList = append(argList, name.Name+" ...")
					} else {
						argList = append(argList, name.Name)
					}
				}
			}
		}
		res += strings.Join(args, ",")
	}
	res += ")"

	genericsMap := utils_golang_ast_fields.CountGenericsMap(astFunc.Type.TypeParams)

	var isReturnErrors = false
	{
		var rets = make([]string, 0, len(results))
		if anonymous {
			for _, res := range results {
				if res.Type == "error" {
					isReturnErrors = true
					continue
				}
				rets = append(rets, cvtAZType(packageName, genericsMap, res))
			}
		} else {
			for _, res := range results {
				if res.Type == "error" {
					isReturnErrors = true
					continue
				}
				rets = append(rets, res.Name+" "+cvtAZType(packageName, genericsMap, res))
			}
		}
		if len(rets) > 0 {
			if len(rets) < 2 && anonymous {
				res += " " + rets[0]
			} else {
				res += " " + "(" + strings.Join(rets, ",") + ")"
			}
		}
	}
	res += " " + "{" + "\n"
	if len(results) > 0 {
		var rets = make([]string, 0, len(results))
		for _, res := range results {
			rets = append(rets, res.Name)
		}
		res += strings.Join(rets, ",")
		if anonymous || isReturnErrors {
			res += ":="
		} else {
			res += "="
		}
	}
	res += packageName + "." + astFunc.Name.Name
	if astFunc.Type.TypeParams != nil {
		res += "["
		var genericTypeNames []string
		for _, xts := range astFunc.Type.TypeParams.List {
			for _, xtx := range xts.Names {
				genericTypeNames = append(genericTypeNames, xtx.Name)
			}
		}
		res += strings.Join(genericTypeNames, ",")
		res += "]"
	}
	res += "(" + strings.Join(argList, ",") + ")" + "\n"
	if len(results) > 0 {
		for _, x := range results {
			if x.Type == "error" {
				res += flexUseNode + "." + string(flexibleType) + "(" + x.Name + ")" + "\n"
			}
		}
	}
	if len(results) > 0 {
		var rets = make([]string, 0, len(results))
		for _, res := range results {
			if res.Type == "error" {
				continue
			}
			rets = append(rets, res.Name)
		}
		if len(rets) > 0 {
			res += "return" + " " + strings.Join(rets, ",") + "\n"
		}
	}
	res += "}" + "\n"
	return res
}

func cvtAZType(packageName string, genericsMap map[string]int, res *retType) string {
	if utils.C0IsUpperString(res.Type) {
		classType := res.Type
		if _, ok := genericsMap[classType]; ok {
			return res.Type
		}
		return packageName + "." + classType
	}
	if res.Type[0] == '*' {
		classType := res.Type[1:]
		if _, ok := genericsMap[classType]; ok {
			return res.Type
		}
		if utils.C0IsUpperString(classType) {
			return "*" + packageName + "." + classType
		}
	}
	return res.Type
}

type retType struct {
	Name string
	Type string
}

func parseResults(srcData []byte, astFunc *ast.FuncDecl) ([]*retType, bool) {
	var results []*retType
	var anonymous = true
	if astFunc.Type.Results == nil || len(astFunc.Type.Results.List) == 0 {
		results = make([]*retType, 0)
	} else {
		var errNum int
		for _, x := range astFunc.Type.Results.List {
			resType := utils_golang_ast.GetNodeString(srcData, x.Type)
			eIs := bool(resType == "error")
			if len(x.Names) == 0 {
				var resName string
				if eIs {
					if errNum == 0 {
						resName = "err"
					} else {
						resName = "err" + strconv.Itoa(errNum)
					}
				} else {
					resName = "res" + strconv.Itoa(len(results))
				}
				results = append(results, &retType{
					Name: resName,
					Type: resType,
				})
				if eIs {
					errNum++
				}
			} else {
				anonymous = false
				for _, name := range x.Names {
					results = append(results, &retType{
						Name: name.Name,
						Type: resType,
					})
					if eIs {
						errNum++
					}
				}
			}
		}
	}
	return results, anonymous
}