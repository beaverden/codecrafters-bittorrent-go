package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	ext_bencode "github.com/jackpal/bencode-go"
	// Available if you need it!
)

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]
		data, err := ext_bencode.Decode(strings.NewReader(bencodedValue))
		if err != nil {
			panic(err)
		}
		if jdata, err := json.Marshal(&data); err != nil {
			panic(err)
		} else {
			fmt.Println(string(jdata))
		}
	} else if command == "info" {
		filePath := os.Args[2]
		torrent, err := NewTorrent(filePath)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Tracker URL: %s\n", torrent.Announce)
		fmt.Printf("Length: %d\n", torrent.Info.Length)
		fmt.Printf("Info Hash: %s\n", torrent.InfoHash)
		fmt.Printf("Piece Length: %d\n", torrent.Info.PieceLength)
		fmt.Printf("Piece Hashes:\n")
		for _, piece := range torrent.Pieces {
			fmt.Println(piece)
		}
	} else if command == "peers" {
		filePath := os.Args[2]
		torrent, err := NewTorrent(filePath)
		if err != nil {
			panic(err)
		}
		if err := torrent.GetPeers(); err != nil {
			panic(err)
		}
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
