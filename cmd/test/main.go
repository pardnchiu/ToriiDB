package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/pardnchiu/ToriiDB/core/store"
)

func main() {
	torii, err := store.New()
	if err != nil {
		slog.Error("NewStore",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer torii.Close()

	fmt.Print("toriidb> ")

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
			fmt.Print("toriidb> ")
			continue
		}

		if map[string]bool{
			"quit": true,
			"exit": true,
		}[input] {
			break
		}

		fmt.Println(torii.Exec(input))
		fmt.Print("toriidb> ")
	}
}
