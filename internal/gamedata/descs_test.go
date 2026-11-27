package gamedata

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"reflect"
	"testing"
)

func TestItemDescsRoundTrip(t *testing.T) {
	c := &Catalog{Items: map[int]*Item{
		1: {ID: 1, Desc: Lines{"pt": {"Faca"}, "en": {"Knife"}}}, // no es: falls back to pt
		2: {ID: 2, Desc: Lines{"en": {"Only English"}}},
		3: {ID: 3},
	}}
	want := map[string][]string{}
	for _, l := range Langs {
		want[l] = c.Items[1].Desc.Get(l)
	}
	d := TakeItemDescs(c)
	if c.Items[1].Desc != nil || c.Items[2].Desc != nil {
		t.Fatal("descriptions left in the catalog")
	}
	if got := d["es"][1]; !reflect.DeepEqual(got, []string{"Faca"}) {
		t.Errorf("es of 1 = %q, want the pt fallback", got)
	}
	if got := d["pt"][2]; !reflect.DeepEqual(got, []string{"Only English"}) {
		t.Errorf("pt of 2 = %q, want the only text there is", got)
	}
	if _, ok := d["pt"][3]; ok {
		t.Error("item 3 has no description")
	}

	for _, gz := range []bool{true, false} {
		var buf bytes.Buffer
		var w io.Writer = &buf
		zw := gzip.NewWriter(&buf)
		if gz {
			w = zw
		}
		json.NewEncoder(w).Encode(d["en"])
		if gz {
			zw.Close()
		}
		pauses := 0
		got, err := ReadItemDescs(&buf, func() { pauses++ })
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, d["en"]) || pauses != len(d["en"]) {
			t.Errorf("gz %v: read %v (%d pauses), want %v", gz, got, pauses, d["en"])
		}
		PutItemDescs(c, "en", got)
	}
	if got := c.Items[1].Desc.Get("en"); !reflect.DeepEqual(got, want["en"]) {
		t.Errorf("en of 1 after put = %q, want %q", got, want["en"])
	}
}

func TestReadItemDescsBad(t *testing.T) {
	for _, in := range []string{`[]`, `{"x":["a"]}`, `{"1":"a"}`} {
		if _, err := ReadItemDescs(bytes.NewBufferString(in), nil); err == nil {
			t.Errorf("%s: no error", in)
		}
	}
}
