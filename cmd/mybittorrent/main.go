package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
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

		client := http.Client{}
		req, err := http.NewRequest("GET", torrent.Announce, nil)
		if err != nil {
			panic(err)
		}
		q := req.URL.Query()
		decoded, err := hex.DecodeString(torrent.InfoHash)
		if err != nil {
			panic(err)
		}
		q.Add("info_hash", string(decoded))
		q.Add("peer_id", "11111111111111111111")
		q.Add("port", "6881")
		q.Add("uploaded", "0")
		q.Add("downloaded", "0")
		q.Add("left", fmt.Sprintf("%d", torrent.Info.Length))
		q.Add("compact", "1")
		req.URL.RawQuery = q.Encode()
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		data, err := ext_bencode.Decode(resp.Body)
		ipsBytes := []byte(data.(map[string]any)["peers"].(string))
		for i := 0; i < len(ipsBytes); i += 6 {
			port := int64(256)*int64(ipsBytes[i+4]) + int64(ipsBytes[i+5])
			humanIP := fmt.Sprintf("%d.%d.%d.%d:%d", ipsBytes[i], ipsBytes[i+1], ipsBytes[i+2], ipsBytes[i+3], port)
			fmt.Println(humanIP)
		}
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
