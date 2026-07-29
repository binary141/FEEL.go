package feel

// This file dynamically runs the DMN TCK (https://github.com/dmn-tck/tck)
// "compliance level 2" and "compliance level 3" test suites against
// FEEL.go, but only for the subset of test models whose decision logic is
// plain FEEL (a literalExpression), optionally backed by a Business
// Knowledge Model FEEL function. Test models that rely on decision tables
// (hit-policy evaluation), contexts, relations, invocations, lists, or
// conditionals as decision logic are outside FEEL.go's scope (it evaluates
// FEEL, not DMN decision tables/graphs) and are skipped.

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/cockroachdb/apd/v3"
)

const tckComplianceLevel2Dir = "tck/TestCases/compliance-level-2"
const tckComplianceLevel3Dir = "tck/TestCases/compliance-level-3"

// ---- DMN model (subset of the schema we care about) ----

type tckDMNDefinitions struct {
	Namespace string                 `xml:"namespace,attr"`
	Decisions []tckDMNDecision       `xml:"decision"`
	BKMs      []tckDMNBKM            `xml:"businessKnowledgeModel"`
	Inputs    []tckDMNInput          `xml:"inputData"`
	ItemDefs  []tckDMNItemDefinition `xml:"itemDefinition"`
	Imports   []tckDMNImport         `xml:"import"`
}

// tckDMNImport is a DMN cross-model import: FEEL text in the importing
// model refers to the imported model's decisions/inputs via
// "<Name>.<decision or input name>" dotted access.
type tckDMNImport struct {
	Name      string `xml:"name,attr"`
	Namespace string `xml:"namespace,attr"`
}

// tckDMNItemDefinition is a DMN custom type declaration (name -> either a
// base typeRef, a collection of a typeRef, or a set of named
// itemComponents forming a context/struct type). allowedValues is
// intentionally not modeled: FEEL's "instance of" type conformance does
// not take allowedValues restrictions into account.
type tckDMNItemDefinition struct {
	Name           string                `xml:"name,attr"`
	TypeRef        string                `xml:"typeRef"`
	IsCollection   bool                  `xml:"isCollection,attr"`
	ItemComponents []tckDMNItemComponent `xml:"itemComponent"`
}

type tckDMNItemComponent struct {
	Name    string `xml:"name,attr"`
	TypeRef string `xml:"typeRef"`
}

type tckDMNHref struct {
	Href string `xml:"href,attr"`
}

func (h tckDMNHref) id() string {
	// hrefs are either a bare local reference ("#_id") or a fully
	// qualified one ("http://.../namespace#_id"); the id is always the
	// part after the last '#'.
	if idx := strings.LastIndex(h.Href, "#"); idx >= 0 {
		return h.Href[idx+1:]
	}
	return h.Href
}

type tckDMNInfoReq struct {
	RequiredInput    *tckDMNHref `xml:"requiredInput"`
	RequiredDecision *tckDMNHref `xml:"requiredDecision"`
}

type tckDMNKnowledgeReq struct {
	RequiredKnowledge tckDMNHref `xml:"requiredKnowledge"`
}

type tckDMNLiteralExpression struct {
	Text string `xml:"text"`
}

type tckDMNDecision struct {
	Name     string `xml:"name,attr"`
	ID       string `xml:"id,attr"`
	Variable struct {
		Name string `xml:"name,attr"`
	} `xml:"variable"`
	InfoReqs []tckDMNInfoReq          `xml:"informationRequirement"`
	KnowReqs []tckDMNKnowledgeReq     `xml:"knowledgeRequirement"`
	LitExpr  *tckDMNLiteralExpression `xml:"literalExpression"`
	// presence of any of these marks the decision as unsupported (not plain FEEL)
	DecisionTable *struct{} `xml:"decisionTable"`
	Context       *struct{} `xml:"context"`
	Relation      *struct{} `xml:"relation"`
	Invocation    *struct{} `xml:"invocation"`
	List          *struct{} `xml:"list"`
	Conditional   *struct{} `xml:"conditional"`
}

