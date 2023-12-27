package main

import (
	"bufio"
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
		infoDict := torrent.(map[string]any)["info"]

		fmt.Printf("Tracker URL: %s\n", torrent.(map[string]any)["announce"])
		fmt.Printf("Length: %d\n", infoDict.(map[string]any)["length"])

		var encodedDict bytes.Buffer
		if err := ext_bencode.Marshal(bufio.NewWriter(&encodedDict), infoDict); err != nil {
			panic(err)
		}
		sha1Builder := sha1.New()
		sha1Builder.Write(encodedDict.Bytes())
		hash := hex.EncodeToString(sha1Builder.Sum(nil))
		fmt.Printf("Info Hash: %s\n", hash)

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
