package app

import (
	"context"
	"testing"

	"github.com/gogomail/gogomail/internal/config"
)

func TestSearchIndexerForConfigBuildsOpenSearchIndexer(t *testing.T) {
	t.Parallel()

	_, err := searchIndexerForConfig(config.Config{
		SearchIndexBackend:            "opensearch",
		SearchIndexOpenSearchEndpoint: "http://localhost:9200",
		SearchIndexOpenSearchIndex:    "gogomail-messages",
	}, nil)
	if err != nil {
		t.Fatalf("searchIndexerForConfig returned error: %v", err)
	}
}

func TestSearchIndexerForConfigRejectsUnknownBackend(t *testing.T) {
	t.Parallel()

	if _, err := searchIndexerForConfig(config.Config{SearchIndexBackend: "bad"}, nil); err == nil {
		t.Fatal("searchIndexerForConfig accepted unknown backend")
	}
}

func TestMaybeBootstrapSearchIndexCallsEnsureIndex(t *testing.T) {
	t.Parallel()

	indexer := &fakeBootstrapIndexer{}
	err := maybeBootstrapSearchIndex(context.Background(), config.Config{
		SearchIndexBackend:             "opensearch",
		SearchIndexOpenSearchBootstrap: true,
		SearchIndexOpenSearchEndpoint:  "http://localhost:9200",
		SearchIndexOpenSearchIndex:     "gogomail-messages",
	}, indexer)
	if err != nil {
		t.Fatalf("maybeBootstrapSearchIndex returned error: %v", err)
	}
	if !indexer.called {
		t.Fatal("EnsureIndex was not called")
	}
}

func TestMaybeBootstrapSearchIndexSkipsWhenDisabled(t *testing.T) {
	t.Parallel()

	indexer := &fakeBootstrapIndexer{}
	if err := maybeBootstrapSearchIndex(context.Background(), config.Config{SearchIndexBackend: "opensearch"}, indexer); err != nil {
		t.Fatalf("maybeBootstrapSearchIndex returned error: %v", err)
	}
	if indexer.called {
		t.Fatal("EnsureIndex was called with bootstrap disabled")
	}
}

type fakeBootstrapIndexer struct {
	called bool
}

func (i *fakeBootstrapIndexer) EnsureIndex(context.Context) error {
	i.called = true
	return nil
}
