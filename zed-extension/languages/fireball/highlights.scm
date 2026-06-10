; Keywords

"pub" @keyword
"mut" @keyword

"mod" @keyword
"import" @keyword

"struct" @keyword
"interface" @keyword
"impl" @keyword
"func" @keyword.function

"var" @keyword
"if" @keyword.control.conditional
"else" @keyword.control.conditional
"while" @keyword.control.repeat
"for" @keyword.control.repeat

"return" @keyword.control.return
"break" @keyword.control
"continue" @keyword.control

"as" @keyword

"sizeof" @keyword
"alignof" @keyword
"offsetof" @keyword

; Attributes

(mod path: (identifier_path (identifier) @namespace))

(import path: (identifier) @namespace)
(import name: (identifier) @namespace)

(import symbol: (identifier) @variable)
((import symbol: (identifier) @type) (#match? @type "^[A-Z]"))

(attribute_group "#" @punctuation.special)
(attribute name: (identifier) @attribute)

; Declarations

(struct name: (identifier) @type)
(struct type_param: (type_param name: (identifier) @type.parameter))
(struct field: (field name: (identifier) @property))

(interface name: (identifier) @type)
(interface type_param: (type_param name: (identifier) @type.parameter))

(impl type_param: (type_param name: (identifier) @type.parameter))

(func name: (identifier) @function)
(func type_param: (type_param name: (identifier) @type.parameter))
(func receiver: (identifier) @keyword)
(func param: (param name: (identifier) @variable.parameter))

; Expressions

(member_expr name: (identifier) @property)

(call_expr callee: (identifier_path) @function.call)
(call_expr callee: (member_expr name: (identifier) @function.method.call))

(var name: (identifier) @variable)

(offsetof field: (identifier) @property)

; Operators

[
    "=" "+=" "-=" "*=" "/=" "%=" "<<=" ">>=" ">>>=" "|=" "^=" "&="
    "+" "-" "/" "%"
    "++" "--"
    "<<" ">>" ">>>"
    "&" "|" "^"
    "!" "&&" "||"
    "==" "!="
    "<" "<=" ">" ">="
] @operator

(prefix_expr "*" @operator)

(binary_expr "*" @operator)

"..." @punctuation.special

"::" @punctuation.delimiter

; Types

(primitive_type) @type.builtin

(array_type size: (integer) @number)

(pointer_type "*" @punctuation.special)

(identifier_type (identifier_path (identifier) @type))
(identifier_path (identifier) @namespace . "::")

(call_expr callee: (identifier_path (identifier) @type . "::") (#match? @type "^[A-Z]"))

; Literals

(bool)   @boolean
((number) @number.float (#match? @number.float "[0-9]+\\.[0-9]+"))
(number) @number
(char)   @character
(string) @string
(null_expr) @constant.builtin

; Other

(comment) @comment

[ "(" ")" "[" "]" "{" "}" ] @punctuation.bracket
[ "," ";" ":" "."         ] @punctuation.delimiter
