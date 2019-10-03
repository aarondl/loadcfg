package loadcfg

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

type A struct {
	Int     int       `toml:"int"`
	IntPtr  *int      `toml:"intptr"`
	Strings []string  `toml:"strings"`
	Time    time.Time `toml:"time"`

	Embedded struct {
		Int int `toml:"int"`
	} `toml:"embedded"`

	Map        map[string]B    `toml:"map"`
	MapPtr     map[string]*B   `toml:"mapptr"`
	MapPrim    map[string]int  `toml:"mapprim"`
	MapPrimPtr map[string]*int `toml:"mapprimptr"`

	Slice    []B  `toml:"slice"`
	SlicePtr []*B `toml:"sliceptr"`

	Struct    B  `toml:"struct"`
	StructPtr *B `toml:"structptr"`
	Ignored   *B `toml:"-"`
}

type B struct {
	Float float64 `toml:"float"`
}

func TestTOML(t *testing.T) {
	t.Parallel()

	got := new(A)
	// two.toml doesn't exist, we're explicitly checking env only
	_, err := TOML("test0", "testdata/one.toml", got)
	if err != nil {
		t.Error(err)
	}

	if got.Int != 5 {
		t.Error("int wrong:", got.Int)
	}
	if g := got.Map["one"].Float; g != 4.5 {
		t.Error("map float wrong:", g)
	}
	if g := got.Map["two"].Float; g != 4.5 {
		t.Error("map float wrong:", g)
	}
	if g := got.MapPtr["one"].Float; g != 4.5 {
		t.Error("mapptr float wrong:", g)
	}
	if g := got.MapPtr["two"].Float; g != 4.5 {
		t.Error("mapptr float wrong:", g)
	}
	if g := got.MapPrim["one"]; g != 1 {
		t.Error("mapprim int wrong:", g)
	}
	if g := got.MapPrim["two"]; g != 1 {
		t.Error("mapprim int wrong:", g)
	}
	if g := *got.MapPrimPtr["one"]; g != 1 {
		t.Error("mapprimptr int wrong:", g)
	}
	if g := *got.MapPrimPtr["two"]; g != 1 {
		t.Error("mapprimptr int wrong:", g)
	}
	if g := got.Slice[0].Float; g != 4.5 {
		t.Error("slice float wrong:", g)
	}
	if g := got.Slice[1].Float; g != 4.5 {
		t.Error("slice float wrong:", g)
	}
}

func TestTOMLWithOverride(t *testing.T) {
	got := new(A)

	keys := setEnvs(
		"TEST1_INT", "6",
		"TEST1_MAP_ONE_FLOAT", "5.5",
		"TEST1_MAPPTR_ONE_FLOAT", "5.5",
		"TEST1_MAPPRIM_ONE", "2",
		"TEST1_MAPPRIMPTR_ONE", "2",
		"TEST1_SLICE_0_FLOAT", "5.5",
	)

	defer unsetEnvs(keys)

	// load real one.toml and override all its values
	_, err := TOML("test1", "testdata/one.toml", got)
	if err != nil {
		t.Error(err)
	}

	if got.Int != 6 {
		t.Error("int wrong:", got.Int)
	}
	if g := got.Map["one"].Float; g != 5.5 {
		t.Error("map float wrong:", g)
	}
	if g := got.Map["two"].Float; g != 4.5 {
		t.Error("map float wrong:", g)
	}
	if g := got.MapPtr["one"].Float; g != 5.5 {
		t.Error("mapptr float wrong:", g)
	}
	if g := got.MapPtr["two"].Float; g != 4.5 {
		t.Error("mapptr float wrong:", g)
	}
	if g := got.MapPrim["one"]; g != 2 {
		t.Error("mapprim int wrong:", g)
	}
	if g := got.MapPrim["two"]; g != 1 {
		t.Error("mapprim int wrong:", g)
	}
	if g := *got.MapPrimPtr["one"]; g != 2 {
		t.Error("mapprimptr int wrong:", g)
	}
	if g := *got.MapPrimPtr["two"]; g != 1 {
		t.Error("mapprimptr int wrong:", g)
	}
	if g := got.Slice[0].Float; g != 5.5 {
		t.Error("slice float wrong:", g)
	}
	if g := got.Slice[1].Float; g != 4.5 {
		t.Error("slice float wrong:", g)
	}
}

