package ir

type Intrinsic struct {
	Name       string
	ParamNames []string
	Signature  *Signature
}

var Memcpy = Intrinsic{
	Name:       "llvm.memcpy.p0.p0.i64",
	ParamNames: []string{"dst", "src", "len", "volatile"},
	Signature: &Signature{
		Returns: Void,
		Params:  []Type{Pointer, Pointer, I64, I1},
	},
}

var Memmove = Intrinsic{
	Name:       "llvm.memmove.p0.p0.i64",
	ParamNames: []string{"dst", "src", "len", "volatile"},
	Signature: &Signature{
		Returns: Void,
		Params:  []Type{Pointer, Pointer, I64, I1},
	},
}

var Memset = Intrinsic{
	Name:       "llvm.memset.p0.i64",
	ParamNames: []string{"dst", "val", "len", "volatile"},
	Signature: &Signature{
		Returns: Void,
		Params:  []Type{Pointer, I8, I64, I1},
	},
}
