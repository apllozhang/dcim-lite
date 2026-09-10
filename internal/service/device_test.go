package service

import "testing"

func TestRangesOverlap(t *testing.T) {
	cases := []struct {
		name                 string
		aStart, aEnd, bStart, bEnd int
		want                 bool
	}{
		{"完全相同", 1, 2, 1, 2, true},
		{"部分重叠", 1, 3, 2, 4, true},
		{"相邻不重叠-低", 1, 2, 3, 4, false},
		{"相邻不重叠-高", 3, 4, 1, 2, false},
		{"包含", 1, 10, 3, 5, true},
		{"被包含", 3, 5, 1, 10, true},
		{"首尾相接", 5, 5, 5, 5, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rangesOverlap(tc.aStart, tc.aEnd, tc.bStart, tc.bEnd); got != tc.want {
				t.Fatalf("rangesOverlap(%d,%d,%d,%d) = %v, want %v",
					tc.aStart, tc.aEnd, tc.bStart, tc.bEnd, got, tc.want)
			}
		})
	}
}
