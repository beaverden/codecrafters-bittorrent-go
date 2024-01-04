package main

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
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
	Peers    []string
	PeerID   string
}

type HandShake struct {
	Length   byte
	Protocol [19]byte
	Reserved [8]byte
	Hash     [20]byte
	PeerID   [20]byte
}

func NewTorrent(filePath string) (*Torrent, error) {
	var torrent Torrent

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to open %s (%w)", filePath, err)
	}

	if err := ext_bencode.Unmarshal(f, &torrent); err != nil {
		return nil, fmt.Errorf("Failed to decode torrent file (%w)", err)
	}

	for i := 0; i < len(torrent.Info.Pieces); i += 20 {
		torrent.Pieces = append(torrent.Pieces, hex.EncodeToString([]byte(torrent.Info.Pieces[i:i+20])))
	}

	sha1Builder := sha1.New()
	if err = ext_bencode.Marshal(sha1Builder, torrent.Info); err != nil {
		return nil, fmt.Errorf("Failed to encode info dict (%w)", err)
	}
	torrent.InfoHash = hex.EncodeToString(sha1Builder.Sum(nil))
	torrent.PeerID = "11111111111111111111"
	return &torrent, nil
}

func (t *Torrent) GetPeers() error {
	client := http.Client{}
	req, err := http.NewRequest("GET", t.Announce, nil)
	if err != nil {
		return fmt.Errorf("Can't initiate GET request (%w)", err)
	}
	q := req.URL.Query()
	decoded, err := hex.DecodeString(t.InfoHash)
	if err != nil {
		return fmt.Errorf("Can't decode torrent info hash (%w)", err)
	}
	q.Add("info_hash", string(decoded))
	q.Add("peer_id", t.PeerID)
	q.Add("port", "6881")
	q.Add("uploaded", "0")
	q.Add("downloaded", "0")
	q.Add("left", fmt.Sprintf("%d", t.Info.Length))
	q.Add("compact", "1")
	req.URL.RawQuery = q.Encode()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Can't request peers (%w)", err)
	}
	defer resp.Body.Close()

	data, err := ext_bencode.Decode(resp.Body)
	ipsBytes := []byte(data.(map[string]any)["peers"].(string))
	for i := 0; i < len(ipsBytes); i += 6 {
		port := int64(256)*int64(ipsBytes[i+4]) + int64(ipsBytes[i+5])
		humanIP := fmt.Sprintf("%d.%d.%d.%d:%d", ipsBytes[i], ipsBytes[i+1], ipsBytes[i+2], ipsBytes[i+3], port)
		t.Peers = append(t.Peers, humanIP)
	}
	return nil
}

func (t *Torrent) Handshake(peer string) error {

	hashAsBytes, err := hex.DecodeString(t.InfoHash)
	if err != nil {
		return fmt.Errorf("Failed to decode info hash (%w)", err)
	}
	var outgoing HandShake
	outgoing.Length = 19
	copy(outgoing.Protocol[:], []byte("BitTorrent protocol"))
	copy(outgoing.Hash[:], hashAsBytes)
	copy(outgoing.PeerID[:], []byte(t.PeerID))

	conn, err := net.Dial("tcp", peer)
	if err != nil {
		return fmt.Errorf("Failed to establish peer connection (%w)", err)
	}
	defer conn.Close()

	if err := binary.Write(conn, binary.LittleEndian, outgoing); err != nil {
		return fmt.Errorf("Failed to encode struct (%w)", err)
	}

	var incoming HandShake
	binary.Read(conn, binary.LittleEndian, &incoming)
	fmt.Printf("Peer ID: %+v\n", hex.EncodeToString(incoming.PeerID[:]))

	return nil
}