func TestTOMLOnlyEnv(t *testing.T) {
	date := time.Date(2009, 11, 10, 23, 0, 0, 0, time.UTC)

	keys := setEnvs(
		"TEST2_INT", "5",
		"TEST2_INTPTR", "5",
		"TEST2_STRINGS", "one,two,three",
		"TEST2_TIME", date.Format(time.RFC3339),

		// Inline struct
		"TEST2_EMBEDDED_INT", "6",

		// Maps
		"TEST2_MAP_ONE_FLOAT", "4.5",
		"TEST2_MAP_TWO_FLOAT", "5.5",
		"TEST2_MAPPTR_ONE_FLOAT", "6.5",
		"TEST2_MAPPTR_TWO_FLOAT", "7.5",

		"TEST2_MAPPRIM_ONE", "1",
		"TEST2_MAPPRIM_TWO", "2",
		"TEST2_MAPPRIMPTR_ONE", "3",
		"TEST2_MAPPRIMPTR_TWO", "4",

		// Slices
		"TEST2_SLICE_0_FLOAT", "8.5",
		"TEST2_SLICE_1_FLOAT", "9.5",
		"TEST2_SLICEPTR_0_FLOAT", "10.5",
		"TEST2_SLICEPTR_1_FLOAT", "11.5",

		// Struct things
		"TEST2_STRUCT_FLOAT", "12.5",
		"TEST2_STRUCTPTR_FLOAT", "13.5",

		// Ignore
		"TEST2_-_FLOAT", "13.5",
	)

	defer unsetEnvs(keys)

	got := new(A)
	// two.toml doesn't exist, we're explicitly checking env only
	_, err := TOML("test2", "testdata/two.toml", got)
	if err != nil {
		t.Error(err)
	}

	int3 := 3
	int4 := 4
	int5 := 5

	want := &A{
		Int:     5,
		IntPtr:  &int5,
		Strings: []string{"one", "two", "three"},
		Time:    date,
		Map: map[string]B{
			"one": B{Float: 4.5},
			"two": B{Float: 5.5},
		},
		MapPtr: map[string]*B{
			"one": &B{Float: 6.5},
			"two": &B{Float: 7.5},
		},

		MapPrim: map[string]int{
			"one": 1,
			"two": 2,
		},
		MapPrimPtr: map[string]*int{
			"one": &int3,
			"two": &int4,
		},

		Slice:    []B{{Float: 8.5}, {Float: 9.5}},
		SlicePtr: []*B{&B{Float: 10.5}, &B{Float: 11.5}},

		Struct:    B{Float: 12.5},
		StructPtr: &B{Float: 13.5},
	}

	want.Embedded.Int = 6

	if !reflect.DeepEqual(want, got) {
		t.Errorf("structs differ:\nwant:\n%v\n\ngot:\n%v\n", want, got)
	}
}

func TestEnv(t *testing.T) {
	date := time.Date(2009, 11, 10, 23, 0, 0, 0, time.UTC)

	keys := setEnvs(
		"TEST3_INT", "5",
		"TEST3_INTPTR", "5",
		"TEST3_STRINGS", "one,two,three",
		"TEST3_TIME", date.Format(time.RFC3339),

		// Inline struct
		"TEST3_EMBEDDED_INT", "6",

		// Maps
		"TEST3_MAP_ONE_FLOAT", "4.5",
		"TEST3_MAP_TWO_FLOAT", "5.5",
		"TEST3_MAPPTR_ONE_FLOAT", "6.5",
		"TEST3_MAPPTR_TWO_FLOAT", "7.5",

		"TEST3_MAPPRIM_ONE", "1",
		"TEST3_MAPPRIM_TWO", "2",
		"TEST3_MAPPRIMPTR_ONE", "3",
		"TEST3_MAPPRIMPTR_TWO", "4",

		// Slices
		"TEST3_SLICE_0_FLOAT", "8.5",
		"TEST3_SLICE_1_FLOAT", "9.5",
		"TEST3_SLICEPTR_0_FLOAT", "10.5",
		"TEST3_SLICEPTR_1_FLOAT", "11.5",

		// Struct things
		"TEST3_STRUCT_FLOAT", "12.5",
		"TEST3_STRUCTPTR_FLOAT", "13.5",

		// Ignore
		"TEST3_-_FLOAT", "13.5",
	)

	defer unsetEnvs(keys)

	got := new(A)
	err := Env("test3", "toml", got)
	if err != nil {
		t.Error(err)
	}

	int3 := 3
	int4 := 4
	int5 := 5

	want := &A{
		Int:     5,
		IntPtr:  &int5,
		Strings: []string{"one", "two", "three"},
		Time:    date,
		Map: map[string]B{
			"one": B{Float: 4.5},
			"two": B{Float: 5.5},
		},
		MapPtr: map[string]*B{
			"one": &B{Float: 6.5},
			"two": &B{Float: 7.5},
		},

		MapPrim: map[string]int{
			"one": 1,
			"two": 2,
		},
		MapPrimPtr: map[string]*int{
			"one": &int3,
			"two": &int4,
		},

		Slice:    []B{{Float: 8.5}, {Float: 9.5}},
		SlicePtr: []*B{&B{Float: 10.5}, &B{Float: 11.5}},

		Struct:    B{Float: 12.5},
		StructPtr: &B{Float: 13.5},
	}

	want.Embedded.Int = 6

	if !reflect.DeepEqual(want, got) {
		t.Errorf("structs differ:\nwant:\n%v\n\ngot:\n%v\n", want, got)
	}
}

