package probe

import (
	"reflect"
	"testing"
)

func TestParseKeyframes(t *testing.T) {
	out := []byte("0.000000,K__\n0.041000,___\n4.004000,K_\n2.002000,K__\nN/A,K__\n")
	got := parseKeyframes(out)
	want := []float64{0, 2.002, 4.004}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
