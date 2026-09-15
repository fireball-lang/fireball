package types

type typeMapEntry[K Type, V any] struct {
	key   K
	value V
}

type TypeMap[V any] struct {
	aliases    []typeMapEntry[*Alias, V]
	arrays     []typeMapEntry[*Array, V]
	enums      []typeMapEntry[*Enum, V]
	funcs      []typeMapEntry[*Func, V]
	integers   []typeMapEntry[*Integer, V]
	interfaces []typeMapEntry[*Interface, V]
	params     []typeMapEntry[*Param, V]
	pointers   []typeMapEntry[*Pointer, V]
	primitives []typeMapEntry[*Primitive, V]
	references []typeMapEntry[*Reference, V]
	structs    []typeMapEntry[*Struct, V]
	other      []typeMapEntry[Type, V]
}

func (t *TypeMap[V]) Get(key Type) (V, bool) {
	switch key := key.(type) {
	case *Alias:
		return getEntry(t.aliases, key)
	case *Array:
		return getEntry(t.arrays, key)
	case *Enum:
		return getEntry(t.enums, key)
	case *Func:
		return getEntry(t.funcs, key)
	case *Integer:
		return getEntry(t.integers, key)
	case *Interface:
		return getEntry(t.interfaces, key)
	case *Param:
		return getEntry(t.params, key)
	case *Pointer:
		return getEntry(t.pointers, key)
	case *Primitive:
		return getEntry(t.primitives, key)
	case *Reference:
		return getEntry(t.references, key)
	case *Struct:
		return getEntry(t.structs, key)
	default:
		return getEntry(t.other, key)
	}
}

func (t *TypeMap[V]) Insert(key Type, value V) {
	switch key := key.(type) {
	case *Alias:
		t.aliases = insertEntry(t.aliases, key, value)
	case *Array:
		t.arrays = insertEntry(t.arrays, key, value)
	case *Enum:
		t.enums = insertEntry(t.enums, key, value)
	case *Func:
		t.funcs = insertEntry(t.funcs, key, value)
	case *Integer:
		t.integers = insertEntry(t.integers, key, value)
	case *Interface:
		t.interfaces = insertEntry(t.interfaces, key, value)
	case *Param:
		t.params = insertEntry(t.params, key, value)
	case *Pointer:
		t.pointers = insertEntry(t.pointers, key, value)
	case *Primitive:
		t.primitives = insertEntry(t.primitives, key, value)
	case *Reference:
		t.references = insertEntry(t.references, key, value)
	case *Struct:
		t.structs = insertEntry(t.structs, key, value)
	default:
		t.other = insertEntry(t.other, key, value)
	}
}

// InsertUnsafe inserts a key-value pair without check if the key is already present
func (t *TypeMap[V]) InsertUnsafe(key Type, value V) {
	switch key := key.(type) {
	case *Alias:
		t.aliases = insertEntryUnsafe(t.aliases, key, value)
	case *Array:
		t.arrays = insertEntryUnsafe(t.arrays, key, value)
	case *Enum:
		t.enums = insertEntryUnsafe(t.enums, key, value)
	case *Func:
		t.funcs = insertEntryUnsafe(t.funcs, key, value)
	case *Integer:
		t.integers = insertEntryUnsafe(t.integers, key, value)
	case *Interface:
		t.interfaces = insertEntryUnsafe(t.interfaces, key, value)
	case *Param:
		t.params = insertEntryUnsafe(t.params, key, value)
	case *Pointer:
		t.pointers = insertEntryUnsafe(t.pointers, key, value)
	case *Primitive:
		t.primitives = insertEntryUnsafe(t.primitives, key, value)
	case *Reference:
		t.references = insertEntryUnsafe(t.references, key, value)
	case *Struct:
		t.structs = insertEntryUnsafe(t.structs, key, value)
	default:
		t.other = insertEntryUnsafe(t.other, key, value)
	}
}

func getEntry[K Type, V any](entries []typeMapEntry[K, V], key K) (V, bool) {
	for _, entry := range entries {
		if entry.key.Equals(key) {
			return entry.value, true
		}
	}

	var empty V
	return empty, false
}

func insertEntryUnsafe[K Type, V any](entries []typeMapEntry[K, V], key K, value V) []typeMapEntry[K, V] {
	return append(entries, typeMapEntry[K, V]{
		key:   key,
		value: value,
	})
}

func insertEntry[K Type, V any](entries []typeMapEntry[K, V], key K, value V) []typeMapEntry[K, V] {
	for i := range entries {
		entry := &entries[i]

		if entry.key.Equals(key) {
			entry.value = value
			return entries
		}
	}

	return append(entries, typeMapEntry[K, V]{
		key:   key,
		value: value,
	})
}
