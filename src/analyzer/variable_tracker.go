package analyzer

import "fireball/ast"

type variableTracker struct {
	scopes []variableScope
}

type variableScope struct {
	variables []variable
}

type variable struct {
	name  string
	type_ ast.Type
}

func (v *variableTracker) find(name string) ast.Type {
	for i := len(v.scopes) - 1; i >= 0; i-- {
		for _, variable := range v.scopes[i].variables {
			if variable.name == name {
				return variable.type_
			}
		}
	}

	return nil
}

func (v *variableTracker) add(name string, type_ ast.Type) bool {
	scope := &v.scopes[len(v.scopes)-1]

	for _, variable := range scope.variables {
		if variable.name == name {
			return false
		}
	}

	scope.variables = append(scope.variables, variable{
		name:  name,
		type_: type_,
	})

	return true
}

func (v *variableTracker) pushScope() {
	v.scopes = append(v.scopes, variableScope{})
}

func (v *variableTracker) popScope() {
	v.scopes = v.scopes[:len(v.scopes)-1]
}
