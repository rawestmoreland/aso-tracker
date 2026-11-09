package keywords

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	in := "# comment\n\n  Gym   Log \r\nyoga\nGYM LOG\n# another\nvpn\n"
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"gym log", "yoga", "vpn"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}