func (d tckDMNDecision) supported() bool {
	return d.LitExpr != nil && d.DecisionTable == nil && d.Context == nil &&
		d.Relation == nil && d.Invocation == nil && d.List == nil && d.Conditional == nil
}

type tckDMNBKM struct {
	Name              string `xml:"name,attr"`
	ID                string `xml:"id,attr"`
	EncapsulatedLogic struct {
		FormalParameters []struct {
			Name string `xml:"name,attr"`
		} `xml:"formalParameter"`
		LitExpr *tckDMNLiteralExpression `xml:"literalExpression"`
	} `xml:"encapsulatedLogic"`
}

func (b tckDMNBKM) supported() bool {
	return b.EncapsulatedLogic.LitExpr != nil
}

func (b tckDMNBKM) funcText() string {
	names := make([]string, len(b.EncapsulatedLogic.FormalParameters))
	for i, p := range b.EncapsulatedLogic.FormalParameters {
		names[i] = p.Name
	}
	return fmt.Sprintf("function(%s) %s", strings.Join(names, ", "), b.EncapsulatedLogic.LitExpr.Text)
}

type tckDMNInput struct {
	Name string `xml:"name,attr"`
	ID   string `xml:"id,attr"`
}

// ---- TCK test-case XML (subset) ----

type tckTestCases struct {
	ModelName string        `xml:"modelName"`
	TestCase  []tckTestCase `xml:"testCase"`
}

type tckTestCase struct {
	ID          string          `xml:"id,attr"`
	Description string          `xml:"description"`
	InputNode   []tckInputNode  `xml:"inputNode"`
	ResultNode  []tckResultNode `xml:"resultNode"`
}

type tckValue struct {
	Type string `xml:"type,attr"`
	Nil  bool   `xml:"nil,attr"`
	Text string `xml:",chardata"`
}

// tckNode is a recursive value node: a scalar <value>, a <component>-based
// structure (context/relation row), or a <list> of nested nodes.
type tckNode struct {
	Value     *tckValue `xml:"value"`
	Component []struct {
		Name string `xml:"name,attr"`
		tckNode
	} `xml:"component"`
	List *struct {
		Items []tckNode `xml:"item"`
	} `xml:"list"`
}

type tckInputNode struct {
	Name string `xml:"name,attr"`
	tckNode
}

type tckResultNode struct {
	Name        string  `xml:"name,attr"`
	Type        string  `xml:"type,attr"`
	ErrorResult bool    `xml:"errorResult,attr"`
	Expected    tckNode `xml:"expected"`
}

// convertValue turns a TCK <value xsi:type="..."> into a Go/FEEL value.
func convertValue(v *tckValue) (any, error) {
	if v == nil {
		return nil, nil
	}
	if v.Nil {
		return Null, nil
	}
	text := strings.TrimSpace(v.Text)
	typ := v.Type
	if idx := strings.Index(typ, ":"); idx >= 0 {
		typ = typ[idx+1:]
	}
	switch typ {
	case "boolean":
		return strconv.ParseBool(text)
	case "decimal", "integer", "double", "float":
		return NewNumber(text), nil
	case "string":
		return v.Text, nil
	case "date":
		return ParseDate(text)
	case "time":
		return ParseTime(text)
	case "dateTime":
		return ParseDatetime(text)
	case "duration", "dayTimeDuration", "yearMonthDuration":
		return ParseDuration(text)
	default:
		return nil, fmt.Errorf("unsupported value type %q", v.Type)
	}
}

func (n tckNode) toValue() (any, error) {
	switch {
	case n.List != nil:
		items := make([]any, len(n.List.Items))
		for i, it := range n.List.Items {
			val, err := it.toValue()
			if err != nil {
				return nil, fmt.Errorf("list item %d: %w", i, err)
			}
			items[i] = val
		}
		return items, nil
	case len(n.Component) > 0:
		m := make(map[string]any, len(n.Component))
		for _, c := range n.Component {
			val, err := c.tckNode.toValue()
			if err != nil {
				return nil, fmt.Errorf("component %q: %w", c.Name, err)
			}
			m[c.Name] = val
		}
		return m, nil
	default:
		return convertValue(n.Value)
	}
}

