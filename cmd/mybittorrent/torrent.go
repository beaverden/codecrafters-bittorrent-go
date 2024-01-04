package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"

	ext_bencode "github.com/jackpal/bencode-go"
)

type Torrent struct {
	Announce string "announce"
	Info     struct {
		Length      int64  "length"
		Name        string "name"
		PieceLength int64  "piece length"
		Pieces      string "pieces"
	} "info"
	Pieces   []string
	InfoHash string
}

func NewTorrent(filePath string) (Torrent, error) {
	var torrent Torrent

	f, err := os.Open(filePath)
	if err != nil {
		return torrent, fmt.Errorf("Failed to open %s (%w)", filePath, err)
	}

	if err := ext_bencode.Unmarshal(f, &torrent); err != nil {
		return torrent, fmt.Errorf("Failed to decode torrent file (%w)", err)
	}

	for i := 0; i < len(torrent.Info.Pieces); i += 20 {
		torrent.Pieces = append(torrent.Pieces, hex.EncodeToString([]byte(torrent.Info.Pieces[i:i+20])))
	}

	sha1Builder := sha1.New()
	if err = ext_bencode.Marshal(sha1Builder, torrent.Info); err != nil {
		return torrent, fmt.Errorf("Failed to encode info dict")
	}
	torrent.InfoHash = hex.EncodeToString(sha1Builder.Sum(nil))
	return torrent, nil
}
