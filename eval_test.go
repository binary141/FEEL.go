package feel

import (
	"fmt"
	"testing"
	"time"

	"gotest.tools/assert"
)

type evalPair struct {
	input   string
	expect  any
	context string
}

func TestEvalPairs(t *testing.T) {
	//assert0 := assert.New(t)
	evalPairs := []evalPair{
		// empty input outputs nil
		{"", nil, ""},

		{"5 + -6", N(-1), ""},
		{"5 + 6", N(11), ""},
		{"(function(a) 2 * a)(5)", N(10), ""},
		{"true", true, ""},
		{"false", false, ""},
		{`"hello" + " world"`, "hello world", ""},

		{`{a if c: "hello", b: "world"}`, map[string]any{"a if c": "hello", "b": "world"}, ""},

		// in range and array
		{`5 in (5..8]`, false, ""},
		{`5 in [5..8)`, true, ""},
		{`8 in [5..8)`, false, ""},
		{`8 in [5..8]`, true, ""},

		{`"a" in ["a".."z"]`, true, ""},
		{`5 in [3,5, 8]`, true, ""},
		{`5 in [3, 6, 8]`, false, ""},
		{`5 in []`, false, ""},
		//{`not(5 in [3, 5, 9])`, false, ""},

		// if then else
		{`if a > 3 then "larger" else "smaller"`, "larger", "{a: 5}"},
		{`if a = 5 then "equal" else "not equal"`, "equal", "{a: 5}"},
		{`if a b = 5 then "equal" else "not equal"`, "equal", "{a b: 5}"}, // a name has multiple chunks

		// test not
		{`not( 5 >  6)`, true, ""},

		// loop functions
		{`some x in [3, 4, 5] satisfies x >= 4`, N(4), ""},
		{`every y in [3, 4, 5] satisfies y >= 4`, []any{N(4), N(5)}, ""},

		// exponent
		{`3 ** 2`, N(9), ""},
		{`2 ** 10`, N(1024), ""},
		{`"foo" ** 4`, Null, ""},
		{`true ** 4`, Null, ""},

		// null check
		{`a != null and a.b > 10`, false, ""},
		{`a = null or a.b > 10`, true, ""},

		// keyword arguments
		{`sub(a: 4, b: 2)`, N(2), "{sub: (function(a, b) a - b)}"},

		// today() arg handling
		{`today() instance of date`, true, ""},
		{`today(123)`, Null, ""},

		// now() arg handling
		{`now() instance of date and time`, true, ""},
		{`now(123)`, Null, ""},

		// instance of
		{`123 instance of number`, true, ""},
		{`"hello" instance of string`, true, ""},
		{`true instance of bool`, true, ""},
		{`@"2023-06-07" instance of datetime`, true, ""},
		{`date("2023-06-07") instance of date`, true, ""},
		{`date("2023-06-07") instance of datetime`, false, ""},

		// round up
		{`round up(5.5, 0)`, N(6), ""},
		{`round up(-5.5, 0)`, N(-6), ""},
		{`round up(1.121, 2)`, N("1.13"), ""},
		{`round up(-1.126, 2)`, N("-1.13"), ""},
		{`round up()`, Null, ""},
		{`round up(null, 0)`, Null, ""},
		{`round up(1234.12, null)`, Null, ""},
		{`round up(1234.12, 1, 2)`, Null, ""},
		{`round up(n: 5.5, scale: 0)`, N(6), ""},
		{`round up(scale: 0)`, Null, ""},
		{`round up(n: 5.5, scale: 0, foo: 123)`, Null, ""},
		{`round up("123", 0)`, Null, ""},
		{`round up(5.5, "0")`, Null, ""},
		{`round up(5.5, (-6111 - 1))`, Null, ""},
		{`round up(5.5, 6176)`, N("5.5"), ""},
		{`round up(5.5, (6176 + 1))`, Null, ""},

		// round half down
		{`round half down(5.5, 0)`, N(5), ""},
		{`round half down(-5.5, 0)`, N(-5), ""},
		{`round half down(1.121, 2)`, N("1.12"), ""},
		{`round half down(-1.126, 2)`, N("-1.13"), ""},
		{`round half down()`, Null, ""},
		{`round half down(null, 0)`, Null, ""},
		{`round half down(1234, null)`, Null, ""},
		{`round half down(1234.12, 1, 2)`, Null, ""},
		{`round half down(n: 5.5, scale: 0)`, N(5), ""},
		{`round half down(scale: 0)`, Null, ""},
		{`round half down(n: 5.5, scale: 0, foo: 123)`, Null, ""},
		{`round half down("123", 0)`, Null, ""},
		{`round half down(5.5, "0")`, Null, ""},
		{`round half down(5.5, (-6111 - 1))`, Null, ""},
		{`round half down(5.5, 6176)`, N("5.5"), ""},
		{`round half down(5.5, (6176 + 1))`, Null, ""},

		// round half up
		{`round half up(5.5, 0)`, N(6), ""},
		{`round half up(-5.5, 0)`, N(-6), ""},
		{`round half up(1.121, 2)`, N("1.12"), ""},
		{`round half up(-1.126, 2)`, N("-1.13"), ""},
		{`round half up()`, Null, ""},
		{`round half up(null, 0)`, Null, ""},
		{`round half up(1234, null)`, Null, ""},
		{`round half up(1234.12, 1, 2)`, Null, ""},
		{`round half up(n: 5.5, scale: 0)`, N(6), ""},
		{`round half up(scale: 0)`, Null, ""},
		{`round half up(n: 5.5, scale: 0, foo: 123)`, Null, ""},
		{`round half up("123", 0)`, Null, ""},
		{`round half up(5.5, "0")`, Null, ""},
		{`round half up(5.5, (-6111 - 1))`, Null, ""},
		{`round half up(5.5, 6176)`, N("5.5"), ""},
		{`round half up(5.5, (6176 + 1))`, Null, ""},

		// round down
		{`round down(5.5, 0)`, N(5), ""},
		{`round down(-5.5, 0)`, N(-5), ""},
		{`round down(1.121, 2)`, N("1.12"), ""},
		{`round down(-1.126, 2)`, N("-1.12"), ""},
		{`round down()`, Null, ""},
		{`round down(null, 0)`, Null, ""},
		{`round down(1234, null)`, Null, ""},
		{`round down(1234.12, 1, 2)`, Null, ""},
		{`round down(n: 5.5, scale: 0)`, N(5), ""},
		{`round down(scale: 0)`, Null, ""},
		{`round down(n: 5.5, scale: 0, foo: 123)`, Null, ""},
		{`round down("123", 0)`, Null, ""},
		{`round down(5.5, "0")`, Null, ""},
		{`round down(5.5, (-6111 - 1))`, Null, ""},
		{`round down(5.5, 6176)`, N("5.5"), ""},
		{`round down(5.5, (6176 + 1))`, Null, ""},
		{`round down(-5.5, 0)`, N(-5), ""},
		{`round down(-1.126, 2)`, N("-1.12"), ""},
		{`round up(-1.126, 2)`, N("-1.13"), ""},
		{`round up(-5.5, 0)`, N(-6), ""},

		// temporal expressions
		{`last day of month(@"2020-02-11")`, N(29), ""},
		{`last day of month(@"2021-01-07")`, N(31), ""},
		{`last day of month(@"2023-06-11")`, N(30), ""},
		{`last day of month(@"2023-07-11")`, N(31), ""},

		{`@"2023-07-21T13:57:32@CST" - @"PT2H3M"`, MustParseDatetime("2023-07-21T11:54:32@CST"), ""}, // test day/hour/min duration
		{`@"2023-06-01T10:33:20@CST" + @"P3Y11M"`, MustParseDatetime("2027-05-01T10:33:20@CST"), ""}, // test year/month duration

		// years and months duration
		{`years and months duration(null)`, Null, ""},
		{`years and months duration(null,null)`, Null, ""},
		{`years and months duration(date("2017-08-11"),null)`, Null, ""},
		{`years and months duration(date and time("2017-12-31T13:00:00"),null)`, Null, ""},
		{`years and months duration(null,date("2017-08-11"))`, Null, ""},
		{`years and months duration(null,date and time("2019-10-01T12:32:59"))`, Null, ""},
		{`years and months duration()`, Null, ""},
		{`years and months duration(date("2011-12-22"),date("2013-08-24"))`, MustParseDuration("P1Y8M"), ""},
		{`years and months duration(date("2013-08-24"),date("2011-12-22"))`, MustParseDuration("-P1Y8M"), ""},
		{`years and months duration(date("2015-01-21"),date("2016-01-21"))`, MustParseDuration("P1Y"), ""},
		{`years and months duration(date("2016-01-21"),date("2015-01-21"))`, MustParseDuration("-P1Y"), ""},
		{`years and months duration(date("2016-01-01"),date("2016-01-01"))`, MustParseDuration("P0M"), ""},
		{`years and months duration(date and time("2017-12-31T13:00:00"), date and time("2017-12-31T12:00:00"))`, MustParseDuration("P0M"), ""},
		{`years and months duration(date and time("2016-09-30T23:25:00"), date and time("2017-12-28T12:12:12"))`, MustParseDuration("P1Y2M"), ""},
		{`years and months duration(date and time("2010-05-30T03:55:58"), date and time("2017-12-15T00:59:59"))`, MustParseDuration("P7Y6M"), ""},
		{`years and months duration(date and time("2014-12-31T23:59:59"), date and time("2019-10-01T12:32:59"))`, MustParseDuration("P4Y9M"), ""},
		{`years and months duration(date and time("-2016-01-30T09:05:00"), date and time("-2017-02-28T02:02:02"))`, MustParseDuration("-P11M"), ""},
		{`years and months duration(date and time("2014-12-31T23:59:59"), date and time("-2019-10-01T12:32:59"))`, MustParseDuration("-P4033Y2M"), ""},
		{`years and months duration(date and time("2017-09-05T10:20:00-01:00"), date and time("-2019-10-01T12:32:59+02:00"))`, MustParseDuration("-P4035Y11M"), ""},
		{`years and months duration(date and time("2017-09-05T10:20:00+05:00"), date and time("2019-10-01T12:32:59"))`, MustParseDuration("P2Y"), ""},
		{`years and months duration(date and time("2016-08-25T15:20:59+02:00"), date and time("2017-08-10T10:20:00@Europe/Paris"))`, MustParseDuration("P11M"), ""},
		{`years and months duration(date and time("2011-12-31T10:15:30@Etc/UTC"), date and time("2017-08-10T10:20:00@Europe/Paris"))`, MustParseDuration("P5Y7M"), ""},
		{`years and months duration(date and time("2017-09-05T10:20:00@Etc/UTC"), date and time("2018-10-01T23:59:59"))`, MustParseDuration("P1Y"), ""},
		{`years and months duration(date and time("2011-08-25T15:59:59@Europe/Paris"), date and time("2015-08-25T15:20:59+02:00"))`, MustParseDuration("P4Y"), ""},
		{`years and months duration(date and time("2015-12-31T23:59:59.9999999"), date and time("2018-10-01T12:32:59.111111"))`, MustParseDuration("P2Y9M"), ""},
		{`years and months duration(date and time("2016-09-05T22:20:55.123456+05:00"), date and time("2019-10-01T12:32:59.32415645"))`, MustParseDuration("P3Y"), ""},
		{`years and months duration(date(""),date(""))`, Null, ""},
		{`years and months duration(2017)`, Null, ""},
		{`years and months duration("2012T-12-2511:00:00Z")`, Null, ""},
		{`years and months duration([],[])`, Null, ""},
		{`years and months duration(date("2013-08-24"), date and time("2017-12-15T00:59:59"))`, MustParseDuration("P4Y3M"), ""},
		{`years and months duration(date and time("2017-02-28T23:59:59"), date("2019-07-23"))`, MustParseDuration("P2Y4M"), ""},
		{`years and months duration(from:date and time("2016-12-31T00:00:01"),to:date and time("2017-12-31T23:59:59"))`, MustParseDuration("P1Y"), ""},
		{`years and months duration(from:date and time("2014-12-31T23:59:59"),to:date and time("2016-12-31T00:00:01"))`, MustParseDuration("P2Y"), ""},
		{`years and months duration(from:date("2011-12-22"),to:date("2013-08-24"))`, MustParseDuration("P1Y8M"), ""},
		{`years and months duration(from:date("2016-01-21"),to:date("2015-01-21"))`, MustParseDuration("-P1Y"), ""},

		// builtin functions
		{`is defined(x)`, false, ""},
		{`is defined(x[5])`, false, "{x: [1, 2, 3]}"},
		{`is defined(x.c)`, false, "{x: {a: 3, b: 5}}"},
		{`is defined(x.a)`, true, "{x: {a: 3, b: 5}}"},

		{`is defined(x)`, true, "{x: 666}"},        // `x` is bound
		{`is defined(value: x)`, true, "{x: 888}"}, // macro can use keyword arguments

		{`substring(string: "abcdef", start position: 3, length: 3)`, "cde", ""},
		{`substring(string: "abcdef", start position: 200, length: 3)`, "", ""},
		{`substring("foobar", -2, 1)`, "a", ""},
		{`substring("foob r", -2, 1)`, " ", ""},
		{`substring("foobar", -6, 6)`, "foobar", ""},
		{`substring("foobar", 3, 3.8)`, "oba", ""},
		{`substring(string: "foobar", start position: 3)`, "obar", ""},

		// string() conversion — argument count
		{`string()`, Null, ""},
		{`string("foo", "bar")`, Null, ""},
		{`string(from: "foo")`, "foo", ""},

		// string() — type coercion
		{`string(null)`, Null, ""},
		{`string("foo")`, "foo", ""},
		{`string(123.45)`, "123.45", ""},
		{`string(true)`, "true", ""},
		{`string(false)`, "false", ""},

		// string() — date/time/datetime
		{`string(date("2018-12-10"))`, "2018-12-10", ""},
		{`string(date and time("2018-12-10"))`, "2018-12-10T00:00:00", ""},
		{`string(date and time("2018-12-10T10:30:00.0001"))`, "2018-12-10T10:30:00.0001", ""},
		{`string(date and time("2018-12-10T10:30:00.0001+05:00:01"))`, "2018-12-10T10:30:00.0001+05:00:01", ""},
		{`string(date and time("2018-12-10T10:30:00@Etc/UTC"))`, "2018-12-10T10:30:00@Etc/UTC", ""},
		{`string(time("10:30:00.0001"))`, "10:30:00.0001", ""},
		{`string(time("10:30:00.0001+05:00:01"))`, "10:30:00.0001+05:00:01", ""},
		{`string(time("10:30:00@Etc/UTC"))`, "10:30:00@Etc/UTC", ""},

		// string() — duration
		{`string(duration("P1D"))`, "P1D", ""},
		{`string(duration("-P1D"))`, "-P1D", ""},
		{`string(duration("P0D"))`, "PT0S", ""},
		{`string(duration("P1DT2H3M4.1234S"))`, "P1DT2H3M4.1234S", ""},
		{`string(duration("PT49H"))`, "P2DT1H", ""},
		{`string(duration("P1Y"))`, "P1Y", ""},
		{`string(duration("-P1Y"))`, "-P1Y", ""},
		{`string(duration("P0Y"))`, "P0Y", ""},
		{`string(duration("P1Y2M"))`, "P1Y2M", ""},
		{`string(duration("P25M"))`, "P2Y1M", ""},

		// string() — list and context
		{`string([1, 2, 3, "foo"])`, `[1, 2, 3, "foo"]`, ""},
		{`string([1, 2, 3, [4, 5, "foo"]])`, `[1, 2, 3, [4, 5, "foo"]]`, ""},
		{"string([\"\\\"foo\\\"\"])", `["\"foo\""]`, ""},
		{`string({a: "foo"})`, `{a: "foo"}`, ""},
		{`string({a: "foo", b: {bar: "baz"}})`, `{a: "foo", b: {bar: "baz"}}`, ""},
		{"string({\"{\":\"foo\"})", `{"{": "foo"}`, ""},
		{`string({":": "foo"})`, `{":": "foo"}`, ""},
		{"string({\",\":\"foo\"})", `{",": "foo"}`, ""},
		{"string({\"}\":\"foo\"})", `{"}": "foo"}`, ""},
		{"string({\"\\\"\":\"foo\"})", `{"\"": "foo"}`, ""},

		{`not({})`, true, ""},
		{`not({a: 1})`, false, ""},

		// list functions
		{`mean([1, 2, 3])`, N(2), ""},
		{`mean(null)`, Null, ""},
		{`mean([1, null, 3])`, Null, ""},

		{`median([3, 5, 9, 1, "hello", -2])`, N(3), ""},

		{`sublist(["a","b","c"], 1, 2)`, []any{"a", "b"}, ""},
		{`sublist(["a","b","c"], -1, 1)`, []any{"c"}, ""},
		{`sublist(["a","b","c"], -2, 2)`, []any{"b", "c"}, ""},
		{`sublist(["a","b","c"], -1)`, []any{"c"}, ""},

		{`append(["hello"], " ", "world")`, []any{"hello", " ", "world"}, ""},
		{`concatenate([2, 1], [3])`, []any{N(2), N(1), N(3)}, ""},
		{`insert before(["hello", "world"], 2, "another")`, []any{"hello", "another", "world"}, ""},
		{`remove(["hello", "a", "world"], 2)`, []any{"hello", "world"}, ""},

		{`index of([1,2,3,2],2)`, []any{N(2), N(4)}, ""},

		{`distinct values([1, 2, 1, 2, 3, 2, 1])`, []any{N(1), N(2), N(3)}, ""},
		{`flatten([["a"], [["b", ["c"]]], ["d"]])`, []any{"a", "b", "c", "d"}, ""},
		{`union(["a", "b"], ["b", "c"], ["d"])`, []any{"a", "b", "c", "d"}, ""},

		{`sort(["hello", "a", "world"], function(x, y) x < y)`, []any{"a", "hello", "world"}, ""},
		{`sort([8, -1, 3], function(x, y) x > y)`, []any{N(8), N(3), N(-1)}, ""},

		// list replace
		{`list replace([1,2,3], 2, 4)`, []any{N(1), N(4), N(3)}, ""},
		{`list replace([1,2,3], -1, 4)`, []any{N(1), N(2), N(4)}, ""},
		{`list replace([1,2,3], 0, 4)`, Null, ""},
		{`list replace([1,2,3], 4, 4)`, Null, ""},
		{`list replace([1,2,3], -4, 4)`, Null, ""},
		{`list replace(null, 1, 4)`, Null, ""},
		{`list replace([1,2,3], null, 4)`, Null, ""},
		{`list replace([1,2,3], 3, null)`, []any{N(1), N(2), Null}, ""},
		{`list replace([2, 4, 7, 8], function(item, newItem) item < newItem, 5)`, []any{N(5), N(5), N(7), N(8)}, ""},
		{`list replace([1,2,3], "2", 4)`, Null, ""},
		{`list replace([1,2,3], 2.5, 4)`, []any{N(1), N(4), N(3)}, ""},
		{`list replace([1,2,3], -1.5, 4)`, []any{N(1), N(2), N(4)}, ""},
		{`list replace(position: 2, newItem: 4, list: [1,2,3])`, []any{N(1), N(4), N(3)}, ""},
		{`list replace(match: function(item, newItem) item = 2, newItem: 4, list: [1,2,3])`, []any{N(1), N(4), N(3)}, ""},
		{`list replace([1,2,3], "2", 4, 4)`, Null, ""},
		{`list replace([1,2,3], "2")`, Null, ""},
		{`list replace(position: 2, newItem: 4, list: [1,2,3], foo: 1)`, Null, ""},
		{`list replace([2, 4], function(item, newItem, extraParam) item = 2, 5)`, Null, ""},
		{`list replace([2, 4], function(item) item = 2, 5)`, Null, ""},
		{`list replace([2, 4], function(item, newItem) item, 5)`, Null, ""},
		{`list replace([1, 2, 3, 4], function(item, newItem) true, 5)`, []any{N(5), N(5), N(5), N(5)}, ""},
		{`list replace(1, 1, 5)`, []any{N(5)}, ""},

		{`string join(["hello", "world"])`, "helloworld", ""},
		{`string join(["hello", "world"], " ", "[", "]")`, Null, ""},
		{`string join(123, "X")`, Null, ""},
		{`string join(["A", "b", "c"], ["d"])`, Null, ""},
		{`string join(["A", "b", "c"])`, "Abc", ""},
		{`string join("A", "b", "c")`, Null, ""},
		{`string join(["A"])`, "A", ""},
		{`string join(["a","b","c"], null)`, "abc", ""},
		{`string join("a", "X")`, "a", ""},
		{`string join(["a","b","c"], "_and_")`, "a_and_b_and_c", ""},
		{`string join(["a","b","c"], "")`, "abc", ""},
		{`string join(["a"], "X")`, "a", ""},
		{`string join(["a",null,"c"], "X")`, "aXc", ""},
		{`string join([], "X")`, "", ""},
		{`string join()`, Null, ""},
		{`string join(list: ["a","c"], delimiter: "X")`, "aXc", ""},
		{`string join(lst: ["a","c"], delimiter: "X")`, Null, ""},
		{`string join(list: ["a","c"], delimitr: "X")`, Null, ""},

		{`or([false, 0, true, false, 1])`, true, ""},
		{`and([false, 0, true, false, 1])`, false, ""},
		{`and([true, 1, true, "ok"])`, true, ""},

		// context/map functions
		{`get value({a: 2}, "b")`, Null, ""},
		{`get value({a: 2}, "a")`, N(2), ""},
		{`get value({a: {b: {c: 4}}}, ["a", "b", "c"])`, N(4), ""},
		{`get value({a: {b: {c: 4}}}, ["a", "b"])`, map[string]any{"c": N(4)}, ""},
		{`get value({a: {b: {c: 4}}}, ["a", "k"])`, Null, ""},
		{`get value(context put({a: false}, ["b", "c", "d"], 4), ["b", "c"])`, map[string]any{"d": N(4)}, ""},
		{`context put({}, "a")`, Null, ""},
		{`context put({}, null, 1)`, Null, ""},
		{`context put([], "a", 1)`, Null, ""},
		{`context put(context: {}, ky: "a", value: 1)`, Null, ""},
		{`context put(context: {}, key: "a", value: 1)`, map[string]any{"a": N(1)}, ""},
		{`context put({}, "a", 1, 1)`, Null, ""},
		{`context merge([{x:1, y: 0}, {y:2}])`, map[string]any{"x": N(1), "y": N(2)}, ""},
		{`context merge([{a: 1}])`, map[string]any{"a": N(1)}, ""},
		{`context merge([{}]) = {}`, true, ""},
		{`context merge([{a: 1}, {b: 2}])`, map[string]any{"a": N(1), "b": N(2)}, ""},
		{`context merge([{a: 1}, {a: 2}])`, map[string]any{"a": N(2)}, ""},
		{`context merge([{a: {aa: 1}}, {a: {bb: 2}}])`, map[string]any{"a": map[string]any{"bb": N(2)}}, ""},
		{`context merge(null)`, Null, ""},
		{`context merge()`, Null, ""},
		{`context merge([], "foo")`, Null, ""},
		{`context merge(contexts: [{a: 1}])`, map[string]any{"a": N(1)}, ""},
		{`context merge(context: [{a: 1}])`, Null, ""},
		{`context merge([1, 2, 3])`, Null, ""},
		{`context merge([{a: 1}, 2, {b: 2}])`, Null, ""},
		{`context merge({a: 1})`, Null, ""},
		{`context merge(contexts: {a: 1})`, Null, ""},

		// context() — build context from entries
		{`context([{key:"a", value:1}, {key:"b", value:2}])`, map[string]any{"a": N(1), "b": N(2)}, ""},
		{`context([{key:"a", value:1}])`, map[string]any{"a": N(1)}, ""},
		{`context([{key:"a", value:1},{key:"a", value:2}])`, Null, ""},
		{`context([]) = {}`, true, ""},
		{`context({key:"a", value:1})`, map[string]any{"a": N(1)}, ""},
		{`context({value:1})`, Null, ""},
		{`context({key: null, value:1})`, Null, ""},
		{`context({key: "a"})`, Null, ""},
		{`context({key: "a", value: null})`, map[string]any{"a": Null}, ""},
		{`context({key: "", value: 1})`, Null, ""},
		{`context(null)`, Null, ""},
		{`context()`, Null, ""},
		{`context([], "foo")`, Null, ""},
		{`context(entries: [{key:"a", value:1}])`, map[string]any{"a": N(1)}, ""},
		{`context(entries: {key:"a", value:1})`, map[string]any{"a": N(1)}, ""},
		{`context(entris: {key:"a", value:1})`, Null, ""},
		{`context("foo")`, Null, ""},
		{`context(entries: [{key:"a", value:1, ignored: "foo"}])`, map[string]any{"a": N(1)}, ""},

		// range functions
		{`before(1, 10)`, true, ""},
		{`before(10, 1)`, false, ""},
		{`before([1..5], 10)`, true, ""},
		{`before(1, [2..5])`, true, ""},
		{`before(3, [2..5])`, false, ""},

		{`before([1..5),[5..10])`, true, ""},
		{`before([1..5),(5..10])`, true, ""},
		{`before([1..5],[5..10])`, false, ""},
		{`before([1..5),(5..10])`, true, ""},

		{`after([5..10], [1..5))`, true, ""},
		{`after((5..10], [1..5))`, true, ""},
		{`after([5..10], [1..5])`, false, ""},
		{`after((5..10], [1..5))`, true, ""},

		{`meets([1..5], [5..10])`, true, ""},
		{`meets([1..3], [4..6])`, false, ""},
		{`meets([1..3], [3..5])`, true, ""},
		{`meets([1..5], (5..8])`, false, ""},

		{`met by([5..10], [1..5])`, true, ""},
		{`met by([3..4], [1..2])`, false, ""},
		{`met by([3..5], [1..3])`, true, ""},
		{`met by((5..8], [1..5))`, false, ""},
		{`met by([5..10], [1..5))`, false, ""},

		{`overlaps([5..10], [1..6])`, true, ""},
		{`overlaps((3..7], [1..4])`, true, ""},
		{`overlaps([1..3], (3..6])`, false, ""},
		{`overlaps((5..8], [1..5))`, false, ""},
		{`overlaps([4..10], [1..5))`, true, ""},

		{`overlaps before([1..5], [4..10])`, true, ""},
		{`overlaps before([3..4], [1..2])`, false, ""},
		{`overlaps before([1..3], (3..5])`, false, ""},
		{`overlaps before([1..5), (3..8])`, true, ""},
		{`overlaps before([1..5), [5..10])`, false, ""},

		{`overlaps after([4..10], [1..5])`, true, ""},
		{`overlaps after([3..4], [1..2])`, false, ""},
		{`overlaps after([3..5], [1..3))`, false, ""},
		{`overlaps after((5..8], [1..5))`, false, ""},
		{`overlaps after([4..10], [1..5))`, true, ""},

		{`finishes(5, [1..5])`, true, ""},
		{`finishes(10, [1..7])`, false, ""},
		{`finishes([3..5], [1..5])`, true, ""},
		{`finishes((1..5], [1..5))`, false, ""},
		{`finishes([5..10], [1..10))`, false, ""},

		{`finished by([5..10], 10)`, true, ""},
		{`finished by([3..4], 2)`, false, ""},

		{`finished by([3..5], [1..5])`, true, ""},
		{`finished by((5..8], [1..5))`, false, ""},
		{`finished by([5..10], (1..10))`, true, ""},

		{`includes([5..10], 6)`, true, ""},
		{`includes([3..4], 5)`, false, ""},
		{`includes([1..10], [4..6])`, true, ""},
		{`includes((5..8], [1..5))`, false, ""},
		{`includes([1..10], [1..5))`, true, ""},

		{`during(5, [1..10])`, true, ""},
		{`during(12, [1..10])`, false, ""},
		{`during(1, (1..10])`, false, ""},
		{`during([4..6], [1..10))`, true, ""},
		{`during((1..5], (1..10])`, true, ""},

		{`starts(1, [1..5])`, true, ""},
		{`starts(1, (1..8])`, false, ""},
		{`starts((1..5], [1..5])`, false, ""},
		{`starts([1..10], [1..10])`, true, ""},
		{`starts((1..10), (1..10))`, true, ""},

		{`started by([1..10], 1)`, true, ""},
		{`started by((1..10], 1)`, false, ""},
		{`started by([1..10], [1..5])`, true, ""},
		{`started by((1..10], [1..5))`, false, ""},
		{`started by([1..10], [1..10))`, true, ""},

		{`coincides([1..5], [1..5])`, true, ""},
		{`coincides((1..5], [1..5))`, false, ""},
		{`coincides([1..5], [2..6])`, false, ""},

		// range() builtin: parse string into range value
		{`2 in range("[1..3]")`, true, ""},
		{`range("[1..3]") instance of range<number>`, true, ""},
		{`range(string("[1..3]")) instance of range<number>`, true, ""},
		{`range("[\"a\"..\"c\"]") instance of range<string>`, true, ""},
		{`range("[@\"1970-01-01\"..@\"1970-01-02\"]") instance of range<date>`, true, ""},
		{`range("[@\"1970-01-01T00:00:00\"..@\"1970-01-02T00:00:00\"]") instance of range<date and time>`, true, ""},
		{`range("[@\"00:00:00\"..@\"00:00:00\"]") instance of range<time>`, true, ""},
		{`range("[@\"P1D\"..@\"P2D\"]") instance of range<days and time duration>`, true, ""},
		{`range("[@\"P1Y\"..@\"P2Y\"]") instance of range<years and months duration>`, true, ""},
		{`2 in range(string("[1..3]"))`, true, ""},
		{`range("[18..21]") = [18..21]`, true, ""},
		{`range("(18..21]") = (18..21]`, true, ""},
		{`range("]18..21]") = ]18..21]`, true, ""},
		{`range("[18..21)") = [18..21)`, true, ""},
		{`range("[18..21[") = [18..21[`, true, ""},
		{`range("[..2]")`, Null, ""},
		{`range("[1..]")`, Null, ""},
		{`range("[\"a\"..\"c\"]") = ["a".."c"]`, true, ""},
		{`range("[@\"1970-01-01\"..@\"1970-01-02\"]") = [@"1970-01-01"..@"1970-01-02"]`, true, ""},
		{`range("[@\"1970-01-01T00:00:00\"..@\"1970-01-02T00:00:00\"]") = [@"1970-01-01T00:00:00"..@"1970-01-02T00:00:00"]`, true, ""},
		{`range("[@\"00:00:00\"..@\"00:00:00\"]") = [@"00:00:00"..@"00:00:00"]`, true, ""},
		{`range("[@\"P1D\"..@\"P2D\"]") = [@"P1D"..@"P2D"]`, true, ""},
		{`range("[@\"P1Y\"..@\"P2Y\"]") = [@"P1Y"..@"P2Y"]`, true, ""},
		{`range(" [ 1 .. 3 ] ") = [1..3]`, true, ""},
		{`range("[date(\"1970-01-01\")..date(\"1970-01-02\")]") = [date("1970-01-01")..date("1970-01-02")]`, true, ""},
		{`range("[date(string(\"1970-01-01\"))..date(\"1970-01-02\")]")`, Null, ""},
		{`range("[date(input_001)..date(\"1970-01-02\")]")`, Null, ""},
		{`range("[date and time(\"1970-01-01T00:00:00\")..@\"1970-01-02T00:00:00\"]") = [@"1970-01-01T00:00:00"..@"1970-01-02T00:00:00"]`, true, ""},
		{`range("[date and time(string(\"1970-01-01T00:00:00\"))..@\"1970-01-02T00:00:00\"]")`, Null, ""},
		{`range("[date and time(input_001)..@\"1970-01-02T00:00:00\"]")`, Null, ""},
		{`range("[time(\"00:00:00\")..@\"00:00:00\"]") = [@"00:00:00"..@"00:00:00"]`, true, ""},
		{`range("[time(string(\"00:00:00\"))..@\"00:00:00\"]")`, Null, ""},
		{`range("[time(input_001)..@\"00:00:00\"]")`, Null, ""},
		{`range("[duration(\"P1D\")..@\"P2D\"]") = [@"P1D"..@"P2D"]`, true, ""},
		{`range("[duration(string(\"P1D\"))..@\"P2D\"]")`, Null, ""},
		{`range("[duration(input_001)..@\"P2D\"]")`, Null, ""},
		{`range(from: "[1..3]") = [1..3]`, true, ""},
		{`range(fron: "[1..3]")`, Null, ""},
		{`range("[1..3]", "foo")`, Null, ""},
		{`range()`, Null, ""},
		{`range([1..3])`, Null, ""},
		{`range("")`, Null, ""},
		{`range(" ")`, Null, ""},
		{`range(string(""))`, Null, ""},
		{`range(">=10")`, Null, ""},
		{`range("[1..\"b\"]")`, Null, ""},
		{`range("[@\"1970-01-01\"..@\"1970-01-02T00:00:00\"]")`, Null, ""},
		{`range("[@\"1970-01-01T00:00:00\"..@\"1970-01-02\"]")`, Null, ""},
		{`range("[3..1]")`, Null, ""},
		{`range("[@\"1970-01-02\"..@\"1970-01-01\"]")`, Null, ""},
		{`range("[@\"1970-01-02T00:00:00\"..@\"1970-01-01T00:00:00\"]")`, Null, ""},
		{`range("[\"z\"..\"a\"]")`, Null, ""},
		{`range("[@\"P2D\"..@\"P1D\"]")`, Null, ""},
		{`range("[@\"P2Y\"..@\"P1Y\"]")`, Null, ""},
		{`range("[@\"02:00:00\"..@\"01:00:00\"]")`, Null, ""},
		{`range("[null..null]")`, Null, ""},
	}

	for _, p := range evalPairs {
		res, err := EvalString(p.input, p.context)
		if err != nil {
			fmt.Printf("bad input '%s'\n", p.input)
		}
		assert.NilError(t, err)
		assert.DeepEqual(t, p.expect, res)
	}
}

