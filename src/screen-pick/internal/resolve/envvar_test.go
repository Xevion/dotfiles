package resolve

import (
	"regexp"
	"testing"
)

func TestEnvVarRootAgreesWithRegexp(t *testing.T) {
	ref := regexp.MustCompile(`^\$(?:\{\w+\}|[A-Za-z_]\w*)`)
	check := func(s string) {
		t.Helper()
		if got, want := hasEnvVarRoot(s), ref.MatchString(s); got != want {
			t.Errorf("hasEnvVarRoot(%q) = %v, regexp %v", s, got, want)
		}
	}
	for _, s := range []string{"", "$", "$A", "$_", "$1", "$1a", "${", "${}", "${A}", "${A", "${A-}", "${1}", "$A/x", "$é", "${é}", " $A", "$-", "$$A", "${A}}"} {
		check(s)
	}
}
