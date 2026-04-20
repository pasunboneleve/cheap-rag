package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dmvianna/cheap-rag/internal/chatbot"
	"github.com/dmvianna/cheap-rag/internal/chunking"
	"github.com/dmvianna/cheap-rag/internal/config"
	"github.com/dmvianna/cheap-rag/internal/fsguard"
	"github.com/dmvianna/cheap-rag/internal/llm"
	"github.com/dmvianna/cheap-rag/internal/policy"
	"github.com/dmvianna/cheap-rag/internal/providers"
	"github.com/dmvianna/cheap-rag/internal/repl"
	"github.com/dmvianna/cheap-rag/internal/retrieval"
	"github.com/dmvianna/cheap-rag/internal/store"
	"github.com/dmvianna/cheap-rag/internal/types"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "index":
		return runIndex(ctx, args[1:])
	case "ask":
		return runAsk(ctx, args[1:])
	case "shell":
		return runShell(ctx, args[1:])
	case "inspect":
		return runInspect(ctx, args[1:])
	default:
		return usageError()
	}
}

func runIndex(ctx context.Context, args []string) error {
	cfg, _, err := parseConfigFlags("index", args)
	if err != nil {
		return err
	}
	guard, st, embedProvider, _, err := bootstrap(cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	indexer := chunking.NewIndexer(guard, embedProvider, cfg.EmbeddingModel, cfg.CitationPattern, st)
	count, err := indexer.Run(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("indexed %d chunks\n", count)
	return nil
}

func runAsk(ctx context.Context, args []string) error {
	cfg, remaining, err := parseConfigFlags("ask", args)
	if err != nil {
		return err
	}
	if len(remaining) == 0 {
		return errors.New("ask requires a question")
	}
	question := strings.Join(remaining, " ")
	service, st, err := buildService(cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	outcome, err := service.Ask(ctx, question)
	if err != nil {
		return err
	}
	printOutcome(outcome)
	return nil
}

func runShell(ctx context.Context, args []string) error {
	cfg, _, err := parseConfigFlags("shell", args)
	if err != nil {
		return err
	}
	service, st, err := buildService(cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	return repl.New(service, os.Stdin, os.Stdout).Run(ctx)
}

func runInspect(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "query" {
		return errors.New("inspect usage: inspect query <text>")
	}
	cfg, remaining, err := parseConfigFlags("inspect", args[1:])
	if err != nil {
		return err
	}
	if len(remaining) == 0 {
		return errors.New("inspect query requires text")
	}
	question := strings.Join(remaining, " ")
	_, st, embedProvider, _, err := bootstrap(cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	r := retrieval.New(embedProvider, st, cfg.EmbeddingModel)
	results, err := r.Retrieve(ctx, question, cfg.Retrieval.TopK)
	if err != nil {
		return err
	}
	fmt.Printf("threshold: %.3f\n", cfg.Retrieval.MinQuerySimilarity)
	for _, res := range results {
		cite := res.Chunk.Citation
		if strings.TrimSpace(cite) == "" {
			cite = res.Chunk.ID
		}
		fmt.Printf("%s score=%.4f path=%s cite_as=%s\n", res.Chunk.ID, res.Similarity, res.Chunk.Path, cite)
	}
	return nil
}

func printOutcome(outcome types.AskOutcome) {
	if outcome.Refused {
		fmt.Printf("refusal: %s\n", outcome.RefusalReason)
		if len(outcome.Retrieved) > 0 {
			fmt.Println("retrieval:")
			for _, r := range outcome.Retrieved {
				cite := r.Chunk.Citation
				if strings.TrimSpace(cite) == "" {
					cite = r.Chunk.ID
				}
				fmt.Printf("  %s score=%.4f path=%s cite_as=%s\n", r.Chunk.ID, r.Similarity, r.Chunk.Path, cite)
			}
		}
		if !outcome.Validation.Valid && (len(outcome.Validation.UnsupportedClaims) > 0 || len(outcome.Validation.UnsupportedEntities) > 0) {
			fmt.Printf("validation coverage=%.2f unsupported_claims=%d unsupported_entities=%d\n",
				outcome.Validation.Coverage,
				len(outcome.Validation.UnsupportedClaims),
				len(outcome.Validation.UnsupportedEntities),
			)
		}
		return
	}
	fmt.Println(outcome.Answer)
	fmt.Printf("citations: %s\n", strings.Join(outcome.Citations, ", "))
}

func buildService(cfg config.Config) (*chatbot.Service, *store.SQLiteStore, error) {
	_, st, embedProvider, genProvider, err := bootstrap(cfg)
	if err != nil {
		return nil, nil, err
	}
	retriever := retrieval.New(embedProvider, st, cfg.EmbeddingModel)
	validator := policy.NewValidator(cfg.Validation.MinEvidenceCoverage)
	service := chatbot.New(cfg, retriever, genProvider, validator)
	return service, st, nil
}

func bootstrap(cfg config.Config) (*fsguard.Guard, *store.SQLiteStore, llm.EmbeddingsProvider, llm.GenerationProvider, error) {
	guard, err := fsguard.New(cfg.ContentRoot, cfg.RuntimeRoot)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if err := guard.EnsureRuntimeDir(); err != nil {
		return nil, nil, nil, nil, err
	}
	storePath, err := guard.ResolveRuntimeFile("index.sqlite")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	st, err := store.Open(storePath)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	embedProvider, err := providers.NewEmbeddings(cfg.EmbeddingProvider)
	if err != nil {
		_ = st.Close()
		return nil, nil, nil, nil, err
	}
	genProvider, err := providers.NewGenerator(cfg.GenerationProvider)
	if err != nil {
		_ = st.Close()
		return nil, nil, nil, nil, err
	}
	return guard, st, embedProvider, genProvider, nil
}

func parseConfigFlags(cmd string, args []string) (config.Config, []string, error) {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "config file path")
	contentRoot := fs.String("content", "", "content root")
	runtimeRoot := fs.String("runtime", "", "runtime root")
	provider := fs.String("provider", "", "legacy provider name for both generation and embeddings")
	generationProvider := fs.String("generation-provider", "", "generation provider name")
	embeddingProvider := fs.String("embedding-provider", "", "embeddings provider name")
	model := fs.String("model", "", "generation model")
	embedModel := fs.String("embedding-model", "", "embedding model")
	citationPattern := fs.String("citation-pattern", "", "citation pattern template")
	if err := fs.Parse(args); err != nil {
		return config.Config{}, nil, err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return config.Config{}, nil, err
	}
	cfg.ApplyOverrides(*contentRoot, *runtimeRoot, *provider, *generationProvider, *embeddingProvider, *model, *embedModel, *citationPattern)
	if err := cfg.Validate(); err != nil {
		return config.Config{}, nil, err
	}
	cfg.ContentRoot = absOrOriginal(cfg.ContentRoot)
	cfg.RuntimeRoot = absOrOriginal(cfg.RuntimeRoot)
	return cfg, fs.Args(), nil
}

func absOrOriginal(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

func usageError() error {
	return errors.New("usage:\n  chatbot index --content ./content --runtime ./.chatbot --citation-pattern \"{chunk_id}\"\n  chatbot shell --content ./content --runtime ./.chatbot --generation-provider xai --embedding-provider gemini --model grok-4-1-fast-reasoning --embedding-model gemini-embedding-001 --citation-pattern \"{slug}\"\n  chatbot ask --config ./chatbot.yaml \"what is cheap to change?\"\n  chatbot inspect query --config ./chatbot.yaml \"ci cd\"")
}
