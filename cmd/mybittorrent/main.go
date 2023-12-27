package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	ext_bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

type AnyMap map[string]any

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

		torrent, err := ext_bencode.Decode(f)
		if err != nil {
			panic(err)

		}
		infoDict := torrent.(map[string]any)["info"].(map[string]any)
		var encodedDict bytes.Buffer
		if err := ext_bencode.Marshal(&encodedDict, infoDict); err != nil {
			panic(err)
		}
		sha1Builder := sha1.New()
		sha1Builder.Write(encodedDict.Bytes())
		hash := hex.EncodeToString(sha1Builder.Sum(nil))

		piecesString := []byte(infoDict["pieces"].(string))
		fmt.Println(piecesString)
		piecesHashes := make([]string, 0)
		for i := 0; i < len(piecesString); i += 20 {
			piecesHashes = append(piecesHashes, hex.EncodeToString(piecesString[i:i+20]))
		}

		fmt.Printf("Tracker URL: %s\n", torrent.(map[string]any)["announce"])
		fmt.Printf("Length: %d\n", infoDict["length"])
		fmt.Printf("Info Hash: %s\n", hash)
		fmt.Printf("Piece Length: %d\n", infoDict["piece length"])
		fmt.Printf("Piece Hashes:\n")
		for _, piece := range piecesHashes {
			fmt.Println(piece)
		}

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
