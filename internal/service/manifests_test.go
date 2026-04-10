package service

import "testing"

func TestBucketRankKnownBeforeUnknown(t *testing.T) {
	mainRank := bucketRank("main")
	extrasRank := bucketRank("extras")
	unknownRank := bucketRank("zzz-custom")

	if mainRank >= unknownRank {
		t.Fatalf("expected known bucket rank to be better than unknown: main=%d unknown=%d", mainRank, unknownRank)
	}
	if extrasRank >= unknownRank {
		t.Fatalf("expected known bucket rank to be better than unknown: extras=%d unknown=%d", extrasRank, unknownRank)
	}
}
