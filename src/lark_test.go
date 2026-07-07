package main

import "testing"

func TestDecodeJSON(t *testing.T) {
	type doc struct {
		OK   *bool  `json:"ok"`
		Name string `json:"name"`
	}
	tru := true
	cases := []struct {
		name    string
		bufs    [][]byte
		wantOK  bool // decode succeeded
		wantNm  string
		okIsSet bool
	}{
		{"plain", [][]byte{[]byte(`{"name":"x"}`)}, true, "x", false},
		{"leading prose", [][]byte{[]byte("[WARN] proxy detected http://h\n{\"name\":\"y\"}")}, true, "y", false},
		// a brace inside prose precedes the real JSON — the old first/first-'{' impl broke here
		{"brace in prose", [][]byte{[]byte("note {use this}\n{\"ok\":true}")}, true, "", true},
		{"stdout empty, stderr has it", [][]byte{nil, []byte("warn\n{\"name\":\"z\"}")}, true, "z", false},
		{"no json", [][]byte{[]byte("totally not json")}, false, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var d doc
			got := decodeJSON(&d, c.bufs...)
			if got != c.wantOK {
				t.Fatalf("decodeJSON ok = %v, want %v", got, c.wantOK)
			}
			if c.wantNm != "" && d.Name != c.wantNm {
				t.Fatalf("name = %q, want %q", d.Name, c.wantNm)
			}
			if c.okIsSet && (d.OK == nil || *d.OK != tru) {
				t.Fatalf("OK not decoded from later object: %+v", d.OK)
			}
		})
	}
}

func TestCell(t *testing.T) {
	// must never panic, even at width 0
	if got := cell("abc", 0); got != "" {
		t.Fatalf("cell(_,0) = %q, want empty", got)
	}
	// ASCII pads to width
	if got := cell("ab", 5); got != "ab   " {
		t.Fatalf("cell pad = %q", got)
	}
	// CJK counts as double width: 邢政 = 4 cols, pad to 6 => 2 trailing spaces
	if got, want := cell("邢政", 6), "邢政  "; got != want {
		t.Fatalf("cell CJK = %q, want %q", got, want)
	}
	// truncation adds an ellipsis and stays within width
	if got := cell("abcdef", 4); dispWidth(got) != 4 {
		t.Fatalf("cell trunc width = %d (%q), want 4", dispWidth(got), got)
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"plain": "'plain'",
		"a b":   "'a b'",
		"it's":  `'it'\''s'`,
		"$x`y`": "'$x`y`'",
		"a\nb":  "'a\nb'",
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitNameFlags(t *testing.T) {
	n, rest := splitNameFlags([]string{"A", "--dir", "/x"})
	if n != "A" || len(rest) != 2 {
		t.Fatalf("name-first: got %q %v", n, rest)
	}
	n, rest = splitNameFlags([]string{"--all"})
	if n != "" || len(rest) != 1 {
		t.Fatalf("flag-first: got %q %v", n, rest)
	}
	n, rest = splitNameFlags(nil)
	if n != "" || len(rest) != 0 {
		t.Fatalf("empty: got %q %v", n, rest)
	}
}

func TestDispWidth(t *testing.T) {
	if dispWidth("abc") != 3 {
		t.Fatal("ascii width")
	}
	if dispWidth("邢政") != 4 {
		t.Fatal("cjk width")
	}
	if dispWidth("a邢") != 3 {
		t.Fatal("mixed width")
	}
}