// numbersApproxEqual compares two FEEL numbers with a generous relative
// tolerance. The TCK's expected decimal values are reference outputs from
// other DMN engines (often computed in float64), while FEEL.go computes
// arithmetic at decimal128 precision (34 significant digits); for
// expressions chaining division/exponentiation (e.g. loan payment
// formulas) that difference in precision means the last few significant
// digits can legitimately disagree without either result being wrong.
func numbersApproxEqual(a, b *Number) bool {
	const tolCtxPrecision = 50
	tolCtx := apd.BaseContext.WithPrecision(tolCtxPrecision)

	diff := new(apd.Decimal)
	tolCtx.Sub(diff, a.v, b.v) //nolint:errcheck
	diff.Abs(diff)

	absB := new(apd.Decimal)
	absB.Abs(b.v)

	tol := new(apd.Decimal)
	tolCtx.Mul(tol, absB, apd.New(1, -9)) //nolint:errcheck
	floor := apd.New(1, -9)
	if tol.Cmp(floor) < 0 {
		tol = floor
	}

	if diff.Cmp(tol) <= 0 {
		return true
	}

	// The expected value may itself be given at reduced precision (e.g. an
	// exp/log/sqrt reference truncated to 8 decimal places). If rounding our
	// higher-precision result to that same scale matches exactly, accept it.
	if b.v.Exponent < 0 {
		rounded := new(apd.Decimal)
		displayContext.Quantize(rounded, a.v, b.v.Exponent) //nolint:errcheck
		return rounded.Cmp(b.v) == 0
	}

	return false
}

func valuesEqual(t *testing.T, expected tckNode, actual any) bool {
	t.Helper()
	want, err := expected.toValue()
	if err != nil {
		t.Fatalf("bad expected value: %v", err)
	}
	return deepValuesEqual(want, actual)
}

