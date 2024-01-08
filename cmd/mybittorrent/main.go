package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	ext_bencode "github.com/jackpal/bencode-go"
	"github.com/sirupsen/logrus"
	// Available if you need it!
)

func setupLogging() {
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		lvl = "error"
	}
	ll, err := logrus.ParseLevel(lvl)
	if err != nil {
		ll = logrus.ErrorLevel
	}
	logrus.SetLevel(ll)
}

func main() {
	os.Setenv("LOG_LEVEL", "debug")
	setupLogging()

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
		for _, peer := range torrent.Peers {
			fmt.Println(peer)
		}
	} else if command == "handshake" {
		filePath := os.Args[2]

		torrent, err := NewTorrent(filePath)
		if err != nil {
			panic(err)
		}

		peer := os.Args[3]
		// peer := ""
		id, err := torrent.GetPeerID(peer)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Peer ID: %s\n", id)
	} else if command == "download_piece" {
		downloadPath := os.Args[3]
		filePath := os.Args[4]
		pieceId, err := strconv.Atoi(os.Args[5])
		if err != nil {
			panic(err)
		}

		torrent, err := NewTorrent(filePath)
		if err != nil {
			panic(err)
		}

		if err = torrent.DownloadPiece(pieceId, downloadPath); err != nil {
			panic(err)
		} else {
			fmt.Printf("Piece %d downloaded to %s.", pieceId, downloadPath)
		}
	} else if command == "download" {
		downloadPath := os.Args[3]
		filePath := os.Args[4]
		torrent, err := NewTorrent(filePath)
		if err != nil {
			panic(err)
		}
		if err = torrent.DownloadFile(downloadPath); err != nil {
			panic(err)
		} else {
			fmt.Printf("Downloaded %s to %s.", filePath, downloadPath)
		}
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
