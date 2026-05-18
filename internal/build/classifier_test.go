package build

import "testing"

func TestClassifyFromLog(t *testing.T) {
	tests := []struct {
		name string
		log  string
		want HumanSignal
	}{
		{"infra: no space", "Error: no space left on device", HumanSignalInfraFailure},
		{"infra: dns", "Could not resolve host: registry.npmjs.org", HumanSignalInfraFailure},
		{"infra: timeout", "i/o timeout while reading from peer", HumanSignalInfraFailure},
		{"config: bad yaml", ".travis.yml: YAML syntax error", HumanSignalConfiguration},
		{"config: unknown step", "Unknown step 'deploy_v2'", HumanSignalConfiguration},
		{"cancel: us spelling", "Build was canceled by user", HumanSignalCancelation},
		{"cancel: uk spelling", "Build was cancelled", HumanSignalCancelation},
		{"test failure: assertion", "Assertion failed: expected 200 got 500", HumanSignalTestFailure},
		{"test failure: fail prefix", "FAIL: TestAddition (0.00s)", HumanSignalTestFailure},
		{"none: green log", "All tests passed", HumanSignalNone},
		{"none: empty", "", HumanSignalNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyFromLog(tt.log); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

