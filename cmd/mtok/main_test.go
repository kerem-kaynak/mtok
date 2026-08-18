package main

import (
	"reflect"
	"testing"
)

func TestResolveDirs(t *testing.T) {
	t.Setenv("MTOK_TEST_DIR", "/from/env")

	cases := []struct {
		name       string
		flags, cfg []string
		envVar     string
		want       []string
	}{
		{"flags win", []string{"/from/flag"}, []string{"/from/cfg"}, "MTOK_TEST_DIR", []string{"/from/flag"}},
		{"config next", nil, []string{"/from/cfg"}, "MTOK_TEST_DIR", []string{"/from/cfg"}},
		{"env var next", nil, nil, "MTOK_TEST_DIR", []string{"/from/env"}},
		{"unset env falls back", nil, nil, "MTOK_TEST_DIR_UNSET", []string{"/fallback"}},
	}
	for _, c := range cases {
		if got := resolveDirs(c.flags, c.cfg, c.envVar, "/fallback"); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}

	t.Setenv("MTOK_TEST_DIR_EMPTY", "")
	if got := resolveDirs(nil, nil, "MTOK_TEST_DIR_EMPTY", "/fallback"); !reflect.DeepEqual(got, []string{"/fallback"}) {
		t.Errorf("empty env should fall back, got %v", got)
	}
}
