package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	ext_bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

type AnyMap map[string]any

func readTorrent(reader io.Reader) (Torrent, error) {
	var torrent Torrent

	torrentDecoded, err := ext_bencode.Decode(reader)
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
		f, err := os.Open(filePath)
		if err != nil {
			panic(err)
		}
		torrent, err := readTorrent(f)
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
	} else if command == "peer" {
		// requestMap := map[string]any{
		// 	"info_hash":  5,
		// 	"peer_id":    "11111111111111111111",
		// 	"port":       "6881",
		// 	"uploaded":   0,
		// 	"downloaded": 0,
		// 	"left":       0,
		// 	"compact":    1,
		// }
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