func deepValuesEqual(want, actual any) bool {
	// FEEL semantics: a singleton list may be used wherever the single
	// value it contains is expected, so a decision typed as a scalar may
	// legitimately produce a one-element list.
	if _, wantIsList := want.([]any); !wantIsList {
		if a, ok := actual.([]any); ok && len(a) == 1 {
			actual = a[0]
		}
	}
	switch w := want.(type) {
	case *NullValue:
		_, ok := actual.(*NullValue)
		return ok
	case *Number:
		a, ok := actual.(*Number)
		if !ok {
			return false
		}
		return numbersApproxEqual(a, w)
	case bool:
		a, ok := actual.(bool)
		return ok && a == w
	case string:
		a, ok := actual.(string)
		return ok && a == w
	case *FEELDuration:
		a, ok := actual.(*FEELDuration)
		return ok && a.IsYearMonth() == w.IsYearMonth() && a.TotalMonths() == w.TotalMonths() && a.Duration() == w.Duration()
	case *FEELDate:
		a, ok := actual.(*FEELDate)
		return ok && a.String() == w.String()
	case *FEELTime:
		a, ok := actual.(*FEELTime)
		return ok && a.String() == w.String()
	case *FEELDatetime:
		a, ok := actual.(*FEELDatetime)
		return ok && a.String() == w.String()
	case []any:
		a, ok := actual.([]any)
		if !ok || len(a) != len(w) {
			return false
		}
		for i := range w {
			if !deepValuesEqual(w[i], a[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		a, ok := actual.(map[string]any)
		if !ok || len(a) != len(w) {
			return false
		}
		for k, wv := range w {
			av, ok := a[k]
			if !ok || !deepValuesEqual(wv, av) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// ---- test model loading ----

type tckModel struct {
	folder       string
	dmn          tckDMNDefinitions
	tests        tckTestCases
	idToName     map[string]string
	decisionsBy  map[string]tckDMNDecision
	bkmsBy       map[string]tckDMNBKM
	itemTypeText map[string]string // custom type name -> FEEL instance-of descriptor text
	unsupported  string            // reason, if non-empty this whole model is skipped
}

// resolveItemDefType expands a DMN itemDefinition name (or builtin typeRef)
// into the FEEL "instance of" type-descriptor text FEEL.go understands
// (e.g. "list<number>", "context<a: string, b: string>"). allowedValues
// restrictions are deliberately ignored, matching FEEL's instance-of
// semantics. visiting guards against cyclic itemDefinition references.
func resolveItemDefType(name string, defs map[string]tckDMNItemDefinition, visiting map[string]bool) string {
	def, ok := defs[name]
	if !ok {
		// Not a custom type: either a builtin FEEL type name (number,
		// string, boolean, Any, ...) or an unresolvable reference, either
		// way pass it through unchanged.
		return name
	}
	if visiting[name] {
		return name
	}
	visiting[name] = true
	defer delete(visiting, name)

	if def.IsCollection {
		return "list<" + resolveItemDefType(def.TypeRef, defs, visiting) + ">"
	}
	if len(def.ItemComponents) > 0 {
		parts := make([]string, len(def.ItemComponents))
		for i, c := range def.ItemComponents {
			parts[i] = c.Name + ": " + resolveItemDefType(c.TypeRef, defs, visiting)
		}
		return "context<" + strings.Join(parts, ", ") + ">"
	}
	return resolveItemDefType(def.TypeRef, defs, visiting)
}

// substituteCustomTypes rewrites references to DMN custom (itemDefinition)
// type names within a FEEL expression into FEEL.go's built-in instance-of
// syntax, so e.g. "256 instance of t255" becomes "256 instance of number".
func (m *tckModel) substituteCustomTypes(text string) string {
	for name, resolved := range m.itemTypeText {
		text = regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).ReplaceAllString(text, resolved)
	}
	return text
}

func loadTCKModel(dir string) (*tckModel, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	// The primary model file shares its base name with the folder; other
	// .dmn files present are models it imports, not the model under test.
	primaryDMNName := filepath.Base(dir) + ".dmn"
	var dmnFile, testFile string
	for _, e := range entries {
		name := e.Name()
		switch {
		case name == primaryDMNName:
			dmnFile = filepath.Join(dir, name)
		case strings.HasSuffix(name, ".dmn"):
			if dmnFile == "" {
				dmnFile = filepath.Join(dir, name)
			}
		case strings.Contains(name, "-test-") && strings.HasSuffix(name, ".xml"):
			// only take the first test file if several exist
			if testFile == "" {
				testFile = filepath.Join(dir, name)
			}
		}
	}
	if dmnFile == "" || testFile == "" {
		return nil, fmt.Errorf("folder %s missing .dmn or test xml", dir)
	}

	m := &tckModel{folder: dir}

	dmnBytes, err := os.ReadFile(dmnFile)
	if err != nil {
		return nil, err
	}
	if err := xml.Unmarshal(dmnBytes, &m.dmn); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", dmnFile, err)
	}

	testBytes, err := os.ReadFile(testFile)
	if err != nil {
		return nil, err
	}
	if err := xml.Unmarshal(testBytes, &m.tests); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", testFile, err)
	}

	m.idToName = make(map[string]string)
	m.decisionsBy = make(map[string]tckDMNDecision)
	m.bkmsBy = make(map[string]tckDMNBKM)

	for _, in := range m.dmn.Inputs {
		m.idToName[in.ID] = in.Name
	}
	for _, b := range m.dmn.BKMs {
		m.idToName[b.ID] = b.Name
		m.bkmsBy[b.Name] = b
		if !b.supported() {
			m.unsupported = fmt.Sprintf("business knowledge model %q has unsupported logic", b.Name)
		}
	}
	for _, d := range m.dmn.Decisions {
		m.idToName[d.ID] = d.Name
		m.decisionsBy[d.Name] = d
		if !d.supported() {
			m.unsupported = fmt.Sprintf("decision %q is not a plain FEEL literalExpression (likely a decision table)", d.Name)
		}
	}

	if len(m.dmn.ItemDefs) > 0 {
		defs := make(map[string]tckDMNItemDefinition, len(m.dmn.ItemDefs))
		for _, def := range m.dmn.ItemDefs {
			defs[def.Name] = def
		}
		m.itemTypeText = make(map[string]string, len(defs))
		for name := range defs {
			m.itemTypeText[name] = resolveItemDefType(name, defs, map[string]bool{})
		}
	}

	return m, nil
}

// baseScope builds the scope shared by every test case in the model: the
// FEEL functions bound from Business Knowledge Models.
func (m *tckModel) baseScope() (Scope, error) {
	scope := Scope{}
	for _, b := range m.dmn.BKMs {
		ast, err := ParseString(b.funcText())
		if err != nil {
			return nil, fmt.Errorf("parsing BKM %q: %w", b.Name, err)
		}
		intp := NewIntepreter()
		fn, err := ast.Eval(intp)
		if err != nil {
			return nil, fmt.Errorf("binding BKM %q: %w", b.Name, err)
		}
		scope[b.Name] = fn
	}
	return scope, nil
}

func TestTCKComplianceLevel2(t *testing.T) {
	runTCKSuite(t, tckComplianceLevel2Dir)
}

func TestTCKComplianceLevel3(t *testing.T) {
	runTCKSuite(t, tckComplianceLevel3Dir)
}

// evalDecision evaluates the named decision, first recursively evaluating
// any decisions it requires as inputs (per its informationRequirements) and
// binding their results into scope. Results are memoized directly in scope
// so sibling result nodes in the same test case reuse them.
// findDMNFileByNamespace locates the .dmn file in dir whose root
// "namespace" attribute matches the given namespace (used to resolve
// cross-model <import> references to the file that defines them).
func findDMNFileByNamespace(dir, namespace string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".dmn") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var probe struct {
			Namespace string `xml:"namespace,attr"`
		}
		if err := xml.Unmarshal(b, &probe); err != nil {
			continue
		}
		if probe.Namespace == namespace {
			return path, nil
		}
	}
	return "", fmt.Errorf("no .dmn file in %s with namespace %q", dir, namespace)
}

// buildImportedScope loads the model in dir whose namespace matches
// namespace, evaluates all of its (supported) decisions, and returns a
// Scope holding those decision results plus its own bound inputs and
// BKM functions - so a dotted reference "<import name>.<decision name>"
// in the importing model's FEEL text resolves via ordinary context
// attribute access. Imports of the imported model are themselves
// resolved recursively and bound the same way. inputValues supplies
// input values by (unqualified) name, shared across the whole import
// graph - the TCK test cases in this suite give every input a globally
// unique name regardless of which model declares it.
func buildImportedScope(dir, namespace string, inputValues Scope, visiting map[string]bool) (Scope, error) {
	if visiting[namespace] {
		return nil, fmt.Errorf("cyclic import of namespace %q", namespace)
	}
	visiting[namespace] = true
	defer delete(visiting, namespace)

	path, err := findDMNFileByNamespace(dir, namespace)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var dmn tckDMNDefinitions
	if err := xml.Unmarshal(b, &dmn); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	scope := Scope{}
	for _, in := range dmn.Inputs {
		if v, ok := inputValues[in.Name]; ok {
			scope[in.Name] = v
		}
	}
	for _, bkm := range dmn.BKMs {
		if !bkm.supported() {
			continue
		}
		ast, err := ParseString(bkm.funcText())
		if err != nil {
			return nil, fmt.Errorf("parsing BKM %q: %w", bkm.Name, err)
		}
		intp := NewIntepreter()
		fn, err := ast.Eval(intp)
		if err != nil {
			return nil, fmt.Errorf("binding BKM %q: %w", bkm.Name, err)
		}
		scope[bkm.Name] = fn
	}
	for _, imp := range dmn.Imports {
		importedScope, err := buildImportedScope(dir, imp.Namespace, inputValues, visiting)
		if err != nil {
			return nil, err
		}
		scope[imp.Name] = map[string]any(importedScope)
	}

	sub := &tckModel{
		folder:      dir,
		dmn:         dmn,
		idToName:    make(map[string]string),
		decisionsBy: make(map[string]tckDMNDecision),
	}
	for _, d := range dmn.Decisions {
		sub.idToName[d.ID] = d.Name
		sub.decisionsBy[d.Name] = d
	}
	if len(dmn.ItemDefs) > 0 {
		defs := make(map[string]tckDMNItemDefinition, len(dmn.ItemDefs))
		for _, def := range dmn.ItemDefs {
			defs[def.Name] = def
		}
		sub.itemTypeText = make(map[string]string, len(defs))
		for name := range defs {
			sub.itemTypeText[name] = resolveItemDefType(name, defs, map[string]bool{})
		}
	}
	for _, d := range dmn.Decisions {
		if !d.supported() {
			continue
		}
		if _, err := evalDecision(sub, d.Name, scope, map[string]bool{}); err != nil {
			return nil, fmt.Errorf("decision %q: %w", d.Name, err)
		}
	}
	return scope, nil
}

func evalDecision(m *tckModel, name string, scope Scope, visiting map[string]bool) (any, error) {
	if v, ok := scope[name]; ok {
		return v, nil
	}
	d, ok := m.decisionsBy[name]
	if !ok {
		return nil, fmt.Errorf("no decision named %q in model", name)
	}
	if visiting[name] {
		return nil, fmt.Errorf("cyclic decision dependency involving %q", name)
	}
	visiting[name] = true
	defer delete(visiting, name)

	for _, ir := range d.InfoReqs {
		if ir.RequiredDecision != nil {
			// A required decision that isn't in this model belongs to an
			// imported model; its value is already available via the
			// qualified "<import name>.<decision name>" dotted access
			// bound into scope, so there's nothing to pre-evaluate here.
			depName, ok := m.idToName[ir.RequiredDecision.id()]
			if !ok {
				continue
			}
			if _, err := evalDecision(m, depName, scope, visiting); err != nil {
				return nil, fmt.Errorf("required decision %q: %w", depName, err)
			}
		}
	}

	intp := NewIntepreter()
	intp.Push(scope)
	ast, err := ParseString(m.substituteCustomTypes(d.LitExpr.Text))
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	got, err := ast.Eval(intp)
	if err != nil {
		return nil, fmt.Errorf("eval error: %w", err)
	}
	scope[name] = got
	return got, nil
}

func runTCKSuite(t *testing.T, root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("tck checkout not found at %s: %v", root, err)
	}

	var folders []string
	for _, e := range entries {
		if e.IsDir() {
			folders = append(folders, e.Name())
		}
	}
	sort.Strings(folders)

	for _, folder := range folders {
		folder := folder
		t.Run(folder, func(t *testing.T) {
			dir := filepath.Join(root, folder)
			m, err := loadTCKModel(dir)
			if err != nil {
				t.Fatalf("loading model: %v", err)
			}
			if m.unsupported != "" {
				t.Skip(m.unsupported)
			}

			base, err := m.baseScope()
			if err != nil {
				t.Fatalf("building base scope: %v", err)
			}

			for _, tc := range m.tests.TestCase {
				tc := tc
				t.Run("test-"+tc.ID, func(t *testing.T) {
					scope := Scope{}
					for k, v := range base {
						scope[k] = v
					}
					for _, in := range tc.InputNode {
						val, err := in.toValue()
						if err != nil {
							t.Fatalf("input %q: %v", in.Name, err)
						}
						scope[in.Name] = val
					}

					for _, imp := range m.dmn.Imports {
						importedScope, err := buildImportedScope(m.folder, imp.Namespace, scope, map[string]bool{})
						if err != nil {
							t.Fatalf("resolving import %q: %v", imp.Name, err)
						}
						scope[imp.Name] = map[string]any(importedScope)
					}

					for _, rn := range tc.ResultNode {
						if _, ok := m.decisionsBy[rn.Name]; !ok {
							t.Fatalf("no decision named %q in model", rn.Name)
						}

						got, err := evalDecision(m, rn.Name, scope, map[string]bool{})
						if err != nil {
							if rn.ErrorResult {
								continue
							}
							t.Errorf("decision %q: %v", rn.Name, err)
							continue
						}
						if rn.ErrorResult {
							if _, isNull := got.(*NullValue); isNull {
								continue
							}
							t.Errorf("decision %q: expected an error result, got %v", rn.Name, got)
							continue
						}

						if !valuesEqual(t, rn.Expected, got) {
							want, _ := rn.Expected.toValue()
							t.Errorf("decision %q: got %v, want %v", rn.Name, got, want)
						}
					}
				})
			}
		})
	}
}
