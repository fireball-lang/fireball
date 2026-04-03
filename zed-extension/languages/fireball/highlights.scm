; Keywords

"struct" @keyword
"func" @keyword.function

"var" @keyword
"if" @keyword.control.conditional
"else" @keyword.control.conditional
"while" @keyword.control.repeat
"for" @keyword.control.repeat

"return" @keyword.control.return
"break" @keyword.control
"continue" @keyword.control

; Attributes

(attribute_group "#" @punctuation.special)
(attribute name: (identifier) @attribute)

; Declarations

(struct name: (identifier) @type)
(struct field: (name_type name: (identifier) @property))

(func name: (identifier) @function)
(func param: (name_type name: (identifier) @variable.parameter))

; Expressions

(call_expr callee: (identifier) @function.call)
(call_expr callee: (member_expr name: (identifier) @function.method.call))

(member_expr name: (identifier) @property)

(var name: (identifier) @variable)

; Operators

[
  "="
  "+" "-" "/" "%"
  "&" "|" "^"
  "!" "&&" "||"
  "==" "!="
  "<" "<=" ">" ">="
] @operator

(binary_expr "*" @operator)

"..." @punctuation.special

; Types

(primitive_type) @type.builtin

(array_type size: (integer) @number)

(pointer_type "*" @punctuation.special)

(identifier_type (identifier) @type)

; Literals

(bool)   @boolean
((number) @number.float (#match? @number.float "[0-9]+\\.[0-9]+"))
(number) @number
(char)   @character
(string) @string

; Other

(comment) @comment

[ "(" ")" "[" "]" "{" "}" ] @punctuation.bracket
[ "," ";" ":" "."         ] @punctuation.delimiter
