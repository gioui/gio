// SPDX-License-Identifier: Unlicense OR MIT

package key

import (
	"testing"
)

func TestKeySet(t *testing.T) {
	const allMods = ModAlt | ModShift | ModSuper | ModCtrl | ModCommand
	tests := []struct {
		Set        Set
		Matches    []Event
		Mismatches []Event
	}{
		{"A", []Event{{Name: "A"}}, []Event{{Name: "A", Modifiers: ModShift}}},
		{"[A,B,C]", []Event{{Name: "A"}, {Name: "B"}}, []Event{}},
		{"Short-A", []Event{{Name: "A", Modifiers: ModShortcut}}, []Event{{Name: "A", Modifiers: ModShift}}},
		{"(Ctrl)-A", []Event{{Name: "A", Modifiers: ModCtrl}, {Name: "A"}}, []Event{{Name: "A", Modifiers: ModShift}}},
		{"Shift-[A,B,C]", []Event{{Name: "A", Modifiers: ModShift}}, []Event{{Name: "B", Modifiers: ModShift | ModCtrl}}},
		{Set(allMods.String() + "-A"), []Event{{Name: "A", Modifiers: allMods}}, []Event{}},
	}
	for _, tst := range tests {
		for _, e := range tst.Matches {
			if !tst.Set.Contains(e.Name, e.Modifiers) {
				t.Errorf("key set %q didn't contain %+v", tst.Set, e)
			}
		}
		for _, e := range tst.Mismatches {
			if tst.Set.Contains(e.Name, e.Modifiers) {
				t.Errorf("key set %q contains %+v", tst.Set, e)
			}
		}
	}
}
