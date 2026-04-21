package repl

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pasunboneleve/cheap-rag/internal/chatbot"
)

type Shell struct {
	service *chatbot.Service
	in      io.Reader
	out     io.Writer
}

func New(service *chatbot.Service, in io.Reader, out io.Writer) *Shell {
	return &Shell{service: service, in: in, out: out}
}

func (s *Shell) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	fmt.Fprintln(s.out, "cheaprag shell. type ':quit' to exit.")
	for {
		fmt.Fprint(s.out, "> ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == ":quit" || line == ":exit" {
			return nil
		}
		outcome, err := s.service.Ask(ctx, line)
		if err != nil {
			fmt.Fprintf(s.out, "error: %v\n", err)
			continue
		}
		if outcome.Refused {
			fmt.Fprintf(s.out, "refusal: %s\n", outcome.RefusalReason)
			continue
		}
		fmt.Fprintln(s.out, outcome.Answer)
		fmt.Fprintf(s.out, "citations: %s\n", strings.Join(outcome.Citations, ", "))
	}
}
