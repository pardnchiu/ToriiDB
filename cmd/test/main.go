package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/agenvoy/toriidb/core/store"
)

func main() {
	torii, err := store.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}
	defer torii.Close()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		torii.Close()
		os.Exit(0)
	}()

	command := func() {
		fmt.Printf("toriidb[%d]> ", torii.Current())
	}

	command()

	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println()
			}
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			command()
			continue
		}

		if map[string]bool{
			"quit": true,
			"exit": true,
		}[input] {
			break
		}

		fmt.Println(torii.Exec(input))
		command()
	}
}
