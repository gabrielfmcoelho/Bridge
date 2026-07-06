package vault

import (
	"context"
	"strings"
	"sync"
	"time"

	outlineclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/outline"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Wiki bundle items (Version 79). An Outline document or collection is bundled
// by its string UUID (ref_key) and resolved LIVE at redeem via the shared
// Outline service client — the same freshness contract as secrets/api_docs. A
// document ships its full markdown; a collection ships its (bounded) document
// tree with each node's markdown. All content is delivered by the backend using
// the org token because the redeem page is anonymous — there is no per-recipient
// lazy fetch.

const (
	// maxWikiCollectionDocs caps how many documents one shared collection
	// resolves at redeem — a guard against unbounded fan-out on huge
	// collections. Past it the item is marked Truncated.
	maxWikiCollectionDocs = 50
	// maxWikiDepth caps recursion into the collection document tree.
	maxWikiDepth = 5
	// wikiResolveTimeout bounds the whole live resolution of one wiki item so a
	// slow/unreachable Outline can't hang a public redeem.
	wikiResolveTimeout = 20 * time.Second
	// wikiAuthTimeout bounds a single create-time authorization lookup.
	wikiAuthTimeout = 10 * time.Second
	// wikiFanoutConcurrency bounds parallel DocumentInfo calls per collection.
	wikiFanoutConcurrency = 5
)

// wikiClient loads Outline settings and returns a ready service client, or
// ErrBundleWikiUnavailable when the integration is disabled/unconfigured.
func (r *SecretRepo) wikiClient(ctx context.Context) (*outlineclient.Client, outlineclient.Settings, error) {
	settings, err := outlineclient.LoadSettings(r.db, r.enc)
	if err != nil {
		return nil, settings, err
	}
	if !settings.Enabled {
		return nil, settings, ErrBundleWikiUnavailable
	}
	client := outlineclient.NewServiceClient(settings)
	if client == nil {
		return nil, settings, ErrBundleWikiUnavailable
	}
	return client, settings, nil
}

// allowedWikiCollections is the set of Outline collection IDs an actor may share
// from: the admin-configured common collections plus every project-linked
// collection. This mirrors (and slightly widens) the ACL in
// outline_handlers.go handleGetWikiDocument, which gates on common collections.
func (r *SecretRepo) allowedWikiCollections(ctx context.Context, s outlineclient.Settings) map[string]bool {
	allowed := make(map[string]bool)
	for _, cid := range s.CommonCollectionIDs {
		if cid != "" {
			allowed[cid] = true
		}
	}
	if projects, err := store.NewProjectRepo(r.db).List(ctx); err == nil {
		for _, p := range projects {
			if p.OutlineCollectionID != "" {
				allowed[p.OutlineCollectionID] = true
			}
		}
	}
	return allowed
}

// authorizeWikiRef validates that the given wiki doc/collection may be bundled
// and returns the owner-facing label (doc title / collection name). Enforces the
// allowedWikiCollections ACL at creation time. actor is currently unused (the
// ACL is org-scoped, not per-user) but kept for a future per-user restriction.
func (r *SecretRepo) authorizeWikiRef(ctx context.Context, actor ActorContext, kind, refKey string) (string, error) {
	_ = actor
	refKey = strings.TrimSpace(refKey)
	if refKey == "" {
		return "", ErrBundleItemNotFound
	}
	client, settings, err := r.wikiClient(ctx)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, wikiAuthTimeout)
	defer cancel()
	allowed := r.allowedWikiCollections(ctx, settings)

	switch kind {
	case BundleItemWikiDoc:
		doc, err := client.DocumentInfo(ctx, refKey)
		if err != nil {
			return "", ErrBundleItemNotFound
		}
		if !allowed[doc.CollectionID] {
			return "", ErrBundleItemForbidden
		}
		return doc.Title, nil
	case BundleItemWikiCollection:
		if !allowed[refKey] {
			return "", ErrBundleItemForbidden
		}
		coll, err := client.CollectionInfo(ctx, refKey)
		if err != nil {
			return "", ErrBundleItemNotFound
		}
		return coll.Name, nil
	default:
		return "", ErrBundleInvalidItem
	}
}

