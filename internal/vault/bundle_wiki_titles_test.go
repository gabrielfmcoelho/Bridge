package vault

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveWikiTitles_CachesForADay(t *testing.T) {
	wikiTitleCache.Lock()
	wikiTitleCache.m = map[string]cachedWikiTitle{}
	wikiTitleCache.Unlock()

	var calls atomic.Int32
	fetch := func(_ context.Context, it bundleItemRow) (string, error) {
		calls.Add(1)
		if it.refKey == "gone" {
			return "", errors.New("404")
		}
		return "title " + it.refKey, nil
	}
	items := []bundleItemRow{
		{itemType: BundleItemWikiDoc, refKey: "a"},
		{itemType: BundleItemWikiDoc, refKey: "a"}, // same page in two links: one fetch
		{itemType: BundleItemWikiCollection, refKey: "a"},
		{itemType: BundleItemWikiDoc, refKey: "gone"},
		{itemType: BundleItemWikiDoc, refKey: ""}, // no key: never fetched
	}
	now := time.Now()

	got := resolveWikiTitles(context.Background(), items, now, fetch)
	if calls.Load() != 3 {
		t.Fatalf("first load fetched %d, want 3", calls.Load())
	}
	if got["wiki_doc:a"] != "title a" || got["wiki_collection:a"] != "title a" || len(got) != 2 {
		t.Fatalf("titles = %v", got)
	}

	// Within the day only the failed one is asked again.
	got = resolveWikiTitles(context.Background(), items, now.Add(23*time.Hour), fetch)
	if calls.Load() != 4 || got["wiki_doc:a"] != "title a" {
		t.Fatalf("cached load fetched %d (want 4), titles %v", calls.Load(), got)
	}

	// After the day everything is fetched again.
	resolveWikiTitles(context.Background(), items, now.Add(25*time.Hour), fetch)
	if calls.Load() != 7 {
		t.Fatalf("expired load fetched %d total, want 7", calls.Load())
	}
}
