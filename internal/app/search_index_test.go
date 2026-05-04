package app

import (
	"context"
	"testing"
	"time"

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

func TestSearchIDSourceForConfigBuildsOpenSearchSource(t *testing.T) {
	t.Parallel()

	source, err := searchIDSourceForConfig(config.Config{
		SearchIndexBackend:            "opensearch",
		SearchIndexOpenSearchEndpoint: "http://localhost:9200",
		SearchIndexOpenSearchIndex:    "gogomail-messages",
	})
	if err != nil {
		t.Fatalf("searchIDSourceForConfig returned error: %v", err)
	}
	if source == nil {
		t.Fatal("source is nil")
	}
}

func TestSearchIDSourceForConfigSkipsNonOpenSearchBackends(t *testing.T) {
	t.Parallel()

	source, err := searchIDSourceForConfig(config.Config{SearchIndexBackend: "postgres"})
	if err != nil {
		t.Fatalf("searchIDSourceForConfig returned error: %v", err)
	}
	if source != nil {
		t.Fatalf("source = %#v, want nil", source)
	}
}

func TestOpenSearchOptionsForConfigUsesConfiguredTimeout(t *testing.T) {
	t.Parallel()

	opts := openSearchOptionsForConfig(config.Config{
		SearchIndexOpenSearchEndpoint: "http://localhost:9200",
		SearchIndexOpenSearchIndex:    "gogomail-messages",
		SearchIndexOpenSearchTimeout:  3 * time.Second,
	})
	if opts.Client == nil || opts.Client.Timeout != 3*time.Second {
		t.Fatalf("client timeout = %#v, want 3s", opts.Client)
	}
}

func TestSearchIndexWorkerLogFieldsIncludeOpenSearchDiagnostics(t *testing.T) {
	t.Parallel()

	fields := searchIndexWorkerLogFields(config.Config{
		EventStream:                    "gogomail.events",
		SearchIndexBackend:             " OpenSearch ",
		SearchIndexConsumerGroup:       "group-1",
		SearchIndexConsumerName:        "consumer-1",
		SearchIndexMaxBodyBytes:        1024,
		SearchIndexOpenSearchIndex:     "gogomail-messages",
		SearchIndexOpenSearchBootstrap: true,
		SearchIndexOpenSearchEndpoint:  "https://search.example.com",
		SearchIndexOpenSearchUsername:  "admin",
		SearchIndexOpenSearchPassword:  "secret",
	})
	got := fieldsMap(fields)
	if got["backend"] != "opensearch" {
		t.Fatalf("backend = %#v", got["backend"])
	}
	if got["opensearch_index"] != "gogomail-messages" || got["opensearch_bootstrap"] != true {
		t.Fatalf("opensearch fields = %#v", got)
	}
	if _, ok := got["SearchIndexOpenSearchPassword"]; ok {
		t.Fatalf("password leaked into log fields: %#v", got)
	}
	if _, ok := got["opensearch_endpoint"]; ok {
		t.Fatalf("endpoint leaked into log fields: %#v", got)
	}
}

func fieldsMap(fields []any) map[string]any {
	out := make(map[string]any, len(fields)/2)
	for i := 0; i+1 < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			continue
		}
		out[key] = fields[i+1]
	}
	return out
}

type fakeBootstrapIndexer struct {
	called bool
}

func (i *fakeBootstrapIndexer) EnsureIndex(context.Context) error {
	i.called = true
	return nil
}