// resolveWikiItem builds the guest-facing wiki payload for one item, live from
// Outline. A doc yields a single-root item; a collection yields its bounded
// document tree. Bounded by wikiResolveTimeout, maxWikiCollectionDocs and
// maxWikiDepth. Returns an error only when the item is wholly unresolvable (the
// redeem loop then skips it, like a deleted secret/api_doc).
func (r *SecretRepo) resolveWikiItem(ctx context.Context, kind, refKey string, client *outlineclient.Client, settings outlineclient.Settings) (*BundleWikiItem, error) {
	refKey = strings.TrimSpace(refKey)
	if refKey == "" {
		return nil, ErrBundleItemNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, wikiResolveTimeout)
	defer cancel()

	switch kind {
	case BundleItemWikiDoc:
		doc, err := client.DocumentInfo(ctx, refKey)
		if err != nil {
			return nil, err
		}
		return &BundleWikiItem{
			Kind:      "doc",
			Title:     doc.Title,
			BrowseURL: doc.BrowseURL(settings.BaseURL),
			Documents: []BundleWikiDoc{{
				ID:        doc.ID,
				Title:     doc.Title,
				Emoji:     doc.Emoji,
				Markdown:  doc.Text,
				BrowseURL: doc.BrowseURL(settings.BaseURL),
				UpdatedAt: doc.UpdatedAt,
				UpdatedBy: doc.UpdatedBy.DisplayName(),
			}},
		}, nil

	case BundleItemWikiCollection:
		coll, err := client.CollectionInfo(ctx, refKey)
		if err != nil {
			return nil, err
		}
		item := &BundleWikiItem{
			Kind:           "collection",
			Title:          coll.Name,
			CollectionName: coll.Name,
			Description:    coll.Description,
			BrowseURL:      coll.BrowseURL(settings.BaseURL),
			Documents:      []BundleWikiDoc{},
		}
		nodes, err := client.CollectionDocuments(ctx, refKey)
		if err != nil {
			// The collection resolved but its tree didn't — still a valid item
			// (empty), not a hard failure.
			return item, nil
		}
		count := 0
		tree := r.buildWikiSkeleton(nodes, 0, &count, &item.Truncated)
		r.fillWikiBodies(ctx, client, settings, tree)
		item.Documents = tree
		return item, nil

	default:
		return nil, ErrBundleInvalidItem
	}
}

// buildWikiSkeleton prunes the Outline document tree to the depth/count caps and
// returns title/emoji-only nodes (bodies filled later). count is shared across
// the whole traversal; truncated is set when a cap trims content.
func (r *SecretRepo) buildWikiSkeleton(nodes []outlineclient.DocumentNode, depth int, count *int, truncated *bool) []BundleWikiDoc {
	if depth >= maxWikiDepth {
		if len(nodes) > 0 {
			*truncated = true
		}
		return nil
	}
	out := make([]BundleWikiDoc, 0, len(nodes))
	for _, n := range nodes {
		if *count >= maxWikiCollectionDocs {
			*truncated = true
			break
		}
		*count++
		out = append(out, BundleWikiDoc{
			ID:       n.ID,
			Title:    n.Title,
			Emoji:    n.Emoji,
			Children: r.buildWikiSkeleton(n.Children, depth+1, count, truncated),
		})
	}
	return out
}

// fillWikiBodies fetches each skeleton node's markdown via DocumentInfo with
// bounded concurrency, then fills the tree in place. Nodes whose fetch fails
// keep their title/emoji but no body (graceful partial content).
func (r *SecretRepo) fillWikiBodies(ctx context.Context, client *outlineclient.Client, settings outlineclient.Settings, tree []BundleWikiDoc) {
	var ids []string
	var collect func(docs []BundleWikiDoc)
	collect = func(docs []BundleWikiDoc) {
		for i := range docs {
			ids = append(ids, docs[i].ID)
			collect(docs[i].Children)
		}
	}
	collect(tree)
	if len(ids) == 0 {
		return
	}

	bodies := make(map[string]outlineclient.Document, len(ids))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, wikiFanoutConcurrency)
	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()
			doc, err := client.DocumentInfo(ctx, id)
			if err != nil || doc == nil {
				return
			}
			mu.Lock()
			bodies[id] = *doc
			mu.Unlock()
		}(id)
	}
	wg.Wait()

	var fill func(docs []BundleWikiDoc)
	fill = func(docs []BundleWikiDoc) {
		for i := range docs {
			if d, ok := bodies[docs[i].ID]; ok {
				docs[i].Markdown = d.Text
				docs[i].BrowseURL = d.BrowseURL(settings.BaseURL)
				docs[i].UpdatedAt = d.UpdatedAt
				docs[i].UpdatedBy = d.UpdatedBy.DisplayName()
				if docs[i].Title == "" {
					docs[i].Title = d.Title
				}
			}
			fill(docs[i].Children)
		}
	}
	fill(tree)
}
