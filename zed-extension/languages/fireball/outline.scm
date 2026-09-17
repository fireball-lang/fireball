(type_alias
  "type" @context
  name: (identifier) @name) @item

(struct
  "struct" @context
  name: (identifier) @name) @item

(enum
  "enum" @context
  name: (identifier) @name) @item

(interface
  "interface" @context
  name: (identifier) @name) @item

(associated_type
    "type" @context
    name: (identifier) @name
    type: (type) @context) @item

(associated_const
    "const" @context
    name: (identifier) @name
    value: (expr) @context) @item

(impl
  "impl" @context
  type: (type) @name) @item

(const
  "const" @context
  name: (identifier) @name) @item

(global_var
  "var" @context
  name: (identifier) @name) @item

(func
  "func" @context
  name: (identifier) @name) @item
