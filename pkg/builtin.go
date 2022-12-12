package maqui

import (
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
)

func defineBuiltins(b *LLVMIRBuilder) {
	defineBuiltinFunc(b, "print", builtinPrint)
}

type funcDefinition = func(mod *ir.Module) *ir.Func

func defineBuiltinFunc(b *LLVMIRBuilder, name string, definition funcDefinition) {
	f := definition(b.mod)
	f.SetName(name)
	b.values.Set(name, f)
}

func builtinPrint(mod *ir.Module) *ir.Func {
	f := mod.NewFunc("", types.Void, ir.NewParam("v", types.I32))
	b := f.NewBlock("")

	printf := mod.NewFunc("printf", types.I32, ir.NewParam("format", types.I8Ptr))
	printf.Sig.Variadic = true

	zero := constant.NewInt(types.I32, 0)

	format := constant.NewCharArrayFromString("%d\n")
	formatGlob := mod.NewGlobalDef("._printf_fmt", format)

	fmtAddr := constant.NewGetElementPtr(types.NewArray(3, types.I8), formatGlob, zero, zero)

	b.NewCall(printf, fmtAddr, f.Params[0])

	b.NewRet(nil)

	return f
}
