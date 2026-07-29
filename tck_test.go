package feel

// This file dynamically runs the DMN TCK (https://github.com/dmn-tck/tck)
// "compliance level 2" test suite against FEEL.go, but only for the subset
// of test models whose decision logic is plain FEEL (a literalExpression),
// optionally backed by a Business Knowledge Model FEEL function. Test
// models that rely on decision tables (hit-policy evaluation) are outside
// FEEL.go's scope (it evaluates FEEL, not DMN decision tables) and are
// skipped.

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/cockroachdb/apd/v3"
)

const tckComplianceLevel2Dir = "tck/TestCases/compliance-level-2"

// ---- DMN model (subset of the schema we care about) ----

type tckDMNDefinitions struct {
	Decisions []tckDMNDecision `xml:"decision"`
	BKMs      []tckDMNBKM      `xml:"businessKnowledgeModel"`
	Inputs    []tckDMNInput    `xml:"inputData"`
}

type tckDMNHref struct {
	Href string `xml:"href,attr"`
}

func (h tckDMNHref) id() string {
	return strings.TrimPrefix(h.Href, "#")
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

type tckComponent struct {
	Name      string         `xml:"name,attr"`
	Value     *tckValue      `xml:"value"`
	Component []tckComponent `xml:"component"`
}

type tckInputNode struct {
	Name      string         `xml:"name,attr"`
	Value     *tckValue      `xml:"value"`
	Component []tckComponent `xml:"component"`
}

type tckResultNode struct {
	Name     string `xml:"name,attr"`
	Type     string `xml:"type,attr"`
	Expected struct {
		Value *tckValue `xml:"value"`
	} `xml:"expected"`
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
	default:
		return nil, fmt.Errorf("unsupported value type %q", v.Type)
	}
}

func convertComponents(comps []tckComponent) (map[string]any, error) {
	m := make(map[string]any, len(comps))
	for _, c := range comps {
		if len(c.Component) > 0 {
			sub, err := convertComponents(c.Component)
			if err != nil {
				return nil, err
			}
			m[c.Name] = sub
			continue
		}
		val, err := convertValue(c.Value)
		if err != nil {
			return nil, fmt.Errorf("component %q: %w", c.Name, err)
		}
		m[c.Name] = val
	}
	return m, nil
}

func (n tckInputNode) toValue() (any, error) {
	if len(n.Component) > 0 {
		return convertComponents(n.Component)
	}
	return convertValue(n.Value)
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

	return diff.Cmp(tol) <= 0
}

func valuesEqual(t *testing.T, expected *tckValue, actual any) bool {
	t.Helper()
	want, err := convertValue(expected)
	if err != nil {
		t.Fatalf("bad expected value: %v", err)
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
	default:
		return false
	}
}

// ---- test model loading ----

type tckModel struct {
	folder      string
	dmn         tckDMNDefinitions
	tests       tckTestCases
	idToName    map[string]string
	decisionsBy map[string]tckDMNDecision
	bkmsBy      map[string]tckDMNBKM
	unsupported string // reason, if non-empty this whole model is skipped
}

func loadTCKModel(dir string) (*tckModel, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var dmnFile, testFile string
	for _, e := range entries {
		name := e.Name()
		switch {
		case strings.HasSuffix(name, ".dmn"):
			dmnFile = filepath.Join(dir, name)
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
	root := tckComplianceLevel2Dir
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

					for _, rn := range tc.ResultNode {
						d, ok := m.decisionsBy[rn.Name]
						if !ok {
							t.Fatalf("no decision named %q in model", rn.Name)
						}

						intp := NewIntepreter()
						intp.Push(scope)
						ast, err := ParseString(d.LitExpr.Text)
						if err != nil {
							t.Fatalf("decision %q: parse error: %v", rn.Name, err)
						}
						got, err := ast.Eval(intp)
						if err != nil {
							t.Fatalf("decision %q: eval error: %v", rn.Name, err)
						}

						if !valuesEqual(t, rn.Expected.Value, got) {
							t.Errorf("decision %q: got %v, want %s", rn.Name, got, rn.Expected.Value.Text)
						}
					}
				})
			}
		})
	}
}
