// Package loadcfg loads a configuration using an optional file and
// basic serialization with the standard struct tags, but then also loads
// values from the environment.
//
// Environment variables are defined with some prefix given to the function
// such (eg. PREFIX) and then they can be used to set almost any type of field.
// Below is an example of a complicated set of structs that show how you could
// set the fields for each type.
//
//    type A struct {
//        // PREFIX_INT=5
//        Int     int       `toml:"int"`
//        // PREFIX_INTPTR=5
//        IntPtr  *int      `toml:"intptr"`
//        // PREFIX_STRINGS="one,two,three"
//        Strings []string  `toml:"strings"`
//        // PREFIX_TIME=RFC3339TimeString
//        Time    time.Time `toml:"time"`
//
//        // PREFIX_MAP_KEYNAME_FLOAT=4.5
//        Map        map[string]B    `toml:"map"`
//        // PREFIX_MAP_KEYNAME_FLOAT=4.5
//        MapPtr     map[string]*B   `toml:"mapptr"`
//        // PREFIX_MAPPRIM_KEYNAME=1
//        MapPrim    map[string]int  `toml:"mapprim"`
//        // PREFIX_MAPPRIMPTR_KEYNAME=2
//        MapPrimPtr map[string]*int `toml:"mapprimptr"`
//
//        // PREFIX_SLICE_0_FLOAT=4.5
//        // PREFIX_SLICE_1_FLOAT=4.5
//        Slice    []B  `toml:"slice"`
//        // PREFIX_SLICEPTR_0_FLOAT=4.5
//        SlicePtr []*B `toml:"sliceptr"`
//
//        // PREFIX_STRUCT_FLOAT=4.5
//        Struct    B  `toml:"struct"`
//        // PREFIX_STRUCTPTR_FLOAT=4.5
//        StructPtr *B `toml:"structptr"`
//        // Ignored, cannot be set by env
//        Ignored   *B `toml:"-"`
//    }
//
//    type B struct {
//        Float float64 `toml:"float"`
//    }
package loadcfg

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/BurntSushi/toml"
)

var timeType = reflect.TypeOf(time.Time{})

// TOML loads filename using toml and deserializes it into obj, then
// the environment overrides are applied. There is no error if a config file
// is not found so you must check explicitly for this.
func TOML(envPrefix, filename string, obj interface{}) (m toml.MetaData, err error) {
	m, err = toml.DecodeFile(filename, obj)
	if err != nil && !os.IsNotExist(err) {
		return m, err
	}

	env := os.Environ()

	pseudoKeys, err := envPseudoKeys("toml", obj)
	if err != nil {
		return m, err
	}

	kvs := findKeyValues(env, envPrefix, pseudoKeys)
	if err = overwriteStructVals("toml", kvs, obj); err != nil {
		return m, err
	}

	return m, err
}

// Env deserializes environment variables into a struct. The envPrefix is
// not optional. The structTag is configurable.
func Env(envPrefix, structTag string, obj interface{}) error {
	env := os.Environ()

	pseudoKeys, err := envPseudoKeys("toml", obj)
	if err != nil {
		return err
	}

	kvs := findKeyValues(env, envPrefix, pseudoKeys)
	if err = overwriteStructVals("toml", kvs, obj); err != nil {
		return err
	}

	return nil
}

// overwriteStructVals takes in struct tag paths to values to set
// and an object to set them in
func overwriteStructVals(tag string, values map[string]string, v interface{}) error {
	obj := reflect.ValueOf(v)

	var keys []string
	for k := range values {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		keyParts := strings.Split(k, ".")

		if err := overwriteStructValsHelper(tag, keyParts, values[k], obj); err != nil {
			return err
		}
	}

	return nil
}

