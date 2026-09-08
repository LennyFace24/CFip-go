package core

import "testing"

func TestParseColo(t *testing.T) {
	cases := map[string]string{
		"a37713401ff31ddd-LAX": "LAX",
		"8f2a1b3c4d5e6f70-HKG": "HKG",
		"  abc-NRT ":            "NRT",
		"":                      "",
		"noDash":                "",
		"endsWithDash-":         "",
	}
	for input, want := range cases {
		if got := ParseColo(input); got != want {
			t.Errorf("ParseColo(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSplitColos(t *testing.T) {
	got := SplitColos("HKG sin, NRT、SIN")
	want := []string{"HKG", "SIN", "NRT"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if len(SplitColos("   ")) != 0 {
		t.Error("空白输入应返回空列表")
	}
}

func TestColoAllowed(t *testing.T) {
	if !ColoAllowed("LAX", nil) {
		t.Error("白名单为空时不应过滤")
	}
	if !ColoAllowed("hkg", []string{"HKG", "SIN"}) {
		t.Error("大小写不一致也应匹配")
	}
	if ColoAllowed("LAX", []string{"HKG", "SIN"}) {
		t.Error("不在白名单内应被拒绝")
	}
	if ColoAllowed("", []string{"HKG"}) {
		t.Error("取不到机房代码且开了白名单时应被拒绝")
	}
}
