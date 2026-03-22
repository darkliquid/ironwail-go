package main

import (
	"reflect"
	"testing"
)

func TestBeadsCreateArgsUseCurrentCLIFlags(t *testing.T) {
	task := taskRecord{
		ID:          "ralph-example",
		Title:       "Ralph: error sample",
		Labels:      []string{"ralph", "telemetry", "error"},
		Fingerprint: "sample-fingerprint",
		Description: "sample description",
	}

	got := beadsCreateArgs(task)
	want := []string{
		"create",
		"Ralph: error sample",
		"--id", "ralph-example",
		"--description", "sample description",
		"--priority", "2",
		"--type", "task",
		"--labels", "ralph,telemetry,error",
		"--external-ref", "ralph:sample-fingerprint",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("beadsCreateArgs() = %#v, want %#v", got, want)
	}
}

func TestBeadsUpdateArgsUseCurrentCLIFlags(t *testing.T) {
	task := taskRecord{
		ID:          "ralph-example",
		Title:       "Ralph: warning sample",
		Labels:      []string{"ralph", "telemetry", "warning"},
		Fingerprint: "warning-fingerprint",
		Description: "warning description",
	}

	got := beadsUpdateArgs(task)
	want := []string{
		"update",
		"ralph-example",
		"--title", "Ralph: warning sample",
		"--description", "warning description",
		"--priority", "2",
		"--type", "task",
		"--set-labels", "ralph,telemetry,warning",
		"--external-ref", "ralph:warning-fingerprint",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("beadsUpdateArgs() = %#v, want %#v", got, want)
	}
}