func overwriteStructValsHelper(tag string, key []string, val string, obj reflect.Value) error {
	if obj.Kind() == reflect.Ptr {
		obj = obj.Elem()
	}

	switch obj.Kind() {
	case reflect.Struct:
		if obj.Type() == timeType {
			// This is not the container we're looking for
			break
		}

		sType := obj.Type()
		n := sType.NumField()
		for i := 0; i < n; i++ {
			field := sType.Field(i)

			name, ok := getTag(field, tag)
			if !ok {
				// We don't deal with missing or explicitly ignored struct tags
				continue
			}

			if name != key[0] {
				// Keep searching
				continue
			}

			// If it's a map we have to create it since we're going to put
			// a value inside it.
			// If it's a pointer we have to create whatever's behind it.
			structFieldVal := obj.Field(i)
			switch field.Type.Kind() {
			case reflect.Ptr:
				if structFieldVal.IsNil() {
					ptrType := field.Type.Elem()
					newVal := reflect.New(ptrType)
					reflect.Indirect(newVal)
					structFieldVal.Set(newVal)
				}
				structFieldVal = reflect.Indirect(structFieldVal)
			case reflect.Map:
				if structFieldVal.IsNil() {
					newVal := reflect.MakeMap(field.Type)
					structFieldVal.Set(newVal)
				}
			}
			if !structFieldVal.CanSet() {
				return fmt.Errorf("cannot set: %s (%s) [%s]", field.Name, name, structFieldVal.Type().String())
			}
			return overwriteStructValsHelper(tag, key[1:], val, structFieldVal)
		}

		return fmt.Errorf("cannot set env, could not find struct field: %s (%s)", key[0], val)
	case reflect.Map:
		// The current name is a map key
		keyName := key[0]
		// Let's see if we have an object in the map already
		keyObj := reflect.ValueOf(keyName)
		valObj := obj.MapIndex(keyObj)
		valType := obj.Type().Elem()
		if !valObj.IsValid() {
			// Key does not exist, we have to make a new whatever this is
			// set it's value, then set it into our map
			isValueTypePtr := valType.Kind() == reflect.Ptr
			if isValueTypePtr {
				valType = valType.Elem()
			}

			valObj = reflect.New(valType)
			if err := overwriteStructValsHelper(tag, key[1:], val, valObj); err != nil {
				return err
			}

			if !isValueTypePtr {
				// If it wasn't originally a pointer type in the map we need to
				// deref it.
				valObj = reflect.Indirect(valObj)
			}
			obj.SetMapIndex(keyObj, valObj)
			return nil
		} else if valType.Kind() == reflect.Ptr {
			// If this is the case we just need to set the values on this
			// since it'll be addressable no problem and we don't have to reset
			// in the map
			return overwriteStructValsHelper(tag, key[1:], val, valObj)
		} else {
			// Here we have received a value type from the map itself
			// so we set it and then overwrite the value in the map

			// When we take a value out of a map we first have to clone it
			// into memory we can modify.
			if !valObj.CanSet() {
				newObj := reflect.New(valObj.Type())
				newObj = newObj.Elem()
				newObj.Set(valObj)
				valObj = newObj
			}

			if err := overwriteStructValsHelper(tag, key[1:], val, valObj); err != nil {
				return err
			}
			obj.SetMapIndex(keyObj, valObj)
			return nil
		}
	case reflect.Slice:
		if len(key) == 0 {
			// We're supposed to be setting a value here so we shouldn't
			// go into a container recursively
			break
		}

		index, err := strconv.Atoi(key[0])
		if err != nil {
			return fmt.Errorf("could not convert struct index to int: %s (%v)", key[0], err)
		}
		currentLength := obj.Len()
		if index >= currentLength {
			// We have to grow
			newObj := reflect.MakeSlice(obj.Type(), index+1, index+1)
			reflect.Copy(newObj, obj)
			obj.Set(newObj)
		}

		elem := obj.Index(index)
		switch elem.Kind() {
		case reflect.Ptr:
			if elem.IsNil() {
				elemType := elem.Type().Elem()
				elem.Set(reflect.New(elemType))
			}
		case reflect.Map:
			if elem.IsNil() {
				elemType := elem.Type()
				elem.Set(reflect.MakeMap(elemType))
			}
		}
		return overwriteStructValsHelper(tag, key[1:], val, elem)
	}

	if len(key) != 0 {
		return fmt.Errorf("did not reach the end of key but found no container type: %#v (%s)", key, val)
	}

	// We're not a container type
	return setVal(obj, val)
}

// findKeyValues looks for values matching keys
// The input value envs is typically going to be os.Environ
func findKeyValues(envs []string, envPfx string, pseudoKeys []string) map[string]string {
	kvs := make(map[string]string)

	pfxUnderscore := strings.ToUpper(envPfx) + "_"

	for _, e := range envs {
		envKV := strings.SplitN(e, "=", 2)
		if len(envKV) <= 1 {
			// If an env var is set, we don't care, needs to have a value
			continue
		}
		envKey, envVal := envKV[0], envKV[1]
		if len(envKey) == 0 || len(envVal) == 0 {
			// No idea how this could happen, but check anyway
			continue
		}
		if !strings.HasPrefix(envKey, pfxUnderscore) {
			// No match here
			continue
		}

		envKey = envKey[len(pfxUnderscore):]
		if len(envKey) == 0 {
			// Another weird situation
			continue
		}

		for _, pkey := range pseudoKeys {
			found, ok := compareWildcardEnvs(envKey, pkey)
			if ok {
				kvs[found] = envVal
			}
		}
	}

	return kvs
}

// compareWildcardEnvs compares two strings with wildcards
// it returns the matched string (letters found in a wildcard will be downcased)
// whereas all other letters will be the same case as found in pkey
func compareWildcardEnvs(env string, pkey string) (string, bool) {
	var b strings.Builder
	p := strings.ToUpper(pkey)

	// Char by char check that the inputs are the same
	// _ can only match a _ or a .
	// Everything matches * except _
	// [0-9] matches #
	i, j := 0, 0
	for {
		if i >= len(env) || j >= len(p) {
			break
		}

		if env[i] == p[j] || (env[i] == '_' && p[j] == '.') {
			// Using the non-uppercase pkey here allows us to
			// keep case sensitivity for pseudo keys for non wildcard entries
			b.WriteByte(pkey[j])
			i++
			j++
			continue
		}

		switch p[j] {
		case '*':
			if env[i] == '_' {
				j++
			} else {
				b.WriteRune(unicode.ToLower(rune(env[i])))
				i++
			}
		case '#':
			if env[i] == '_' {
				j++
			} else if unicode.IsDigit(rune(env[i])) {
				b.WriteByte(env[i])
				i++
			} else {
				// Not a digit, not a _, this isn't a match
				return "", false
			}
		default:
			// pkey doesn't contain a wildcard, and we're not matching
			// so we've failed
			return "", false
		}
	}

	finishedEnvKey := i == len(env)
	// If pseudo key ends in a * wildcard, we were on it, and env ran out
	// we're also finished.
	finishedPseudoKey := j == len(p) || (j == len(p)-1 && p[j] == '*')

	if finishedEnvKey && finishedPseudoKey {
		return b.String(), true
	}

	return "", false
}