func TestNonStructs(t *testing.T) {
	t.Parallel()

	obj := make(map[string]int)

	err := overwriteStructVals("", map[string]string{"one": "1"}, obj)
	if err != nil {
		t.Fatal(err)
	}

	if obj["one"] != 1 {
		t.Error("expected a value to be set")
	}

	sliceObj := make([]B, 0, 0)
	err = overwriteStructVals("toml", map[string]string{"0.float": "1.0"}, &sliceObj)
	if err != nil {
		t.Fatal(err)
	}

	if sliceObj[0].Float != 1.0 {
		t.Error("slice not set correctly")
	}
}

func TestFindKeyValues(t *testing.T) {
	expect := map[string]string{
		"array":        "one,two,three",
		"multi.sep":    "one,two,three",
		"map.one.var0": "var10",
		"map.one.var1": "var11",
		"map.two.var0": "var20",
		"map.two.var1": "var21",
		"arr.0.var0":   "var00",
		"arr.0.var1":   "var01",
		"arr.1.var0":   "var10",
		"arr.1.var1":   "var11",
	}

	envs := fakeEnvs(
		"X_ARRAY", "one,two,three",
		"X_MULTI_SEP", "one,two,three",
		"X_MAP_ONE_VAR0", "var10",
		"X_MAP_ONE_VAR1", "var11",
		"X_MAP_TWO_VAR0", "var20",
		"X_MAP_TWO_VAR1", "var21",
		"X_ARR_0_VAR0", "var00",
		"X_ARR_0_VAR1", "var01",
		"X_ARR_1_VAR0", "var10",
		"X_ARR_1_VAR1", "var11",
	)

	kvs := findKeyValues(envs, "x", []string{
		"array",
		"multi.sep",
		"int",
		"map.*.var0",
		"map.*.var1",
		"arr.#.var0",
		"arr.#.var1",
	})

	if len(expect) != len(kvs) {
		t.Errorf("\nwant: %v\ngot: %v", expect, kvs)
	}
	for k, v := range expect {
		vv, ok := kvs[k]
		if !ok {
			t.Errorf("key %s was not in output", k)
			continue
		}

		if v != vv {
			t.Errorf("value wrong, want: %s, got: %s", v, vv)
		}
	}
}

func TestCompareWildcardEnvs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Env   string
		Pkey  string
		Out   string
		Match bool
	}{
		{"", "", "", true},
		{"HELLO", "", "", false},
		{"", "HELLO", "", false},
		{"A", "b", "", false},
		{"A", "a", "a", true},
		{"A_B", "*.b", "a.b", true},
		{"HELLO_THERE_FRIEND", "hello.*.friend", "hello.there.friend", true},
		{"HELLO_0_FRIEND", "hello.#.friend", "hello.0.friend", true},
		{"HELLO_10_FRIEND", "hello.#.friend", "hello.10.friend", true},
		{"HELLO_THERE_FRIEND", "hello.#.friend", "", false},
		{"HELLO_THERE_GUY_FRIEND", "hello.*.friend", "", false},
		{"HELLO_THERE_FRIEND", "hello.there.friend", "hello.there.friend", true},
	}

	for i, test := range tests {
		out, matched := compareWildcardEnvs(test.Env, test.Pkey)
		if test.Match != matched {
			t.Errorf("%d) matched wrong, want: %t, got: %t", i, test.Match, matched)
		} else if matched && test.Out != out {
			t.Errorf("%d) out wrong, want: %q, got: %q", i, test.Out, out)
		}
	}
}

func TestEnvPseudoKeys(t *testing.T) {
	t.Parallel()

	required := []string{
		"int",
		"strings",
		"embedded.int",

		"map.*.float",
		"mapptr.*.float",

		"slice.#.float",
		"sliceptr.#.float",

		"struct.float",
		"structptr.float",
	}

	keys, err := envPseudoKeys("toml", &A{})
	if err != nil {
		t.Fatal(err)
	}

	for i, want := range required {
		found := false
		for _, have := range keys {
			if want == have {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("%d) did not find key: %s", i, want)
		}
	}
}

func fakeEnvs(kvPairs ...string) (keys []string) {
	env := os.Environ()
	for i := 0; i < len(kvPairs)-1; i += 2 {
		env = append(env, fmt.Sprintf("%s=%s", kvPairs[i], kvPairs[i+1]))
	}

	return env
}

func setEnvs(kvPairs ...string) (keys []string) {
	for i := 0; i < len(kvPairs)-1; i += 2 {
		keys = append(keys, kvPairs[i])
		os.Setenv(kvPairs[i], kvPairs[i+1])
	}

	return keys
}

func unsetEnvs(keys []string) {
	for _, k := range keys {
		os.Setenv(k, "")
	}
}
