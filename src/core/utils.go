package core

import "reflect"

func IsNil(v any) bool {
	if v == nil {
		return true
	}

	switch reflect.TypeOf(v).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Chan, reflect.Slice, reflect.Func:
		return reflect.ValueOf(v).IsNil()
	default:
		return false
	}
}