func envPseudoKeys(tag string, obj interface{}) ([]string, error) {
	typ := reflect.TypeOf(obj)

	keys, err := envPseudoKeysHelper(tag, nil, typ)
	if err != nil {
		return nil, err
	}

	return keys, nil
}

func envPseudoKeysHelper(tag string, recurse []string, typ reflect.Type) ([]string, error) {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	switch typ.Kind() {
	case reflect.Struct:
		var keys []string

		// If this is time type we don't recurse
		if typ == timeType {
			break
		}

		n := typ.NumField()
		for i := 0; i < n; i++ {
			field := typ.Field(i)
			name, ok := getTag(field, tag)
			if !ok {
				// We don't deal with missing or explicitly ignored struct tags
				continue
			}

			newRecurse := cloneAndAppend(recurse, name)
			fieldTyp := field.Type

			newKeys, err := envPseudoKeysHelper(tag, newRecurse, fieldTyp)
			if err != nil {
				return nil, err
			}

			keys = append(keys, newKeys...)
		}

		return keys, nil
	case reflect.Map:
		mapElemType := typ.Elem()
		newRecurse := cloneAndAppend(recurse, "*")
		return envPseudoKeysHelper(tag, newRecurse, mapElemType)
	case reflect.Slice:
		// If we're a slice of a container type, recurse, else break
		sliceElemType := typ.Elem()
		sliceElemKind := sliceElemType.Kind()
		if sliceElemKind == reflect.Ptr {
			sliceElemType = sliceElemType.Elem()
			sliceElemKind = sliceElemType.Kind()
		}

		switch sliceElemKind {
		case reflect.Map, reflect.Struct, reflect.Slice:
			newRecurse := cloneAndAppend(recurse, "#")
			return envPseudoKeysHelper(tag, newRecurse, sliceElemType)
		}
	}

	if len(recurse) == 0 {
		return nil, fmt.Errorf("top-level element must be struct/slice/map but got: %s", typ.String())
	}

	key := strings.Join(recurse, ".")
	return []string{key}, nil
}

func getTag(field reflect.StructField, tag string) (string, bool) {
	structTag := field.Tag.Get(tag)

	if len(structTag) == 0 {
		return "", false
	}

	tagParts := strings.Split(structTag, ",")
	name := tagParts[0]
	// We don't deal with unnamed objects in a struct
	if len(name) == 0 || name == "-" {
		return "", false
	}

	return name, true
}

func setVal(val reflect.Value, envVal string) error {
	switch val.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, err := strconv.ParseUint(envVal, 10, 64)
		if err != nil {
			return fmt.Errorf("expected uint but got value: %q", envVal)
		}

		val.SetUint(i)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(envVal, 10, 64)
		if err != nil {
			return fmt.Errorf("expected int but got value: %q", envVal)
		}

		val.SetInt(i)
	case reflect.Bool:
		b, err := strconv.ParseBool(envVal)
		if err != nil {
			return fmt.Errorf("expected bool but got value: %q", envVal)
		}

		val.SetBool(b)
	case reflect.String:
		val.SetString(envVal)
	case reflect.Float64:
		i, err := strconv.ParseFloat(envVal, 64)
		if err != nil {
			return fmt.Errorf("expected float but got value: %q", envVal)
		}

		val.SetFloat(i)
	case reflect.Slice:
		elemType := val.Type().Elem()

		// For each element, append a zero value of it, then try to set it
		// with the corresponding string value in the env var
		splits := strings.Split(envVal, ",")
		for i, s := range splits {
			zero := reflect.Zero(elemType)
			val.Set(reflect.Append(val, zero))

			element := val.Index(i)
			if err := setVal(element, s); err != nil {
				return err
			}
		}
	case reflect.Struct:
		// This should be a time struct
		t, err := time.Parse(time.RFC3339, envVal)
		if err != nil {
			return fmt.Errorf("expected time but got value: %q", envVal)
		}

		val.Set(reflect.ValueOf(t))
	default:
		return fmt.Errorf("type %s not supported", val.Type().String())
	}

	return nil
}

func cloneAndAppend(list []string, item string) []string {
	if len(list) == 0 {
		return []string{item}
	}
	newList := make([]string, len(list)+1)
	copy(newList, list)
	newList[len(newList)-1] = item
	return newList
}
