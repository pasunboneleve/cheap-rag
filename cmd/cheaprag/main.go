package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pasunboneleve/cheap-rag/internal/chatbot"
	"github.com/pasunboneleve/cheap-rag/internal/chunking"
	"github.com/pasunboneleve/cheap-rag/internal/config"
	"github.com/pasunboneleve/cheap-rag/internal/fsguard"
	"github.com/pasunboneleve/cheap-rag/internal/httpserver"
	"github.com/pasunboneleve/cheap-rag/internal/llm"
	"github.com/pasunboneleve/cheap-rag/internal/policy"
	"github.com/pasunboneleve/cheap-rag/internal/providers"
	"github.com/pasunboneleve/cheap-rag/internal/repl"
	"github.com/pasunboneleve/cheap-rag/internal/retrieval"
	"github.com/pasunboneleve/cheap-rag/internal/store"
	"github.com/pasunboneleve/cheap-rag/internal/types"
)

const cliName = "cheaprag"

var (
	cliStdout io.Writer = os.Stdout
	cliStderr io.Writer = os.Stderr
	version   string    = "dev"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(cliStderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-v") {
		printVersion(cliStdout)
		return nil
	}
	if len(args) == 0 || isHelpToken(args[0]) {
		printTopHelp(cliStdout)
		return nil
	}
	if args[0] == "help" {
		return runHelp(args[1:])
	}

	switch args[0] {
	case "index":
		return runIndex(ctx, args[1:])
	case "ask":
		return runAsk(ctx, args[1:])
	case "version":
		return runVersion(args[1:])
	case "shell":
		return runShell(ctx, args[1:])
	case "serve":
		return runServe(ctx, args[1:])
	case "inspect":
		return runInspect(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q (run '%s --help')", args[0], cliName)
	}
}

func runHelp(args []string) error {
	if len(args) == 0 {
		printTopHelp(cliStdout)
		return nil
	}
	if len(args) == 1 {
		switch args[0] {
		case "index":
			printIndexHelp(cliStdout)
			return nil
		case "ask":
			printAskHelp(cliStdout)
			return nil
		case "version":
			printVersionHelp(cliStdout)
			return nil
		case "shell":
			printShellHelp(cliStdout)
			return nil
		case "serve":
			printServeHelp(cliStdout)
			return nil
		case "inspect":
			printInspectHelp(cliStdout)
			return nil
		}
	}
	if len(args) == 2 && args[0] == "inspect" && args[1] == "query" {
		printInspectQueryHelp(cliStdout)
		return nil
	}
	return fmt.Errorf("unknown help topic %q (run '%s --help')", strings.Join(args, " "), cliName)
}

func runVersion(args []string) error {
	if wantsHelp(args) {
		printVersionHelp(cliStdout)
		return nil
	}
	if len(args) > 0 {
		return fmt.Errorf("version takes no positional arguments (run '%s version --help')", cliName)
	}
	printVersion(cliStdout)
	return nil
}

func runIndex(ctx context.Context, args []string) error {
	if wantsHelp(args) {
		printIndexHelp(cliStdout)
		return nil
	}
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
	fmt.Fprintf(cliStdout, "indexed %d chunks\n", count)
	return nil
}

func runAsk(ctx context.Context, args []string) error {
	if wantsHelp(args) {
		printAskHelp(cliStdout)
		return nil
	}
	cfg, remaining, err := parseConfigFlags("ask", args)
	if err != nil {
		return err
	}
	if len(remaining) == 0 {
		return fmt.Errorf("ask requires a question (run '%s ask --help')", cliName)
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
	if wantsHelp(args) {
		printShellHelp(cliStdout)
		return nil
	}
	cfg, _, err := parseConfigFlags("shell", args)
	if err != nil {
		return err
	}
	service, st, err := buildService(cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	return repl.New(service, os.Stdin, cliStdout).Run(ctx)
}

func runInspect(ctx context.Context, args []string) error {
	if len(args) == 0 || isHelpToken(args[0]) {
		printInspectHelp(cliStdout)
		return nil
	}
	if args[0] != "query" {
		return fmt.Errorf("inspect usage: %s inspect query <text>", cliName)
	}
	if wantsHelp(args[1:]) {
		printInspectQueryHelp(cliStdout)
		return nil
	}
	cfg, remaining, err := parseConfigFlags("inspect query", args[1:])
	if err != nil {
		return err
	}
	if len(remaining) == 0 {
		return fmt.Errorf("inspect query requires text (run '%s inspect query --help')", cliName)
	}
	question := strings.Join(remaining, " ")
	_, st, embedProvider, _, err := bootstrap(cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	retriever := retrieval.New(embedProvider, st, cfg.EmbeddingModel)
	results, err := retriever.Retrieve(ctx, question, cfg.Retrieval.TopK)
	if err != nil {
		return err
	}
	fmt.Fprintf(cliStdout, "threshold: %.3f\n", cfg.Retrieval.MinQuerySimilarity)
	for _, res := range results {
		cite := res.Chunk.Citation
		if strings.TrimSpace(cite) == "" {
			cite = res.Chunk.ID
		}
		fmt.Fprintf(cliStdout, "%s score=%.4f path=%s cite_as=%s\n", res.Chunk.ID, res.Similarity, res.Chunk.Path, cite)
	}
	return nil
}

func runServe(ctx context.Context, args []string) error {
	if wantsHelp(args) {
		printServeHelp(cliStdout)
		return nil
	}
	cfg, _, err := parseConfigFlags("serve", args)
	if err != nil {
		return err
	}
	service, st, err := buildService(cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	logger := log.New(cliStderr, "", log.LstdFlags)
	listener, cleanup, err := httpserver.ListenUnixSocket(cfg.Runtime.SocketPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := cleanup(); err != nil {
			logger.Printf("socket cleanup error: %v", err)
		}
	}()

	httpSrv := &http.Server{
		Handler: httpserver.NewWithLimits(service, cfg.InternalToken, logger, httpserver.Limits{
			MaxInflightRequests: int(cfg.Server.MaxInflightRequests),
			MaxRequestBodyBytes: cfg.Server.MaxRequestBodyBytes,
		}).Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-sigCtx.Done()
		logger.Printf("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			logger.Printf("shutdown error: %v", err)
		}
	}()

	logger.Printf("cheap-rag server starting")
	logger.Printf("listening on unix socket %s", cfg.Runtime.SocketPath)
	err = httpSrv.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Printf("server stopped")
	return nil
}

func printOutcome(outcome types.AskOutcome) {
	if outcome.Refused {
		fmt.Fprintf(cliStdout, "refusal: %s\n", outcome.RefusalReason)
		if len(outcome.Retrieved) > 0 {
			fmt.Fprintln(cliStdout, "retrieval:")
			for _, r := range outcome.Retrieved {
				cite := r.Chunk.Citation
				if strings.TrimSpace(cite) == "" {
					cite = r.Chunk.ID
				}
				fmt.Fprintf(cliStdout, "  %s score=%.4f path=%s cite_as=%s\n", r.Chunk.ID, r.Similarity, r.Chunk.Path, cite)
			}
		}
		if !outcome.Validation.Valid && (len(outcome.Validation.UnsupportedClaims) > 0 || len(outcome.Validation.UnsupportedEntities) > 0) {
			fmt.Fprintf(cliStdout, "validation coverage=%.2f unsupported_claims=%d unsupported_entities=%d\n",
				outcome.Validation.Coverage,
				len(outcome.Validation.UnsupportedClaims),
				len(outcome.Validation.UnsupportedEntities),
			)
		}
		return
	}
	fmt.Fprintln(cliStdout, outcome.Answer)
	fmt.Fprintf(cliStdout, "citations: %s\n", strings.Join(outcome.Citations, ", "))
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
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	configPath := fs.String("config", "", "config file path")
	contentRoot := fs.String("content", "", "content root")
	runtimeRoot := fs.String("runtime", "", "runtime root")
	provider := fs.String("provider", "", "legacy provider name for both generation and embeddings")
	generationProvider := fs.String("generation-provider", "", "generation provider name")
	embeddingProvider := fs.String("embedding-provider", "", "embeddings provider name")
	model := fs.String("model", "", "generation model")
	generationTemperature := fs.Float64("generation-temperature", math.NaN(), "generation temperature")
	embedModel := fs.String("embedding-model", "", "embedding model")
	citationPattern := fs.String("citation-pattern", "", "citation pattern template")

	if err := fs.Parse(args); err != nil {
		return config.Config{}, nil, fmt.Errorf("parse flags for %s: %w (run '%s %s --help')", cmd, err, cliName, cmd)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return config.Config{}, nil, err
	}
	cfg.ApplyOverrides(*contentRoot, *runtimeRoot, *provider, *generationProvider, *embeddingProvider, *model, *embedModel, *citationPattern, generationTemperature)
	if err := cfg.Validate(); err != nil {
		return config.Config{}, nil, err
	}
	cfg.ContentRoot = absOrOriginal(cfg.ContentRoot)
	cfg.RuntimeRoot = absOrOriginal(cfg.RuntimeRoot)
	cfg.Runtime.SocketPath = absOrOriginal(cfg.Runtime.SocketPath)
	return cfg, fs.Args(), nil
}

func absOrOriginal(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

func wantsHelp(args []string) bool {
	for _, arg := range args {
		if isHelpToken(arg) {
			return true
		}
	}
	return false
}

func isHelpToken(s string) bool {
	return s == "--help" || s == "-h"
}

func printTopHelp(w io.Writer) {
	fmt.Fprintf(w, `%s
cheap-rag CLI: retrieval-gated local RAG over your content.

Usage:
  %s <command> [options]
  %s help [command]

Commands:
  index          index local content into runtime storage
  ask            ask one question and print answer or refusal
  version        print cheaprag version
  shell          start interactive question loop
  serve          run HTTP API over Unix socket
  inspect query  show retrieval matches and similarity scores

Common flags (available on commands above):
  --config <path>                config file path (optional)
  --content <path>               override content_root
  --runtime <path>               override runtime_root
  --generation-provider <name>   override generation provider
  --embedding-provider <name>    override embedding provider
  --model <name>                 override generation model
  --embedding-model <name>       override embedding model
  --generation-temperature <n>   override generation temperature
  --citation-pattern <pattern>   override citation output pattern
  --provider <name>              legacy override for both providers

Config and flags:
  - If --config is set, values are loaded from that file first.
  - Command flags override config values.
  - If required values are still missing after merge, command fails loudly.
  - Legacy provider fallback: provider still maps to both generation and embedding providers.

Examples:
  %s index --config ./cheaprag.example.yaml
  %s shell --config ./cheaprag.example.yaml
  %s ask --config ./cheaprag.example.yaml "what is cheap to change?"
  %s version
  %s inspect query --config ./cheaprag.example.yaml "ci cd"
  %s serve --config ./cheaprag.example.yaml
`, cliName, cliName, cliName, cliName, cliName, cliName, cliName, cliName, cliName)
}

func printIndexHelp(w io.Writer) {
	fmt.Fprintf(w, `%s
Index local content into runtime storage.

Usage:
  %s index [flags]

Flags:
  --config <path>              config file path
  --content <path>             override content_root
  --runtime <path>             override runtime_root
  --embedding-provider <name>  override embedding provider
  --embedding-model <name>     override embedding model
  --citation-pattern <pattern> override citation pattern
  --provider <name>            legacy override for both providers

Config and flags:
  - Reads content_root and runtime_root from config (or defaults).
  - Flags override config values.
  - Provider/model resolution uses embedding_provider + embedding_model.

Examples:
  %s index --config ./cheaprag.example.yaml
  %s index --content ./content --runtime ./.cheaprag --embedding-provider gemini --embedding-model gemini-embedding-001
`, cliName, cliName, cliName, cliName)
}

func printAskHelp(w io.Writer) {
	fmt.Fprintf(w, `%s
Ask one question from indexed local content.

Usage:
  %s ask [flags] <question>

Arguments:
  <question>  required question text

Flags:
  --config <path>                 config file path
  --content <path>                override content_root
  --runtime <path>                override runtime_root
  --generation-provider <name>    override generation provider
  --embedding-provider <name>     override embedding provider
  --model <name>                  override generation model
  --embedding-model <name>        override embedding model
  --generation-temperature <n>    override generation temperature
  --citation-pattern <pattern>    override citation pattern
  --provider <name>               legacy override for both providers

Config and flags:
  - Loads config first, then applies command flags as overrides.
  - Generation path resolves via generation_provider + model.
  - Retrieval path resolves via embedding_provider + embedding_model.
  - Missing required values after merge causes a clear error.

Examples:
  %s ask --config ./cheaprag.example.yaml "what is cheap to change?"
  %s ask --config ./cheaprag.example.yaml "how do I reduce coupling?"
`, cliName, cliName, cliName, cliName)
}

func printVersionHelp(w io.Writer) {
	fmt.Fprintf(w, `%s
Print cheaprag version.

Usage:
  %s version
  %s --version

Notes:
  - Local builds default to version "dev".
  - Release builds embed the tag version (for example "0.2.1").
`, cliName, cliName, cliName)
}

func printShellHelp(w io.Writer) {
	fmt.Fprintf(w, `%s
Start an interactive shell for repeated questions.

Usage:
  %s shell [flags]

Flags:
  --config <path>                 config file path
  --content <path>                override content_root
  --runtime <path>                override runtime_root
  --generation-provider <name>    override generation provider
  --embedding-provider <name>     override embedding provider
  --model <name>                  override generation model
  --embedding-model <name>        override embedding model
  --generation-temperature <n>    override generation temperature
  --citation-pattern <pattern>    override citation pattern
  --provider <name>               legacy override for both providers

Config and flags:
  - Same provider/model resolution as ask.
  - Flags always override config values.

Examples:
  %s shell --config ./cheaprag.example.yaml
  %s shell --config ./cheaprag.example.yaml --generation-provider anthropic --model claude-sonnet-4-5
`, cliName, cliName, cliName, cliName)
}

func printServeHelp(w io.Writer) {
	fmt.Fprintf(w, `%s
Run local HTTP API over Unix socket.

Usage:
  %s serve [flags]

Flags:
  --config <path>                 config file path
  --content <path>                override content_root
  --runtime <path>                override runtime_root
  --generation-provider <name>    override generation provider
  --embedding-provider <name>     override embedding provider
  --model <name>                  override generation model
  --embedding-model <name>        override embedding model
  --generation-temperature <n>    override generation temperature
  --citation-pattern <pattern>    override citation pattern
  --provider <name>               legacy override for both providers

Config and flags:
  - Uses runtime.socket_path, server.max_inflight_requests, server.max_request_body_bytes.
  - If internal_token is configured, Authorization: Bearer is required.

Examples:
  %s serve --config ./cheaprag.example.yaml
  %s serve --runtime ./.cheaprag
`, cliName, cliName, cliName, cliName)
}

func printInspectHelp(w io.Writer) {
	fmt.Fprintf(w, `%s
Inspect retrieval internals.

Usage:
  %s inspect query [flags] <text>

Subcommands:
  query   embed input text and print retrieved chunks with similarity scores

Run '%s inspect query --help' for query details.
`, cliName, cliName, cliName)
}

func printInspectQueryHelp(w io.Writer) {
	fmt.Fprintf(w, `%s
Inspect query retrieval and similarity scoring.

Usage:
  %s inspect query [flags] <text>

Arguments:
  <text>  required query text

Flags:
  --config <path>              config file path
  --content <path>             override content_root
  --runtime <path>             override runtime_root
  --embedding-provider <name>  override embedding provider
  --embedding-model <name>     override embedding model
  --provider <name>            legacy override for both providers

Config and flags:
  - Uses retrieval.top_k and retrieval.min_query_similarity from config.
  - Flags override config for provider/model/root selection.

Examples:
  %s inspect query --config ./cheaprag.example.yaml "ci cd"
  %s inspect query --runtime ./.cheaprag "cheap to change"
`, cliName, cliName, cliName, cliName)
}

func printVersion(w io.Writer) {
	fmt.Fprintln(w, strings.TrimSpace(version))
}
