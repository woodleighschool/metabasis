package expression

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"

	"github.com/woodleighschool/metabasis/internal/domain"
)

const userTypeName = "domain.User"

// Compiler validates identity policy expressions against Metabasis's typed CEL contract.
type Compiler struct {
	environment *cel.Env
}

// Program is a compiled, thread-safe CEL condition.
type Program struct {
	source  string
	program cel.Program
}

// NewCompiler creates the CEL environment used during configuration loading.
func NewCompiler() (*Compiler, error) {
	environment, err := cel.NewEnv(
		ext.NativeTypes(reflect.TypeFor[domain.User](), ext.ParseStructTags(true)),
		ext.Strings(),
		cel.Variable("user", cel.ObjectType(userTypeName)),
	)
	if err != nil {
		return nil, fmt.Errorf("create CEL environment: %w", err)
	}
	return &Compiler{environment: environment}, nil
}

// CompileCondition compiles an expression that must return a boolean.
func (c *Compiler) CompileCondition(source string) (Program, error) {
	if strings.TrimSpace(source) == "" {
		return Program{}, fmt.Errorf("expression cannot be empty")
	}
	ast, issues := c.environment.Compile(source)
	if issues != nil && issues.Err() != nil {
		return Program{}, fmt.Errorf("compile CEL expression: %w", issues.Err())
	}
	if !ast.OutputType().IsExactType(cel.BoolType) {
		return Program{}, fmt.Errorf("CEL expression returns %s, want bool", ast.OutputType())
	}
	program, err := c.environment.Program(ast)
	if err != nil {
		return Program{}, fmt.Errorf("build CEL program: %w", err)
	}
	return Program{source: source, program: program}, nil
}

// Eval evaluates the condition for a resolved user.
func (p Program) Eval(user domain.User) (bool, error) {
	value, _, err := p.program.Eval(map[string]any{"user": &user})
	if err != nil {
		return false, fmt.Errorf("evaluate CEL expression %q: %w", p.source, err)
	}
	result, ok := value.Value().(bool)
	if !ok {
		return false, fmt.Errorf("evaluate CEL expression %q: result is not boolean", p.source)
	}
	return result, nil
}
