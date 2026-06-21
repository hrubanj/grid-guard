package main

import "testing"

func TestDataField(t *testing.T) {
	got := dataField(48, 10)
	want := "[{'reg': '48', 'val': '10'}]"
	if got != want {
		t.Errorf("dataField = %q, want %q", got, want)
	}
}

func TestExportEncodeDecode(t *testing.T) {
	if v := encodeExportControl(100); v != 10 {
		t.Errorf("encodeExportControl(100) = %d, want 10", v)
	}
	if v := encodeExportControl(13000); v != 1300 {
		t.Errorf("encodeExportControl(13000) = %d, want 1300", v)
	}
	if v := decodeExportControl(10); v != 100 {
		t.Errorf("decodeExportControl(10) = %d, want 100", v)
	}
}

func TestFormValuesFormatsNumbers(t *testing.T) {
	m := map[string]any{"optType": "setReg", "num": float64(86), "deviceType": float64(99)}
	v := toForm(m)
	if v.Get("num") != "86" {
		t.Errorf("num = %q, want 86", v.Get("num"))
	}
	if v.Get("deviceType") != "99" {
		t.Errorf("deviceType = %q, want 99", v.Get("deviceType"))
	}
	if v.Get("optType") != "setReg" {
		t.Errorf("optType = %q, want setReg", v.Get("optType"))
	}
}
