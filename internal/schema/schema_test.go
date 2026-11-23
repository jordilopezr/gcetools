package schema

import "testing"

func TestScanStatus(t *testing.T) {
	for st, want := range map[string][2]bool{
		"OK": {true, true}, "OK (sin osconfig)": {true, true}, "OK (without osconfig)": {true, true},
		"Parcial (2 errores)": {false, true}, "Partial (2 errors)": {false, true},
		"Sin permisos": {false, false}, "Permission denied": {false, false}, "": {false, false},
	} {
		if ScanOK(st) != want[0] || ScanComparable(st) != want[1] {
			t.Errorf("%q: ok=%v comparable=%v", st, ScanOK(st), ScanComparable(st))
		}
	}
}
