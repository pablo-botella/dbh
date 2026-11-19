package dbh

import "testing"

func TestNullIfZero(t *testing.T) {
	if got := NullIfZero(0); got != nil {
		t.Errorf("NullIfZero(0) = %v, want nil", got)
	}
	for _, v := range []int64{1, -1, 42} {
		got, ok := NullIfZero(v).(int64)
		if !ok || got != v {
			t.Errorf("NullIfZero(%d) = %v, want the same int64", v, NullIfZero(v))
		}
	}
}

func TestNullIfEmptyStr(t *testing.T) {
	if got := NullIfEmptyStr(""); got != nil {
		t.Errorf(`NullIfEmptyStr("") = %v, want nil`, got)
	}
	for _, s := range []string{"a", " ", "0"} {
		got, ok := NullIfEmptyStr(s).(string)
		if !ok || got != s {
			t.Errorf("NullIfEmptyStr(%q) = %v, want the same string", s, NullIfEmptyStr(s))
		}
	}
}
