package main

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveVersion(t *testing.T) {
	t.Parallel()

	const testVersion = "1.2.3"

	buildInfo := func(v string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: v}}, true
		}
	}
	noBuildInfo := func() (*debug.BuildInfo, bool) { return nil, false }

	tests := []struct {
		name      string
		ldflags   string
		buildInfo func() (*debug.BuildInfo, bool)
		want      string
	}{
		{name: "ldflags wins", ldflags: testVersion, buildInfo: buildInfo("v9.9.9"), want: testVersion},
		{name: "go install module version", ldflags: devVersion, buildInfo: buildInfo("v" + testVersion), want: testVersion},
		{name: "local build", ldflags: devVersion, buildInfo: buildInfo("(devel)"), want: devVersion},
		{name: "no build info", ldflags: devVersion, buildInfo: noBuildInfo, want: devVersion},
		{name: "empty ldflags", ldflags: "", buildInfo: buildInfo(""), want: devVersion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, resolveVersion(tt.ldflags, tt.buildInfo))
		})
	}
}