func TestEvalUnaryTests(t *testing.T) {
	input := `> 8, <= 5`
	v, err := EvalStringWithScope(input, Scope{"?": 4})
	assert.NilError(t, err)
	assert.Equal(t, v, true)
}

func TestTemporalValue(t *testing.T) {
	input := `@"2023-06-07".day`
	v, err := EvalString(input)
	assert.NilError(t, err)
	assert.DeepEqual(t, v, N(7))

	input1 := `@"2023-06-07T15:08:39".second`
	v1, err := EvalString(input1)
	assert.NilError(t, err)
	assert.DeepEqual(t, v1, N(39))

	input2 := `@"P1DT3H25M60S".minutes`
	v2, err := EvalString(input2)
	assert.NilError(t, err)
	assert.DeepEqual(t, v2, N(25))

	dt, err := ParseDatetime(`2023-06-07T15:04:05`)
	assert.NilError(t, err)
	assert.DeepEqual(t, dt.t.Hour(), 15)
	assert.DeepEqual(t, dt.t.Second(), 5)

	dur, err := ParseDuration("P12Y2M")
	assert.NilError(t, err)
	assert.Equal(t, 12, dur.Years)
	assert.Equal(t, 2, dur.Months)

	dur1, err := ParseDuration("P7M")
	assert.NilError(t, err)
	assert.Equal(t, 0, dur1.Years)
	assert.Equal(t, 7, dur1.Months)

	dur2, err := ParseDuration("PT20H")
	assert.NilError(t, err)
	assert.Equal(t, 20, dur2.Hours)
	assert.Equal(t, 0, dur2.Seconds)

	td, err := time.ParseDuration("3h37m20s")
	assert.NilError(t, err)
	dur3 := NewFEELDuration(td)
	assert.Equal(t, 3, dur3.Hours)
	assert.Equal(t, 37, dur3.Minutes)
	assert.Equal(t, 20, dur3.Seconds)
}
