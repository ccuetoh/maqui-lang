package maqui

import (
	"errors"
	"fmt"
	"io"
	"os/exec"

	"golang.org/x/sync/errgroup"
)

type Arch string
type Vendor string
type OS string

const (
	X86_64 Arch = "x86_64"

	Unknown Vendor = "unknown"

	Windows OS = "windows64"
	Linux   OS = "linux"
	Darwin  OS = "darwin"
)

type Target struct {
	Arch   Arch
	Vendor Vendor
	OS     OS
}

func (t Target) String() string {
	return fmt.Sprintf("%s-%s-%s", t.Arch, t.Vendor, t.OS)
}

type Compiler struct {
	target Target
}

func NewCompiler(target Target) *Compiler {
	return &Compiler{
		target: target,
	}
}

func (c *Compiler) Compile(filename string) ([]CompileError, error) {
	lexer, err := NewLexer(filename)
	if err != nil {
		return nil, err
	}

	parser := NewParser(lexer)
	analyzer := NewContextAnalyser(parser)

	global := NewGlobalSymbolTable()
	analyzer.DefineInto(global)

	ast := analyzer.Do(global)
	if len(ast.Errors) != 0 {
		return ast.Errors, nil
	}

	gen := NewLLVMGenerator(ast)
	ir := gen.Do()

	return nil, c.build(ir)
}

func (c *Compiler) build(ir IR) error {
	outName := "main"
	if c.target.OS == Windows {
		outName += ".exe"
	}

	cmd := exec.Command("clang",
		"-x",
		"ir",
		"--target="+c.target.String(),
		"-o", outName,
		"-",
	)

	r, w := io.Pipe()
	cmd.Stdin = r

	errs := errgroup.Group{}
	errs.Go(func() error {
		_, err := w.Write([]byte(ir.String()))
		if err != nil {
			return err
		}

		return w.Close()
	})

	errs.Go(func() error {
		if cmdOut, err := cmd.CombinedOutput(); err != nil {
			return errors.New(fmt.Sprintf("%v: %s", err, cmdOut))
		}

		return nil
	})

	return errs.Wait()
}
