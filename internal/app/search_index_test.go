package app

import (
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
