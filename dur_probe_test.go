package feel

import (
	"fmt"
	"gotest.tools/assert"
	"testing"
)

func TestDeepEqualDT(t *testing.T) {
	// Check if two independently-parsed identical datetime strings compare equal
	dt1 := MustParseDatetime("2017-09-05T09:15:30.123456Z")
	dt2 := MustParseDatetime("2017-09-05T09:15:30.123456Z")
	fmt.Printf("dt1=%+v\n", dt1)
	fmt.Printf("dt2=%+v\n", dt2)
	assert.DeepEqual(t, dt1, dt2)
}
