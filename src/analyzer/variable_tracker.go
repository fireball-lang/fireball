package analyzer

import "fireball/ast"

type VariableTracker[T any] struct {
	scopes []variableScope[T]
}

type variableScope[T any] struct {
	variables []scopeVariable[T]
}

type scopeVariable[T any] struct {
	name  string
	type_ ast.Type
	data  T
}

func (v *VariableTracker[T]) Find(name string) (ast.Type, T) {
	for i := len(v.scopes) - 1; i >= 0; i-- {
		for _, variable := range v.scopes[i].variables {
			if variable.name == name {
				return variable.type_, variable.data
			}
		}
	}

	var data T
	return nil, data
}

func (v *VariableTracker[T]) Add(name string, type_ ast.Type, data T) bool {
	scope := &v.scopes[len(v.scopes)-1]

	for _, variable := range scope.variables {
		if variable.name == name {
			return false
		}
	}

	scope.variables = append(scope.variables, scopeVariable[T]{
		name:  name,
		type_: type_,
		data:  data,
	})

	return true
}

func (v *VariableTracker[T]) PushScope() {
	v.scopes = append(v.scopes, variableScope[T]{})
}

func (v *VariableTracker[T]) PopScope() {
	v.scopes = v.scopes[:len(v.scopes)-1]
}
