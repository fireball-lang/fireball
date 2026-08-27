; Keywords

"pub" @keyword
"mut" @keyword

"mod" @keyword
"import" @keyword

"struct" @keyword
"enum" @keyword
"interface" @keyword
"impl" @keyword
"type" @keyword
"func" @keyword.function

"var" @keyword
"if" @keyword.control.conditional
"else" @keyword.control.conditional
"while" @keyword.control.repeat
"for" @keyword.control.repeat

"return" @keyword.control.return
"break" @keyword.control
"continue" @keyword.control

"with" @keyword
"as" @keyword
"or" @keyword

"sizeof" @keyword
"alignof" @keyword
"offsetof" @keyword
"typeof" @keyword

; Mod

(mod path: (identifier) @namespace)

; Import

(import path: (identifier) @namespace)
(import name: (identifier) @namespace)

(import symbol: (identifier) @variable)
((import symbol: (identifier) @type) (#match? @type "^[A-Z]"))

; Declarations

(struct name: (identifier) @type)
(struct type_param: (type_param name: (identifier) @type.parameter))
(struct field: (field name: (identifier) @property))

(enum name: (identifier) @enum)
(case name: (identifier) @variant)
(case value: (integer) @number)

(interface name: (identifier) @type)
(interface type_param: (type_param name: (identifier) @type.parameter))

(impl type_param: (type_param name: (identifier) @type.parameter))

(associated_type name: (identifier) @type)

(global_var name: (identifier) @variable.global)

(func name: (identifier) @function)
(func type_param: (type_param name: (identifier) @type.parameter))
(func param: (param name: (identifier) @variable.parameter))

; Expressions

(member_expr name: (identifier) @property)

(var name: (identifier) @variable)

(offsetof field: (identifier) @property)

(struct_initializer field: (field_initializer name: (identifier) @property))

(with_expr field: (field_initializer name: (identifier) @property))

; Operators

[
    "=" "+=" "-=" "*=" "/=" "%=" "<<=" ">>=" ">>>=" "|=" "^=" "&="
    "+" "-" "/" "%"
    "++" "--" "?"
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

(array_type size: (number) @number)

(reference_type "&" @punctuation.special)
(pointer_type "*" @punctuation.special)

(identifier_type (identifier) @type)
(identifier_type (identifier) @namespace . "::")

; Attributes

(attribute_group "#" @punctuation.special)

(required_attribute) @attribute
(init_attribute) @attribute
(test_attribute "test" @attribute)
(extern_attribute) @attribute
(link_name_attribute "link_name" @attribute)
(repr_attribute "repr" @attribute)
(repr_attribute layout: [ "Fireball" "C" "Union" ] @attribute)
(intrinsic_attribute "intrinsic" @attribute)
(intrinsic_attribute kind: [ "syscall" "memcpy" "memmove" "memset" ] @attribute)
(cfg_attribute "cfg" @attribute)

(option_cfg name: (identifier) @attribute)
(call_cfg name: (identifier) @attribute)

; Literals

(bool) @boolean
((number) @number.float (#match? @number.float "[0-9]+\\.[0-9]+"))
(number) @number
(char) @string
(string) @string
(null_expr) @constant.builtin

; Other

(comment) @comment
((comment) @comment.doc (#match? @comment.doc "(^/// |^///$)"))

[ "(" ")" "[" "]" "{" "}" ] @punctuation.bracket
[ "," ";" ":" "."         ] @punctuation.delimiter
