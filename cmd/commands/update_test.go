package commands

import (
	"reflect"
	"testing"
)

func TestNormalizeUpdateArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		all      bool
		wantArgs []string
		wantAll  bool
	}{
		{name: "star alias", args: []string{"*"}, wantAll: true},
		{name: "all flag preserved", args: []string{"git"}, all: true, wantArgs: []string{"git"}, wantAll: true},
		{name: "explicit args unchanged", args: []string{"git", "7zip"}, wantArgs: []string{"git", "7zip"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, gotAll := normalizeUpdateArgs(tt.args, tt.all)
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("normalizeUpdateArgs() args = %#v, want %#v", gotArgs, tt.wantArgs)
			}
			if gotAll != tt.wantAll {
				t.Fatalf("normalizeUpdateArgs() all = %v, want %v", gotAll, tt.wantAll)
			}
		})
	}
}
