package main

import (
	"fmt"
	"os"
	"strings"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		var v string
		err := NewDecoder(strings.NewReader(bencodedValue)).DecodeToStr(&v)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(v)
	} else if command == "info" {
		filePath := os.Args[2]
		f, err := os.Open(filePath)
		if err != nil {
			fmt.Println(err)
			return
		}

		var torrent Torrent
		err = NewDecoder(f).DecodeToStruct(&torrent)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Printf("Tracker URL: %s\n", torrent.Announce)
		fmt.Printf("Length: %d\n", torrent.Info.Length)

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
