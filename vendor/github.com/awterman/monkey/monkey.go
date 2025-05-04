package monkey

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/agiledragon/gomonkey/v2/creflect"
)

// NewPatches returns a new Patches. Be sure to call `defer Patches.Reset()` on every `Patches`.
func NewPatches() *gomonkey.Patches {
	return gomonkey.NewPatches()
}

// Func applies a function patch.
func Func[Fn any](patches *gomonkey.Patches, target Fn, replacement Fn) *gomonkey.Patches {
	if patches == nil {
		patches = gomonkey.NewPatches()
	}

	return patches.ApplyFunc(target, replacement)
}

// Var applies a variable patch.
func Var[Var any](patches *gomonkey.Patches, target *Var, replacement Var) *gomonkey.Patches {
	if patches == nil {
		patches = gomonkey.NewPatches()
	}

	return patches.ApplyGlobalVar(target, replacement)
}

func getMethodName(m any) string {
	fullName := runtime.FuncForPC(reflect.ValueOf(m).Pointer()).Name()
	return strings.Split(fullName[strings.LastIndex(fullName, ".")+1:], "-")[0]
}

func funcToMethod(receiverType reflect.Type, doubleFunc any) reflect.Value {
	rf := reflect.TypeOf(doubleFunc)
	if rf.Kind() != reflect.Func {
		panic("doubleFunc is not a func")
	}

	vf := reflect.ValueOf(doubleFunc)

	inParams := make([]reflect.Type, 0, rf.NumIn()+1)
	inParams = append(inParams, receiverType)
	for i := 0; i < rf.NumIn(); i++ {
		inParams = append(inParams, rf.In(i))
	}

	outParams := make([]reflect.Type, 0, rf.NumOut())
	for i := 0; i < rf.NumOut(); i++ {
		outParams = append(outParams, rf.Out(i))
	}

	funcType := reflect.FuncOf(
		inParams,
		outParams,
		rf.IsVariadic(),
	)

	return reflect.MakeFunc(funcType, func(in []reflect.Value) []reflect.Value {
		if funcType.IsVariadic() {
			return vf.CallSlice(in[1:])
		} else {
			return vf.Call(in[1:])
		}
	})
}

// Method applies a method patch.
func Method[Receiver any, Fn any](patches *gomonkey.Patches, receiver Receiver, method Fn, replacement Fn) *gomonkey.Patches {
	if patches == nil {
		patches = gomonkey.NewPatches()
	}

	name := getMethodName(method)
	if name == "" {
		panic("method is not a method")
	}

	if name[0] >= 'A' && name[0] <= 'Z' {
		return patches.ApplyMethodFunc(receiver, name, replacement)
	}

	m, ok := creflect.MethodByName(reflect.TypeOf(receiver), name)
	if !ok {
		panic("retrieve method by name failed")
	}

	r := reflect.TypeOf(receiver)
	doubleFunc := funcToMethod(r, replacement)

	return patches.ApplyCoreOnlyForPrivateMethod(m, doubleFunc)
}
