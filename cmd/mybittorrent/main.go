package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	ext_bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

type AnyMap map[string]any

func readTorrent(filePath string) (Torrent, error) {
	var torrent Torrent

	f, err := os.Open(filePath)
	if err != nil {
		return torrent, fmt.Errorf("Failed to open %s (%w)", filePath, err)
	}

	torrentDecoded, err := ext_bencode.Decode(f)
	if err != nil {
		return torrent, fmt.Errorf("Failed to decode torrent file (%w)", err)

	}
	torrent.Announce = torrentDecoded.(map[string]any)["announce"].(string)

	infoDict := torrentDecoded.(map[string]any)["info"].(map[string]any)
	var encodedDict bytes.Buffer
	if err := ext_bencode.Marshal(&encodedDict, infoDict); err != nil {
		panic(err)
	}
	sha1Builder := sha1.New()
	sha1Builder.Write(encodedDict.Bytes())
	torrent.InfoHash = hex.EncodeToString(sha1Builder.Sum(nil))

	torrent.Length = infoDict["length"].(int64)
	torrent.PieceLength = infoDict["piece length"].(int64)
	torrent.Pieces = make([]string, 0)

	piecesString := []byte(infoDict["pieces"].(string))
	for i := 0; i < len(piecesString); i += 20 {
		torrent.Pieces = append(torrent.Pieces, hex.EncodeToString(piecesString[i:i+20]))
	}
	return torrent, nil
}

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
		torrent, err := readTorrent(filePath)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Tracker URL: %s\n", torrent.Announce)
		fmt.Printf("Length: %d\n", torrent.Length)
		fmt.Printf("Info Hash: %s\n", torrent.InfoHash)
		fmt.Printf("Piece Length: %d\n", torrent.PieceLength)
		fmt.Printf("Piece Hashes:\n")
		for _, piece := range torrent.Pieces {
			fmt.Println(piece)
		}
	} else if command == "peers" {
		filePath := os.Args[2]
		torrent, err := readTorrent(filePath)
		if err != nil {
			panic(err)
		}

		client := http.Client{}
		req, err := http.NewRequest("GET", torrent.Announce, nil)
		if err != nil {
			panic(err)
		}
		q := req.URL.Query()
		q.Add("info_hash", torrent.InfoHash)
		q.Add("peer_id", "11111111111111111111")
		q.Add("port", "6881")
		q.Add("uploaded", "0")
		q.Add("downloaded", "0")
		q.Add("left", string(torrent.Length))
		q.Add("compact", "1")
		req.URL.RawQuery = q.Encode()
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		debugData, err := io.ReadAll(resp.Body)
		fmt.Println(debugData)

		data, err := ext_bencode.Decode(resp.Body)
		ipsBytes := data.(map[string]any)["peers"].([]byte)
		for i := 0; i < len(ipsBytes); i += 6 {
			port := 16*ipsBytes[4] + ipsBytes[5]
			humanIP := fmt.Sprintf("%d.%d.%d.%d:%d", ipsBytes[0], ipsBytes[1], ipsBytes[2], ipsBytes[3], port)
			fmt.Println(humanIP)
		}
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
